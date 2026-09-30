package c99

import (
	"fmt"
	"go/types"
	"hash/fnv"
	"io"
	"regexp"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// StatementGo starts a goroutine, that calls the function with the function value,
// receiver and arguments evaluated by the go statement, like a deferred call.
func (c99 Target) StatementGo(stmt source.StatementGo) error {
	closure, err := c99.callClosure(stmt.Call, "go")
	if err != nil {
		return stmt.Location.Errorf("%w", err)
	}
	fmt.Fprintf(c99, "go_start(%s)", closure)
	return nil
}

func (c99 Target) FunctionCall(expr source.FunctionCall) error {
	if ok, err := c99.hoisted(expr.Location.Node, expr.TypeAndValue().Type, func(cc Target) error { return cc.FunctionCall(expr) }); ok {
		return err
	}
	if tv := expr.TypeAndValue(); tv.Value != nil {
		return c99.Constant(tv) // unsafe.Sizeof, len of arrays, conversions...
	}
	if c99.Deferred == nil {
		var err error
		if expr, err = c99.spread(expr); err != nil {
			return err
		}
	}
	function := expr.Function
	if xyz.ValueOf(function) == source.Expressions.Parenthesized {
		function = source.Expressions.Parenthesized.Get(function).X
	}
	if xyz.ValueOf(function) == source.Expressions.Selector {
		sel := source.Expressions.Selector.Get(function)
		switch xyz.ValueOf(sel.Selection) {
		case source.Expressions.BuiltinFunction:
			return c99.unsafeBuiltin(expr, source.Expressions.BuiltinFunction.Get(sel.Selection).String)
		case source.Expressions.DefinedType: // pkg.T(x)
			if ok, err := c99.conversion(expr, function.TypeAndValue().Type); ok || err != nil {
				return err
			}
			return expr.Errorf("unsupported conversion to %s", function.TypeAndValue().Type)
		}
	}
	switch xyz.ValueOf(function) { // f[T](...), the instance is recorded for f.
	case source.Expressions.Index:
		if x := source.Expressions.Index.Get(function).X; xyz.ValueOf(x) == source.Expressions.DefinedFunction {
			function = x
		}
	case source.Expressions.Indices:
		if x := source.Expressions.Indices.Get(function).X; xyz.ValueOf(x) == source.Expressions.DefinedFunction {
			function = x
		}
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
		case "close":
			fmt.Fprintf(c99, "go_close(%s)", c99.toString(expr.Arguments[0]))
			return nil
		case "delete":
			return c99.delete(expr)
		case "min", "max":
			return c99.minmax(expr, call.String)
		case "complex", "complex_": // escaped, see source.CIdent.
			fmt.Fprintf(c99, "((%s){ %s, %s })", c99.TypeOf(expr.TypeAndValue().Type), c99.toString(expr.Arguments[0]), c99.toString(expr.Arguments[1]))
			return nil
		case "real":
			fmt.Fprintf(c99, "(%s).f1", c99.toString(expr.Arguments[0]))
			return nil
		case "imag":
			fmt.Fprintf(c99, "(%s).f2", c99.toString(expr.Arguments[0]))
			return nil
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
		name, err := c99.FunctionInstance(call)
		if err != nil {
			return err
		}
		fmt.Fprint(c99, name)
		if !call.IsGlobal {
			isVariable = true
		}
	case source.Expressions.DefinedVariable:
		call := source.Expressions.DefinedVariable.Get(function)
		if err := c99.invoke(function, deferred.Callee); err != nil {
			return err
		}
		invoked = true
		if !call.IsGlobal {
			isVariable = true
		}
	case source.Expressions.Selector:
		left := source.Expressions.Selector.Get(function)
		if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction && left.X.TypeAndValue().IsType() {
			// T.M(x): a method expression, called as a func value.
			if err := c99.invoke(function, deferred.Callee); err != nil {
				return err
			}
			invoked = true
		} else if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
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
					var pkg *types.Package
					if fn, ok := defined.Unique.(*types.Func); ok {
						pkg = fn.Pkg()
					}
					recv, err := c99.receiverType(left.X.TypeAndValue().Type, pkg, defined.String)
					if err != nil {
						return left.Errorf("%w", err)
					}
					fmt.Fprint(c99, c99.methodCName(recv, defined.String))
				}
			} else {
				name, err := c99.FunctionInstance(defined) // pkg.F
				if err != nil {
					return err
				}
				fmt.Fprint(c99, name)
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
		switch ctype.TypeAndValue().Type.Underlying().(type) {
		case *types.Interface:
			value, err := c99.InterfaceOf(expr.Arguments[0], ctype.TypeAndValue().Type)
			if err != nil {
				return expr.Errorf("%w", err)
			}
			fmt.Fprint(c99, value)
			return nil
		default:
			return expr.Errorf("unsupported conversion from %s to %s", expr.Arguments[0].TypeAndValue().Type, ctype.TypeAndValue().Type)
		}
	default:
		if tv := function.TypeAndValue(); tv.IsType() { // T[A](x), (*T)(x)...
			if ok, err := c99.conversion(expr, tv.Type); ok || err != nil {
				return err
			}
			return expr.Errorf("unsupported conversion from %s to %s", expr.Arguments[0].TypeAndValue().Type, tv.Type)
		}
		if _, ok := function.TypeAndValue().Type.Underlying().(*types.Signature); !ok {
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
	if !isInterface && !invoked {
		fmt.Fprintf(c99, "(")
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
		if !variadic && ftype.Variadic() && i >= ftype.Params().Len()-1 && !expr.Ellipsis.Open.IsValid() {
			fmt.Fprintf(c99, "go_variadic(%d, %s, ", len(expr.Arguments)+1-ftype.Params().Len(), c99.TypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().Underlying().(*types.Slice).Elem()))
			variadic = true
		}
		var target types.Type
		if params := ftype.Params(); ftype.Variadic() && i >= params.Len()-1 && !expr.Ellipsis.Open.IsValid() {
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
	if ftype.Variadic() && !expr.Ellipsis.Open.IsValid() {
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
	if tv := expr.TypeAndValue(); tv.Value != nil {
		return true, c99.Constant(tv)
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
	from := arg.TypeAndValue().Type
	if isInterfaceType(t) && isInterfaceType(from) && !types.Identical(t.Underlying(), from.Underlying()) {
		fmt.Fprint(c99, c99.toInterface(asEmptyInterface(c99.toString(arg), from), t, false, "NULL"))
		return true, nil
	}
	if _, ok := t.Underlying().(*types.Interface); ok && !isInterfaceType(from) {
		value, err := c99.InterfaceOf(arg, t)
		if err != nil {
			return true, err
		}
		fmt.Fprint(c99, value)
		return true, nil
	}
	if c99.TypeOf(t) == c99.TypeOf(from) { // the same C type.
		return true, c99.Expression(arg)
	}
	if basic, ok := t.Underlying().(*types.Basic); ok && basic.Info()&types.IsComplex != 0 { // complex64 <-> complex128
		fmt.Fprintf(c99, "%s_convert(%s)", c99.TypeOf(t), c99.toString(arg))
		return true, nil
	}
	if types.IdenticalIgnoreTags(t.Underlying(), from.Underlying()) { // distinct C types.
		symbol := "go_convert_" + identifier.ReplaceAllString(c99.TypeOf(from)+"_to_"+c99.TypeOf(t), "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(%s v) { %[1]s r; memcpy(&r, &v, sizeof(r)); return r; }\n", c99.TypeOf(t), symbol, c99.TypeOf(from))
			return nil
		})
		fmt.Fprintf(c99, "%s(", symbol)
		if err := c99.Expression(arg); err != nil {
			return true, err
		}
		fmt.Fprintf(c99, ")")
		return true, nil
	}
	if isUnsafePointer(t) && isInteger(from) {
		fmt.Fprintf(c99, "((go_pt){ .ptr = (void*)(go_up)(%s) })", c99.toString(arg))
		return true, nil
	}
	if isInteger(t) && isUnsafePointer(from) {
		fmt.Fprintf(c99, "((%s)(go_up)(%s).ptr)", c99.TypeOf(t), c99.toString(arg))
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
	return c99.methodReceiver(source.Expressions.DefinedFunction.Get(source.Expressions.Selector.Get(method).Selection), x)
}

// methodReceiver is [Target.receiverOf] for the method fn.
func (c99 Target) methodReceiver(fn source.DefinedFunction, x source.Expression) (string, error) {
	value := c99.toString(x)
	obj, ok := fn.Unique.(*types.Func)
	if !ok {
		return value, nil
	}
	if _, iface := obj.Type().(*types.Signature).Recv().Type().Underlying().(*types.Interface); iface {
		// a method of a type parameter's constraint, in an instance: the concrete method.
		if concrete, _, _ := types.LookupFieldOrMethod(x.TypeAndValue().Type, true, obj.Pkg(), obj.Name()); concrete != nil {
			if m, ok := concrete.(*types.Func); ok {
				obj = m
			}
		}
	}
	wantPointer := pointerReceiver(x.TypeAndValue().Type, obj)
	_, isPointer := x.TypeAndValue().Type.Underlying().(*types.Pointer)
	switch {
	case wantPointer && !isPointer:
		return fmt.Sprintf("((go_pt){ .ptr = &(%s) })", value), nil
	case !wantPointer && isPointer:
		return fmt.Sprintf("go_pointer_get(%s, %s)", value, c99.TypeOf(derefType(x.TypeAndValue().Type))), nil
	}
	return value, nil
}

// InterfaceOf returns a C expression that converts expr (of a concrete type, or nil) to the
// interface type iface: its value, dynamic type, and table of methods.
func (c99 Target) InterfaceOf(expr source.Expression, iface types.Type) (string, error) {
	typ := iface.Underlying().(*types.Interface)
	if typ.Empty() {
		return c99.AnyOf(expr)
	}
	if isNil(expr) {
		return fmt.Sprintf("((%s){0})", c99.TypeOf(iface)), nil
	}
	dynamic := expr.TypeAndValue().Type
	if _, ok := dynamic.Underlying().(*types.Interface); ok {
		return "", fmt.Errorf("unsupported conversion between interfaces")
	}
	rtype, err := c99.reflectTypeOf(dynamic)
	if err != nil {
		return "", err
	}
	ctype := c99.TypeOf(dynamic)
	symbol := "go_interface_pack_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline go_if %s(%s v, const go_type* t, void* vtable) { return go_interface_new(sizeof(v), &v, t, vtable); }\n",
			symbol, ctype)
		return nil
	})
	_, isPointer := dynamic.Underlying().(*types.Pointer)
	var methods []string
	for i := range typ.NumMethods() {
		method := typ.Method(i)
		recv, err := c99.receiverType(dynamic, method.Pkg(), method.Name())
		if err != nil {
			return "", fmt.Errorf("unsupported conversion of %s to an interface: %w", dynamic, err)
		}
		prefix := "I_"
		if obj, _, _ := types.LookupFieldOrMethod(dynamic, true, method.Pkg(), method.Name()); isPointer && obj != nil {
			if !pointerReceiver(recv, obj.(*types.Func)) {
				prefix = "IP_" // the receiver is the value pointed to.
			}
		}
		methods = append(methods, fmt.Sprintf(".%s = %s%s", source.CIdent(method.Name()), prefix, c99.methodCName(recv, method.Name())))
	}
	// The table of methods is static, as interface values may outlive any function.
	hash := fnv.New64a()
	hash.Write([]byte(typeName(dynamic) + "|" + typeName(iface))) // by Go type: C types are shared.
	vtable := fmt.Sprintf("go_vtable_%x", hash.Sum64())
	c99.Requires(vtable, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static %s %s = {%s};\n", c99.InterfaceTypeOf(iface), vtable, strings.Join(methods, ", "))
		return nil
	})
	return fmt.Sprintf("%s(%s, %s, &%s)", symbol, c99.toString(expr), rtype, vtable), nil
}

