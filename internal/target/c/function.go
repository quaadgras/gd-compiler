package c

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (cc Target) FunctionDefinition(decl source.FunctionDefinition) error {
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	body, ok := decl.Body.Get()
	if !ok {
		return decl.Errorf("function missing body")
	}
	for i, stmt := range body.Statements {
		if xyz.ValueOf(stmt) == source.Statements.Defer {
			stmt := source.Statements.Defer.Get(stmt)
			stmt.OutermostScope = true
			body.Statements[i] = source.Statements.Defer.As(stmt)
		}
	}
	if decl.IsTest {
		fmt.Fprintf(cc, "test \"%s\" { var chan = go.routine{}; const goto = &chan; defer goto.exit();", strings.TrimPrefix(decl.Name.String, "Test"))
		t, ok := decl.Type.Arguments.Fields[0].Names.Get()
		if ok {
			fmt.Fprintf(cc, "const %[1]s = go.new(goto,go.testing.T); go.use(%[1]s);", cc.toString(t[0]))
		}
		for _, stmt := range body.Statements {
			cc.Tabs++
			if err := cc.Statement(stmt); err != nil {
				return err
			}
			cc.Tabs--
		}
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		fmt.Fprintf(cc, "}")
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
		return nil
	}
	receiver, isMethod := decl.Receiver.Get()
	var fnName = decl.Name.String
	if isMethod {
		fnName = fmt.Sprintf(`@"%s.%s"`, receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named).Obj().Name(), fnName)
	}
	if decl.Name.String == "main" {
		fmt.Fprintf(cc, "main() { ")
	} else {
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(cc, "%s ", cc.Type(results.Fields[0].Type))
			default:
				fmt.Fprintf(cc, ".{")
				for i, field := range results.Fields {
					if i > 0 {
						fmt.Fprintf(cc, ", ")
					}
					fmt.Fprintf(cc, "%s", cc.Type(field.Type))
				}
				fmt.Fprintf(cc, "} ")
			}
		} else {
			fmt.Fprintf(cc, "void ")
		}
		fmt.Fprintf(cc, "%s(", fnName)
		if isMethod {
			field := receiver.Fields[0]
			var name = "_"
			names, hasName := field.Names.Get()
			if hasName {
				name = names[0].String
			}
			fmt.Fprintf(cc, ", %s: %s", name, cc.Type(field.Type))
		}
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					if i > 0 {
						fmt.Fprintf(cc, ", ")
					}
					fmt.Fprintf(cc, "%s %s", cc.Type(param.Type), cc.toString(name))
					i++
				}
			}
		}
		fmt.Fprintf(cc, ") ")
		fmt.Fprintf(cc, "{")
	}
	for _, stmt := range body.Statements {
		cc.Tabs++
		if err := cc.Statement(stmt); err != nil {
			return err
		}
		cc.Tabs--
	}
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "}")
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	// Interface wrapper.
	if isMethod {
		named := receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named)
		fmt.Fprintf(cc, `pub fn @"%s.%s.%s.(itfc)"(default: ?*go.routine`, decl.Package, named.Obj().Name(), decl.Name.String)
		field := receiver.Fields[0]
		var name = "_"
		names, hasName := field.Names.Get()
		if hasName {
			name = names[0].String
		}
		fmt.Fprintf(cc, ", %s: *const anyopaque", name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(cc, ", ")
					fmt.Fprintf(cc, "%s: %s", cc.toString(name), cc.Type(param.Type))
					i++
				}
			}
		}
		fmt.Fprintf(cc, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(cc, "%s ", cc.Type(results.Fields[0].Type))
			default:
				return results.Opening.Errorf("unsupported number of function results: %d", len(results.Fields))
			}
		} else {
			fmt.Fprintf(cc, "void ")
		}
		fmt.Fprintf(cc, "{ return %s(default, @as(*const %s, @ptrCast(%s)).*", fnName, cc.Type(field.Type), name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(cc, ", ")
					fmt.Fprintf(cc, "%v", name)
					i++
				}
			}
		}
		fmt.Fprintf(cc, "); }")
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	}
	return nil
}
