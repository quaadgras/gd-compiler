package c99

import (
	"fmt"
	"go/ast"
	"go/token"
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
	init, hasInit := stmt.Init.Get()
	if post, ok := stmt.Statement.Get(); ok && xyz.ValueOf(post) == source.Statements.Assignment {
		if assign := source.Statements.Assignment.Get(post); len(assign.Variables) > 1 && len(assign.Values) == 1 {
			return c99.forLoop(stmt) // the post statement is not a C expression.
		}
	}
	if hasInit && xyz.ValueOf(init) == source.Statements.Assignment && !c99.inlineDefinition(source.Statements.Assignment.Get(init)) {
		// defined before the loop (in a block, for their scope).
		fmt.Fprintf(c99, "{ ")
		if err := c99.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		defer fmt.Fprintf(c99, " }")
		hasInit = false
	}
	fmt.Fprintf(c99, "for (")
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
				rebox = append(rebox, fmt.Sprintf("%s = %s(*%[1]s)", name.String, c99.BoxOf(subst(name.Unique.Type()))))
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

// forLoop writes a for statement whose post statement is not a C expression, as a C for
// statement without one, that runs it (and checks the condition) before every iteration
// but the first, which continue statements go to.
func (c99 Target) forLoop(stmt source.StatementFor) error {
	indent := strings.Repeat("\t", c99.Tabs)
	first := fmt.Sprintf("go_loop_%d", c99.Closures.count)
	c99.Closures.count++
	fmt.Fprintf(c99, "{ ")
	if init, ok := stmt.Init.Get(); ok {
		if err := c99.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s", indent)
	}
	fmt.Fprintf(c99, "go_tf %s = true; for (;;) {\n%s\tif (!%[1]s) {", first, indent)
	if init, ok := stmt.Init.Get(); ok && xyz.ValueOf(init) == source.Statements.Assignment {
		for _, v := range source.Statements.Assignment.Get(init).Variables {
			if xyz.ValueOf(v) != source.Expressions.DefinedVariable {
				continue
			}
			if name := source.Expressions.DefinedVariable.Get(v); !c99.StackAllocated(name) {
				fmt.Fprintf(c99, " %s = %s(*%[1]s);", name.String, c99.BoxOf(subst(name.Unique.Type())))
			}
		}
	}
	post, _ := stmt.Statement.Get()
	c99.Tabs += 2
	fmt.Fprintf(c99, "\n%s\t\t", indent)
	if err := c99.Statement(post); err != nil {
		return err
	}
	c99.Tabs -= 2
	fmt.Fprintf(c99, "\n%s\t} %s = false;", indent, first)
	if condition, ok := stmt.Condition.Get(); ok {
		fmt.Fprintf(c99, "\n%s\t", indent)
		c99.Tabs++
		if err := c99.ordered([]source.Expression{condition}, nil, false, func(c99 Target) error {
			fmt.Fprintf(c99, "if (!(")
			if err := c99.Expression(condition); err != nil {
				return err
			}
			fmt.Fprintf(c99, ")) break;")
			return nil
		}); err != nil {
			return err
		}
		c99.Tabs--
	}
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s} }", indent)
	return nil
}

