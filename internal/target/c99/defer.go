package c99

import (
	"fmt"
	"go/ast"
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
	closure, err := c99.callClosure(stmt.Call, "deferred")
	if err != nil {
		return stmt.Location.Errorf("%w", err)
	}
	fmt.Fprintf(c99, "go_defer_push(go_fr, %s)", closure)
	return nil
}

// callClosure returns a closure (a C expression for a go_fn) that makes the call, with its
// function value, receiver and arguments evaluated now, into its environment (for defer
// and go statements).
func (c99 Target) callClosure(call source.FunctionCall, kind string) (string, error) {
	call.Go = false
	symbol := fmt.Sprintf("go_%s_%s_%d", kind, c99.CurrentFunction, c99.Closures.count)
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
		case "close", "copy", "delete", "clear": // the arguments, evaluated now, are variables then.
			args := make([]source.Expression, len(call.Arguments))
			for i, arg := range call.Arguments {
				t := types.Default(arg.TypeAndValue().Type)
				if m, ok := call.Arguments[0].TypeAndValue().Type.Underlying().(*types.Map); ok && name == "delete" && i == 1 {
					t = m.Key()
				}
				var buf strings.Builder
				cc := c99
				cc.Writer = &buf
				if err := cc.ExpressionAs(arg, t); err != nil {
					return "", err
				}
				args[i] = source.Expressions.DefinedVariable.New(source.DefinedVariable{
					Typed:    source.Typed{TV: types.TypeAndValue{Type: t}},
					Location: source.Location{Node: &ast.Ident{Name: "go_e"}},
					String:   store(c99.TypeOf(t), buf.String()),
				})
			}
			call.Arguments = args
		default:
			return "", fmt.Errorf("unsupported %s of builtin %s", kind, name)
		}
		for i, arg := range call.Arguments {
			if reevaluate(arg) || name == "close" || name == "copy" || name == "delete" || name == "clear" {
				continue
			}
			if name == "panic" {
				value, err := c99.AnyOf(arg)
				if err != nil {
					return "", err
				}
				deferred.Args[i] = store("go_vv", value)
				continue
			}
			deferred.Args[i] = store(c99.TypeOf(types.Default(arg.TypeAndValue().Type)), c99.toString(arg))
		}
	default:
		var ok bool
		if sig, ok = function.TypeAndValue().Type.Underlying().(*types.Signature); !ok {
			return "", fmt.Errorf("unsupported %s of %s", kind, function.TypeAndValue().Type)
		}
		switch xyz.ValueOf(function) {
		case source.Expressions.DefinedFunction:
		case source.Expressions.Selector:
			left := source.Expressions.Selector.Get(function)
			if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
				if method := source.Expressions.DefinedFunction.Get(left.Selection); method.Method {
					rtype := left.X.TypeAndValue().Type
					if fn, ok := method.Unique.(*types.Func); ok {
						if _, iface := rtype.Underlying().(*types.Interface); !iface {
							// the receiver of the method of the named type (which may be promoted).
							rtype = derefType(left.X.TypeAndValue().Type)
							if pointerReceiver(rtype, fn) {
								rtype = types.NewPointer(rtype)
							}
						}
					}
					value := c99.toString(left.X)
					if _, iface := rtype.Underlying().(*types.Interface); !iface {
						var err error
						if value, err = c99.receiverOf(function, left.X); err != nil {
							return "", err
						}
					}
					if _, iface := rtype.Underlying().(*types.Interface); iface && c99.TypeOf(rtype) == "go_if" {
						value = "go_if_check(" + value + ")" // (defer x.M() evaluates x.M now)
					}
					deferred.Receiver = store(c99.TypeOf(rtype), value)
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
				target = params.At(params.Len() - 1).Type().Underlying().(*types.Slice).Elem()
			} else if i < params.Len() {
				target = params.At(i).Type()
			}
			var buf strings.Builder
			cc := c99
			cc.Writer = &buf
			if err := cc.ExpressionAs(arg, target); err != nil {
				return "", err
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
		cc.Order = nil
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
		return "", err
	}
	env := "NULL"
	if len(ctypes) > 0 {
		env = fmt.Sprintf("go_new(sizeof(go_env_%s), &(go_env_%[1]s){ %s }).ptr", symbol, strings.Join(values, ", "))
	}
	return fmt.Sprintf("go_make_closure(%s, %s)", symbol, env), nil
}

