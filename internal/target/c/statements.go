package c

import (
	"fmt"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (cc Target) Statement(stmt source.Statement) error {
	switch xyz.ValueOf(stmt) {
	case source.Statements.Definitions:
	default:
		if cc.Tabs >= 0 {
			fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		}
	}
	if cc.Tabs < 0 {
		cc.Tabs = -cc.Tabs
	}
	value, _ := stmt.Get()
	if err := cc.Compile(value); err != nil {
		return err
	}
	switch xyz.ValueOf(stmt) {
	case source.Statements.Block, source.Statements.Empty, source.Statements.For, source.Statements.Range,
		source.Statements.If, source.Statements.Definitions, source.Statements.Switch:
		return nil
	default:
		fmt.Fprintf(cc, ";")
		return nil
	}
}

func (cc Target) StatementBlock(stmt source.StatementBlock) error {
	fmt.Fprintf(cc, "{")
	for _, stmt := range stmt.Statements {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "}")
	return nil
}

func (cc Target) StatementDecrement(stmt source.StatementDecrement) error {
	value, _ := stmt.WithLocation.Value.Get()
	if err := cc.Compile(value); err != nil {
		return err
	}
	fmt.Fprintf(cc, "-=1")
	return nil
}

func (cc Target) StatementDefer(stmt source.StatementDefer) error {
	// TODO arguments need to be evaluated at the time of the defer statement.
	if stmt.OutermostScope {
		fmt.Fprintf(cc, "defer ")
		return cc.FunctionCall(stmt.Call)
	}
	return stmt.Location.Errorf("only defer at the outermost scope of a function is currently supported")
}

func (cc Target) StatementEmpty(stmt source.StatementEmpty) error { return nil }

func (cc Target) StatementBreak(stmt source.StatementBreak) error {
	fmt.Fprintf(cc, "break")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(cc, " : %s", label.String)
	}
	return nil
}

func (cc Target) StatementIncrement(stmt source.StatementIncrement) error {
	if err := cc.Expression(stmt.WithLocation.Value); err != nil {
		return err
	}
	fmt.Fprintf(cc, "+=1")
	return nil
}

func (cc Target) StatementReturn(stmt source.StatementReturn) error {
	fmt.Fprintf(cc, "return")
	for _, result := range stmt.Results {
		fmt.Fprintf(cc, " ")
		if err := cc.Expression(result); err != nil {
			return err
		}
	}
	return nil
}

func (cc Target) StatementSend(stmt source.StatementSend) error {
	if err := cc.Expression(stmt.X); err != nil {
		return err
	}
	fmt.Fprint(cc, ".send(goto,")
	if err := cc.Expression(stmt.Value); err != nil {
		return err
	}
	fmt.Fprint(cc, ")")
	return nil
}

func (cc Target) StatementSwitchType(stmt source.StatementSwitchType) error {
	fmt.Fprintf(cc, "switch ")
	if init, ok := stmt.Init.Get(); ok {
		if err := cc.Statement(init); err != nil {
			return err
		}
	}
	if err := cc.Statement(stmt.Assign); err != nil {
		return err
	}
	fmt.Fprintf(cc, " {")
	for _, clause := range stmt.Claused {
		if err := cc.SwitchCaseClause(clause); err != nil {
			return err
		}
	}
	fmt.Fprintf(cc, "}")
	return nil
}

func (cc Target) SwitchCaseClause(clause source.SwitchCaseClause) error {
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	if len(clause.Expressions) == 0 {
		fmt.Fprintf(cc, "else")
	} else {
		for i, expr := range clause.Expressions {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			if err := cc.Expression(expr); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(cc, " => {")
	for _, stmt := range clause.Body {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "},")
	return nil
}
