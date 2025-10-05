package c

import (
	"fmt"
	"go/types"

	"github.com/quaadgras/go-compiler/internal/source"
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
	var receiver xyz.Maybe[source.Expression]
	var variable bool
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
		if err := c99.DefinedFunction(call); err != nil {
			return err
		}
		if !call.Package {
			variable = true
		}
	case source.Expressions.DefinedVariable:
		call := source.Expressions.DefinedVariable.Get(function)
		if err := c99.DefinedVariable(call); err != nil {
			return err
		}
		if !call.Package {
			variable = true
		}
	case source.Expressions.Selector:
		left := source.Expressions.Selector.Get(function)
		if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
			defined := source.Expressions.DefinedFunction.Get(left.Selection)
			if defined.Method {
				_, isInterface = left.X.TypeAndValue().Type.Underlying().(*types.Interface)
				if isInterface {
					if err := c99.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(c99, `.itype.`)
					if err := c99.DefinedFunction(defined); err != nil {
						return err
					}
					fmt.Fprintf(c99, `(`)
					if err := c99.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(c99, ".value")
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
					fmt.Fprintf(c99, `%s_%s_go_%s_package`, named.Obj().Name(), c99.toString(defined), c99.PackageOf(named.Obj().Pkg().Name()))
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
			fmt.Fprintf(c99, "%s.make(goto,", c99.Type(ctype))
			if err := c99.Expression(expr.Arguments[0]); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", %s, .{", c99.ReflectTypeOf(expr.Arguments[0].TypeAndValue().Type))
			for i := range typ.NumMethods() {
				if i > 0 {
					fmt.Fprintf(c99, ", ")
				}
				method := typ.Method(i)
				named := expr.Arguments[0].TypeAndValue().Type.(*types.Named)
				fmt.Fprintf(c99, `.%s = &@"%s.%[1]s.(itfc)"`, method.Name(), named.Obj().Pkg().Name()+"."+named.Obj().Name())
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
	ftype, ok := expr.Function.TypeAndValue().Type.(*types.Signature)
	if !ok {
		return expr.Errorf("unsupported function type %T", expr.Function.TypeAndValue().Type)
	}
	if !isInterface {
		if variable && expr.Go {
			fmt.Fprintf(c99, ".go(.{null")
		} else if variable {
			fmt.Fprintf(c99, ".call(.{goto")
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
			fmt.Fprintf(c99, "go.variadic(%d, %s, .{", len(expr.Arguments)+1-ftype.Params().Len(), c99.TypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().(*types.Slice).Elem()))
			variadic = true
		}
		if err := c99.Expression(arg); err != nil {
			return err
		}
	}
	if ftype.Variadic() {
		if variadic {
			fmt.Fprintf(c99, "})")
		} else {
			fmt.Fprintf(c99, ".{}")
		}
	}
	if variable {
		fmt.Fprintf(c99, "})")
	} else {
		fmt.Fprintf(c99, ")")
	}
	return nil
}
