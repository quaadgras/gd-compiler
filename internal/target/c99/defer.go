package c99

import (
	"fmt"
	"go/constant"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// DeferredCall holds the C expressions for the parts of a deferred call that are evaluated
// by the defer statement (the func value, receiver and arguments), stored in the deferred
// call's environment. Empty strings are parts that are compiled again, when the call runs
// (constants, nil and names of functions).
type DeferredCall struct {
	Callee   string
	Receiver string
	Args     []string
}

// StatementDefer evaluates the function value, receiver and arguments of the deferred call
// into an environment, and pushes a closure that makes the call onto the function's frame,
// see [Closures].
func (c99 Target) StatementDefer(stmt source.StatementDefer) error {
	if !c99.Frame {
		return stmt.Location.Errorf("defer in a function without a frame")
	}
	call := stmt.Call
	symbol := fmt.Sprintf("go_deferred_%s_%d", c99.CurrentFunction, c99.Closures.count)
	c99.Closures.count++

	var ctypes, values []string
	store := func(ctype, value string) string {
		ctypes = append(ctypes, ctype)
		values = append(values, value)
		return fmt.Sprintf("go_e->v%d", len(values)-1)
	}
	deferred := &DeferredCall{Args: make([]string, len(call.Arguments))}
	function := call.Function
	for xyz.ValueOf(function) == source.Expressions.Parenthesized {
		function = source.Expressions.Parenthesized.Get(function).X
	}
	var sig *types.Signature
	switch xyz.ValueOf(function) {
	case source.Expressions.BuiltinFunction:
		name := source.Expressions.BuiltinFunction.Get(function).String
		switch name {
		case "print", "println", "panic", "recover":
		default:
			return stmt.Location.Errorf("unsupported defer of builtin %s", name)
		}
		for i, arg := range call.Arguments {
			if reevaluate(arg) {
				continue
			}
			if name == "panic" {
				value, err := c99.AnyOf(arg)
				if err != nil {
					return stmt.Location.Errorf("%w", err)
				}
				deferred.Args[i] = store("go_vv", value)
				continue
			}
			deferred.Args[i] = store(c99.TypeOf(types.Default(arg.TypeAndValue().Type)), c99.toString(arg))
		}
	default:
		var ok bool
		if sig, ok = function.TypeAndValue().Type.Underlying().(*types.Signature); !ok {
			return stmt.Location.Errorf("unsupported defer of %s", function.TypeAndValue().Type)
		}
		switch xyz.ValueOf(function) {
		case source.Expressions.DefinedFunction:
		case source.Expressions.Selector:
			left := source.Expressions.Selector.Get(function)
			if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
				if source.Expressions.DefinedFunction.Get(left.Selection).Method {
					deferred.Receiver = store(c99.TypeOf(left.X.TypeAndValue().Type), c99.toString(left.X))
				}
			} else {
				deferred.Callee = store("go_fn", c99.toString(function))
			}
		default:
			deferred.Callee = store("go_fn", c99.toString(function))
		}
		params := sig.Params()
		for i, arg := range call.Arguments {
			if reevaluate(arg) {
				continue
			}
			target := arg.TypeAndValue().Type
			if sig.Variadic() && i >= params.Len()-1 {
				target = params.At(params.Len() - 1).Type().(*types.Slice).Elem()
			} else if i < params.Len() {
				target = params.At(i).Type()
			}
			var buf strings.Builder
			cc := c99
			cc.Writer = &buf
			if err := cc.ExpressionAs(arg, target); err != nil {
				return err
			}
			deferred.Args[i] = store(c99.TypeOf(target), buf.String())
		}
	}

	if err := c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		var body strings.Builder
		cc := c99
		cc.Writer = &body
		cc.Tabs = 0
		cc.Deferred = deferred
		if err := cc.FunctionCall(call); err != nil {
			return err
		}
		if len(ctypes) > 0 {
			fmt.Fprintf(w, "typedef struct { ")
			for i, ctype := range ctypes {
				fmt.Fprintf(w, "%s v%d; ", ctype, i)
			}
			fmt.Fprintf(w, "} go_env_%s;\n", symbol)
		}
		// no go_split, so that a function it calls receives the token that allows recover.
		fmt.Fprintf(w, "static void %s(void* go_env) { ", symbol)
		if len(ctypes) > 0 {
			fmt.Fprintf(w, "go_env_%s* go_e = go_env; ", symbol)
		} else {
			fmt.Fprintf(w, "(void)go_env; ")
		}
		fmt.Fprintf(w, "%s; }\n", body.String())
		return nil
	}); err != nil {
		return err
	}
	env := "NULL"
	if len(ctypes) > 0 {
		env = fmt.Sprintf("go_new(sizeof(go_env_%s), &(go_env_%[1]s){ %s }).ptr", symbol, strings.Join(values, ", "))
	}
	fmt.Fprintf(c99, "go_defer_push(go_fr, go_make_closure(%s, %s))", symbol, env)
	return nil
}

