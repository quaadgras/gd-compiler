package c99

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) Expression(expr source.Expression) error {
	e, _ := expr.Get()
	return c99.Compile(e)
}

func (c99 Target) ImportedPackage(id source.ImportedPackage) error {
	fmt.Fprintf(c99, "%s", c99.PackageOf(id.String))
	return nil
}

// Nil writes the zero value of the type that nil has in its context (a compound literal
// works for all of the C representations of Go's nilable types).
func (c99 Target) Nil(expr source.Nil) error {
	typ := expr.TypeAndValue().Type
	if typ == nil || typ == types.Typ[types.UntypedNil] {
		fmt.Fprintf(c99, "NULL")
		return nil
	}
	fmt.Fprintf(c99, "((%s){0})", c99.TypeOf(typ))
	return nil
}

func (c99 Target) ExpressionBinary(expr source.ExpressionBinary) error {
	if ok, err := c99.hoistLogical(expr); ok {
		return err
	}
	if tv := expr.TypeAndValue(); tv.Value != nil {
		return c99.Constant(tv)
	}
	switch expr.Operation.Value {
	case token.QUO, token.REM:
		if isInteger(expr.TypeAndValue().Type) {
			fmt.Fprintf(c99, "%s(", c99.DivisionOf(expr.Operation.Value, expr.TypeAndValue().Type))
			if err := c99.Expression(expr.X); err != nil {
				return err
			}
			fmt.Fprintf(c99, ", ")
			if err := c99.Expression(expr.Y); err != nil {
				return err
			}
			fmt.Fprintf(c99, ")")
			return nil
		}
	case token.EQL, token.NEQ:
		x, y := expr.X, expr.Y
		if isNil(x) {
			x, y = y, x
		}
		if isNil(y) {
			return c99.compareNil(expr.Operation.Value, x)
		}
	}
	switch expr.Operation.Value {
	case token.AND_NOT:
		fmt.Fprintf(c99, "(%s & ~%s)", c99.toString(expr.X), c99.toString(expr.Y))
		return nil
	case token.ADD:
		if isString(expr.X.TypeAndValue().Type) {
			fmt.Fprintf(c99, "go_string_concat(%s, %s)", c99.toString(expr.X), c99.toString(expr.Y))
			return nil
		}
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		if isString(expr.X.TypeAndValue().Type) {
			fmt.Fprintf(c99, "(go_string_cmp(%s, %s) %s 0)", c99.toString(expr.X), c99.toString(expr.Y), expr.Operation.Value)
			return nil
		}
	case token.EQL, token.NEQ:
		if basic, ok := expr.X.TypeAndValue().Type.Underlying().(*types.Basic); ok && basic.Info()&types.IsString != 0 {
			not := ""
			if expr.Operation.Value == token.NEQ {
				not = "!"
			}
			fmt.Fprintf(c99, "(%sgo_string_eq(%s, %s))", not, c99.toString(expr.X), c99.toString(expr.Y))
			return nil
		}
	}
	if err := c99.Expression(expr.X); err != nil {
		return err
	}
	switch expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(c99, ".ptr")
	}
	switch expr.Operation.Value {
	case token.LOR:
		fmt.Fprintf(c99, " || ")
	case token.LAND:
		fmt.Fprintf(c99, " && ")
	default:
		fmt.Fprintf(c99, " %s ", expr.Operation.Value)
	}
	if err := c99.Expression(expr.Y); err != nil {
		return err
	}
	switch expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(c99, ".ptr")
	}
	return nil
}

// ExpressionAs writes expr, where the context expects a value of type target. This matters
// for nil, which the type checker records as untyped.
func (c99 Target) ExpressionAs(expr source.Expression, target types.Type) error {
	if target != nil { // implicit conversions of concrete values to interfaces.
		if _, toInterface := target.Underlying().(*types.Interface); toInterface && expr.TypeAndValue().Type != nil {
			if _, fromInterface := expr.TypeAndValue().Type.Underlying().(*types.Interface); !fromInterface {
				value, err := c99.InterfaceOf(expr, target)
				if err != nil {
					return err
				}
				fmt.Fprint(c99, value)
				return nil
			}
		}
	}
	if isNil(expr) && target != nil {
		if _, ok := target.Underlying().(*types.Basic); !ok || isUnsafePointer(target) {
			fmt.Fprintf(c99, "((%s){0})", c99.TypeOf(target))
			return nil
		}
	}
	return c99.Expression(expr)
}

