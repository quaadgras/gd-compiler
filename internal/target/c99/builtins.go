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
		case typ.Kind() == types.UnsafePointer:
			return fmt.Sprintf("go_print_pointer((go_up)(%s).ptr)", value), nil
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
	if !expr.Arguments[0].TypeAndValue().IsType() { // new(v), a copy of a value (Go 1.26)
		elem := expr.TypeAndValue().Type.Underlying().(*types.Pointer).Elem()
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ExpressionAs(expr.Arguments[0], elem); err != nil {
			return err
		}
		fmt.Fprintf(c99, "((go_pt){ %s(%s) })", c99.BoxOf(elem), buf.String())
		return nil
	}
	if zeroSize(expr.Arguments[0].TypeAndValue().Type) { // (all zero-size values have the same address, as in Go)
		fmt.Fprintf(c99, "((go_pt){ go_zerobase })")
		return nil
	}
	fmt.Fprintf(c99, "go_pointer_new(%[1]s)", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (c99 Target) make(expr source.FunctionCall) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.Underlying().(type) {
	case *types.Slice:
		switch len(expr.Arguments) {
		case 2, 3:
		default:
			return expr.Errorf("make expects two or three arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(c99, "go_slice_make_sized(%s, ", c99.elemSize(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem()))
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
		fmt.Fprintf(c99, "go_chan(sizeof(%s), ", c99.TypeOf(typ.Elem()))
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
		return expr.Errorf("unsupported make of %s", expr.Arguments[0].TypeAndValue().Type)
	}
}

// append appends values to a slice (one at a time), a slice to a slice (s, t...), or the
// bytes of a string to a byte slice (b, str...).
func (c99 Target) append(expr source.FunctionCall) error {
	slice := expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice)
	value := c99.toString(expr.Arguments[0])
	if len(expr.Arguments) == 2 && expr.Ellipsis.Open.IsValid() {
		if isString(expr.Arguments[1].TypeAndValue().Type) {
			fmt.Fprintf(c99, "go_append_string(%s, %s)", value, c99.toString(expr.Arguments[1]))
		} else {
			fmt.Fprintf(c99, "go_append_slice(%s, %s, %s)", value, c99.elemSize(slice.Elem()), c99.toString(expr.Arguments[1]))
		}
		return nil
	}
	elemType := c99.TypeOf(slice.Elem())
	symbol := "go_append_" + identifier.ReplaceAllString(elemType, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline go_ll %s(go_ll s, %s v) { return go_append(s, %s, &v); }\n", symbol, elemType, c99.elemSize(slice.Elem()))
		return nil
	})
	var values []string
	for _, arg := range expr.Arguments[1:] {
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ExpressionAs(arg, slice.Elem()); err != nil {
			return err
		}
		values = append(values, buf.String())
	}
	switch len(values) {
	case 0:
		fmt.Fprint(c99, value)
	case 1:
		fmt.Fprintf(c99, "%s(%s, %s)", symbol, value, values[0])
	default: // the values are all evaluated before any is appended (they may read s).
		fmt.Fprintf(c99, "go_append_slice(%[1]s, %[5]s, go_slice_literal(%[3]d, %[2]s, %[4]s))",
			value, elemType, len(values), strings.Join(values, ", "), c99.elemSize(slice.Elem()))
	}
	return nil
}

func (c99 Target) copy(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	if isString(expr.Arguments[1].TypeAndValue().Type) { // copy(bytes, string)
		fmt.Fprintf(c99, "go_copy(1, %s, go_bytes_of_string(%s))", c99.toString(expr.Arguments[0]), c99.toString(expr.Arguments[1]))
		return nil
	}
	fmt.Fprintf(c99, "go_copy(%s, ", c99.elemSize(expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Slice).Elem()))
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
	switch typ := expr.Arguments[0].TypeAndValue().Type.Underlying().(type) {
	case *types.Map:
		fmt.Fprintf(c99, "go_map_clear(%s)", c99.toString(expr.Arguments[0]))
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice_clear(%s, %s)", c99.toString(expr.Arguments[0]), c99.elemSize(typ.Elem()))
	default:
		return expr.Errorf("unsupported clear of %s", expr.Arguments[0].TypeAndValue().Type)
	}
	return nil
}

