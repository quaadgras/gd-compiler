package c99

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// StatementAssignment assigns (or defines) variables. Go evaluates all of the values before
// assigning any of them, so with more than one variable, the values are evaluated into
// temporaries first (except in the header of a for statement, where they are assigned in
// turn, when that is equivalent).
func (c99 Target) StatementAssignment(stmt source.StatementAssignment) error {
	if len(stmt.Variables) != len(stmt.Values) {
		if len(stmt.Values) != 1 {
			return stmt.Location.Errorf("unsupported assignment")
		}
		return c99.assignTuple(stmt)
	}
	if len(stmt.Variables) == 1 {
		return c99.assignment(stmt)
	}
	if c99.Header {
		for i := 1; i < len(stmt.Values); i++ {
			for _, name := range namesIn(stmt.Values[i]) {
				for _, earlier := range stmt.Variables[:i] {
					if earlier := namesIn(earlier); len(earlier) > 0 && earlier[0] == name {
						return stmt.Location.Errorf("unsupported assignment in for statement that depends on its order")
					}
				}
			}
		}
		if stmt.Token.Value == token.DEFINE {
			return c99.defineMany(stmt)
		}
		for i := range stmt.Variables {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if err := c99.assignment(source.StatementAssignment{Location: stmt.Location, Token: stmt.Token,
				Variables: stmt.Variables[i : i+1], Values: stmt.Values[i : i+1]}); err != nil {
				return err
			}
		}
		return nil
	}
	subs := make(map[ast.Node]string)
	for k, v := range c99.Substitutes {
		subs[k] = v
	}
	for _, value := range stmt.Values {
		if reevaluate(value) {
			continue
		}
		name := fmt.Sprintf("go_assign_%d", c99.Closures.count)
		c99.Closures.count++
		fmt.Fprintf(c99, "%s %s = ", c99.TypeOf(types.Default(value.TypeAndValue().Type)), name)
		if err := c99.Expression(value); err != nil {
			return err
		}
		fmt.Fprintf(c99, "; ")
		subs[source.LocationOf(value).Node] = name
	}
	c99.Substitutes = subs
	for i := range stmt.Variables {
		if i > 0 {
			fmt.Fprintf(c99, "; ")
		}
		if err := c99.assignment(source.StatementAssignment{Location: stmt.Location, Token: stmt.Token,
			Variables: stmt.Variables[i : i+1], Values: stmt.Values[i : i+1]}); err != nil {
			return err
		}
	}
	return nil
}

// defineMany defines several variables in the init statement of a for statement, which
// can only be a single C declaration, so they must have the same type.
func (c99 Target) defineMany(stmt source.StatementAssignment) error {
	var ctype string
	for i, variable := range stmt.Variables {
		if xyz.ValueOf(variable) != source.Expressions.DefinedVariable {
			return stmt.Location.Errorf("unsupported definition")
		}
		name := source.Expressions.DefinedVariable.Get(variable)
		t := c99.TypeOf(types.Default(stmt.Values[i].TypeAndValue().Type))
		if (ctype != "" && t != ctype) || !c99.StackAllocated(name) || !name.Defines() {
			return stmt.Location.Errorf("unsupported definition of variables with different types in for statement")
		}
		if i == 0 {
			fmt.Fprintf(c99, "%s ", t)
		} else {
			fmt.Fprintf(c99, ", ")
		}
		ctype = t
		fmt.Fprintf(c99, "%s = ", name.String)
		if err := c99.ExpressionAs(stmt.Values[i], types.Default(stmt.Values[i].TypeAndValue().Type)); err != nil {
			return err
		}
	}
	return nil
}

// namesIn returns the identifiers used in expr.
func namesIn(expr source.Expression) []string {
	var names []string
	if node := source.LocationOf(expr).Node; node != nil {
		ast.Inspect(node, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok {
				names = append(names, id.Name)
			}
			return true
		})
	}
	return names
}