// arrayIndex writes the index of an array, which panics when out of range (constant
// indexes are checked by the type checker).
func (c99 Target) arrayIndex(index source.Expression, array *types.Array) error {
	if index.TypeAndValue().Value != nil {
		return c99.Expression(index)
	}
	fmt.Fprintf(c99, "go_index_check((go_ii)(")
	if err := c99.Expression(index); err != nil {
		return err
	}
	fmt.Fprintf(c99, "), %d)", array.Len())
	return nil
}

// DivisionOf returns the name of a function that divides (op is token.QUO) or takes the
// remainder (token.REM) of integers of type t, panicking on division by zero, and wrapping
// on overflow (the most negative value divided by -1) as Go does, rather than C.
func (c99 Target) DivisionOf(op token.Token, t types.Type) string {
	ctype := c99.TypeOf(t)
	name := "div"
	if op == token.REM {
		name = "rem"
	}
	symbol := "go_" + name + "_" + identifier.ReplaceAllString(ctype, "_")
	signed := t.Underlying().(*types.Basic).Info()&types.IsUnsigned == 0
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline %s %s(%[1]s a, %[1]s b) { ", ctype, symbol)
		fmt.Fprintf(w, `if (b == 0) go_panic_error("runtime error: integer divide by zero"); `)
		if signed {
			if op == token.QUO {
				fmt.Fprintf(w, "if (b == -1) return (%s)(0 - (go_u8)a); ", ctype)
			} else {
				fmt.Fprintf(w, "if (b == -1) return 0; ")
			}
		}
		fmt.Fprintf(w, "return a %s b; }\n", op)
		return nil
	})
	return symbol
}

func isString(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

func isInteger(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

func isNil(expr source.Expression) bool {
	for xyz.ValueOf(expr) == source.Expressions.Parenthesized {
		expr = source.Expressions.Parenthesized.Get(expr).X
	}
	return xyz.ValueOf(expr) == source.Expressions.Nil
}

// compareNil compares x with nil, by checking the field of its C representation that is
// nil when x is nil (C can't compare structs with ==).
func (c99 Target) compareNil(op token.Token, x source.Expression) error {
	var field string
	switch typ := x.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer, *types.Signature:
		field = ".ptr"
	case *types.Basic: // unsafe.Pointer
		field = ".ptr"
	case *types.Slice:
		field = ".ptr.ptr"
	case *types.Interface:
		field = ".go_type"
	case *types.Map, *types.Chan:
	default:
		return fmt.Errorf("unsupported comparison of %s with nil", typ)
	}
	fmt.Fprintf(c99, "((")
	if err := c99.Expression(x); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")%s %s NULL)", field, op)
	return nil
}

func (c99 Target) Parenthesized(par source.Parenthesized) error {
	fmt.Fprintf(c99, "(")
	if err := c99.Expression(par.X); err != nil {
		return err
	}
	fmt.Fprintf(c99, ")")
	return nil
}

