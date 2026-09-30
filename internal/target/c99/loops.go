package c99

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) StatementFor(stmt source.StatementFor) error {
	if stmt.Label != "" {
		fmt.Fprintf(c99, "go_label_%s:; ", stmt.Label)
		defer fmt.Fprintf(c99, " go_break_%s:;", stmt.Label)
	}
	fmt.Fprintf(c99, "for (")
	init, hasInit := stmt.Init.Get()
	if hasInit {
		tabs := c99.Tabs
		c99.Tabs = -c99.Tabs
		c99.Header = true
		if err := c99.Statement(init); err != nil {
			return err
		}
		c99.Header = false
		c99.Tabs = tabs
	} else {
		fmt.Fprintf(c99, ";")
	}
	fmt.Fprintf(c99, " ")
	condition, hasCondition := stmt.Condition.Get()
	if hasCondition {
		if err := c99.Expression(condition); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c99, "true")
	}
	fmt.Fprintf(c99, "; ")
	// Each iteration has its own copy of the loop variables (since Go 1.22), which matters
	// when they are captured by closures: give each iteration a new box, with the value of
	// the previous iteration, before the post statement.
	var rebox []string
	if init, ok := stmt.Init.Get(); ok && xyz.ValueOf(init) == source.Statements.Assignment {
		for _, v := range source.Statements.Assignment.Get(init).Variables {
			if xyz.ValueOf(v) != source.Expressions.DefinedVariable {
				continue
			}
			if name := source.Expressions.DefinedVariable.Get(v); !c99.StackAllocated(name) {
				rebox = append(rebox, fmt.Sprintf("%s = %s(*%[1]s)", name.String, c99.BoxOf(name.Unique.Type())))
			}
		}
	}
	fmt.Fprint(c99, strings.Join(rebox, ", "))
	statement, hasStatement := stmt.Statement.Get()
	if hasStatement && len(rebox) > 0 {
		fmt.Fprintf(c99, ", ")
	}
	if hasStatement {
		c99.Tabs = -c99.Tabs
		c99.Header = true
		stmt, _ := statement.Get()
		if err := c99.Compile(stmt); err != nil {
			return err
		}
		c99.Header = false
		c99.Tabs = -c99.Tabs
	}
	fmt.Fprintf(c99, ") {")
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}")
	return nil
}

func (c99 Target) StatementRange(stmt source.StatementRange) error {
	if stmt.Label != "" {
		fmt.Fprintf(c99, "go_label_%s:; ", stmt.Label)
		defer fmt.Fprintf(c99, " go_break_%s:;", stmt.Label)
	}
	switch typ := stmt.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Basic:
		if typ.Info()&types.IsString != 0 {
			return c99.rangeString(stmt)
		}
		rtype := c99.TypeOf(stmt.X.TypeAndValue().Type)
		iter_name := "go_iter"
		key, hasKey := stmt.Key.Get()
		if hasKey {
			iter_name = c99.toString(key)
		}
		boxed := hasKey && !c99.StackAllocated(key)
		if boxed {
			iter_name = "go_iter_" + key.String
		}
		fmt.Fprintf(c99, "for (%s %s = 0; %[2]s < %[3]s; %[2]s++) {", rtype, iter_name, c99.toString(stmt.X))
		if boxed { // a new variable for each iteration, captured by a closure.
			fmt.Fprintf(c99, "\n%s%s* %s = %s(%s);", strings.Repeat("\t", c99.Tabs+1), rtype, key.String, c99.BoxOf(key.Unique.Type()), iter_name)
		}
		if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		fmt.Fprintf(c99, "}")
		return nil
	case *types.Slice:
		key, hasKey := stmt.Key.Get()
		if !hasKey || key.String == "_" {
			key.String = "go_iter"
			hasKey = false
		}
		indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
		index := key.String
		boxed := hasKey && !c99.StackAllocated(key)
		if boxed {
			index = "go_iter_" + key.String
		}
		fmt.Fprintf(c99, "for (go_ii %s = 0; %[1]s < go_slice_len(%[2]s); %[1]s++) {", index, c99.toString(stmt.X))
		if boxed { // a new variable for each iteration, captured by a closure.
			fmt.Fprintf(c99, "%sgo_ii* %s = %s(%s);", indent, key.String, c99.BoxOf(key.Unique.Type()), index)
		}
		val, hasVal := stmt.Value.Get()
		if hasVal {
			elem := fmt.Sprintf("go_slice_index(%s, %s, %s)", c99.toString(stmt.X), c99.TypeOf(typ.Elem()), index)
			if !c99.StackAllocated(val) {
				fmt.Fprintf(c99, "%s%s* %s = %s(%s);", indent, c99.TypeOf(typ.Elem()), val.String, c99.BoxOf(typ.Elem()), elem)
			} else {
				fmt.Fprintf(c99, "%s%s %s = %s;", indent, c99.TypeOf(typ.Elem()), c99.toString(val), elem)
			}
		}
		if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		fmt.Fprintf(c99, "}")
		return nil
	}
	return stmt.Errorf("range over unsupported type %T", stmt.X.TypeAndValue().Type)
}

