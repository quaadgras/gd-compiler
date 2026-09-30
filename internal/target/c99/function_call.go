package c99

import (
	"fmt"
	"go/constant"
	"go/types"
	"io"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) StatementGo(stmt source.StatementGo) error {
	return c99.FunctionCall(stmt.Call)
}

func (c99 Target) FunctionCall(expr source.FunctionCall) error {
	if ok, err := c99.hoisted(expr.Location.Node, expr.TypeAndValue().Type, func(cc Target) error { return cc.FunctionCall(expr) }); ok {
		return err
	}
	function := expr.Function
	if xyz.ValueOf(function) == source.Expressions.Parenthesized {
		function = source.Expressions.Parenthesized.Get(function).X
	}
	if expr.Go {
		fmt.Fprintf(c99, "go_call(")
	}
	var receiver xyz.Maybe[source.Expression]
	var isVariable bool
	var isInterface bool
	var invoked bool // called through a func value, see [Target.invoke].
	// A deferred call (see [DeferredCall]) uses the values evaluated by the defer statement.
	deferred := c99.Deferred
	if xyz.ValueOf(function) != source.Expressions.BuiltinFunction {
		c99.Deferred = nil
	}
	if deferred == nil {
		deferred = &DeferredCall{}
	}
	switch xyz.ValueOf(function) {
	case source.Expressions.BuiltinFunction:
		call := source.Expressions.BuiltinFunction.Get(function)
		switch call.String {
		case "print":
			return c99.print(expr, false)
		case "println":
			return c99.println(expr)
		case "new":
			return c99.new(expr)
		case "make":
			return c99.make(expr)
		case "append":
			return c99.append(expr)
		case "copy":
			return c99.copy(expr)
		case "clear":
			return c99.clear(expr)
		case "len":
			return c99.len(expr)
		case "cap":
			return c99.cap(expr)
		case "panic":
			return c99.panic(expr)
		case "recover":
			if c99.Deferred != nil { // defer recover(): not called by a deferred function.
				fmt.Fprintf(c99, "go_recover(false)")
			} else {
				fmt.Fprintf(c99, "go_recover(go_can_recover)")
			}
			return nil
		default:
			return expr.Errorf("unsupported builtin function %s", call)
		}
	case source.Expressions.DefinedFunction:
		call := source.Expressions.DefinedFunction.Get(function)
		if expr.Go {
			fmt.Fprintf(c99, "go_make_func(%s), ", c99.FunctionName(call))
		} else {
			fmt.Fprint(c99, c99.FunctionName(call))
		}
		if !call.IsGlobal {
			isVariable = true
		}
	case source.Expressions.DefinedVariable:
		call := source.Expressions.DefinedVariable.Get(function)
		if expr.Go {
			fmt.Fprint(c99, call.String+", ")
		} else if err := c99.invoke(function, deferred.Callee); err != nil {
			return err
		} else {
			invoked = true
		}
		if !call.IsGlobal {
			isVariable = true
		}
	case source.Expressions.Selector:
		left := source.Expressions.Selector.Get(function)
		if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
			defined := source.Expressions.DefinedFunction.Get(left.Selection)
			if defined.Method {
				_, isInterface = left.X.TypeAndValue().Type.Underlying().(*types.Interface)
				if isInterface {
					x := deferred.Receiver
					if x == "" {
						x = c99.toString(left.X)
					}
					fmt.Fprintf(c99, `go_interface_methods(%s, %s)->%s(%[2]s.ptr.ptr`,
						c99.InterfaceTypeOf(left.X.TypeAndValue().Type), x, defined.String)
				} else {
					receiver = xyz.New(left.X)
					named, ok := types.Unalias(derefType(left.X.TypeAndValue().Type)).(*types.Named)
					if !ok {
						return left.Errorf("unsupported receiver type %s", left.X.TypeAndValue().Type)
					}
					fmt.Fprintf(c99, `%s_%s`, named.Obj().Name(), c99.FunctionName(defined))
				}
			} else {
				fmt.Fprint(c99, c99.FunctionName(defined))
			}
		} else { // a func value, such as a struct field.
			if err := c99.invoke(function, deferred.Callee); err != nil {
				return err
			}
			invoked = true
		}
	case source.Expressions.DefinedType:
		if ok, err := c99.conversion(expr, function.TypeAndValue().Type); ok || err != nil {
			return err
		}
		return expr.Opening.Errorf("unsupported conversion to %s", function.TypeAndValue().Type)
	case source.Expressions.Type:
		ctype := source.Expressions.Type.Get(function)
		if ok, err := c99.conversion(expr, ctype.TypeAndValue().Type); ok || err != nil {
			return err
		}
		switch typ := ctype.TypeAndValue().Type.Underlying().(type) {
		case *types.Interface:
			if typ.Empty() {
				value, err := c99.AnyOf(expr.Arguments[0])
				if err != nil {
					return expr.Errorf("%w", err)
				}
				fmt.Fprint(c99, value)
				return nil
			}
			dynamic := expr.Arguments[0].TypeAndValue().Type
			if _, ok := dynamic.Underlying().(*types.Interface); ok {
				return expr.Errorf("unsupported conversion between interfaces")
			}
			rtype, err := c99.reflectTypeOf(dynamic)
			if err != nil {
				return expr.Errorf("%w", err)
			}
			ctype2 := c99.TypeOf(dynamic)
			symbol := "go_interface_pack_" + identifier.ReplaceAllString(ctype2, "_")
			c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
				fmt.Fprintf(w, "static inline go_if %s(%s v, const go_type* t, void* vtable) { return go_interface_new(sizeof(v), &v, t, vtable); }\n",
					symbol, ctype2)
				return nil
			})
			fmt.Fprintf(c99, "%s(", symbol)
			if err := c99.Expression(expr.Arguments[0]); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", %s, &(%s){", rtype, c99.InterfaceTypeOf(ctype.TypeAndValue().Type))
			named, ok := types.Unalias(derefType(dynamic)).(*types.Named)
			if !ok {
				return expr.Errorf("unsupported conversion of %s to an interface", dynamic)
			}
			_, isPointer := dynamic.Underlying().(*types.Pointer)
			for i := range typ.NumMethods() {
				if i > 0 {
					fmt.Fprintf(c99, ", ")
				}
				method := typ.Method(i)
				prefix := "I_"
				if obj, _, _ := types.LookupFieldOrMethod(dynamic, true, method.Pkg(), method.Name()); isPointer && obj != nil {
					if _, ptrRecv := obj.Type().(*types.Signature).Recv().Type().Underlying().(*types.Pointer); !ptrRecv {
						prefix = "IP_" // the receiver is the value pointed to.
					}
				}
				fmt.Fprintf(c99, `.%s = %s%s_%s_go_%s_package`,
					method.Name(), prefix, named.Obj().Name(), method.Name(), named.Obj().Pkg().Name())
			}
			fmt.Fprintf(c99, "})")
			return nil
		default:
			fmt.Fprintf(c99, "@as(%s)", c99.Type(ctype))
			return nil
		}
	default:
		if _, ok := function.TypeAndValue().Type.Underlying().(*types.Signature); !ok || expr.Go {
			return expr.Opening.Errorf("unsupported call for function of type %T", xyz.ValueOf(function))
		}
		// a func value: function literal, call result, index expression...
		if err := c99.invoke(function, deferred.Callee); err != nil {
			return err
		}
		invoked = true
	}
	_ = isVariable
	ftype, ok := expr.Function.TypeAndValue().Type.Underlying().(*types.Signature)
	if !ok {
		return expr.Errorf("unsupported function type %T", expr.Function.TypeAndValue().Type)
	}
	if !isInterface {
		if expr.Go {
			results := c99.TupleTypeOf(ftype.Results())
			params := c99.TupleTypeOf(ftype.Params())
			symbol := "go_call_" + params + results
			c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
				fmt.Fprintf(w, "static int %s(void* ptr) {}\n", symbol)
				return nil
			})
			fmt.Fprintf(c99, "%s, %s, ", params, results)
		} else if !invoked {
			fmt.Fprintf(c99, "(")
		}
	}
	recv, hasReceiver := receiver.Get()
	if hasReceiver && deferred.Receiver != "" {
		fmt.Fprint(c99, deferred.Receiver)
	} else if hasReceiver {
		value, err := c99.receiverOf(function, recv)
		if err != nil {
			return err
		}
		fmt.Fprint(c99, value)
	}
	var variadic bool
	for i, arg := range expr.Arguments {
		if i > 0 || hasReceiver || isInterface || invoked {
			fmt.Fprintf(c99, ", ")
		}
		if !variadic && (ftype.Variadic() && i >= ftype.Params().Len()-1) {
			fmt.Fprintf(c99, "go_variadic(%d, %s, ", len(expr.Arguments)+1-ftype.Params().Len(), c99.TypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().Underlying().(*types.Slice).Elem()))
			variadic = true
		}
		var target types.Type
		if params := ftype.Params(); ftype.Variadic() && i >= params.Len()-1 {
			if slice, ok := params.At(params.Len() - 1).Type().Underlying().(*types.Slice); ok {
				target = slice.Elem()
			}
		} else if i < params.Len() {
			target = params.At(i).Type()
		}
		if i < len(deferred.Args) && deferred.Args[i] != "" {
			fmt.Fprint(c99, deferred.Args[i])
		} else if err := c99.ExpressionAs(arg, target); err != nil {
			return err
		}
	}
	if ftype.Variadic() {
		if variadic {
			fmt.Fprintf(c99, ")")
		} else {
			if len(expr.Arguments) > 0 || hasReceiver || isInterface || invoked {
				fmt.Fprintf(c99, ", ")
			}
			fmt.Fprintf(c99, "(go_ll){0}")
		}
	}
	fmt.Fprintf(c99, ")")
	return nil
}

