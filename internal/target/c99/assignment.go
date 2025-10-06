package c99

import (
	"fmt"
	"go/token"
	"go/types"
	"io"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) StatementAssignment(stmt source.StatementAssignment) error {
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
			if err := c99.Expression(stmt.Values[i]); err != nil {
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
				if err := c99.Expression(stmt.Values[i]); err != nil {
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
					if err := c99.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(c99, ")")
					break
				}
			}
			if err := c99.Expression(variable); err != nil {
				return err
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
				if err := c99.Expression(stmt.Values[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
