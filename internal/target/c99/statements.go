package c99

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) Statement(stmt source.Statement) error {
	switch xyz.ValueOf(stmt) {
	case source.Statements.Definitions:
	default:
		if c99.Tabs >= 0 {
			fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		}
	}
	if c99.Tabs < 0 {
		c99.Tabs = -c99.Tabs
	}
	value, _ := stmt.Get()
	var exprs, roots []source.Expression
	declares := false
	switch xyz.ValueOf(stmt) {
	case source.Statements.Assignment:
		assign := source.Statements.Assignment.Get(stmt)
		exprs = append(append(exprs, assign.Variables...), assign.Values...)
		declares = assign.Token.Value == token.DEFINE
	case source.Statements.Expression:
		expr := source.Statements.Expression.Get(stmt)
		exprs, roots = []source.Expression{expr}, []source.Expression{expr}
	case source.Statements.Return:
		exprs = source.Statements.Return.Get(stmt).Results
	case source.Statements.Send:
		send := source.Statements.Send.Get(stmt)
		exprs = []source.Expression{send.X, send.Value}
	case source.Statements.Defer:
		call := source.Statements.Defer.Get(stmt).Call
		exprs = append([]source.Expression{call.Function}, call.Arguments...)
	case source.Statements.Go:
		call := source.Statements.Go.Get(stmt).Call
		exprs = append([]source.Expression{call.Function}, call.Arguments...)
	case source.Statements.Definitions:
		for _, def := range source.Statements.Definitions.Get(stmt) {
			if xyz.ValueOf(def) == source.Definitions.Variable {
				if value, ok := source.Definitions.Variable.Get(def).Value.Get(); ok {
					exprs = append(exprs, value)
				}
			}
		}
		declares = true
	}
	if err := c99.ordered(exprs, roots, declares, func(cc Target) error { return cc.Compile(value) }); err != nil {
		return err
	}
	switch xyz.ValueOf(stmt) {
	case source.Statements.Block, source.Statements.Empty, source.Statements.For, source.Statements.Range,
		source.Statements.If, source.Statements.Definitions, source.Statements.Switch:
		return nil
	default:
		fmt.Fprintf(c99, ";")
		return nil
	}
}

func (c99 Target) StatementBlock(stmt source.StatementBlock) error {
	fmt.Fprintf(c99, "{")
	for _, stmt := range stmt.Statements {
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

func (c99 Target) StatementDecrement(stmt source.StatementDecrement) error {
	return c99.incDec(stmt.WithLocation.SourceLocation, stmt.WithLocation.Value, token.SUB_ASSIGN)
}

// incDec writes x++ or x-- (op is token.ADD_ASSIGN or token.SUB_ASSIGN) as x op= 1.
func (c99 Target) incDec(loc source.Location, x source.Expression, op token.Token) error {
	t := x.TypeAndValue().Type
	one := source.Expressions.DefinedVariable.New(source.DefinedVariable{
		Typed:    source.Typed{TV: types.TypeAndValue{Type: t, Value: constant.MakeInt64(1)}},
		Location: loc,
		String:   "1",
	})
	return c99.assignment(source.StatementAssignment{Location: loc,
		Token:     source.WithLocation[token.Token]{Value: op},
		Variables: []source.Expression{x},
		Values:    []source.Expression{one},
	})
}

func (c99 Target) StatementEmpty(stmt source.StatementEmpty) error { return nil }

func (c99 Target) StatementBreak(stmt source.StatementBreak) error {

	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(c99, "goto go_break_%s", label.String)
	} else if c99.BreakLabel != "" { // in a switch.
		fmt.Fprintf(c99, "goto %s", c99.BreakLabel)
	} else {
		fmt.Fprintf(c99, "break")
	}
	return nil
}

func (c99 Target) StatementIncrement(stmt source.StatementIncrement) error {
	return c99.incDec(stmt.WithLocation.SourceLocation, stmt.WithLocation.Value, token.ADD_ASSIGN)
}

func (c99 Target) StatementSend(stmt source.StatementSend) error {
	symbol := fmt.Sprintf("go_send_%s", c99.Mangle(stmt.X.TypeAndValue().Type.Underlying().(*types.Chan).Elem()))
	c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline void %s(go_ch c, %s v) { go_send(c, sizeof(%[2]s), &v); }\n", symbol, c99.TypeOf(stmt.X.TypeAndValue().Type.Underlying().(*types.Chan).Elem()))
		return nil
	})
	fmt.Fprintf(c99, "%s(", symbol)
	if err := c99.Expression(stmt.X); err != nil {
		return err
	}
	fmt.Fprint(c99, ", ")
	if err := c99.Expression(stmt.Value); err != nil {
		return err
	}
	fmt.Fprint(c99, ")")
	return nil
}
