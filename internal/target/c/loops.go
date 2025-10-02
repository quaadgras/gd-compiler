package c

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) StatementFor(stmt source.StatementFor) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(cc, "{")
		if err := cc.Statement(init); err != nil {
			return err
		}
	}
	if stmt.Label != "" {
		fmt.Fprintf(cc, " %s:", stmt.Label)
	}
	fmt.Fprintf(cc, "while (")
	condition, hasCondition := stmt.Condition.Get()
	if hasCondition {
		if err := cc.Expression(condition); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cc, "true")
	}
	fmt.Fprintf(cc, ")")
	statement, hasStatement := stmt.Statement.Get()
	if hasStatement {
		fmt.Fprintf(cc, ": (")
		cc.Tabs = -cc.Tabs
		stmt, _ := statement.Get()
		if err := cc.Compile(stmt); err != nil {
			return err
		}
		cc.Tabs = -cc.Tabs
		fmt.Fprintf(cc, ")")
	}
	fmt.Fprintf(cc, " {")
	for _, stmt := range stmt.Body.Statements {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "}")
	if hasInit {
		fmt.Fprintf(cc, "}")
	}
	return nil
}

func (cc Target) StatementRange(stmt source.StatementRange) error {
	switch stmt.X.TypeAndValue().Type.(type) {
	case *types.Basic:
		fmt.Fprintf(cc, "for (0..@as(%s,", cc.TypeOf(stmt.X.TypeAndValue().Type))
		if err := cc.Expression(stmt.X); err != nil {
			return err
		}
		fmt.Fprintf(cc, "))")
		key, hasKey := stmt.Key.Get()
		if hasKey {
			fmt.Fprintf(cc, " | ")
			if err := cc.DefinedVariable(key); err != nil {
				return err
			}
			fmt.Fprintf(cc, " |")
		} else {
			fmt.Fprintf(cc, " |_|")
		}
		if stmt.Label != "" {
			fmt.Fprintf(cc, " %s:", stmt.Label)
		}
		fmt.Fprintf(cc, " {")
		for _, stmt := range stmt.Body.Statements {
			cc.Tabs++
			if err := cc.Statement(stmt); err != nil {
				return err
			}
			cc.Tabs--
		}
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		fmt.Fprintf(cc, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(cc, "for (")
		key, hasKey := stmt.Key.Get()
		if key.String == "_" {
			hasKey = false
		}
		val, hasVal := stmt.Value.Get()
		if hasKey {
			fmt.Fprintf(cc, "0..,")
		}
		if err := cc.Expression(stmt.X); err != nil {
			return err
		}
		fmt.Fprintf(cc, ".arraylist.items) |")
		if hasKey {
			if err := cc.DefinedVariable(key); err != nil {
				return err
			}
		}
		if hasKey && hasVal {
			fmt.Fprintf(cc, ",")
		}
		if hasVal {
			if err := cc.DefinedVariable(val); err != nil {
				return err
			}
		}
		fmt.Fprintf(cc, "|")
		if stmt.Label != "" {
			fmt.Fprintf(cc, " %s:", stmt.Label)
		}
		fmt.Fprintf(cc, " {")
		for _, stmt := range stmt.Body.Statements {
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
	return stmt.Errorf("range over unsupported type %T", stmt.X.TypeAndValue().Type)
}

func (cc Target) StatementContinue(stmt source.StatementContinue) error {
	fmt.Fprintf(cc, "continue")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(cc, " : %s", label.String)
	}
	return nil
}
