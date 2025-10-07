package c99

import (
	"fmt"
	"go/types"
	"io"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) StatementGo(stmt source.StatementGo) error {
	return c99.FunctionCall(stmt.Call)
}

func (c99 Target) FunctionCall(expr source.FunctionCall) error {
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
	switch xyz.ValueOf(function) {
	case source.Expressions.BuiltinFunction:
		call := source.Expressions.BuiltinFunction.Get(function)
		switch call.String {
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
		default:
			return expr.Errorf("unsupported builtin function %s", call)
		}
	case source.Expressions.DefinedFunction:
		call := source.Expressions.DefinedFunction.Get(function)
		if expr.Go {
			fmt.Fprintf(c99, "go_make_func(%s), ", call.String)
		} else {
			if err := c99.DefinedFunction(call); err != nil {
				return err
			}
		}
		if !call.IsGlobal {
			isVariable = true
		}
	case source.Expressions.DefinedVariable:
		call := source.Expressions.DefinedVariable.Get(function)
		if expr.Go {
			fmt.Fprint(c99, call.String+", ")
		} else {
			fmt.Fprintf(c99, "(go_func_get(")
			if err := c99.DefinedVariable(call); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", ")
			ftype := expr.Function.TypeAndValue().Type.(*types.Signature)
			switch ftype.Results().Len() {
			case 0:
				fmt.Fprintf(c99, "void")
			case 1:
				fmt.Fprint(c99, c99.TypeOf(ftype.Results().At(0).Type()))
			default:
				panic("multiple return values not supported for function variables")
			}
			fmt.Fprintf(c99, "(*)(")
			for i := 0; i < ftype.Params().Len(); i++ {
				if i > 0 {
					fmt.Fprintf(c99, ", ")
				}
				fmt.Fprint(c99, c99.TypeOf(ftype.Params().At(i).Type()))
			}
			fmt.Fprintf(c99, ")))")
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
					fmt.Fprintf(c99, `go_interface_methods(%s, `, c99.InterfaceTypeOf(left.X.TypeAndValue().Type))
					if err := c99.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(c99, `)->%s(`, defined.String)
					if err := c99.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(c99, ".ptr.ptr")
				} else {
					receiver = xyz.New(left.X)
					rtype := left.X.TypeAndValue().Type
					for {
						pointer, ok := rtype.Underlying().(*types.Pointer)
						if !ok {
							break
						}
						rtype = pointer.Elem()
					}
					named, ok := rtype.(*types.Named)
					if !ok {
						return left.Errorf("unsupported receiver type %s", rtype)
					}
					fmt.Fprintf(c99, `%s_%s`, named.Obj().Name(), c99.toString(defined))
				}
			} else {
				if err := c99.Compile(left); err != nil {
					return err
				}
			}
		} else {
			if err := c99.Compile(left); err != nil {
				return err
			}
		}
	case source.Expressions.Type:
		ctype := source.Expressions.Type.Get(function)
		switch typ := ctype.TypeAndValue().Type.Underlying().(type) {
		case *types.Interface:
			symbol := fmt.Sprintf("go_interface_pack__%s", c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
			c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
				fmt.Fprintf(w, "static inline go_if %s(%[2]s v, void* vtable) { return go_interface_new(sizeof(%[2]s), &v, &go_type_%[2]s, vtable); }\n",
					symbol, c99.TypeOf(expr.Arguments[0].TypeAndValue().Type))
				return nil
			})
			fmt.Fprintf(c99, "%s(", symbol)
			if err := c99.Expression(expr.Arguments[0]); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", &(%s){", c99.InterfaceTypeOf(ctype.TypeAndValue().Type))
			for i := range typ.NumMethods() {
				if i > 0 {
					fmt.Fprintf(c99, ", ")
				}
				method := typ.Method(i)
				named := expr.Arguments[0].TypeAndValue().Type.(*types.Named)
				fmt.Fprintf(c99, `.%s = I_%s_%s_go_%s_package`,
					method.Name(), named.Obj().Name(), method.Name(), named.Obj().Pkg().Name())
			}
			fmt.Fprintf(c99, "})")
			return nil
		default:
			fmt.Fprintf(c99, "@as(%s)", c99.Type(ctype))
			return nil
		}
	case source.Expressions.Function:
		if err := c99.Expression(function); err != nil {
			return err
		}
	default:
		return expr.Opening.Errorf("unsupported call for function of type %T", xyz.ValueOf(function))
	}
	_ = isVariable
	ftype, ok := expr.Function.TypeAndValue().Type.(*types.Signature)
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
		} else {
			fmt.Fprintf(c99, "(")
		}
	}
	recv, hasReceiver := receiver.Get()
	if hasReceiver {
		if err := c99.Expression(recv); err != nil {
			return err
		}
	}
	var variadic bool
	for i, arg := range expr.Arguments {
		if i > 0 || hasReceiver {
			fmt.Fprintf(c99, ", ")
		}
		if !variadic && (ftype.Variadic() && i >= ftype.Params().Len()-1) {
			fmt.Fprintf(c99, "go_variadic(%d, %s, ", len(expr.Arguments)+1-ftype.Params().Len(), c99.TypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().(*types.Slice).Elem()))
			variadic = true
		}
		if err := c99.Expression(arg); err != nil {
			return err
		}
	}
	if ftype.Variadic() {
		if variadic {
			fmt.Fprintf(c99, ")")
		} else {
			fmt.Fprintf(c99, ".{}")
		}
	}
	fmt.Fprintf(c99, ")")
	return nil
}
