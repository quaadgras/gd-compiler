package c99

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"
)

// Closures describes the variables captured by the closures (function literals) of a
// package. Go closures capture variables by reference, so captured variables are boxed:
// allocated on the heap, and accessed through a pointer, by both the function that
// declares them and the closures that capture them. A closure's environment holds the
// pointers to the boxes it needs.
type Closures struct {
	captures map[*ast.FuncLit][]*types.Var // in order of first use.
	captured map[types.Object]bool
	count    int // closures compiled so far, for unique names.
}

// NewClosures analyzes the closures in files.
func NewClosures(info *types.Info, files []*ast.File) *Closures {
	closures := &Closures{
		captures: make(map[*ast.FuncLit][]*types.Var),
		captured: make(map[types.Object]bool),
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.FuncLit)
			if !ok {
				return true
			}
			seen := make(map[*types.Var]bool)
			ast.Inspect(lit.Body, func(node ast.Node) bool {
				id, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				v, ok := info.Uses[id].(*types.Var)
				if !ok || v.IsField() || v.Parent() == nil || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
					return true // not a local variable.
				}
				if v.Pos() >= lit.Pos() && v.Pos() < lit.End() {
					return true // declared inside the closure.
				}
				if !seen[v] {
					seen[v] = true
					closures.captures[lit] = append(closures.captures[lit], v)
					closures.captured[v] = true
				}
				return true
			})
			return true
		})
	}
	return closures
}

// Captured reports whether v is captured by a closure, and so is boxed.
func (closures *Closures) Captured(v types.Object) bool {
	return closures != nil && v != nil && closures.captured[v]
}

// BoxOf returns the name of a function that boxes a value of type t.
func (c99 Target) BoxOf(t types.Type) string {
	ctype := c99.TypeOf(t)
	symbol := "go_box_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline %[1]s* %[2]s(%[1]s v) { return go_new(sizeof(%[1]s), &v).ptr; }\n", ctype, symbol)
		return nil
	})
	return symbol
}

// FunctionValue returns a func value for the package-level function name, of type sig,
// through a thunk that takes (and ignores) the closure environment.
func (c99 Target) FunctionValue(name string, sig *types.Signature) (string, error) {
	symbol := "go_thunk_" + name
	result, err := c99.resultTypeOf(sig)
	if err != nil {
		return "", err
	}
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		var params, args []string
		params = append(params, "void* go_env")
		for i := range sig.Params().Len() {
			params = append(params, fmt.Sprintf("%s p%d", c99.TypeOf(sig.Params().At(i).Type()), i))
			args = append(args, fmt.Sprintf("p%d", i))
		}
		ret := "return "
		if result == "void" {
			ret = ""
		}
		fmt.Fprintf(w, "static %s %s(%s) { %s%s(%s); }\n", result, symbol,
			strings.Join(params, ", "), ret, name, strings.Join(args, ", "))
		return nil
	})
	return fmt.Sprintf("go_make_func(%s)", symbol), nil
}

// InvokerOf returns the name of a function that calls a func value of type sig, with the
// func value as its first argument, followed by the arguments of the call.
func (c99 Target) InvokerOf(sig *types.Signature) (string, error) {
	result, err := c99.resultTypeOf(sig)
	if err != nil {
		return "", err
	}
	var ptypes []string
	for i := range sig.Params().Len() {
		ptypes = append(ptypes, c99.TypeOf(sig.Params().At(i).Type()))
	}
	symbol := "go_invoke_" + identifier.ReplaceAllString(strings.Join(ptypes, "_")+"__"+result, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		params := []string{"go_fn go_f"}
		args := []string{"go_f.env"}
		for i, ptype := range ptypes {
			params = append(params, fmt.Sprintf("%s p%d", ptype, i))
			args = append(args, fmt.Sprintf("p%d", i))
		}
		ret := "return "
		if result == "void" {
			ret = ""
		}
		fmt.Fprintf(w, "static inline %s %s(%s) { %s((%s(*)(%s))go_f.ptr)(%s); }\n", result, symbol,
			strings.Join(params, ", "), ret, result, strings.Join(append([]string{"void*"}, ptypes...), ", "),
			strings.Join(args, ", "))
		return nil
	})
	return symbol, nil
}

func (c99 Target) resultTypeOf(sig *types.Signature) (string, error) {
	switch sig.Results().Len() {
	case 0:
		return "void", nil
	case 1:
		return c99.TypeOf(sig.Results().At(0).Type()), nil
	default:
		return "", fmt.Errorf("multiple return values are not supported for func values")
	}
}