func (c99 Target) StatementContinue(stmt source.StatementContinue) error {
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(c99, "goto go_continue_%s", label.String)
	} else {
		fmt.Fprintf(c99, "continue")
	}
	return nil
}

// rangeString ranges over the runes of a string, decoding UTF-8 like Go (invalid encodings
// are U+FFFD, one byte wide).
func (c99 Target) rangeString(stmt source.StatementRange) error {
	n := c99.Closures.count
	c99.Closures.count++
	str, index, width, r := fmt.Sprintf("go_rs_%d", n), fmt.Sprintf("go_ri_%d", n), fmt.Sprintf("go_rw_%d", n), fmt.Sprintf("go_rr_%d", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	fmt.Fprintf(c99, "{ go_ss %s = %s; for (go_ii %s = 0, %s = 0; %[3]s < go_string_len(%[1]s); %[3]s += %[4]s) {", str, c99.toString(stmt.X), index, width)
	fmt.Fprintf(c99, "%sgo_i4 %s; %s = go_string_decode(%s, %s, &%[2]s);", indent, r, width, str, index)
	if key, ok := stmt.Key.Get(); ok && key.String != "_" {
		fmt.Fprintf(c99, "%s%s", indent, c99.declare(key, types.Typ[types.Int], index))
	}
	if value, ok := stmt.Value.Get(); ok && value.String != "_" {
		fmt.Fprintf(c99, "%s%s", indent, c99.declare(value, types.Typ[types.Int32], r))
	}
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s}}", strings.Repeat("\t", c99.Tabs))
	return nil
}

// declare returns a C declaration of the variable name, of type t, with the value (a C
// expression), boxed when captured by a closure.
func (c99 Target) declare(name source.DefinedVariable, t types.Type, value string) string {
	if !c99.StackAllocated(name) {
		return fmt.Sprintf("%s* %s = %s(%s);", c99.TypeOf(t), name.String, c99.BoxOf(t), value)
	}
	return fmt.Sprintf("%s %s = %s;", c99.TypeOf(t), name.String, value)
}

// loopBody writes the body of a loop: continue goes to the end of it (for a labeled loop,
// continue L goes to go_continue_L), and break exits the loop (rather than a switch that
// contains it).
func (c99 Target) loopBody(label string, body []source.Statement) error {
	c99.BreakLabel = ""
	for _, stmt := range body {
		c99.Tabs++
		if err := c99.Statement(stmt); err != nil {
			return err
		}
		c99.Tabs--
	}
	if label != "" {
		fmt.Fprintf(c99, "\n%sgo_continue_%s:;", strings.Repeat("\t", c99.Tabs+1), label)
	}
	return nil
}

// StatementLabel writes a labeled statement, with a label after it for break L (the
// statement after a label may be a declaration in Go, but not C11, so the label is on an
// empty statement).
func (c99 Target) StatementLabel(stmt source.StatementLabel) error {
	fmt.Fprintf(c99, "go_label_%s:;", stmt.Label.String)
	if err := c99.Statement(stmt.Statement); err != nil {
		return err
	}
	fmt.Fprintf(c99, " go_break_%s:;", stmt.Label.String)
	return nil
}

func (c99 Target) StatementGoto(stmt source.StatementGoto) error {
	label, ok := stmt.Label.Get()
	if !ok {
		return stmt.Location.Errorf("goto without a label")
	}
	fmt.Fprintf(c99, "goto go_label_%s", label.String)
	return nil
}