// reevaluate reports whether arg can be compiled again when a deferred call runs, instead
// of being stored by the defer statement.
func reevaluate(arg source.Expression) bool {
	return arg.TypeAndValue().Value != nil || isNil(arg)
}

// StatementReturn sets the result variables, runs the deferred calls, and returns.
func (c99 Target) StatementReturn(stmt source.StatementReturn) error {
	if len(stmt.Results) > 1 {
		return stmt.Location.Errorf("multiple return values are not supported")
	}
	if !c99.Frame && (len(stmt.Results) > 0 || len(c99.ResultVars) == 0) {
		fmt.Fprintf(c99, "return")
		for i, result := range stmt.Results {
			fmt.Fprintf(c99, " ")
			var target types.Type
			if i < len(c99.Results) {
				target = c99.Results[i]
			}
			if err := c99.ExpressionAs(result, target); err != nil {
				return err
			}
		}
		return nil
	}
	fmt.Fprintf(c99, "{ ")
	for i, result := range stmt.Results {
		fmt.Fprintf(c99, "%s = ", c99.ResultVars[i])
		if err := c99.ExpressionAs(result, c99.Results[i]); err != nil {
			return err
		}
		fmt.Fprintf(c99, "; ")
	}
	if c99.Frame {
		fmt.Fprintf(c99, "go_frame_return(go_fr); ")
	}
	fmt.Fprintf(c99, "return%s; }", c99.returnValues())
	return nil
}

// AnyOf returns a C expression that converts expr to an empty interface value.
func (c99 Target) AnyOf(expr source.Expression) (string, error) {
	tv := expr.TypeAndValue()
	if isNil(expr) || tv.Type == nil {
		return "((go_vv){0})", nil
	}
	typ := types.Default(tv.Type)
	var value string
	if tv.Value != nil && tv.Value.Kind() != constant.Complex {
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ConstantValue(tv.Value, false); err != nil {
			return "", err
		}
		value = buf.String()
	} else {
		value = c99.toString(expr)
	}
	if iface, ok := typ.Underlying().(*types.Interface); ok {
		if iface.Empty() {
			return value, nil
		}
		return fmt.Sprintf("go_if_to_vv(%s)", value), nil
	}
	rtype, err := c99.reflectTypeOf(typ)
	if err != nil {
		return "", err
	}
	ctype := c99.TypeOf(typ)
	symbol := "go_any_of_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline go_vv %s(%s v, const go_type* t) { return go_any_new(sizeof(v), &v, t); }\n", symbol, ctype)
		return nil
	})
	return fmt.Sprintf("%s(%s, %s)", symbol, value, rtype), nil
}

// reflectTypeOf is [Target.ReflectTypeOf], for the types it supports.
func (c99 Target) reflectTypeOf(t types.Type) (s string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("unsupported type %s", t)
		}
	}()
	return c99.ReflectTypeOf(t), nil
}

// ExpressionTypeAssertion writes a single valued type assertion to a concrete type, which
// panics if the dynamic type differs.
func (c99 Target) ExpressionTypeAssertion(e source.ExpressionTypeAssertion) error {
	target := e.TypeAndValue().Type
	if _, ok := target.Underlying().(*types.Interface); ok {
		return e.Location.Errorf("unsupported type assertion to interface %s", target)
	}
	rtype, err := c99.reflectTypeOf(target)
	if err != nil {
		return e.Location.Errorf("%w", err)
	}
	value, err := c99.AnyOf(e.X)
	if err != nil {
		return e.Location.Errorf("%w", err)
	}
	ctype := c99.TypeOf(target)
	symbol := "go_assert_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline %s %s(go_vv v, const go_type* t) { if (!go_type_eq(v.go_type, t)) go_panic_assertion(t, v); return *(%[1]s*)v.ptr.ptr; }\n", ctype, symbol)
		return nil
	})
	fmt.Fprintf(c99, "%s(%s, %s)", symbol, value, rtype)
	return nil
}