func (c99 Target) len(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("len expects exactly one argument, got %d", len(expr.Arguments))
	}
	if tv := expr.TypeAndValue(); tv.Value != nil { // arrays, and constant strings.
		return c99.Constant(tv)
	}
	arg := expr.Arguments[0]
	if n, ok := arrayLength(arg.TypeAndValue().Type); ok { // (with the operand's side effects)
		fmt.Fprintf(c99, "((go_ii)((void)(%s), %d))", c99.toString(arg), n)
		return nil
	}
	switch typ := arg.TypeAndValue().Type.Underlying().(type) {
	case *types.Basic:
		fmt.Fprintf(c99, "go_string_len(")
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice_len(")
	case *types.Map:
		fmt.Fprintf(c99, "go_map_len(")
	case *types.Chan:
		fmt.Fprintf(c99, "go_chan_len(")
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
	if tv := expr.TypeAndValue(); tv.Value != nil { // arrays.
		return c99.Constant(tv)
	}
	if n, ok := arrayLength(expr.Arguments[0].TypeAndValue().Type); ok {
		fmt.Fprintf(c99, "((go_ii)((void)(%s), %d))", c99.toString(expr.Arguments[0]), n)
		return nil
	}
	switch expr.Arguments[0].TypeAndValue().Type.Underlying().(type) {
	case *types.Slice:
		fmt.Fprintf(c99, "(%s).cap", c99.toString(expr.Arguments[0]))
	case *types.Chan:
		fmt.Fprintf(c99, "go_chan_cap(%s)", c99.toString(expr.Arguments[0]))
	default:
		return expr.Errorf("unsupported cap of %s", expr.Arguments[0].TypeAndValue().Type)
	}
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

func (c99 Target) delete(expr source.FunctionCall) error {
	mtype, ok := expr.Arguments[0].TypeAndValue().Type.Underlying().(*types.Map)
	if !ok || len(expr.Arguments) != 2 {
		return expr.Errorf("unsupported delete")
	}
	var key strings.Builder
	cc := c99
	cc.Writer = &key
	if err := cc.ExpressionAs(expr.Arguments[1], mtype.Key()); err != nil {
		return err
	}
	symbol := "go_map_delete_" + identifier.ReplaceAllString(c99.TypeOf(mtype.Key()), "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline void %s(go_kv m, %s key) { go_map_delete(m, &key); }\n", symbol, c99.TypeOf(mtype.Key()))
		return nil
	})
	fmt.Fprintf(c99, "%s(%s, %s)", symbol, c99.toString(expr.Arguments[0]), key.String())
	return nil
}

// minmax implements min and max: for floats, NaN wins, and -0 is less than +0.
func (c99 Target) minmax(expr source.FunctionCall, name string) error {
	if tv := expr.TypeAndValue(); tv.Value != nil {
		return c99.Constant(tv)
	}
	t := expr.TypeAndValue().Type
	ctype := c99.TypeOf(t)
	symbol := "go_" + name + "_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		op := "<"
		if name == "max" {
			op = ">"
		}
		switch {
		case isString(t):
			fmt.Fprintf(w, "static inline go_ss %s(go_ss a, go_ss b) { return go_string_cmp(b, a) %s 0 ? b : a; }\n", symbol, op)
		case isFloat(t):
			zero := "signbit(b) ? b : a" // min(-0, +0) is -0
			if name == "max" {
				zero = "signbit(b) ? a : b"
			}
			fmt.Fprintf(w, "static inline %[1]s %[2]s(%[1]s a, %[1]s b) { if (a != a) return a; if (b != b) return b; if (a == b) return %[3]s; return b %[4]s a ? b : a; }\n",
				ctype, symbol, zero, op)
		default:
			fmt.Fprintf(w, "static inline %[1]s %[2]s(%[1]s a, %[1]s b) { return b %[3]s a ? b : a; }\n", ctype, symbol, op)
		}
		return nil
	})
	value := ""
	for i, arg := range expr.Arguments {
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ExpressionAs(arg, t); err != nil {
			return err
		}
		if i == 0 {
			value = buf.String()
		} else {
			value = fmt.Sprintf("%s(%s, %s)", symbol, value, buf.String())
		}
	}
	fmt.Fprint(c99, value)
	return nil
}

func isFloat(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsFloat != 0
}

// arrayLength returns the length of t, an array or a pointer to an array.
func arrayLength(t types.Type) (int64, bool) {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	if a, ok := t.Underlying().(*types.Array); ok {
		return a.Len(), true
	}
	return 0, false
}
