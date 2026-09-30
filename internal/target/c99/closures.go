package c99

import (
	"fmt"
	"go/ast"
	"go/token"
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
	count    int // closures, deferred calls and temporaries so far, for unique names.
	info     *types.Info
	generics *Generics

	// frames are the functions (*ast.FuncDecl or *ast.FuncLit) with deferred calls, which
	// need a frame for panics to unwind to. Their named results are boxed, as they are
	// read after a panic recovers (and may be set by deferred calls).
	frames map[ast.Node]bool

	// tuples are the temporaries holding the values of var declarations of several
	// variables from a single tuple (var a, b = f()), by the AST node of the value.
	tuples map[ast.Node]tupleVar
}

type tupleVar struct {
	name  string
	types []types.Type
}

// NewClosures analyzes the closures in files.
func NewClosures(info *types.Info, files []*ast.File) *Closures {
	closures := &Closures{
		captures: make(map[*ast.FuncLit][]*types.Var),
		captured: make(map[types.Object]bool),
		frames:   make(map[ast.Node]bool),
		tuples:   make(map[ast.Node]tupleVar),
		info:     info,
	}
	// Local variables whose address is taken are boxed too, as the address may outlive the
	// function (C would point to its stack frame).
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			var root ast.Expr
			switch expr := node.(type) {
			case *ast.UnaryExpr:
				if expr.Op == token.AND {
					root = expr.X
				}
			case *ast.SliceExpr:
				if tv, ok := info.Types[expr.X]; ok {
					if _, isArray := tv.Type.Underlying().(*types.Array); isArray {
						root = expr.X
					}
				}
			case *ast.SelectorExpr: // x.M(), where M has a pointer receiver, is (&x).M()
				if sel, ok := info.Selections[expr]; ok && sel.Kind() == types.MethodVal {
					wantPointer := pointerReceiver(sel.Recv(), sel.Obj().(*types.Func))
					_, isPointer := sel.Recv().Underlying().(*types.Pointer)
					if wantPointer && !isPointer {
						root = expr.X
					}
				}
			}
			if v := addressedVariable(info, root); v != nil {
				closures.captured[v] = true
			}
			return true
		})
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			var ftype *ast.FuncType
			var body *ast.BlockStmt
			switch fn := node.(type) {
			case *ast.FuncDecl:
				ftype, body = fn.Type, fn.Body
			case *ast.FuncLit:
				ftype, body = fn.Type, fn.Body
			default:
				return true
			}
			if body == nil || !hasDefer(body) {
				return true
			}
			closures.frames[node] = true
			if ftype.Results != nil {
				for _, field := range ftype.Results.List {
					for _, name := range field.Names {
						closures.captured[info.Defs[name]] = true
					}
				}
			}
			return true
		})
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

// addressedVariable returns the local variable whose storage expr (an operand of &)
// refers to, if any: x, x.f (for a struct x), x[i] (for an array x).
func addressedVariable(info *types.Info, expr ast.Expr) *types.Var {
	for expr != nil {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.SelectorExpr:
			if tv, ok := info.Types[e.X]; !ok || isPointerType(tv.Type) {
				return nil // through a pointer.
			}
			expr = e.X
		case *ast.IndexExpr:
			if tv, ok := info.Types[e.X]; !ok {
				return nil
			} else if _, isArray := tv.Type.Underlying().(*types.Array); !isArray {
				return nil // slices and maps.
			}
			expr = e.X
		case *ast.Ident:
			v, ok := info.Uses[e].(*types.Var)
			if !ok || v.IsField() || v.Parent() == nil || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
				return nil // not a local variable.
			}
			return v
		default:
			return nil
		}
	}
	return nil
}

func isPointerType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}

// hasDefer reports whether body has a defer statement, outside of any function literals.
func hasDefer(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.DeferStmt:
			found = true
		case *ast.FuncLit:
			return false
		}
		return !found
	})
	return found
}

// Frame reports whether fn (an *ast.FuncDecl or *ast.FuncLit) needs a frame, see [Closures].
func (closures *Closures) Frame(fn ast.Node) bool {
	return closures != nil && closures.frames[fn]
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
		fmt.Fprintf(w, "static inline %s %s(%s) { go_nil_check((void*)(go_up)go_f.ptr); %s((%s(*)(%s))go_f.ptr)(%s); }\n", result, symbol,
			strings.Join(params, ", "), ret, result, strings.Join(append([]string{"void*"}, ptypes...), ", "),
			strings.Join(args, ", "))
		return nil
	})
	return symbol, nil
}

func (c99 Target) resultTypeOf(sig *types.Signature) (string, error) {
	return c99.TupleOfResults(sig), nil
}
