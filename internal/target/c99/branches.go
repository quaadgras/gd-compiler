package c99

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
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
	if err := c99.ordered([]source.Expression{stmt.Condition}, nil, false, func(c99 Target) error {
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
	}); err != nil {
		return err
	}
	if hasInit {
		fmt.Fprintf(c99, "}") // scope of the init statement.
	}
	return nil
}

// StatementSwitch dispatches with gotos (C switches only support integers): the tag is
// evaluated once, then the case expressions in order, until one matches. break exits the
// switch, and fallthrough continues into the next case.
func (c99 Target) StatementSwitch(stmt source.StatementSwitch) error {
	n := c99.Closures.count
	c99.Closures.count++
	end := fmt.Sprintf("go_switch_%d_end", n)
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	fmt.Fprintf(c99, "{")
	if init, ok := stmt.Init.Get(); ok {
		if err := c99.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(c99, ";")
	}
	value, hasTag := stmt.Value.Get()
	var tag source.Expression
	if hasTag {
		t := types.Default(value.TypeAndValue().Type)
		name := fmt.Sprintf("go_tag_%d", n)
		fmt.Fprint(c99, indent)
		if err := c99.ordered([]source.Expression{value}, nil, true, func(c99 Target) error {
			fmt.Fprintf(c99, "%s %s = ", c99.TypeOf(t), name)
			return c99.Expression(value)
		}); err != nil {
			return err
		}
		fmt.Fprintf(c99, ";")
		tag = c99.element(t, name)
	}
	defaultCase := end
	for i, clause := range stmt.Clauses {
		label := fmt.Sprintf("go_switch_%d_case_%d", n, i)
		if len(clause.Expressions) == 0 {
			defaultCase = label
		}
		for _, expr := range clause.Expressions {
			fmt.Fprint(c99, indent)
			if err := c99.ordered([]source.Expression{expr}, nil, false, func(c99 Target) error {
				fmt.Fprintf(c99, "if (")
				if hasTag {
					if err := c99.ExpressionBinary(source.ExpressionBinary{
						Typed:     source.Typed{TV: types.TypeAndValue{Type: types.Typ[types.Bool]}},
						X:         tag,
						Operation: source.WithLocation[token.Token]{Value: token.EQL},
						Y:         expr,
					}); err != nil {
						return err
					}
				} else if err := c99.Expression(expr); err != nil {
					return err
				}
				fmt.Fprintf(c99, ") goto %s", label)
				return nil
			}); err != nil {
				return err
			}
			fmt.Fprintf(c99, ";")
		}
	}
	fmt.Fprintf(c99, "%sgoto %s;", indent, defaultCase)
	c99.BreakLabel = end
	for i, clause := range stmt.Clauses {
		fmt.Fprintf(c99, "%sgo_switch_%d_case_%d:; {", indent, n, i)
		c99.Tabs += 2
		for _, stmt := range clause.Body {
			if err := c99.Statement(stmt); err != nil {
				return err
			}
		}
		c99.Tabs -= 2
		fmt.Fprintf(c99, "%s}", indent)
		if !clause.Fallsthrough {
			fmt.Fprintf(c99, " goto %s;", end)
		}
	}
	fmt.Fprintf(c99, "%s%s:;\n%s}", indent, end, strings.Repeat("\t", c99.Tabs))
	return nil
}

// StatementFallthrough is implemented by the clause that falls through, see
// [Target.StatementSwitch].
func (c99 Target) StatementFallthrough(stmt source.StatementFallthrough) error { return nil }

