package c

import (
	"fmt"
	"go/types"
	"io"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) DataComposite(data source.DataComposite) error {
	dtype, ok := data.Type.Get()
	if !ok {
		return data.Errorf("composite literal missing type")
	}
	switch typ := dtype.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(c99, "(%s)", c99.Type(dtype))
		fmt.Fprintf(c99, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if err := c99.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice_literal(%d, %s, ", len(data.Elements), c99.TypeOf(typ.Elem()))
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if err := c99.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Map:
		ktype := c99.TypeOf(typ.Key())
		vtype := c99.TypeOf(typ.Elem())
		fmt.Fprintf(c99, "go_map_literal(%s, %s, %d, ", ktype, vtype, len(data.Elements))
		symbol := fmt.Sprintf("go_map_entry__%s__%s", c99.Mangle(c99.TypeOf(typ.Key())), c99.Mangle(c99.TypeOf(typ.Elem())))
		c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
			fmt.Fprintf(w, "typedef struct { %s key; %s val; } %s;\n",
				c99.TypeOf(typ.Key()), c99.TypeOf(typ.Elem()), symbol)
			return nil
		})
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			pair := source.Expressions.KeyValue.Get(elem)
			fmt.Fprintf(c99, "(%s){", symbol)
			if err := c99.Compile(pair.Key); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", ")
			if err := c99.Compile(pair.Value); err != nil {
				return err
			}
			fmt.Fprintf(c99, "}")
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Struct:
		fmt.Fprintf(c99, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			switch xyz.ValueOf(elem) {
			case source.Expressions.KeyValue:
				if err := c99.Compile(elem); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				fmt.Fprintf(c99, ".%s = ", field.Name())
				if err := c99.Compile(elem); err != nil {
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
