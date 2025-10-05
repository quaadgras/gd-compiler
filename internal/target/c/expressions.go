package c

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
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
	fmt.Fprintf(c99, "%s.make(&struct{pub fn call(package: *const anyopaque, default: ?*go.routine", c99.TypeOf(e.Type.TypeAndValue().Type))
	for _, arg := range e.Type.Arguments.Fields {
		names, ok := arg.Names.Get()
		if ok {
			for _, name := range names {
				fmt.Fprintf(c99, ",%s: %s", c99.toString(name), c99.Type(arg.Type))
			}
		} else {
			fmt.Fprintf(c99, ",_: %s", c99.Type(arg.Type))
		}
	}
	fmt.Fprintf(c99, ") ")
	results, ok := e.Type.Results.Get()
	if !ok {
		fmt.Fprintf(c99, "void")
	} else {
		switch len(results.Fields) {
		case 1:
			fmt.Fprintf(c99, "%s", c99.Type(results.Fields[0].Type))
		default:
			return e.Errorf("multiple return values not supported")
		}
	}
	fmt.Fprintf(c99, " { var chan2 = go.routine{}; const goto2: *go.routine = if (default) |select| select else &chan2; if (default == null) {defer goto2.exit();} go.use(package);")
	for _, stmt := range e.Body.Statements {
		c99.Tabs++
		if err := c99.Statement(stmt); err != nil {
			return err
		}
		c99.Tabs--
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}}{})")
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
		symbol := fmt.Sprintf("go_map_get__%s__%s", c99.Mangle(c99.TypeOf(mtype.Key())), c99.Mangle(c99.TypeOf(mtype.Elem())))
		c99.Requires(symbol, func() {
			fmt.Fprintf(c99.Prelude, "static inline %s %s(go_map m, %s key) { %s val; go_map_get(m, &key, &val); return val; }\n",
				c99.TypeOf(mtype.Elem()), symbol, c99.TypeOf(mtype.Key()), c99.TypeOf(mtype.Elem()))
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
	if err := c99.Expression(e.Chan); err != nil {
		return err
	}
	fmt.Fprint(c99, ".recv(goto)")
	return nil
}

func (c99 Target) ExpressionSlice(e source.ExpressionSlice) error {
	if err := c99.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(c99, ".range(")
	from, ok := e.From.Get()
	if ok {
		if err := c99.Expression(from); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c99, "0")
	}
	fmt.Fprintf(c99, ", ")
	high, ok := e.High.Get()
	if ok {
		if err := c99.Expression(high); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c99, "0")
	}
	fmt.Fprintf(c99, ")")
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
