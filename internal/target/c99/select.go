package c99

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// StatementSelect evaluates the channels and values of its cases once, in source order,
// then chooses a case that can proceed with go_select (or waits for one), and dispatches to
// it with gotos, where the received values are assigned. break exits the select.
func (c99 Target) StatementSelect(stmt source.StatementSelect) error {
	n := c99.Closures.count
	c99.Closures.count++
	prefix := fmt.Sprintf("go_select_%d", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	fmt.Fprintf(c99, "{")
	type recv struct {
		assign source.StatementAssignment
		value  string // C variable received into.
		elem   types.Type
	}
	var entries []string
	cases := make(map[int]int) // clause → index in the cases.
	receives := make(map[int]recv)
	defaultClause := -1
	chanOf := func(x source.Expression) (*types.Chan, error) {
		ch, ok := x.TypeAndValue().Type.Underlying().(*types.Chan)
		if !ok {
			return nil, fmt.Errorf("unsupported select on %s", x.TypeAndValue().Type)
		}
		return ch, nil
	}
	for i, clause := range stmt.Clauses {
		comm, ok := clause.Statement.Get()
		if !ok {
			defaultClause = i
			continue
		}
		c, v := fmt.Sprintf("%s_c%d", prefix, i), fmt.Sprintf("%s_v%d", prefix, i)
		var receive source.Expression
		var assign source.StatementAssignment
		switch xyz.ValueOf(comm) {
		case source.Statements.Send:
			send := source.Statements.Send.Get(comm)
			ch, err := chanOf(send.X)
			if err != nil {
				return stmt.Location.Errorf("%w", err)
			}
			fmt.Fprintf(c99, "%sgo_ch %s = %s; %s %s = ", indent, c, c99.toString(send.X), c99.TypeOf(ch.Elem()), v)
			if err := c99.ExpressionAs(send.Value, ch.Elem()); err != nil {
				return err
			}
			fmt.Fprintf(c99, ";")
			cases[i] = len(entries)
			entries = append(entries, fmt.Sprintf("{ %s, true, &%s, false }", c, v))
			continue
		case source.Statements.Expression:
			receive = source.Statements.Expression.Get(comm)
		case source.Statements.Assignment:
			assign = source.Statements.Assignment.Get(comm)
			if len(assign.Values) != 1 {
				return stmt.Location.Errorf("unsupported select case")
			}
			receive = assign.Values[0]
		default:
			return stmt.Location.Errorf("unsupported select case")
		}
		for xyz.ValueOf(receive) == source.Expressions.Parenthesized {
			receive = source.Expressions.Parenthesized.Get(receive).X
		}
		if xyz.ValueOf(receive) != source.Expressions.AwaitChannel {
			return stmt.Location.Errorf("unsupported select case")
		}
		chanExpr := source.Expressions.AwaitChannel.Get(receive).Chan
		ch, err := chanOf(chanExpr)
		if err != nil {
			return stmt.Location.Errorf("%w", err)
		}
		fmt.Fprintf(c99, "%sgo_ch %s = %s; %s %s;", indent, c, c99.toString(chanExpr), c99.TypeOf(ch.Elem()), v)
		cases[i] = len(entries)
		entries = append(entries, fmt.Sprintf("{ %s, false, &%s, false }", c, v))
		if len(assign.Variables) > 0 {
			receives[i] = recv{assign: assign, value: v, elem: ch.Elem()}
		}
	}
	chosen := prefix + "_i"
	if len(entries) == 0 {
		fmt.Fprintf(c99, "%sint %s = go_select(NULL, 0, %t);", indent, chosen, defaultClause < 0)
	} else {
		fmt.Fprintf(c99, "%sgo_select_case %s[] = { %s };", indent, prefix, strings.Join(entries, ", "))
		fmt.Fprintf(c99, "%sint %s = go_select(%s, %d, %t);", indent, chosen, prefix, len(entries), defaultClause < 0)
	}
	end := prefix + "_end"
	for i := range stmt.Clauses {
		if index, ok := cases[i]; ok {
			fmt.Fprintf(c99, "%sif (%s == %d) goto %s_%d;", indent, chosen, index, prefix, i)
		}
	}
	if defaultClause >= 0 {
		fmt.Fprintf(c99, "%sgoto %s_%d;", indent, prefix, defaultClause)
	} else {
		fmt.Fprintf(c99, "%sgoto %s;", indent, end)
	}
	c99.BreakLabel = end
	for i, clause := range stmt.Clauses {
		fmt.Fprintf(c99, "%s%s_%d:; {", indent, prefix, i)
		c99.Tabs += 2
		if r, ok := receives[i]; ok {
			values := []source.Expression{c99.element(r.elem, r.value)}
			if len(r.assign.Variables) == 2 {
				values = append(values, c99.element(types.Typ[types.Bool], fmt.Sprintf("%s[%d].ok", prefix, cases[i])))
			}
			fmt.Fprintf(c99, "%s", indent)
			if err := c99.StatementAssignment(source.StatementAssignment{Location: r.assign.Location,
				Token: r.assign.Token, Variables: r.assign.Variables, Values: values}); err != nil {
				return err
			}
			fmt.Fprintf(c99, ";")
		}
		for _, stmt := range clause.Body {
			if err := c99.Statement(stmt); err != nil {
				return err
			}
		}
		c99.Tabs -= 2
		fmt.Fprintf(c99, "%s} goto %s;", indent, end)
	}
	fmt.Fprintf(c99, "%s%s:;\n%s}", indent, end, strings.Repeat("\t", c99.Tabs))
	return nil
}
