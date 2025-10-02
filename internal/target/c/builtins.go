package c

import (
	"fmt"
	"go/types"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) println(expr source.FunctionCall) error {
	fmt.Fprintf(cc, "go_print(")
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
	fmt.Fprintf(cc, "%q", format)
	for _, arg := range expr.Arguments {
		fmt.Fprintf(cc, ", ")
		if err := cc.Expression(arg); err != nil {
			return err
		}
	}
	fmt.Fprintf(cc, ")")
	return nil
}

func (cc Target) new(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return expr.Errorf("new expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(cc, "(%s*)go_new(sizeof(%[1]s))", cc.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (cc Target) make(expr source.FunctionCall) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		switch len(expr.Arguments) {
		case 2, 3:
		default:
			return expr.Errorf("make expects two or three arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(cc, "go_slice_make(sizeof(%s),",
			cc.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
		if err := cc.Expression(expr.Arguments[1]); err != nil {
			return err
		}
		fmt.Fprintf(cc, ",")
		if len(expr.Arguments) == 3 {
			if err := cc.Expression(expr.Arguments[2]); err != nil {
				return err
			}
		} else {
			if err := cc.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		}
		fmt.Fprintf(cc, ")")
		return nil
	case *types.Chan:
		switch len(expr.Arguments) {
		case 1, 2:
		default:
			return expr.Errorf("make expects one or two arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(cc, "go.chan(%s).make(goto,", cc.TypeOf(typ.Elem()))
		if len(expr.Arguments) == 2 {
			if err := cc.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cc, "0")
		}
		fmt.Fprintf(cc, ")")
		return nil
	case *types.Map:
		if len(expr.Arguments) != 1 {
			return expr.Errorf("make expects exactly one argument, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(cc, "go_map_make(sizeof(%s), sizeof(%s), go_hash_%[2]s, go_same_%[1]s, 0)", cc.TypeOf(typ.Key()), cc.TypeOf(typ.Elem()))
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (cc Target) append(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return expr.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	elemType := cc.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem())
	fmt.Fprintf(cc, "go_append(sizeof(%s), ", elemType)
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ", 1, &(%s){", elemType)
	if err := cc.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(cc, "})")
	return nil
}

func (cc Target) copy(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(cc, "go_slice_copy(sizeof(%s),", cc.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ", ")
	if err := cc.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ")")
	return nil
}

func (cc Target) clear(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("clear expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(cc, "go_slice_clear(")
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ")")
	return nil
}

func (cc Target) len(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("len expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ".len()")
	return nil
}

func (cc Target) cap(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("cap expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ".cap()")
	return nil
}

func (cc Target) panic(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("panic expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(cc, "@panic(")
	if err := cc.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(cc, ")")
	return nil
}
