package c

import (
	"fmt"
	"go/types"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (cc Target) DataComposite(data source.DataComposite) error {
	dtype, ok := data.Type.Get()
	if !ok {
		return data.Errorf("composite literal missing type")
	}
	fmt.Fprintf(cc, "%s", cc.Type(dtype))
	switch typ := dtype.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(cc, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			if err := cc.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(cc, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(cc, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(cc, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			if err := cc.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(cc, "})")
		return nil
	case *types.Map:
		fmt.Fprintf(cc, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(cc, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			pair := source.Expressions.KeyValue.Get(elem)
			fmt.Fprintf(cc, ".{")
			if err := cc.Compile(pair.Key); err != nil {
				return err
			}
			fmt.Fprintf(cc, ", ")
			if err := cc.Compile(pair.Value); err != nil {
				return err
			}
			fmt.Fprintf(cc, "}")
		}
		fmt.Fprintf(cc, "})")
		return nil
	case *types.Struct:
		fmt.Fprintf(cc, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			switch xyz.ValueOf(elem) {
			case source.Expressions.KeyValue:
				if err := cc.Compile(elem); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				fmt.Fprintf(cc, ".%s = ", field.Name())
				if err := cc.Compile(elem); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(cc, "}")
		return nil
	default:
		return data.Errorf("unexpected composite type: " + typ.String())
	}
}
