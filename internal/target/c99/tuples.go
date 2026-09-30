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

// TupleOf returns the C type for the results ts of a function: void, the type of a single
// result, or a struct with a field for each result (r0, r1...), defined in the package's
// private header, so that function prototypes can use it.
func (c99 Target) TupleOf(ts []types.Type) string {
	switch len(ts) {
	case 0:
		return "void"
	case 1:
		return c99.TypeOf(ts[0])
	}
	var ctypes []string
	for _, t := range ts {
		ctypes = append(ctypes, c99.TypeOf(t))
	}
	symbol := "go_tuple_" + identifier.ReplaceAllString(strings.Join(ctypes, "_"), "_")
	c99.defineType(symbol, ts, func(w io.Writer) {
		fmt.Fprintf(w, "\n#ifndef %[1]s_defined\n#define %[1]s_defined\ntypedef struct { ", symbol)
		for i, ctype := range ctypes {
			fmt.Fprintf(w, "%s r%d; ", ctype, i)
		}
		fmt.Fprintf(w, "} %s;\n#endif\n", symbol)
	})
	return symbol
}

// TupleOfResults is [Target.TupleOf] for the results of sig.
func (c99 Target) TupleOfResults(sig *types.Signature) string {
	var ts []types.Type
	for v := range sig.Results().Variables() {
		ts = append(ts, v.Type())
	}
	return c99.TupleOf(ts)
}

// tupleValue returns a C expression for the tuple of values that expr (with n values)
// evaluates to, along with their types: a call of a function with multiple results, or the
// comma-ok forms of map indexes, type assertions and receives.
func (c99 Target) tupleValue(expr source.Expression, n int) (string, []types.Type, error) {
	for xyz.ValueOf(expr) == source.Expressions.Parenthesized {
		expr = source.Expressions.Parenthesized.Get(expr).X
	}
	commaOK := false
	switch xyz.ValueOf(expr) {
	case source.Expressions.Index, source.Expressions.TypeAssertion, source.Expressions.AwaitChannel:
		commaOK = true // the type checker records these as tuples too.
	}
	if tuple, ok := expr.TypeAndValue().Type.(*types.Tuple); ok && !commaOK && (tuple.Len() == n || n < 0) {
		var ts []types.Type
		for v := range tuple.Variables() {
			ts = append(ts, v.Type())
		}
		c99.TupleOf(ts)
		return c99.toString(expr), ts, nil
	}
	if n != 2 && n >= 0 {
		return "", nil, fmt.Errorf("unsupported assignment of %d values from %s", n, expr.TypeAndValue().Type)
	}
	switch xyz.ValueOf(expr) {
	case source.Expressions.Index:
		index := source.Expressions.Index.Get(expr)
		mtype, ok := index.X.TypeAndValue().Type.Underlying().(*types.Map)
		if !ok {
			break
		}
		ts := []types.Type{mtype.Elem(), types.Typ[types.Bool]}
		tuple := c99.TupleOf(ts)
		symbol := "go_map_get2_" + identifier.ReplaceAllString(c99.TypeOf(mtype.Key())+"_"+c99.TypeOf(mtype.Elem()), "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(go_kv m, %s key) { %[1]s r = {0}; r.r1 = go_map_get(m, &key, &r.r0); return r; }\n",
				tuple, symbol, c99.TypeOf(mtype.Key()))
			return nil
		})
		var key strings.Builder
		cc := c99
		cc.Writer = &key
		if err := cc.ExpressionAs(index.Index, mtype.Key()); err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s(%s, %s)", symbol, c99.toString(index.X), key.String()), ts, nil
	case source.Expressions.TypeAssertion:
		assert := source.Expressions.TypeAssertion.Get(expr)
		typ, ok := assert.Type.Get()
		if !ok {
			break
		}
		target := typ.TypeAndValue().Type
		if _, ok := target.Underlying().(*types.Interface); ok {
			return "", nil, fmt.Errorf("unsupported type assertion to interface %s", target)
		}
		rtype, err := c99.reflectTypeOf(target)
		if err != nil {
			return "", nil, err
		}
		value, err := c99.AnyOf(assert.X)
		if err != nil {
			return "", nil, err
		}
		ts := []types.Type{target, types.Typ[types.Bool]}
		tuple := c99.TupleOf(ts)
		ctype := c99.TypeOf(target)
		symbol := "go_assert2_" + identifier.ReplaceAllString(ctype, "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(go_vv v, const go_type* t) { %[1]s r = {0}; if (go_type_eq(v.go_type, t)) { r.r0 = *(%[3]s*)v.ptr.ptr; r.r1 = true; } return r; }\n",
				tuple, symbol, ctype)
			return nil
		})
		return fmt.Sprintf("%s(%s, %s)", symbol, value, rtype), ts, nil
	case source.Expressions.AwaitChannel:
		recv := source.Expressions.AwaitChannel.Get(expr)
		elem := recv.Chan.TypeAndValue().Type.Underlying().(*types.Chan).Elem()
		ts := []types.Type{elem, types.Typ[types.Bool]}
		tuple := c99.TupleOf(ts)
		symbol := "go_recv2_" + identifier.ReplaceAllString(c99.TypeOf(elem), "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(go_ch c) { %[1]s r = {0}; r.r1 = go_recv(c, sizeof(r.r0), &r.r0); return r; }\n", tuple, symbol)
			return nil
		})
		return fmt.Sprintf("%s(%s)", symbol, c99.toString(recv.Chan)), ts, nil
	}
	return "", nil, fmt.Errorf("unsupported assignment of 2 values from %s", expr.TypeAndValue().Type)
}

