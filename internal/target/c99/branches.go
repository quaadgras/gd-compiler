package c99

import (
	"fmt"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) StatementIf(stmt source.StatementIf) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(c99, "{")
		if err := c99.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(c99, "; ")
	}
	fmt.Fprintf(c99, "if (")
	if err := c99.Expression(stmt.Condition); err != nil {
		return err
	}
	fmt.Fprintf(c99, ") {")
	for _, stmt := range stmt.Body.Statements {
		c99.Tabs++
		if err := c99.Statement(stmt); err != nil {
			return err
		}
		c99.Tabs--
	}
	ifelse, hasElse := stmt.Else.Get()
	if hasElse {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		fmt.Fprintf(c99, "} else ")
		c99.Tabs = -c99.Tabs
		if err := c99.Statement(ifelse); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		fmt.Fprintf(c99, "}")
	}
	return nil
}

func (c99 Target) StatementSwitch(stmt source.StatementSwitch) error {
	fmt.Fprintf(c99, "{")
	if init, ok := stmt.Init.Get(); ok {
		c99.Tabs = -c99.Tabs
		if err := c99.Statement(init); err != nil {
			return err
		}
		c99.Tabs = -c99.Tabs
	}
	fmt.Fprintf(c99, "switch (")
	if value, ok := stmt.Value.Get(); ok {
		if err := c99.Expression(value); err != nil {
			return err
		}
	}
	fmt.Fprintf(c99, ") {")
	for _, clause := range stmt.Clauses {
		c99.Tabs++
		if err := c99.SwitchCaseClause(clause); err != nil {
			return err
		}
		c99.Tabs--
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}}")
	return nil
}
