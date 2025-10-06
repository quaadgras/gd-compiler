package c99

import (
	"fmt"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
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
	if err := c99.Compile(value); err != nil {
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
	value, _ := stmt.WithLocation.Value.Get()
	if err := c99.Compile(value); err != nil {
		return err
	}
	fmt.Fprintf(c99, "-=1")
	return nil
}

func (c99 Target) StatementDefer(stmt source.StatementDefer) error {
	// TODO arguments need to be evaluated at the time of the defer statement.
	if stmt.OutermostScope {
		fmt.Fprintf(c99, "go_defer(")
		if err := c99.Expression(stmt.Call.Function); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", go_tuple")
		for _, arg := range stmt.Call.Arguments {
			fmt.Fprintf(c99, "__%s", c99.TypeOf(arg.TypeAndValue().Type))
		}
		for _, arg := range stmt.Call.Arguments {
			fmt.Fprintf(c99, ", ")
			if err := c99.Expression(arg); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, ")")
		return nil
	}
	return stmt.Location.Errorf("only defer at the outermost scope of a function is currently supported")
}

func (c99 Target) StatementEmpty(stmt source.StatementEmpty) error { return nil }

func (c99 Target) StatementBreak(stmt source.StatementBreak) error {

	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(c99, "goto %s_end", label.String)
	} else {
		fmt.Fprintf(c99, "break")
	}
	return nil
}

func (c99 Target) StatementIncrement(stmt source.StatementIncrement) error {
	if err := c99.Expression(stmt.WithLocation.Value); err != nil {
		return err
	}
	fmt.Fprintf(c99, "+=1")
	return nil
}

func (c99 Target) StatementReturn(stmt source.StatementReturn) error {
	fmt.Fprintf(c99, "return")
	for _, result := range stmt.Results {
		fmt.Fprintf(c99, " ")
		if err := c99.Expression(result); err != nil {
			return err
		}
	}
	return nil
}

func (c99 Target) StatementSend(stmt source.StatementSend) error {
	symbol := fmt.Sprintf("go_send_%s", c99.Mangle(stmt.X.TypeAndValue().Type.(*types.Chan).Elem()))
	c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline void %s(go_ch c, %s v) { go_send(c, sizeof(%[2]s), &v); }\n", symbol, c99.TypeOf(stmt.X.TypeAndValue().Type.(*types.Chan).Elem()))
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

func (c99 Target) StatementSwitchType(stmt source.StatementSwitchType) error {
	fmt.Fprintf(c99, "switch ")
	if init, ok := stmt.Init.Get(); ok {
		if err := c99.Statement(init); err != nil {
			return err
		}
	}
	if err := c99.Statement(stmt.Assign); err != nil {
		return err
	}
	fmt.Fprintf(c99, " {")
	for _, clause := range stmt.Claused {
		if err := c99.SwitchCaseClause(clause); err != nil {
			return err
		}
	}
	fmt.Fprintf(c99, "}")
	return nil
}

func (c99 Target) SwitchCaseClause(clause source.SwitchCaseClause) error {
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	if len(clause.Expressions) == 0 {
		fmt.Fprintf(c99, "default:")
	} else {
		for _, expr := range clause.Expressions {
			fmt.Fprintf(c99, "case ")
			if err := c99.Expression(expr); err != nil {
				return err
			}
			fmt.Fprintf(c99, ": ")
		}
	}
	c99.Tabs++
	for _, stmt := range clause.Body {
		if err := c99.Statement(stmt); err != nil {
			return err
		}
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "break;")
	c99.Tabs--
	return nil
}
