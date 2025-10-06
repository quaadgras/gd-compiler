package c99

import (
	"fmt"
	"go/token"
	"go/types"
	"io"

	"github.com/quaadgras/go-compiler/internal/source"
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

func (c99 Target) Nil(expr source.Nil) error {
	fmt.Fprintf(c99, "go.nil")
	return nil
}

func (c99 Target) ExpressionBinary(expr source.ExpressionBinary) error {
	switch expr.Operation.Value {
	case token.NEQ:
		switch etype := expr.X.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch etype.Kind() {
			case types.String, types.UntypedString:
				fmt.Fprintf(c99, "(!go_string_eq(%s, %s))", c99.toString(expr.X), c99.toString(expr.Y))
				return nil
			}
		}
	}
	if err := c99.Expression(expr.X); err != nil {
		return err
	}
	switch expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(c99, ".address")
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
		fmt.Fprintf(c99, ".address")
	}
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
	symbol := fmt.Sprintf("go_func_%s_%d", c99.CurrentFunction, c99.CurrentClosures)
	if err := c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		var c99 = c99
		c99.Writer = w
		c99.Tabs = 0
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
	fmt.Fprintf(c99, "go_make_func(%s)", symbol)
	return nil
}

func (c99 Target) ExpressionIndex(expr source.ExpressionIndex) error {
	switch expr.X.TypeAndValue().Type.(type) {
	case *types.Slice:
		elemType := c99.TypeOf(expr.X.TypeAndValue().Type.(*types.Slice).Elem())
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
		mtype := expr.X.TypeAndValue().Type.(*types.Map)
		symbol := fmt.Sprintf("go_map_get__%s__%s", c99.Mangle(mtype.Key()), c99.Mangle(mtype.Elem()))
		c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline %s %s(go_kv m, %s key) { %s val; go_map_get(m, &key, &val); return val; }\n",
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
		fmt.Fprintf(c99, "[")
		if err := c99.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(c99, "]")
		return nil
	default:
		return fmt.Errorf("unsupported index of type %T", expr)
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
	symbol := fmt.Sprintf("go_chan_recv__%s", c99.Mangle(e.Chan.TypeAndValue().Type.(*types.Chan).Elem()))
	c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline %s %s(go_ch c) { %s v; go_chan_recv(c, &v); return v; }\n",
			c99.TypeOf(e.Chan.TypeAndValue().Type.(*types.Chan).Elem()), symbol, c99.TypeOf(e.Chan.TypeAndValue().Type.(*types.Chan).Elem()))
		return nil
	})
	fmt.Fprintf(c99, "%s(", symbol)
	if err := c99.Expression(e.Chan); err != nil {
		return err
	}
	fmt.Fprint(c99, ")")
	return nil
}

func (c99 Target) ExpressionSlice(e source.ExpressionSlice) error {
	var from, high, cap string = "0", "0", "0"
	if expr, ok := e.From.Get(); ok {
		from = c99.toString(expr)
	}
	if expr, ok := e.High.Get(); ok {
		high = c99.toString(expr)
	}
	if expr, ok := e.Capacity.Get(); ok {
		cap = c99.toString(expr)
	} else {
		cap = high
	}
	switch e.X.TypeAndValue().Type.(type) {
	case *types.Pointer:
		array := e.X.TypeAndValue().Type.(*types.Pointer).Elem().(*types.Array)
		fmt.Fprintf(c99, "go_pointer_slice(%s, %d, %s, %s, %s, %s)",
			c99.toString(e.X),
			array.Len(),
			c99.TypeOf(array.Elem()),
			from, high, cap)
	case *types.Slice:
		fmt.Fprintf(c99, "go_slice_slice(%s, %s, %s, %s)",
			c99.toString(e.X),
			c99.TypeOf(e.X.TypeAndValue().Type.(*types.Pointer).Elem()),
			from, high)
	}
	return nil
}

func (c99 Target) ExpressionTypeAssertion(e source.ExpressionTypeAssertion) error {
	if err := c99.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(c99, " .(%s)", e.Type)
	return nil
}

func (c99 Target) ExpressionUnary(e source.ExpressionUnary) error {
	switch e.Operation.Value {
	case token.AND:
		ident := source.Expressions.DefinedVariable.Get(e.X)
		if !c99.StackAllocated(ident) {
			fmt.Fprintf(c99, "%s{.address=%s}", c99.TypeOf(e.TypeAndValue().Type), c99.toString(e.X))
		} else {
			fmt.Fprintf(c99, "%s{.address=&%s}", c99.TypeOf(e.TypeAndValue().Type), c99.toString(e.X))
		}
		return nil
	default:
		fmt.Fprintf(c99, "%s", e.Operation.Value)
	}
	if err := c99.Expression(e.X); err != nil {
		return err
	}
	return nil
}