// assignment assigns (or defines) a single variable.
func (c99 Target) assignment(stmt source.StatementAssignment) error {
	if stmt.Token.Value == token.DEFINE {
		var names []source.DefinedVariable
		for i, variable := range stmt.Variables {
			switch xyz.ValueOf(variable) {
			case source.Expressions.DefinedVariable:
				ident := source.Expressions.DefinedVariable.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(c99, "go_ignore(")
					if err := c99.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(c99, ")")
					break
				}
				if !ident.Defines() { // redeclared by :=, so assigned.
					return c99.assignment(source.StatementAssignment{Location: stmt.Location,
						Token:     source.WithLocation[token.Token]{Value: token.ASSIGN, SourceLocation: stmt.Token.SourceLocation},
						Variables: stmt.Variables, Values: stmt.Values})
				}
				names = append(names, ident)
			default:
				return stmt.Location.Errorf("unsupported variable assignment")
			}
		}
		c99.Tabs = -c99.Tabs
		for i, name := range names {
			var value xyz.Maybe[source.Expression]
			if len(stmt.Values) > 0 {
				value = xyz.New(stmt.Values[i])
			}
			if err := c99.VariableDefinition(source.VariableDefinition{
				Location: stmt.Location,
				Typed: source.Typed{
					TV: stmt.Values[i].TypeAndValue(),
				},
				Name:  name,
				Value: value,
			}); err != nil {
				return err
			}
		}
		return nil
	}
	for i, variable := range stmt.Variables {
		switch xyz.ValueOf(variable) {
		case source.Expressions.Star:
			star := source.Expressions.Star.Get(variable)
			fmt.Fprintf(c99, "go_pointer_set(")
			if err := c99.Expression(star.Value); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", %s, ", c99.TypeOf(star.Value.TypeAndValue().Type.(*types.Pointer).Elem()))
			if err := c99.ExpressionAs(stmt.Values[i], stmt.Variables[i].TypeAndValue().Type); err != nil {
				return err
			}
			fmt.Fprintf(c99, ")")
		case source.Expressions.Index:
			expr := source.Expressions.Index.Get(variable)
			if mtype, ok := expr.X.TypeAndValue().Type.(*types.Map); ok {
				symbol := fmt.Sprintf("go_map_%s_%s_set", c99.Mangle(mtype.Key()), c99.Mangle(mtype.Elem()))
				c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
					fmt.Fprintf(w, "static inline void %s(go_kv m, %s key, %s val) { go_map_set(m, &key, &val); }\n",
						symbol, c99.TypeOf(mtype.Key()), c99.TypeOf(mtype.Elem()))
					return nil
				})
				fmt.Fprintf(c99, "%s(", symbol)
				if err := c99.Expression(expr.X); err != nil {
					return err
				}
				fmt.Fprintf(c99, ", ")
				if err := c99.Expression(expr.Index); err != nil {
					return err
				}
				fmt.Fprintf(c99, ", ")
				if err := c99.ExpressionAs(stmt.Values[i], stmt.Variables[i].TypeAndValue().Type); err != nil {
					return err
				}
				fmt.Fprintf(c99, ")")
				return nil
			}
			fallthrough
		default:
			if xyz.ValueOf(variable) == source.Expressions.DefinedVariable {
				ident := source.Expressions.DefinedVariable.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(c99, "go_ignore(")
					if err := c99.ExpressionAs(stmt.Values[i], stmt.Variables[i].TypeAndValue().Type); err != nil {
						return err
					}
					fmt.Fprintf(c99, ")")
					break
				}
			}
			if err := c99.Expression(variable); err != nil {
				return err
			}
			if stmt.Token.Value == token.ADD_ASSIGN && isString(variable.TypeAndValue().Type) {
				fmt.Fprintf(c99, " = go_string_concat(%s, ", c99.toString(variable))
				if err := c99.ExpressionAs(stmt.Values[i], variable.TypeAndValue().Type); err != nil {
					return err
				}
				fmt.Fprintf(c99, ")")
				continue
			}
			if op := stmt.Token.Value; (op == token.QUO_ASSIGN || op == token.REM_ASSIGN) && isInteger(variable.TypeAndValue().Type) {
				div := token.QUO
				if op == token.REM_ASSIGN {
					div = token.REM
				}
				fmt.Fprintf(c99, " = %s(%s, ", c99.DivisionOf(div, variable.TypeAndValue().Type), c99.toString(variable))
				if err := c99.ExpressionAs(stmt.Values[i], variable.TypeAndValue().Type); err != nil {
					return err
				}
				fmt.Fprintf(c99, ")")
				continue
			}
			fmt.Fprintf(c99, " %s ", stmt.Token.Value)
			switch variable.TypeAndValue().Type.(type) {
			case *types.Interface:
				symbol := fmt.Sprintf("go_any__%s", c99.Mangle(stmt.Values[i].TypeAndValue().Type))
				c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
					fmt.Fprintf(w, "static inline go_vv %s(%[2]s v, const go_type* t) { return go_any_new(sizeof(%[2]s), &v, t); }\n",
						symbol, c99.TypeOf(stmt.Values[i].TypeAndValue().Type))
					return nil
				})
				fmt.Fprintf(c99, "%s(%s, %s)",
					symbol,
					c99.toString(stmt.Values[i]),
					c99.ReflectTypeOf(stmt.Values[i].TypeAndValue().Type))
			default:
				if err := c99.ExpressionAs(stmt.Values[i], stmt.Variables[i].TypeAndValue().Type); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
