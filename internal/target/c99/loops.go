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
		fmt.Fprintf(c99, " %s: ", stmt.Label)
		defer fmt.Fprintf(c99, " %s_end:;\n", stmt.Label)
	}
	fmt.Fprintf(c99, "for (")
	init, hasInit := stmt.Init.Get()
	if hasInit {
		tabs := c99.Tabs
		c99.Tabs = -c99.Tabs
		if err := c99.Statement(init); err != nil {
			return err
		}
		c99.Tabs = tabs
	}
	fmt.Fprintf(c99, " ")
	condition, hasCondition := stmt.Condition.Get()
	if hasCondition {
		if err := c99.Expression(condition); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c99, "go_true")
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
		stmt, _ := statement.Get()
		if err := c99.Compile(stmt); err != nil {
			return err
		}
		c99.Tabs = -c99.Tabs
	}
	fmt.Fprintf(c99, ") {")
	for _, stmt := range stmt.Body.Statements {
		c99.Tabs++
		if err := c99.Statement(stmt); err != nil {
			return err
		}
		c99.Tabs--
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}")
	return nil
}

func (c99 Target) StatementRange(stmt source.StatementRange) error {
	if stmt.Label != "" {
		fmt.Fprintf(c99, "%s: ", stmt.Label)
		defer fmt.Fprintf(c99, " %s_end:;\n", stmt.Label)
	}
	switch typ := stmt.X.TypeAndValue().Type.(type) {
	case *types.Basic:
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
		for _, stmt := range stmt.Body.Statements {
			c99.Tabs++
			if err := c99.Statement(stmt); err != nil {
				return err
			}
			c99.Tabs--
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
		for _, stmt := range stmt.Body.Statements {
			c99.Tabs++
			if err := c99.Statement(stmt); err != nil {
				return err
			}
			c99.Tabs--
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
		fmt.Fprintf(c99, "goto %s", label.String)
	} else {
		fmt.Fprintf(c99, "continue")
	}
	return nil
}