// invoke starts a call through the func value fn (or the C expression value, when set), the
// arguments follow.
func (c99 Target) invoke(fn source.Expression, value string) error {
	sig, ok := fn.TypeAndValue().Type.Underlying().(*types.Signature)
	if !ok {
		return fmt.Errorf("unsupported call of %s", fn.TypeAndValue().Type)
	}
	invoker, err := c99.InvokerOf(sig)
	if err != nil {
		return err
	}
	fmt.Fprintf(c99, "%s(", invoker)
	if value != "" {
		fmt.Fprint(c99, value)
		return nil
	}
	return c99.Expression(fn)
}

// conversion writes the conversion of expr's argument to type t, reporting whether it
// is one of the conversions it supports: of nil, of constants and between numeric types.
func (c99 Target) conversion(expr source.FunctionCall, t types.Type) (bool, error) {
	if len(expr.Arguments) != 1 {
		return false, nil
	}
	arg := expr.Arguments[0]
	if isNil(arg) { // T(nil)
		return true, c99.ExpressionAs(arg, t)
	}
	if tv := expr.TypeAndValue(); tv.Value != nil && tv.Value.Kind() != constant.Complex {
		return true, c99.ConstantValue(tv.Value, false)
	}
	if isString(t) && isString(arg.TypeAndValue().Type) {
		return true, c99.Expression(arg)
	}
	if helper := stringConversion(t, arg.TypeAndValue().Type); helper != "" {
		fmt.Fprintf(c99, "%s(", helper)
		if isInteger(arg.TypeAndValue().Type) {
			fmt.Fprintf(c99, "(go_i8)")
		}
		if err := c99.Expression(arg); err != nil {
			return true, err
		}
		fmt.Fprintf(c99, ")")
		return true, nil
	}
	if isNumeric(t) && isNumeric(arg.TypeAndValue().Type) {
		fmt.Fprintf(c99, "((%s)(", c99.TypeOf(t))
		if err := c99.Expression(arg); err != nil {
			return true, err
		}
		fmt.Fprintf(c99, "))")
		return true, nil
	}
	return false, nil
}

