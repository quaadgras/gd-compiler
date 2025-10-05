package c

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) FunctionDefinition(decl source.FunctionDefinition) error {
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
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
	receiver, isMethod := decl.Receiver.Get()
	var fnName = decl.Name.String
	if isMethod {
		fnName = fmt.Sprintf(`@"%s.%s"`, receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named).Obj().Name(), fnName)
	}
	old := c99.CurrentFunction
	old_count := c99.CurrentClosures
	c99.CurrentFunction = fnName
	c99.CurrentClosures = 0
	defer func() {
		c99.CurrentFunction = old
		c99.CurrentClosures = old_count
	}()

	if decl.Name.String == "main" {
		fmt.Fprintf(c99, "go_main() { ")
	} else {
		var suffix string
		if ast.IsExported(fnName) {
			suffix = "_go_" + c99.PackageOf(c99.CurrentPackage) + "_package"
		} else {
			fmt.Fprintf(c99, "static ")
		}
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(c99, "%s ", c99.Type(results.Fields[0].Type))
			default:
				fmt.Fprintf(c99, ".{")
				for i, field := range results.Fields {
					if i > 0 {
						fmt.Fprintf(c99, ", ")
					}
					fmt.Fprintf(c99, "%s", c99.Type(field.Type))
				}
				fmt.Fprintf(c99, "} ")
			}
		} else {
			fmt.Fprintf(c99, "void ")
		}

		fmt.Fprintf(c99, "%s%s(", fnName, suffix)
		if isMethod {
			field := receiver.Fields[0]
			var name = "_"
			names, hasName := field.Names.Get()
			if hasName {
				name = names[0].String
			}
			fmt.Fprintf(c99, ", %s: %s", name, c99.Type(field.Type))
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
						fmt.Fprintf(c99, ", ")
					}
					fmt.Fprintf(c99, "%s %s", c99.Type(param.Type), c99.toString(name))
					i++
				}
			}
		}
		fmt.Fprintf(c99, ") ")
		fmt.Fprintf(c99, "{")
	}
	for _, stmt := range body.Statements {
		c99.Tabs++
		if err := c99.Statement(stmt); err != nil {
			return err
		}
		c99.Tabs--
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}")
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	// Interface wrapper.
	if isMethod {
		named := receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named)
		fmt.Fprintf(c99, `pub fn @"%s.%s.%s.(itfc)"(default: ?*go.routine`, decl.Package, named.Obj().Name(), decl.Name.String)
		field := receiver.Fields[0]
		var name = "_"
		names, hasName := field.Names.Get()
		if hasName {
			name = names[0].String
		}
		fmt.Fprintf(c99, ", %s: *const anyopaque", name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(c99, ", ")
					fmt.Fprintf(c99, "%s: %s", c99.toString(name), c99.Type(param.Type))
					i++
				}
			}
		}
		fmt.Fprintf(c99, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(c99, "%s ", c99.Type(results.Fields[0].Type))
			default:
				return results.Opening.Errorf("unsupported number of function results: %d", len(results.Fields))
			}
		} else {
			fmt.Fprintf(c99, "void ")
		}
		fmt.Fprintf(c99, "{ return %s(default, @as(*const %s, @ptrCast(%s)).*", fnName, c99.Type(field.Type), name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(c99, ", ")
					fmt.Fprintf(c99, "%v", name)
					i++
				}
			}
		}
		fmt.Fprintf(c99, "); }")
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	return nil
}
