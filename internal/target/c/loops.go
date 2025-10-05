package c

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) StatementFor(stmt source.StatementFor) error {
	if stmt.Label != "" {
		fmt.Fprintf(c99, " %s:", stmt.Label)
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
	statement, hasStatement := stmt.Statement.Get()
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
	switch stmt.X.TypeAndValue().Type.(type) {
	case *types.Basic:
		rtype := c99.TypeOf(stmt.X.TypeAndValue().Type)
		iter_name := "go_iter"
		key, hasKey := stmt.Key.Get()
		if hasKey {
			iter_name = c99.toString(key)
		}
		if stmt.Label != "" {
			fmt.Fprintf(c99, " %s:", stmt.Label)
		}
		fmt.Fprintf(c99, "for (%s %s = 0; %[2]s < %[3]s; %[2]s++) {", rtype, iter_name, c99.toString(stmt.X))
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
		fmt.Fprintf(c99, "for (")
		key, hasKey := stmt.Key.Get()
		if key.String == "_" {
			hasKey = false
		}
		val, hasVal := stmt.Value.Get()
		if hasKey {
			fmt.Fprintf(c99, "0..,")
		}
		if err := c99.Expression(stmt.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ".arraylist.items) |")
		if hasKey {
			if err := c99.DefinedVariable(key); err != nil {
				return err
			}
		}
		if hasKey && hasVal {
			fmt.Fprintf(c99, ",")
		}
		if hasVal {
			if err := c99.DefinedVariable(val); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, "|")
		if stmt.Label != "" {
			fmt.Fprintf(c99, " %s:", stmt.Label)
		}
		fmt.Fprintf(c99, " {")
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
	fmt.Fprintf(c99, "continue")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(c99, " : %s", label.String)
	}
	return nil
}
