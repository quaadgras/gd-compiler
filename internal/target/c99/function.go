package c99

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
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
	if fnName == "init" && !isMethod && !decl.IsClosure {
		fnName = c99.Initializers.Func("_go_" + c99.PackageOf(c99.CurrentPackage) + "_package")
	}
	if isMethod {
		fnName = fmt.Sprintf(`%s_%s`, receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named).Obj().Name(), fnName)
	}
	c99.Results = nil
	if results, ok := decl.Type.Results.Get(); ok {
		for _, field := range results.Fields {
			names, _ := field.Names.Get()
			for range max(len(names), 1) {
				c99.Results = append(c99.Results, field.Type.TypeAndValue().Type)
			}
		}
	}
	old := c99.CurrentFunction
	c99.CurrentFunction = fnName
	defer func() {
		c99.CurrentFunction = old
	}()

	return_type := func(w io.Writer) {
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(w, "%s ", c99.Type(results.Fields[0].Type))
			default:
				fmt.Fprintf(w, ".{")
				for i, field := range results.Fields {
					if i > 0 {
						fmt.Fprintf(w, ", ")
					}
					fmt.Fprintf(w, "%s", c99.Type(field.Type))
				}
				fmt.Fprintf(w, "} ")
			}
		} else {
			fmt.Fprintf(w, "void ")
		}
	}
	// Package-level functions have external linkage and a package qualified name, closures
	// are only referenced from the file they are defined in.
	var suffix string
	if !decl.IsClosure {
		suffix = "_go_" + c99.PackageOf(c99.CurrentPackage) + "_package"
	}
	// Prototypes that refer to exported types have to be in the public header, after the
	// type definitions (methods are prefixed with their receiver's type name).
	exported, closure := ast.IsExported(fnName), decl.IsClosure
	if decl.Name.String == "main" {
		fmt.Fprintf(c99, "go_main() { init_go_%s_package();", c99.CurrentPackage)
	} else {
		for _, param := range decl.Type.Arguments.Fields {
			if _, ok := param.Names.Get(); !ok {
				return param.Location.Errorf("missing names for function argument")
			}
		}
		decl := func(w io.Writer) {
			if closure {
				fmt.Fprintf(w, "static ")
			}
			return_type(w)
			fmt.Fprintf(w, "%s%s(", fnName, suffix)
			if closure {
				fmt.Fprintf(w, "void* go_env")
			}
			if isMethod {
				field := receiver.Fields[0]
				var name = "_"
				names, hasName := field.Names.Get()
				if hasName {
					name = c99.parameterName(names[0])
				}
				fmt.Fprintf(w, "%s %s", c99.Type(field.Type), name)
			}
			{
				var i int
				for _, param := range decl.Type.Arguments.Fields {
					names, _ := param.Names.Get()
					for _, name := range names {
						if i > 0 || isMethod || closure {
							fmt.Fprintf(w, ", ")
						}
						fmt.Fprintf(w, "%s %s", c99.Type(param.Type), c99.parameterName(name))
						i++
					}
				}
			}
			fmt.Fprintf(w, ")")
		}
		if closure {
			// defined before use, in the same file.
		} else if exported {
			fmt.Fprintln(c99.Exports)
			decl(c99.Exports)
			fmt.Fprintf(c99.Exports, ";")
		} else {
			fmt.Fprintln(c99.Private)
			decl(c99.Private)
			fmt.Fprintf(c99.Private, ";")
		}
		decl(c99)
		fmt.Fprintf(c99, " {")
	}
	c99.Tabs++
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "go_split();")
	// Captured variables: a closure accesses the boxes in its environment, and captured
	// parameters are boxed on entry.
	indent := "\n" + strings.Repeat("\t", c99.Tabs)
	if closure {
		for _, v := range c99.Environment {
			fmt.Fprintf(c99, "%s%s* %s = ((go_env_%s*)go_env)->%[3]s;", indent, c99.TypeOf(v.Type()), v.Name(), fnName)
		}
	}
	var params []source.Field
	if isMethod {
		params = append(params, receiver.Fields[0])
	}
	params = append(params, decl.Type.Arguments.Fields...)
	for _, param := range params {
		names, _ := param.Names.Get()
		for _, name := range names {
			if !c99.StackAllocated(name) {
				fmt.Fprintf(c99, "%s%s* %s = %s(%s);", indent, c99.Type(param.Type), name.String, c99.BoxOf(param.Type.TypeAndValue().Type), c99.parameterName(name))
			}
		}
	}
	for _, stmt := range body.Statements {
		if err := c99.Statement(stmt); err != nil {
			return err
		}
	}
	c99.Tabs--
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}")
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	// Interface wrapper.
	if isMethod {
		field := receiver.Fields[0]
		var name = "_"
		names, hasName := field.Names.Get()
		if hasName {
			name = names[0].String
		}
		return_type(c99)
		fmt.Fprintf(c99, `I_%s%s(void* %s`, fnName, suffix, name)
		var args strings.Builder
		for _, param := range decl.Type.Arguments.Fields {
			names, ok := param.Names.Get()
			if !ok {
				return param.Location.Errorf("missing names for function argument")
			}
			for _, name := range names {
				fmt.Fprintf(c99, ", %s %s", c99.Type(param.Type), c99.toString(name))
				fmt.Fprintf(&args, ", %s", c99.toString(name))
			}
		}
		fmt.Fprintf(c99, ") { ")
		if _, ok := decl.Type.Results.Get(); ok {
			fmt.Fprintf(c99, "return ")
		}
		fmt.Fprintf(c99, "%s%s(*(%s*)%s%s); }", fnName, suffix, c99.Type(field.Type), name, args.String())
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	return nil
}

// parameterName returns the C name of a parameter, captured parameters are received under
// another name, as the parameter itself is boxed (see [Closures]).
func (c99 Target) parameterName(name source.DefinedVariable) string {
	if !c99.StackAllocated(name) {
		return "go_param_" + name.String
	}
	return c99.toString(name)
}