func (c99 Target) ExpressionFunction(e source.ExpressionFunction) error {
	if c99.Tabs < 0 {
		c99.Tabs = -c99.Tabs
	}
	symbol := fmt.Sprintf("go_func_%s_%d", c99.CurrentFunction, c99.Closures.count)
	c99.Closures.count++
	lit, _ := e.Location.Node.(*ast.FuncLit)
	captures := c99.Closures.captures[lit]
	if err := c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		var c99 = c99
		c99.Writer = w
		c99.Tabs = 0
		c99.Environment = captures
		if len(captures) > 0 {
			fmt.Fprintf(w, "typedef struct { ")
			for _, v := range captures {
				fmt.Fprintf(w, "%s* %s; ", c99.TypeOf(subst(v.Type())), source.CIdent(v.Name()))
			}
			fmt.Fprintf(w, "} go_env_%s;\n", symbol)
		}
		return c99.FunctionDefinition(source.FunctionDefinition{
			Location: e.Location,
			Name: source.DefinedFunction{
				String: symbol,
			},
			Type:      e.Type,
			Body:      xyz.New(e.Body),
			IsClosure: true,
		})
	}); err != nil {
		return err
	}
	if len(captures) == 0 {
		fmt.Fprintf(c99, "go_make_func(%s)", symbol)
		return nil
	}
	// The environment holds the boxes of the captured variables, which are pointers named
	// after the variables, in the function (or closure) creating this closure.
	var boxes []string
	for _, v := range captures {
		boxes = append(boxes, source.CIdent(v.Name()))
	}
	fmt.Fprintf(c99, "go_make_closure(%s, go_new(sizeof(go_env_%[1]s), &(go_env_%[1]s){ %s }).ptr)", symbol, strings.Join(boxes, ", "))
	return nil
}

func (c99 Target) ExpressionIndex(expr source.ExpressionIndex) error {
	if xyz.ValueOf(expr.X) == source.Expressions.DefinedFunction { // f[T], an instance.
		return c99.DefinedFunction(source.Expressions.DefinedFunction.Get(expr.X))
	}
	if tv := expr.TypeAndValue(); tv.Value != nil {
		return c99.Constant(tv)
	}
	switch xtype := expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Basic: // strings
		fmt.Fprintf(c99, "go_string_index(")
		if err := c99.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", (go_ii)(")
		if err := c99.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(c99, "))")
		return nil
	case *types.Slice:
		elemType := c99.TypeOf(xtype.Elem())
		fmt.Fprintf(c99, "go_slice_index(")
		if err := c99.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", %s, ", elemType)
		if err := c99.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Map:
		mtype := expr.X.TypeAndValue().Type.Underlying().(*types.Map)
		symbol := "go_map_get_" + identifier.ReplaceAllString(c99.TypeOf(mtype.Key())+"_"+c99.TypeOf(mtype.Elem()), "_")
		c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(go_kv m, %s key) { %s val = {0}; go_map_get(m, &key, &val); return val; }\n",
				c99.TypeOf(mtype.Elem()), symbol, c99.TypeOf(mtype.Key()), c99.TypeOf(mtype.Elem()))
			return nil
		})
		fmt.Fprintf(c99, "%s(", symbol)
		if err := c99.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", ")
		if err := c99.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Array:
		if err := c99.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ".a[")
		if err := c99.arrayIndex(expr.Index, xtype); err != nil {
			return err
		}
		fmt.Fprintf(c99, "]")
		return nil
	case *types.Pointer:
		array, ok := xtype.Elem().Underlying().(*types.Array)
		if !ok {
			return fmt.Errorf("unsupported index of type %s", xtype)
		}
		fmt.Fprintf(c99, "go_pointer_get(")
		if err := c99.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", %s).a[", c99.ArrayTypeOf(array))
		if err := c99.arrayIndex(expr.Index, array); err != nil {
			return err
		}
		fmt.Fprintf(c99, "]")
		return nil
	default:
		return expr.Location.Errorf("unsupported index of %s", expr.X.TypeAndValue().Type)
	}
}

func (c99 Target) ExpressionKeyValue(e source.ExpressionKeyValue) error {
	fmt.Fprintf(c99, ".")
	if err := c99.Expression(e.Key); err != nil {
		return err
	}
	fmt.Fprintf(c99, "=")
	if err := c99.Expression(e.Value); err != nil {
		return err
	}
	return nil
}

func (c99 Target) AwaitChannel(e source.AwaitChannel) error {
	if ok, err := c99.hoisted(e.Location.Node, e.TypeAndValue().Type, func(cc Target) error { return cc.AwaitChannel(e) }); ok {
		return err
	}
	symbol := fmt.Sprintf("go_recv_%s", c99.Mangle(e.Chan.TypeAndValue().Type.Underlying().(*types.Chan).Elem()))
	c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline %s %s(go_ch c) { %s v; go_recv(c, sizeof(%[1]s), &v); return v; }\n",
			c99.TypeOf(e.Chan.TypeAndValue().Type.Underlying().(*types.Chan).Elem()), symbol, c99.TypeOf(e.Chan.TypeAndValue().Type.Underlying().(*types.Chan).Elem()))
		return nil
	})
	fmt.Fprintf(c99, "%s(", symbol)
	if err := c99.Expression(e.Chan); err != nil {
		return err
	}
	fmt.Fprint(c99, ")")
	return nil
}

