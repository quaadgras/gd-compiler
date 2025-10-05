package c

import (
	"fmt"
	"go/types"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) println(expr source.FunctionCall) error {
	fmt.Fprintf(c99, "go_print(")
	var format string
	for i, arg := range expr.Arguments {
		if i > 0 {
			format += " "
		}
		switch rtype := arg.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch rtype.Kind() {
			case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
				format += "%d"
			case types.Float64, types.Float32:
				format += "%e"
			case types.String:
				format += "%s"
			case types.Bool:
				format += "%b"
			default:
				return expr.Location.Errorf("unsupported type %s", rtype)
			}
		default:
			return fmt.Errorf("unsupported type %T", rtype)
		}
	}
	format += "\n"
	fmt.Fprintf(c99, "%q", format)
	for _, arg := range expr.Arguments {
		fmt.Fprintf(c99, ", ")
		if err := c99.Expression(arg); err != nil {
			return err
		}
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) new(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return expr.Errorf("new expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(c99, "go_pointer_new(%[1]s)", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (c99 Target) make(expr source.FunctionCall) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		switch len(expr.Arguments) {
		case 2, 3:
		default:
			return expr.Errorf("make expects two or three arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(c99, "go_slice_make(%s, ",
			c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
		if err := c99.Expression(expr.Arguments[1]); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", ")
		if len(expr.Arguments) == 3 {
			if err := c99.Expression(expr.Arguments[2]); err != nil {
				return err
			}
		} else {
			if err := c99.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Chan:
		switch len(expr.Arguments) {
		case 1, 2:
		default:
			return expr.Errorf("make expects one or two arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(c99, "go.chan(%s).make(goto,", c99.TypeOf(typ.Elem()))
		if len(expr.Arguments) == 2 {
			if err := c99.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(c99, "0")
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Map:
		if len(expr.Arguments) != 1 {
			return expr.Errorf("make expects exactly one argument, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(c99, "go_map_make(%s, %s, 0)", c99.TypeOf(typ.Key()), c99.TypeOf(typ.Elem()))
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (c99 Target) append(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return expr.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	elemType := c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem())
	symbol := fmt.Sprintf("go_append__%s", c99.Mangle(elemType))
	c99.Requires(symbol, func() {
		fmt.Fprintf(c99.Prelude, "static inline go_slice %s(go_slice s, %s v) { return go_append(s, sizeof(%s), &v); }\n", symbol, elemType, elemType)
	})
	fmt.Fprintf(c99, "%s(", symbol)
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ", ")
	if err := c99.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) copy(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(c99, "go_slice_copy(%s, ", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ", ")
	if err := c99.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) clear(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("clear expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(c99, "go_slice_clear(")
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) len(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("len expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(c99, "go_slice_len(")
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) cap(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("cap expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ".cap()")
	return nil
}

func (c99 Target) panic(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("panic expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(c99, "@panic(")
	if err := c99.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}
