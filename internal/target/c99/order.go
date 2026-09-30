package c99

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// Order of evaluation: Go evaluates the function calls, method calls and receive operations
// of a statement in lexical left-to-right order, while C leaves the order of function
// arguments, the operands of most operators, and initializers unspecified. So when a
// statement has more than one of these, they are hoisted into temporaries, declared before
// the statement in Go's order (like the order pass of gc). The hoisting happens as the
// statement is written: hoisted expressions are written into their temporary instead.
type Order struct {
	hoist     map[ast.Node]bool
	rendering map[ast.Node]bool
	names     map[ast.Node]string
	temps     []string // declarations, in order of evaluation.
}

// orderOf returns the expressions of a statement that need to be hoisted, given its
// expressions (roots are calls that are the statement itself, and so are not hoisted).
func (c99 Target) orderOf(exprs []source.Expression, roots ...source.Expression) *Order {
	info := c99.Closures.info
	if info == nil {
		return nil
	}
	root := make(map[ast.Node]bool)
	for _, expr := range roots {
		root[source.LocationOf(expr).Node] = true
	}
	var events []ast.Node
	var visit func(node ast.Node) bool
	visit = func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.FuncLit:
			return false // evaluated when called.
		case *ast.BinaryExpr:
			if expr.Op == token.LAND || expr.Op == token.LOR {
				ast.Inspect(expr.X, visit)
				// the right operand is evaluated conditionally, so the whole expression is
				// evaluated at once, if it needs to be ordered.
				if countEvents(info, expr.Y) > 0 {
					events = append(events, expr)
				}
				return false
			}
		case *ast.CallExpr:
			ast.Inspect(expr.Fun, visit)
			for _, arg := range expr.Args {
				ast.Inspect(arg, visit)
			}
			if isEvent(info, expr) && !root[expr] {
				events = append(events, expr)
			}
			return false
		case *ast.UnaryExpr:
			if expr.Op == token.ARROW {
				ast.Inspect(expr.X, visit)
				events = append(events, expr)
				return false
			}
		}
		return true
	}
	for _, expr := range exprs {
		if node := source.LocationOf(expr).Node; node != nil {
			ast.Inspect(node, visit)
		}
	}
	if len(events) < 2 {
		return nil
	}
	order := &Order{
		hoist:     make(map[ast.Node]bool),
		rendering: make(map[ast.Node]bool),
		names:     make(map[ast.Node]string),
	}
	for _, event := range events {
		order.hoist[event] = true
	}
	return order
}

// isEvent reports whether call is a function or method call (or a builtin with side
// effects) with a single result, which can be hoisted into a temporary.
func isEvent(info *types.Info, call *ast.CallExpr) bool {
	tv, ok := info.Types[call]
	if !ok || tv.Type == nil || tv.Value != nil {
		return false
	}
	if _, ok := tv.Type.(*types.Tuple); ok {
		return false // void, or multiple results.
	}
	if fn, ok := info.Types[call.Fun]; ok && fn.IsType() {
		return false // conversion.
	}
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if builtin, ok := info.Uses[id].(*types.Builtin); ok {
			switch builtin.Name() {
			case "append", "copy", "recover":
				return true
			default:
				return false // no side effects.
			}
		}
	}
	return true
}

func countEvents(info *types.Info, node ast.Node) int {
	n := 0
	ast.Inspect(node, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if isEvent(info, expr) {
				n++
			}
		case *ast.UnaryExpr:
			if expr.Op == token.ARROW {
				n++
			}
		}
		return true
	})
	return n
}

// hoisted writes the temporary for node, when it is to be hoisted, reporting whether it
// did, render writes the expression (of type t) itself.
func (c99 Target) hoisted(node ast.Node, t types.Type, render func(Target) error) (bool, error) {
	order := c99.Order
	if order == nil || !order.hoist[node] || order.rendering[node] {
		return false, nil
	}
	if name, ok := order.names[node]; ok {
		fmt.Fprint(c99, name)
		return true, nil
	}
	order.rendering[node] = true
	var buf strings.Builder
	cc := c99
	cc.Writer = &buf
	cc.Tabs = 0
	err := render(cc)
	delete(order.rendering, node)
	if err != nil {
		return true, err
	}
	name := fmt.Sprintf("go_order_%d", c99.Closures.count)
	c99.Closures.count++
	order.temps = append(order.temps, fmt.Sprintf("%s %s = %s;", c99.TypeOf(t), name, buf.String()))
	order.names[node] = name
	fmt.Fprint(c99, name)
	return true, nil
}

// ordered writes a statement (with render) that evaluates exprs, hoisting what needs to be
// ordered. Statements that declare variables are not put in a block.
func (c99 Target) ordered(exprs []source.Expression, roots []source.Expression, declares bool, render func(Target) error) error {
	order := c99.orderOf(exprs, roots...)
	if order == nil || c99.Tabs < 0 { // no temporaries in the header of a for statement.
		return render(c99)
	}
	var buf strings.Builder
	cc := c99
	cc.Writer = &buf
	cc.Order = order
	if err := render(cc); err != nil {
		return err
	}
	if !declares {
		fmt.Fprint(c99, "{ ")
	}
	fmt.Fprintf(c99, "%s %s", strings.Join(order.temps, " "), buf.String())
	if !declares {
		fmt.Fprint(c99, "; }")
	}
	return nil
}