// StatementSwitchType dispatches on the dynamic type of an interface value, with gotos (see
// [Target.StatementSwitch]). The variable of each clause (switch v := x.(type)) has the
// clause's type, when it has one, or the type of x.
func (c99 Target) StatementSwitchType(stmt source.StatementSwitchType) error {
	n := c99.Closures.count
	c99.Closures.count++
	prefix := fmt.Sprintf("go_typeswitch_%d", n)
	end := prefix + "_end"
	indent := "\n" + strings.Repeat("\t", c99.Tabs+1)
	fmt.Fprintf(c99, "{")
	if init, ok := stmt.Init.Get(); ok {
		if err := c99.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(c99, ";")
	}
	var guard source.Expression
	var binding string
	switch xyz.ValueOf(stmt.Assign) {
	case source.Statements.Expression:
		guard = source.Statements.Expression.Get(stmt.Assign)
	case source.Statements.Assignment:
		assign := source.Statements.Assignment.Get(stmt.Assign)
		guard = assign.Values[0]
		if xyz.ValueOf(assign.Variables[0]) == source.Expressions.DefinedVariable {
			binding = source.Expressions.DefinedVariable.Get(assign.Variables[0]).String
		}
	default:
		return stmt.Location.Errorf("unsupported type switch")
	}
	for xyz.ValueOf(guard) == source.Expressions.Parenthesized {
		guard = source.Expressions.Parenthesized.Get(guard).X
	}
	if xyz.ValueOf(guard) != source.Expressions.TypeAssertion {
		return stmt.Location.Errorf("unsupported type switch")
	}
	x := source.Expressions.TypeAssertion.Get(guard).X
	xtype := x.TypeAndValue().Type
	original, dynamic := prefix+"_x", prefix+"_v"
	fmt.Fprintf(c99, "%s%s %s = ", indent, c99.TypeOf(xtype), original)
	if err := c99.Expression(x); err != nil {
		return err
	}
	fmt.Fprintf(c99, "; go_vv %s = ", dynamic)
	if iface, ok := xtype.Underlying().(*types.Interface); ok && !iface.Empty() {
		fmt.Fprintf(c99, "go_if_to_vv(%s);", original)
	} else {
		fmt.Fprintf(c99, "%s;", original)
	}
	defaultCase := end
	for i, clause := range stmt.Claused {
		label := fmt.Sprintf("%s_case_%d", prefix, i)
		if len(clause.Expressions) == 0 {
			defaultCase = label
		}
		for _, expr := range clause.Expressions {
			var cond string
			if isNil(expr) {
				cond = dynamic + ".go_type == NULL"
			} else if iface, ok := expr.TypeAndValue().Type.Underlying().(*types.Interface); ok {
				if !iface.Empty() {
					return stmt.Location.Errorf("unsupported type switch case %s (an interface with methods)", expr.TypeAndValue().Type)
				}
				cond = dynamic + ".go_type != NULL"
			} else {
				rtype, err := c99.reflectTypeOf(expr.TypeAndValue().Type)
				if err != nil {
					return stmt.Location.Errorf("%w", err)
				}
				cond = fmt.Sprintf("go_type_eq(%s.go_type, %s)", dynamic, rtype)
			}
			fmt.Fprintf(c99, "%sif (%s) goto %s;", indent, cond, label)
		}
	}
	fmt.Fprintf(c99, "%sgoto %s;", indent, defaultCase)
	c99.BreakLabel = end
	for i, clause := range stmt.Claused {
		fmt.Fprintf(c99, "%s%s_case_%d:; {", indent, prefix, i)
		if binding != "" && c99.Closures.info != nil {
			if obj, ok := c99.Closures.info.Implicits[clause.Location.Node].(*types.Var); ok {
				t := subst(obj.Type())
				value := original
				if len(clause.Expressions) == 1 && !isNil(clause.Expressions[0]) {
					if iface, ok := t.Underlying().(*types.Interface); ok {
						if iface.Empty() {
							value = dynamic
						}
					} else {
						value = fmt.Sprintf("(*(%s*)%s.ptr.ptr)", c99.TypeOf(t), dynamic)
					}
				}
				name := source.DefinedVariable{String: binding, Unique: obj}
				fmt.Fprintf(c99, " %s (void)%s;", c99.declare(name, t, value), binding)
			}
		}
		c99.Tabs += 2
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