// stringConversion returns the runtime function for a conversion from a value of type
// from to type to, involving strings, or "" if it is not one.
func stringConversion(to, from types.Type) string {
	elem := func(t types.Type) types.BasicKind {
		if slice, ok := t.Underlying().(*types.Slice); ok {
			if basic, ok := slice.Elem().Underlying().(*types.Basic); ok {
				return basic.Kind()
			}
		}
		return types.Invalid
	}
	switch {
	case isString(to) && isString(from):
		return "" // the same C type.
	case isString(to) && isInteger(from):
		return "go_string_from_rune"
	case isString(to) && elem(from) == types.Byte:
		return "go_string_from_bytes"
	case isString(to) && elem(from) == types.Rune:
		return "go_string_from_runes"
	case isString(from) && elem(to) == types.Byte:
		return "go_bytes_from_string"
	case isString(from) && elem(to) == types.Rune:
		return "go_runes_from_string"
	}
	return ""
}

// isNumeric reports whether t is an integer or floating-point type (conversions between
// them are C casts).
func isNumeric(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsInteger|types.IsFloat) != 0
}

// receiverOf returns the receiver of the method call of method (a selector), adjusted for
// the method's receiver type, as Go takes the address of addressable values for methods
// with pointer receivers (x.M() is (&x).M()), and dereferences pointers for methods with
// value receivers (p.M() is (*p).M()).
func (c99 Target) receiverOf(method, x source.Expression) (string, error) {
	value := c99.toString(x)
	sel := source.Expressions.Selector.Get(method)
	fn := source.Expressions.DefinedFunction.Get(sel.Selection)
	obj, ok := fn.Unique.(*types.Func)
	if !ok {
		return value, nil
	}
	_, wantPointer := obj.Type().(*types.Signature).Recv().Type().Underlying().(*types.Pointer)
	_, isPointer := x.TypeAndValue().Type.Underlying().(*types.Pointer)
	switch {
	case wantPointer && !isPointer:
		return fmt.Sprintf("((go_pt){ .ptr = &(%s) })", value), nil
	case !wantPointer && isPointer:
		return fmt.Sprintf("go_pointer_get(%s, %s)", value, c99.TypeOf(derefType(x.TypeAndValue().Type))), nil
	}
	return value, nil
}