func isUnsafePointer(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.UnsafePointer
}

// unsafeBuiltin writes a call of a function of package unsafe (which are builtins).
func (c99 Target) unsafeBuiltin(expr source.FunctionCall, name string) error {
	arg := func(i int) string { return c99.toString(expr.Arguments[i]) }
	switch name {
	case "Add":
		fmt.Fprintf(c99, "((go_pt){ .ptr = (char*)(%s).ptr + (go_ii)(%s) })", arg(0), arg(1))
	case "Slice":
		fmt.Fprintf(c99, "((go_ll){ .ptr = %s, .len = (go_ii)(%s), .cap = (go_ii)(%[2]s) })", arg(0), arg(1))
	case "SliceData":
		fmt.Fprintf(c99, "((%s).ptr)", arg(0))
	case "String":
		fmt.Fprintf(c99, "((go_ss){ .ptr = (const char*)(%s).ptr, .len = (go_ii)(%s) })", arg(0), arg(1))
	case "StringData":
		fmt.Fprintf(c99, "((go_pt){ .ptr = (void*)(%s).ptr })", arg(0))
	case "Sizeof": // not constant, in instances of generic functions.
		fmt.Fprintf(c99, "((go_up)sizeof(%s))", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	case "Alignof":
		fmt.Fprintf(c99, "((go_up)_Alignof(%s))", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	case "Offsetof":
		arg := expr.Arguments[0]
		for xyz.ValueOf(arg) == source.Expressions.Parenthesized {
			arg = source.Expressions.Parenthesized.Get(arg).X
		}
		if xyz.ValueOf(arg) != source.Expressions.Selector {
			return expr.Errorf("unsupported unsafe.Offsetof")
		}
		sel := source.Expressions.Selector.Get(arg)
		fields := append(append([]string{}, sel.Path...), c99.toString(sel.Selection))
		fmt.Fprintf(c99, "((go_up)offsetof(%s, %s))", c99.TypeOf(derefType(sel.X.TypeAndValue().Type)), strings.Join(fields, "."))
	default:
		return expr.Errorf("unsupported unsafe.%s", name)
	}
	return nil
}

func isInterfaceType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// temporary matches the C names of temporaries (and of variables).
var temporary = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// spread returns the call f(g()), where g has several results, as a call with those results
// as arguments (g() is a temporary, see [Order]).
func (c99 Target) spread(expr source.FunctionCall) (source.FunctionCall, error) {
	if len(expr.Arguments) != 1 {
		return expr, nil
	}
	tuple, ok := expr.Arguments[0].TypeAndValue().Type.(*types.Tuple)
	if !ok || tuple.Len() < 2 {
		return expr, nil
	}
	if c99.Order != nil { // the function is evaluated first: f()(g())
		_ = c99.toString(expr.Function)
	}
	value := c99.toString(expr.Arguments[0])
	if !temporary.MatchString(value) {
		return expr, expr.Errorf("unsupported call with the results of another call, outside of a statement")
	}
	var args []source.Expression
	for i := range tuple.Len() {
		args = append(args, source.Expressions.DefinedVariable.New(source.DefinedVariable{
			Typed:    source.Typed{TV: types.TypeAndValue{Type: tuple.At(i).Type()}},
			Location: expr.Location,
			String:   fmt.Sprintf("%s.r%d", value, i),
		}))
	}
	expr.Arguments = args
	return expr, nil
}
