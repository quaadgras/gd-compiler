package c

import (
	"fmt"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) StatementIf(stmt source.StatementIf) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(cc, "{")
		if err := cc.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(cc, "; ")
	}
	fmt.Fprintf(cc, "if (")
	if err := cc.Expression(stmt.Condition); err != nil {
		return err
	}
	fmt.Fprintf(cc, ") {")
	for _, stmt := range stmt.Body.Statements {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	ifelse, hasElse := stmt.Else.Get()
	if hasElse {
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		fmt.Fprintf(cc, "} else ")
		cc.Tabs = -cc.Tabs
		if err := cc.Statement(ifelse); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		fmt.Fprintf(cc, "}")
	}
	return nil
}

func (cc Target) StatementSwitch(stmt source.StatementSwitch) error {
	fmt.Fprintf(cc, "{")
	if init, ok := stmt.Init.Get(); ok {
		cc.Tabs = -cc.Tabs
		if err := cc.Statement(init); err != nil {
			return err
		}
		cc.Tabs = -cc.Tabs
	}
	fmt.Fprintf(cc, "switch (")
	if value, ok := stmt.Value.Get(); ok {
		if err := cc.Expression(value); err != nil {
			return err
		}
	}
	fmt.Fprintf(cc, ") {")
	for _, clause := range stmt.Clauses {
		cc.Tabs++
		if err := cc.SwitchCaseClause(clause); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "}}")
	return nil
}
