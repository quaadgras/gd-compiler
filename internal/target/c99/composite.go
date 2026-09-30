package c99

import (
	"fmt"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// DataComposite writes a composite literal. The type of a literal may be elided (within
// another composite literal), and when that type is a pointer, it's &T{...}.
func (c99 Target) DataComposite(data source.DataComposite) error {
	ctype := data.TypeAndValue().Type
	if ctype == nil {
		return data.Errorf("composite literal missing type")
	}
	if pointer, ok := ctype.Underlying().(*types.Pointer); ok {
		fmt.Fprintf(c99, "go_new(sizeof(%s), &", c99.TypeOf(pointer.Elem()))
		defer fmt.Fprintf(c99, ")")
		ctype = pointer.Elem()
	}
	switch typ := ctype.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(c99, "(%s){{", c99.TypeOf(ctype))
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "0")
		}
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				pair := source.Expressions.KeyValue.Get(elem)
				fmt.Fprintf(c99, "[%s]=", c99.toString(pair.Key))
				elem = pair.Value
			}
			if err := c99.ExpressionAs(elem, typ.Elem()); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, "}}")
		return nil
	case *types.Slice:
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "go_slice_make(%s, 0, 0)", c99.TypeOf(typ.Elem()))
			return nil
		}
		fmt.Fprintf(c99, "go_slice_literal(%d, %s, ", len(data.Elements), c99.TypeOf(typ.Elem()))
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if err := c99.ExpressionAs(elem, typ.Elem()); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Map:
		entry := "go_map_entry_" + identifier.ReplaceAllString(c99.TypeOf(typ.Key())+"_"+c99.TypeOf(typ.Elem()), "_")
		c99.Requires(entry, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "typedef struct { %s key; %s val; } %s;\n", c99.TypeOf(typ.Key()), c99.TypeOf(typ.Elem()), entry)
			return nil
		})
		var entries strings.Builder
		cc := c99
		cc.Writer = &entries
		fmt.Fprintf(&entries, "(%s[]){", entry)
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(&entries, ", ")
			}
			pair := source.Expressions.KeyValue.Get(elem)
			fmt.Fprintf(&entries, "{ ")
			if err := cc.ExpressionAs(pair.Key, typ.Key()); err != nil {
				return err
			}
			fmt.Fprintf(&entries, ", ")
			if err := cc.ExpressionAs(pair.Value, typ.Elem()); err != nil {
				return err
			}
			fmt.Fprintf(&entries, " }")
		}
		fmt.Fprintf(&entries, "}")
		if err := c99.MakeMap(typ, fmt.Sprint(len(data.Elements)), entries.String(), entry, len(data.Elements)); err != nil {
			return data.Errorf("%w", err)
		}
		return nil
	case *types.Struct:
		fmt.Fprintf(c99, "(%s){", c99.TypeOf(ctype))
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "0") // C has no empty initializers before C23.
		}
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			switch xyz.ValueOf(elem) {
			case source.Expressions.KeyValue:
				pair := source.Expressions.KeyValue.Get(elem)
				name := c99.toString(pair.Key)
				var ftype types.Type
				for f := range typ.Fields() {
					if f.Name() == name {
						ftype = f.Type()
					}
				}
				fmt.Fprintf(c99, ".%s=", name)
				if err := c99.ExpressionAs(pair.Value, ftype); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				fmt.Fprintf(c99, ".%s = ", fieldName(field, i))
				if err := c99.ExpressionAs(elem, field.Type()); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(c99, "}")
		return nil
	default:
		return data.Errorf("unexpected composite type: " + typ.String())
	}
}