func (c99 Target) StatementRange(stmt source.StatementRange) error {
	if err := c99.rangeTargets(&stmt); err != nil {
		return err
	}
	if stmt.Label != "" {
		fmt.Fprintf(c99, "go_label_%s:; ", stmt.Label)
		defer fmt.Fprintf(c99, " go_break_%s:;", stmt.Label)
	}
	switch typ := stmt.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		return c99.rangeArray(stmt, typ, false)
	case *types.Chan:
		return c99.rangeChan(stmt, typ)
	case *types.Pointer:
		if array, ok := typ.Elem().Underlying().(*types.Array); ok {
			return c99.rangeArray(stmt, array, true)
		}
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
		n := c99.Closures.count // (the bound is evaluated once)
		c99.Closures.count++
		fmt.Fprintf(c99, "for (%[1]s %[2]s = 0, go_rn_%[4]d = %[3]s; %[2]s < go_rn_%[4]d; %[2]s++) {", rtype, iter_name, c99.toString(stmt.X), n)
		if boxed { // a new variable for each iteration, captured by a closure.
			fmt.Fprintf(c99, "\n%s%s* %s = %s(%s);", strings.Repeat("\t", c99.Tabs+1), rtype, key.String, c99.BoxOf(subst(key.Unique.Type())), iter_name)
		}
		if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		fmt.Fprintf(c99, "}")
		return nil
	case *types.Slice: // (the slice is evaluated once)
		n := c99.Closures.count
		c99.Closures.count++
		slice, index := fmt.Sprintf("go_rs_%d", n), fmt.Sprintf("go_ri_%d", n)
		indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
		fmt.Fprintf(c99, "{ go_ll %[1]s = %[2]s; for (go_ii %[3]s = 0; %[3]s < go_slice_len(%[1]s); %[3]s++) {", slice, c99.toString(stmt.X), index)
		if key, ok := stmt.Key.Get(); ok && key.String != "_" {
			fmt.Fprintf(c99, "%s%s", indent, c99.bind(key, types.Typ[types.Int], index))
		}
		if val, ok := stmt.Value.Get(); ok && val.String != "_" {
			fmt.Fprintf(c99, "%s%s", indent, c99.bind(val, typ.Elem(), fmt.Sprintf("go_slice_index(%s, %s, %s)", slice, c99.TypeOf(typ.Elem()), index)))
		}
		if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
			return err
		}
		fmt.Fprintf(c99, "\n%s}}", strings.Repeat("\t", c99.Tabs))
		return nil
	}
	if typ, ok := stmt.X.TypeAndValue().Type.Underlying().(*types.Map); ok {
		return c99.rangeMap(stmt, typ)
	}
	return stmt.Errorf("range over unsupported type %T", stmt.X.TypeAndValue().Type)
}

// rangeMap writes a range over a map, see go_map_range.
func (c99 Target) rangeMap(stmt source.StatementRange, typ *types.Map) error {
	n := c99.Closures.count
	c99.Closures.count++
	it, k, v := fmt.Sprintf("go_mi_%d", n), fmt.Sprintf("go_mk_%d", n), fmt.Sprintf("go_mv_%d", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	key, hasKey := stmt.Key.Get()
	val, hasVal := stmt.Value.Get()
	hasKey = hasKey && key.String != "_"
	hasVal = hasVal && val.String != "_"
	vp := "NULL"
	if hasVal {
		vp = "&" + v
	}
	fmt.Fprintf(c99, "{ go_map_iter %[1]s = go_map_range(%[2]s); for (;;) { %[3]s %[4]s; %[5]s %[6]s; if (!go_map_next(&%[1]s, &%[4]s, %[7]s)) break;",
		it, c99.toString(stmt.X), c99.TypeOf(typ.Key()), k, c99.TypeOf(typ.Elem()), v, vp)
	if !hasVal {
		fmt.Fprintf(c99, " (void)%s;", v)
	}
	if hasKey {
		fmt.Fprintf(c99, "%s%s", indent, c99.bind(key, typ.Key(), k))
	}
	if hasVal {
		fmt.Fprintf(c99, "%s%s", indent, c99.bind(val, typ.Elem(), v))
	}
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s}}", strings.Repeat("\t", c99.Tabs))
	return nil
}

// bind defines the variable of a range statement (or assigns it, for range with =).
func (c99 Target) bind(name source.DefinedVariable, t types.Type, value string) string {
	if !name.Defines() {
		return fmt.Sprintf("%s = %s;", c99.toString(source.Expressions.DefinedVariable.New(name)), value)
	}
	return c99.declare(name, t, value)
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
		return fmt.Sprintf("%s* %s = go_new(sizeof(%[1]s), NULL).ptr; *%[2]s = %s;", c99.TypeOf(t), name.String, value)
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

// rangeArray ranges over an array, which is evaluated once (a copy), or over the array that
// a pointer points to.
func (c99 Target) rangeArray(stmt source.StatementRange, array *types.Array, pointer bool) error {
	n := c99.Closures.count
	c99.Closures.count++
	x, index := fmt.Sprintf("go_ra_%d", n), fmt.Sprintf("go_ri_%d", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	elem := x + ".a[" + index + "]"
	if pointer {
		fmt.Fprintf(c99, "{ go_pt %s = %s;", x, c99.toString(stmt.X))
		elem = fmt.Sprintf("go_pointer_get(%s, %s).a[%s]", x, c99.ArrayTypeOf(array), index)
	} else {
		fmt.Fprintf(c99, "{ %s %s = %s;", c99.ArrayTypeOf(array), x, c99.toString(stmt.X))
	}
	fmt.Fprintf(c99, " for (go_ii %[1]s = 0; %[1]s < %[2]d; %[1]s++) {", index, array.Len())
	if key, ok := stmt.Key.Get(); ok && key.String != "_" {
		fmt.Fprintf(c99, "%s%s", indent, c99.declare(key, types.Typ[types.Int], index))
	}
	if value, ok := stmt.Value.Get(); ok && value.String != "_" {
		fmt.Fprintf(c99, "%s%s", indent, c99.declare(value, array.Elem(), elem))
	}
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s}}", strings.Repeat("\t", c99.Tabs))
	return nil
}

