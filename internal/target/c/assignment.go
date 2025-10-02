package c

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (cc Target) StatementAssignment(stmt source.StatementAssignment) error {
	if stmt.Token.Value == token.DEFINE {
		var names []source.DefinedVariable
		for i, variable := range stmt.Variables {
			switch xyz.ValueOf(variable) {
			case source.Expressions.DefinedVariable:
				ident := source.Expressions.DefinedVariable.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(cc, "go.use(")
					if err := cc.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(cc, ")")
					break
				}
				names = append(names, ident)
			default:
				return stmt.Location.Errorf("unsupported variable assignment")
			}
		}
		cc.Tabs = -cc.Tabs
		for i, name := range names {
			var value xyz.Maybe[source.Expression]
			if len(stmt.Values) > 0 {
				value = xyz.New(stmt.Values[i])
			}
			if err := cc.VariableDefinition(source.VariableDefinition{
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
			if err := cc.Expression(variable); err != nil {
				return err
			}
			fmt.Fprintf(cc, " = ")
			if err := cc.Expression(stmt.Values[i]); err != nil {
				return err
			}
		case source.Expressions.Index:
			expr := source.Expressions.Index.Get(variable)
			if mtype, ok := expr.X.TypeAndValue().Type.(*types.Map); ok {
				symbol := fmt.Sprintf("go_map_%s__%s_set", cc.Mangle(cc.TypeOf(mtype.Key())), cc.Mangle(cc.TypeOf(mtype.Elem())))
				cc.Requires(symbol, func() {
					fmt.Fprintf(cc.Prelude, "static inline void %s(go_map m, %s key, %s val) { go_map_set(m, &key, &val); }\n",
						symbol, cc.TypeOf(mtype.Key()), cc.TypeOf(mtype.Elem()))
				})
				fmt.Fprintf(cc, "%s(", symbol)
				if err := cc.Expression(expr.X); err != nil {
					return err
				}
				fmt.Fprintf(cc, ", ")
				if err := cc.Expression(expr.Index); err != nil {
					return err
				}
				fmt.Fprintf(cc, ", ")
				if err := cc.Expression(stmt.Values[i]); err != nil {
					return err
				}
				fmt.Fprintf(cc, ")")
				return nil
			}
			fallthrough
		default:
			if xyz.ValueOf(variable) == source.Expressions.DefinedVariable {
				ident := source.Expressions.DefinedVariable.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(cc, "go.use(")
					if err := cc.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(cc, ")")
					break
				}
			}
			if err := cc.Expression(variable); err != nil {
				return err
			}
			fmt.Fprintf(cc, " %s ", stmt.Token.Value)
			switch variable.TypeAndValue().Type.(type) {
			case *types.Interface:
				if strings.HasPrefix(cc.TypeOf(stmt.Values[i].TypeAndValue().Type), "go.pointer(") {
					fmt.Fprintf(cc, "go.any{.rtype=%s,.value=", cc.ReflectTypeOf(stmt.Values[i].TypeAndValue().Type))
					if err := cc.Expression(stmt.Values[i]); err != nil {
						return nil
					}
					fmt.Fprintf(cc, ".address}")
				} else {
					fmt.Fprintf(cc, "go.any.make(%s, goto, %s, ", cc.TypeOf(stmt.Values[i].TypeAndValue().Type), cc.ReflectTypeOf(stmt.Values[i].TypeAndValue().Type))
					if err := cc.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(cc, ")")
				}
			default:
				if err := cc.Expression(stmt.Values[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