// reevaluate reports whether arg can be compiled again when a deferred call runs, instead
// of being stored by the defer statement.
func reevaluate(arg source.Expression) bool {
	return arg.TypeAndValue().Value != nil || isNil(arg)
}

// StatementReturn returns the results (a tuple for multiple results). When the function
// has result variables (they are named, or it has deferred calls), they are set, and the
// deferred calls run, before returning them.
func (c99 Target) StatementReturn(stmt source.StatementReturn) error {
	if c99.Yield != nil {
		return c99.yieldReturn(stmt)
	}
	results := stmt.Results
	var tuple string // return f(), where f has multiple results.
	if len(results) == 1 && len(c99.Results) > 1 {
		value, ts, err := c99.tupleValue(results[0], len(c99.Results))
		if err != nil {
			return stmt.Location.Errorf("%w", err)
		}
		if c99.TupleOf(ts) != c99.TupleOf(c99.Results) {
			return stmt.Location.Errorf("unsupported return of %s results", results[0].TypeAndValue().Type)
		}
		tuple = value
	}
	value := func(i int) error {
		return c99.ExpressionAs(results[i], c99.Results[i])
	}
	if !c99.Frame && (len(results) > 0 || len(c99.ResultVars) == 0) {
		fmt.Fprintf(c99, "return")
		switch {
		case tuple != "":
			fmt.Fprintf(c99, " %s", tuple)
		case len(results) == 1:
			fmt.Fprintf(c99, " ")
			return value(0)
		case len(results) > 1:
			fmt.Fprintf(c99, " (%s){ ", c99.TupleOf(c99.Results))
			for i := range results {
				if i > 0 {
					fmt.Fprintf(c99, ", ")
				}
				if err := value(i); err != nil {
					return err
				}
			}
			fmt.Fprintf(c99, " }")
		}
		return nil
	}
	fmt.Fprintf(c99, "{ ")
	if tuple != "" {
		name := fmt.Sprintf("go_return_%d", c99.Closures.count)
		c99.Closures.count++
		fmt.Fprintf(c99, "%s %s = %s; ", c99.TupleOf(c99.Results), name, tuple)
		for i, rv := range c99.ResultVars {
			fmt.Fprintf(c99, "%s = %s.r%d; ", rv, name, i)
		}
	} else { // (all evaluated before any is set: return true, *p may panic after true)
		var temps []string
		for i := range results {
			if results[i].TypeAndValue().Value != nil {
				temps = append(temps, "")
				continue
			}
			temp := fmt.Sprintf("go_return_%d", c99.Closures.count)
			c99.Closures.count++
			fmt.Fprintf(c99, "%s %s = ", c99.TypeOf(c99.Results[i]), temp)
			if err := value(i); err != nil {
				return err
			}
			fmt.Fprintf(c99, "; ")
			temps = append(temps, temp)
		}
		for i := range results {
			fmt.Fprintf(c99, "%s = ", c99.ResultVars[i])
			if temps[i] != "" {
				fmt.Fprint(c99, temps[i])
			} else if err := value(i); err != nil {
				return err
			}
			fmt.Fprintf(c99, "; ")
		}
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
		value, err := c99.AnyOf(e.X)
		if err != nil {
			return e.Location.Errorf("%w", err)
		}
		fmt.Fprint(c99, c99.toInterface(value, target, true, "NULL"))
		return nil
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