// rangeChan receives from a channel, until it is closed.
func (c99 Target) rangeChan(stmt source.StatementRange, typ *types.Chan) error {
	n := c99.Closures.count
	c99.Closures.count++
	ch, v := fmt.Sprintf("go_rc_%d", n), fmt.Sprintf("go_rv_%d", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	elem := c99.TypeOf(typ.Elem())
	fmt.Fprintf(c99, "{ go_ch %s = %s; for (;;) { %s %s; if (!go_recv(%[1]s, sizeof(%[4]s), &%[4]s)) break;", ch, c99.toString(stmt.X), elem, v)
	if key, ok := stmt.Key.Get(); ok && key.String != "_" { // the key is the element.
		fmt.Fprintf(c99, "%s%s", indent, c99.declare(key, typ.Elem(), v))
	}
	if err := c99.loopBody(stmt.Label, stmt.Body.Statements); err != nil {
		return err
	}
	fmt.Fprintf(c99, "\n%s}}", strings.Repeat("\t", c99.Tabs))
	return nil
}

// rangeTargets rewrites a range statement that assigns (for i, a[j] = range x) to one that
// defines variables, and assigns them first in its body.
func (c99 Target) rangeTargets(stmt *source.StatementRange) error {
	var targets, values []source.Expression
	temp := func(target source.Expression) source.DefinedVariable {
		name := fmt.Sprintf("go_rt_%d", c99.Closures.count)
		c99.Closures.count++
		t := target.TypeAndValue().Type
		loc := stmt.Location
		loc.Node = &ast.Ident{Name: name, NamePos: loc.Open} // (a node of its own, see [Target.Substitutes])
		v := source.DefinedVariable{
			Typed:    source.Typed{TV: types.TypeAndValue{Type: t}},
			Location: loc,
			String:   name,
			Unique:   types.NewVar(stmt.Location.Open, nil, name, t),
		}
		targets = append(targets, target)
		values = append(values, source.Expressions.DefinedVariable.New(v))
		return v
	}
	if key, ok := stmt.Key.Get(); ok && key.String != "_" && !key.Defines() { // for i = range x
		stmt.KeyTarget, stmt.Key = xyz.New(source.Expressions.DefinedVariable.New(key)), xyz.Maybe[source.DefinedVariable]{}
	}
	if val, ok := stmt.Value.Get(); ok && val.String != "_" && !val.Defines() {
		stmt.ValueTarget, stmt.Value = xyz.New(source.Expressions.DefinedVariable.New(val)), xyz.Maybe[source.DefinedVariable]{}
	}
	if target, ok := stmt.KeyTarget.Get(); ok {
		stmt.Key = xyz.New(temp(target))
		stmt.KeyTarget = xyz.Maybe[source.Expression]{}
	}
	if target, ok := stmt.ValueTarget.Get(); ok {
		stmt.Value = xyz.New(temp(target))
		stmt.ValueTarget = xyz.Maybe[source.Expression]{}
	}
	if len(targets) == 0 {
		return nil
	}
	assign := source.Statements.Assignment.New(source.StatementAssignment{
		Location:  stmt.Location,
		Token:     source.WithLocation[token.Token]{Value: token.ASSIGN},
		Variables: targets,
		Values:    values,
	})
	stmt.Body.Statements = append([]source.Statement{assign}, stmt.Body.Statements...)
	return nil
}
