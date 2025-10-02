package c

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) Expression(expr source.Expression) error {
	e, _ := expr.Get()
	return cc.Compile(e)
}

func (cc Target) ImportedPackage(id source.ImportedPackage) error {
	fmt.Fprintf(cc, "%s", cc.PackageOf(id.String))
	return nil
}

func (cc Target) Nil(expr source.Nil) error {
	fmt.Fprintf(cc, "go.nil")
	return nil
}

func (cc Target) ExpressionBinary(expr source.ExpressionBinary) error {
	switch expr.Operation.Value {
	case token.NEQ:
		switch etype := expr.X.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch etype.Kind() {
			case types.String, types.UntypedString:
				fmt.Fprintf(cc, "(!go.equality(%s, %s,%s))", cc.TypeOf(expr.X.TypeAndValue().Type), cc.toString(expr.X), cc.toString(expr.Y))
				return nil
			}
		}
	}
	if err := cc.Expression(expr.X); err != nil {
		return err
	}
	switch expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(cc, ".address")
	}
	switch expr.Operation.Value {
	case token.LOR:
		fmt.Fprintf(cc, " or ")
	case token.LAND:
		fmt.Fprintf(cc, " and ")
	default:
		fmt.Fprintf(cc, " %s ", expr.Operation.Value)
	}
	if err := cc.Expression(expr.Y); err != nil {
		return err
	}
	switch expr.X.TypeAndValue().Type.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(cc, ".address")
	}
	return nil
}

func (cc Target) Parenthesized(par source.Parenthesized) error {
	fmt.Fprintf(cc, "(")
	if err := cc.Expression(par.X); err != nil {
		return err
	}
	fmt.Fprintf(cc, ")")
	return nil
}

func (cc Target) ExpressionFunction(e source.ExpressionFunction) error {
	if cc.Tabs < 0 {
		cc.Tabs = -cc.Tabs
	}
	fmt.Fprintf(cc, "%s.make(&struct{pub fn call(package: *const anyopaque, default: ?*go.routine", cc.TypeOf(e.Type.TypeAndValue().Type))
	for _, arg := range e.Type.Arguments.Fields {
		names, ok := arg.Names.Get()
		if ok {
			for _, name := range names {
				fmt.Fprintf(cc, ",%s: %s", cc.toString(name), cc.Type(arg.Type))
			}
		} else {
			fmt.Fprintf(cc, ",_: %s", cc.Type(arg.Type))
		}
	}
	fmt.Fprintf(cc, ") ")
	results, ok := e.Type.Results.Get()
	if !ok {
		fmt.Fprintf(cc, "void")
	} else {
		switch len(results.Fields) {
		case 1:
			fmt.Fprintf(cc, "%s", cc.Type(results.Fields[0].Type))
		default:
			return e.Errorf("multiple return values not supported")
		}
	}
	fmt.Fprintf(cc, " { var chan2 = go.routine{}; const goto2: *go.routine = if (default) |select| select else &chan2; if (default == null) {defer goto2.exit();} go.use(package);")
	for _, stmt := range e.Body.Statements {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "}}{})")
	return nil
}

func (cc Target) ExpressionIndex(expr source.ExpressionIndex) error {
	switch expr.X.TypeAndValue().Type.(type) {
	case *types.Slice:
		elemType := cc.TypeOf(expr.X.TypeAndValue().Type.(*types.Slice).Elem())
		fmt.Fprintf(cc, "((%s*)", elemType)
		if err := cc.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(cc, ".ptr)[")
		if err := cc.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(cc, "]")
		return nil
	case *types.Map:
		mtype := expr.X.TypeAndValue().Type.(*types.Map)
		symbol := fmt.Sprintf("go_map_%s__%s_get", cc.Mangle(cc.TypeOf(mtype.Key())), cc.Mangle(cc.TypeOf(mtype.Elem())))
		cc.Requires(symbol, func() {
			fmt.Fprintf(cc.Prelude, "static inline %s %s(go_map m, %s key) { %s val; go_map_get(m, &key, &val); return val; }\n",
				cc.TypeOf(mtype.Elem()), symbol, cc.TypeOf(mtype.Key()), cc.TypeOf(mtype.Elem()))
		})
		fmt.Fprintf(cc, "%s(", symbol)
		if err := cc.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(cc, ", ")
		if err := cc.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(cc, ")")
		return nil
	case *types.Array:
		if err := cc.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(cc, "[")
		if err := cc.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(cc, "]")
		return nil
	default:
		return fmt.Errorf("unsupported index of type %T", expr)
	}
}

func (cc Target) ExpressionKeyValue(e source.ExpressionKeyValue) error {
	fmt.Fprintf(cc, ".")
	if err := cc.Expression(e.Key); err != nil {
		return err
	}
	fmt.Fprintf(cc, "=")
	if err := cc.Expression(e.Value); err != nil {
		return err
	}
	return nil
}

func (cc Target) AwaitChannel(e source.AwaitChannel) error {
	if err := cc.Expression(e.Chan); err != nil {
		return err
	}
	fmt.Fprint(cc, ".recv(goto)")
	return nil
}

func (cc Target) ExpressionSlice(e source.ExpressionSlice) error {
	if err := cc.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(cc, ".range(")
	from, ok := e.From.Get()
	if ok {
		if err := cc.Expression(from); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cc, "0")
	}
	fmt.Fprintf(cc, ", ")
	high, ok := e.High.Get()
	if ok {
		if err := cc.Expression(high); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cc, "0")
	}
	fmt.Fprintf(cc, ")")
	return nil
}

func (cc Target) ExpressionTypeAssertion(e source.ExpressionTypeAssertion) error {
	if err := cc.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(cc, " .(%s)", e.Type)
	return nil
}

func (cc Target) ExpressionUnary(e source.ExpressionUnary) error {
	switch e.Operation.Value {
	case token.AND:
		ident := source.Expressions.DefinedVariable.Get(e.X)
		if !cc.StackAllocated(ident) {
			fmt.Fprintf(cc, "%s{.address=%s}", cc.TypeOf(e.TypeAndValue().Type), cc.toString(e.X))
		} else {
			fmt.Fprintf(cc, "%s{.address=&%s}", cc.TypeOf(e.TypeAndValue().Type), cc.toString(e.X))
		}
		return nil
	default:
		fmt.Fprintf(cc, "%s", e.Operation.Value)
	}
	if err := cc.Expression(e.X); err != nil {
		return err
	}
	return nil
}