// ExpressionSlice slices a slice, an array, or a pointer to an array, sharing its
// elements, see go_slice.
func (c99 Target) ExpressionSlice(e source.ExpressionSlice) error {
	bound := func(expr xyz.Maybe[source.Expression]) string {
		if value, ok := expr.Get(); ok {
			return "(go_i8)(" + c99.toString(value) + ")"
		}
		return "go_slice_default"
	}
	low, high, max := bound(e.From), bound(e.High), bound(e.Capacity)
	switch typ := e.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		array, ok := typ.Elem().Underlying().(*types.Array)
		if !ok {
			return e.Location.Errorf("unsupported slice of %s", typ)
		}
		fmt.Fprintf(c99, "go_pointer_slice(%s, %d, %s, %s, %s, %s)",
			c99.toString(e.X), array.Len(), c99.TypeOf(array.Elem()), low, high, max)
	case *types.Array:
		fmt.Fprintf(c99, "go_slice((go_ll){ (go_pt){ (%s).a }, %d, %[2]d }, sizeof(%s), %s, %s, %s)",
			c99.toString(e.X), typ.Len(), c99.TypeOf(typ.Elem()), low, high, max)
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice(%s, sizeof(%s), %s, %s, %s)",
			c99.toString(e.X), c99.TypeOf(typ.Elem()), low, high, max)
	case *types.Basic: // strings
		fmt.Fprintf(c99, "go_string_slice(%s, %s, %s)", c99.toString(e.X), low, high)
	default:
		return e.Location.Errorf("unsupported slice of %s", typ)
	}
	return nil
}

func (c99 Target) ExpressionUnary(e source.ExpressionUnary) error {
	switch e.Operation.Value {
	case token.AND:
		if xyz.ValueOf(e.X) == source.Expressions.Composite { // &T{...} is allocated.
			fmt.Fprintf(c99, "go_new(sizeof(%s), &", c99.TypeOf(e.X.TypeAndValue().Type))
			if err := c99.Expression(e.X); err != nil {
				return err
			}
			fmt.Fprintf(c99, ")")
			return nil
		}
		if xyz.ValueOf(e.X) != source.Expressions.DefinedVariable {
			fmt.Fprintf(c99, "((go_pt){ .ptr = &(")
			if err := c99.Expression(e.X); err != nil {
				return err
			}
			fmt.Fprintf(c99, ") })")
			return nil
		}
		ident := source.Expressions.DefinedVariable.Get(e.X)
		if !c99.StackAllocated(ident) {
			fmt.Fprintf(c99, "(%s){.ptr=%s}", c99.TypeOf(e.TypeAndValue().Type), ident.String)
		} else {
			fmt.Fprintf(c99, "(%s){.ptr=&%s}", c99.TypeOf(e.TypeAndValue().Type), c99.toString(e.X))
		}
		return nil
	case token.XOR: // bitwise complement.
		fmt.Fprintf(c99, "(%s)~", c99.TypeOf(e.TypeAndValue().Type))
	default:
		fmt.Fprintf(c99, "%s", e.Operation.Value)
	}
	if err := c99.Expression(e.X); err != nil {
		return err
	}
	return nil
}

// ExpressionIndices is f[T1, T2], an instance of a generic function (as a value).
func (c99 Target) ExpressionIndices(expr source.ExpressionIndices) error {
	if xyz.ValueOf(expr.X) == source.Expressions.DefinedFunction {
		return c99.DefinedFunction(source.Expressions.DefinedFunction.Get(expr.X))
	}
	return expr.Location.Errorf("unsupported index of %s", expr.X.TypeAndValue().Type)
}