// tupleTypes returns the types of the values of the tuple expr, see [Target.tupleValue].
func tupleTypes(expr source.Expression) []types.Type {
	for xyz.ValueOf(expr) == source.Expressions.Parenthesized {
		expr = source.Expressions.Parenthesized.Get(expr).X
	}
	boolean := types.Typ[types.Bool]
	switch xyz.ValueOf(expr) {
	case source.Expressions.Index:
		if mtype, ok := source.Expressions.Index.Get(expr).X.TypeAndValue().Type.Underlying().(*types.Map); ok {
			return []types.Type{mtype.Elem(), boolean}
		}
	case source.Expressions.TypeAssertion:
		if typ, ok := source.Expressions.TypeAssertion.Get(expr).Type.Get(); ok {
			return []types.Type{typ.TypeAndValue().Type, boolean}
		}
	case source.Expressions.AwaitChannel:
		if ctype, ok := source.Expressions.AwaitChannel.Get(expr).Chan.TypeAndValue().Type.Underlying().(*types.Chan); ok {
			return []types.Type{ctype.Elem(), boolean}
		}
	}
	var ts []types.Type
	if tuple, ok := expr.TypeAndValue().Type.(*types.Tuple); ok {
		for v := range tuple.Variables() {
			ts = append(ts, v.Type())
		}
	}
	return ts
}

// element returns a placeholder expression of type t, that is written as the C expression
// value (see [Target.Substitutes]), for assigning the elements of a tuple.
func (c99 *Target) element(t types.Type, value string) source.Expression {
	node := &ast.Ident{Name: value, NamePos: token.NoPos}
	subs := make(map[ast.Node]string, len(c99.Substitutes)+1)
	for k, v := range c99.Substitutes {
		subs[k] = v
	}
	subs[node] = value
	c99.Substitutes = subs
	return source.Expressions.DefinedVariable.New(source.DefinedVariable{
		Typed:    source.Typed{TV: types.TypeAndValue{Type: t}},
		Location: source.Location{Node: node},
		String:   value,
	})
}

// assignTuple assigns (or defines) the variables of stmt from its single value, which is a
// tuple, see [Target.tupleValue].
func (c99 Target) assignTuple(stmt source.StatementAssignment) error {
	if c99.Header {
		return stmt.Location.Errorf("unsupported assignment of multiple results in for statement")
	}
	value, ts, err := c99.tupleValue(stmt.Values[0], len(stmt.Variables))
	if err != nil {
		return stmt.Location.Errorf("%w", err)
	}
	name := fmt.Sprintf("go_assign_%d", c99.Closures.count)
	c99.Closures.count++
	fmt.Fprintf(c99, "%s %s = %s", c99.TupleOf(ts), name, value)
	for i, variable := range stmt.Variables {
		fmt.Fprintf(c99, "; ")
		elem := c99.element(ts[i], fmt.Sprintf("%s.r%d", name, i))
		if err := c99.assignment(source.StatementAssignment{Location: stmt.Location, Token: stmt.Token,
			Variables: []source.Expression{variable}, Values: []source.Expression{elem}}); err != nil {
			return err
		}
	}
	return nil
}
