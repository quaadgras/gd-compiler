package c99

import (
	"fmt"
	"go/constant"
	"go/types"
	"io"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

func (c99 Target) println(expr source.FunctionCall) error { return c99.print(expr, true) }

// print implements the print and println builtins, with the same output as gc (see
// runtime/print.go). Arguments are evaluated and printed from left to right.
func (c99 Target) print(expr source.FunctionCall, newline bool) error {
	var calls []string
	for i, arg := range expr.Arguments {
		if i > 0 && newline {
			calls = append(calls, `go_print_cstring(" ")`)
		}
		call, err := c99.printArgument(expr, i, arg)
		if err != nil {
			return err
		}
		calls = append(calls, call)
	}
	if newline {
		calls = append(calls, `go_print_cstring("\n")`)
	}
	if len(calls) == 0 {
		fmt.Fprintf(c99, "((void)0)")
		return nil
	}
	fmt.Fprintf(c99, "(%s)", strings.Join(calls, ", "))
	return nil
}

func (c99 Target) printArgument(call source.FunctionCall, index int, arg source.Expression) (string, error) {
	tv := arg.TypeAndValue()
	var value string
	if tv.Value != nil && tv.Value.Kind() != constant.Complex {
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ConstantValue(tv.Value, false); err != nil {
			return "", err
		}
		value = buf.String()
	} else if tv.Value != nil {
		re, _ := constant.Float64Val(constant.Real(tv.Value))
		im, _ := constant.Float64Val(constant.Imag(tv.Value))
		ctor := "go_complex128"
		if basic, ok := tv.Type.Underlying().(*types.Basic); ok && basic.Kind() == types.Complex64 {
			ctor = "go_complex64"
		}
		value = fmt.Sprintf("%s(%s, %s)", ctor,
			strconv.FormatFloat(re, 'g', -1, 64), strconv.FormatFloat(im, 'g', -1, 64))
	} else if c99.Deferred != nil && index < len(c99.Deferred.Args) && c99.Deferred.Args[index] != "" {
		value = c99.Deferred.Args[index]
	} else {
		value = c99.toString(arg)
	}
	switch typ := tv.Type.Underlying().(type) {
	case *types.Basic:
		switch {
		case typ.Info()&types.IsBoolean != 0:
			return fmt.Sprintf("go_print_bool(%s)", value), nil
		case typ.Info()&types.IsUnsigned != 0:
			return fmt.Sprintf("go_print_uint((go_u8)(%s))", value), nil
		case typ.Info()&types.IsInteger != 0:
			return fmt.Sprintf("go_print_int((go_i8)(%s))", value), nil
		case typ.Kind() == types.Float32:
			return fmt.Sprintf("go_print_float32(%s)", value), nil
		case typ.Info()&types.IsFloat != 0:
			return fmt.Sprintf("go_print_float64(%s)", value), nil
		case typ.Kind() == types.Complex64:
			return fmt.Sprintf("go_print_complex64(%s)", value), nil
		case typ.Info()&types.IsComplex != 0:
			return fmt.Sprintf("go_print_complex128(%s)", value), nil
		case typ.Info()&types.IsString != 0:
			return fmt.Sprintf("go_print_string(%s)", value), nil
		case typ.Kind() == types.UntypedNil:
			return "go_print_pointer(0)", nil
		}
	case *types.Pointer, *types.Signature:
		return fmt.Sprintf("go_print_pointer((go_up)(%s).ptr)", value), nil
	case *types.Chan, *types.Map:
		return fmt.Sprintf("go_print_pointer((go_up)(%s))", value), nil
	case *types.Slice:
		return fmt.Sprintf("go_print_slice(%s)", value), nil
	case *types.Interface:
		return fmt.Sprintf("go_print_iface((go_up)(%[1]s).go_type, (go_up)(%[1]s).ptr.ptr)", value), nil
	}
	return "", call.Errorf("illegal type for print: %s", tv.Type)
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
			c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem()))
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
		fmt.Fprintf(c99, "go_chan_make(%s, ", c99.TypeOf(typ.Elem()))
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
		hint := "0"
		if len(expr.Arguments) == 2 {
			hint = "(go_ii)(" + c99.toString(expr.Arguments[1]) + ")"
		}
		if err := c99.MakeMap(typ, hint, "", "", 0); err != nil {
			return expr.Errorf("%w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (c99 Target) append(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return expr.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	elemType := c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem())
	symbol := fmt.Sprintf("go_append_%s", c99.Mangle(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem()))
	c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline go_ll %s(go_ll s, %s v) { return go_append(s, sizeof(%s), &v); }\n", symbol, elemType, elemType)
		return nil
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
	fmt.Fprintf(c99, "go_slice_copy(%s, ", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem()))
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
	if tv := expr.TypeAndValue(); tv.Value != nil { // arrays, and constant strings.
		return c99.ConstantValue(tv.Value, false)
	}
	arg := expr.Arguments[0]
	switch typ := arg.TypeAndValue().Type.Underlying().(type) {
	case *types.Basic:
		fmt.Fprintf(c99, "go_string_len(")
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice_len(")
	default:
		return expr.Errorf("unsupported len of %s", typ)
	}
	if err := c99.Expression(arg); err != nil {
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
	value := ""
	if c99.Deferred != nil && c99.Deferred.Args[0] != "" {
		value = c99.Deferred.Args[0]
	} else {
		var err error
		if value, err = c99.AnyOf(expr.Arguments[0]); err != nil {
			return expr.Errorf("%w", err)
		}
	}
	fmt.Fprintf(c99, "go_panic_any(%s)", value)
	return nil
}
