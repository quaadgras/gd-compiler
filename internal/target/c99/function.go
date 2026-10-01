package c99

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

func (c99 Target) FunctionDefinition(decl source.FunctionDefinition) error {
	if decl.Name.String == "_" && !decl.IsClosure {
		return nil // (blank functions can't be called)
	}
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	instance := c99.Instance
	c99.Instance = ""
	if instance == "" && !decl.IsClosure && IsGeneric(decl) {
		return nil // compiled as instances, where they are used.
	}
	// Functions without a body are implemented elsewhere (in assembly, or linked from the
	// runtime): they are only declared, so programs that don't call them can be linked.
	body, hasBody := decl.Body.Get()
	receiver, isMethod := decl.Receiver.Get()
	var fnName = decl.Name.String
	if fnName == "init" && !isMethod && !decl.IsClosure {
		fnName = c99.Initializers.Func("_go_" + c99.PackageOf(c99.CurrentPackage) + "_package")
	}
	if instance != "" {
		fnName = instance
	} else if isMethod {
		named, ok := types.Unalias(derefType(receiver.Fields[0].Type.TypeAndValue().Type)).(*types.Named)
		if !ok {
			return decl.Errorf("unsupported receiver type %s", receiver.Fields[0].Type.TypeAndValue().Type)
		}
		fnName = fmt.Sprintf(`%s_%s`, named.Obj().Name(), fnName)
	}
	c99.Order = nil
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
		fmt.Fprintf(w, "%s ", c99.TupleOf(c99.Results))
	}

	// Package-level functions have external linkage and a package qualified name, closures
	// are only referenced from the file they are defined in.
	var suffix string
	if !decl.IsClosure && instance == "" {
		suffix = "_go_" + c99.PackageOf(c99.CurrentPackage) + "_package"
	}
	closure := decl.IsClosure
	// Interface wrappers, that take the receiver from the data of an interface value: I_
	// where it's the receiver, and IP_ (for methods with value receivers) where it's a
	// pointer to the receiver. They are declared with the method (as its body may need
	// them, for its type's table of methods), and defined after it.
	wrappers := func(define bool) {
		recvType := receiver.Fields[0].Type.TypeAndValue().Type
		var params, args []string
		for _, param := range decl.Type.Arguments.Fields {
			names, _ := param.Names.Get()
			for range max(len(names), 1) {
				args = append(args, fmt.Sprintf("p%d", len(args)))
				params = append(params, c99.Type(param.Type)+" "+args[len(args)-1])
			}
		}
		ret := ""
		if len(c99.Results) > 0 {
			ret = "return "
		}
		header, static := c99.Declarations, ""
		if instance != "" {
			header, static = c99.Prelude, "static "
		}
		wrapper := func(prefix, recv string) {
			sig := fmt.Sprintf("%s%s %s%s%s(%s)", static, c99.TupleOf(c99.Results), prefix, fnName, suffix,
				strings.Join(append([]string{"void* go_recv"}, params...), ", "))
			if define {
				fmt.Fprintf(c99, "%s { %s%s%s(%s); }\n", sig, ret, fnName, suffix, strings.Join(append([]string{recv}, args...), ", "))
			} else {
				fmt.Fprintf(header, "\n%s;", sig)
			}
		}
		wrapper("I_", fmt.Sprintf("*(%s*)go_recv", c99.TypeOf(recvType)))
		if _, isPointer := recvType.Underlying().(*types.Pointer); !isPointer {
			typeName := strings.TrimPrefix(types.TypeString(recvType, func(*types.Package) string { return "" }), ".")
			wrapper("IP_", fmt.Sprintf("(*(%s*)go_panicwrap((*(go_pt*)go_recv).ptr, %s, %s, %s))", c99.TypeOf(recvType),
				cString(c99.CurrentName), cString(typeName), cString(decl.Name.String)))
		}
	}
	isMain := decl.Name.String == "main" && !isMethod && !decl.IsClosure && c99.CurrentPackage == "main"
	if isMain {
		fmt.Fprintf(c99, "go_main() { init_go_%s_package();", c99.CurrentPackage)
	} else {
		decl := func(w io.Writer) {
			if closure || instance != "" {
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
					for k := range max(len(names), 1) {
						if i > 0 || isMethod || closure {
							fmt.Fprintf(w, ", ")
						}
						// unnamed and blank parameters still need (distinct) C names.
						cname := fmt.Sprintf("go_param_%d", i)
						if k < len(names) && names[k].String != "_" {
							cname = c99.parameterName(names[k])
						}
						fmt.Fprintf(w, "%s %s", c99.Type(param.Type), cname)
						i++
					}
				}
			}
			fmt.Fprintf(w, ")")
		}
		switch {
		case closure: // closures are defined before use, in the same file.
		case instance != "": // static to the file, before other instances that may use it.
			decl(c99.Prelude)
			fmt.Fprintf(c99.Prelude, ";\n")
		default:
			fmt.Fprintln(c99.Declarations)
			decl(c99.Declarations)
			fmt.Fprintf(c99.Declarations, ";")
		}
		if isMethod {
			wrappers(false)
		}
		if !hasBody {
			return nil
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
			fmt.Fprintf(c99, "%s%s* %s = ((go_env_%s*)go_env)->%[3]s;", indent, c99.TypeOf(subst(v.Type())), source.CIdent(v.Name()), fnName)
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
	// Result variables: named results are variables, and a function with a frame needs its
	// results in (boxed) variables too, as it returns them after a recovered panic.
	c99.Frame = c99.Closures.Frame(decl.Location.Node)
	c99.DeferFrame, c99.Yield, c99.YieldLoop, c99.Labels = "", nil, false, nil // (of an enclosing range over a function)
	c99.ResultVars = nil
	if results, ok := decl.Type.Results.Get(); ok {
		i := 0
		for _, field := range results.Fields {
			ctype := c99.Type(field.Type)
			names, named := field.Names.Get()
			if !named {
				if c99.Frame {
					fmt.Fprintf(c99, "%s%s* go_result_%d = go_new(sizeof(%[2]s), NULL).ptr;", indent, ctype, i)
					c99.ResultVars = append(c99.ResultVars, fmt.Sprintf("(*go_result_%d)", i))
				}
				i++
				continue
			}
			for _, name := range names {
				if name.String == "_" {
					name.String = fmt.Sprintf("go_result_%d", i)
					fmt.Fprintf(c99, "%s%s %s = {0};", indent, ctype, name.String)
					c99.ResultVars = append(c99.ResultVars, name.String)
				} else if !c99.StackAllocated(name) {
					fmt.Fprintf(c99, "%s%s* %s = go_new(sizeof(%[2]s), NULL).ptr;", indent, ctype, name.String)
					c99.ResultVars = append(c99.ResultVars, "(*"+name.String+")")
					if c99.shadowed(body.Location.Node, name.String) { // (return sets it, where the name is another variable)
						fmt.Fprintf(c99, " %s* go_result_%d = %s;", ctype, i, name.String)
						c99.ResultVars[len(c99.ResultVars)-1] = fmt.Sprintf("(*go_result_%d)", i)
					}
				} else {
					fmt.Fprintf(c99, "%s%s %s = {0};", indent, ctype, name.String)
					c99.ResultVars = append(c99.ResultVars, name.String)
					if c99.shadowed(body.Location.Node, name.String) {
						fmt.Fprintf(c99, " %s* go_result_%d = &%s;", ctype, i, name.String)
						c99.ResultVars[len(c99.ResultVars)-1] = fmt.Sprintf("(*go_result_%d)", i)
					}
				}
				i++
			}
		}
	}
	if isMain {
		c99.ResultVars = []string{"0"} // C's main returns int.
	}
	if c99.Frame {
		fmt.Fprintf(c99, "%sgo_frame* go_fr = go_frame_push();", indent)
		fmt.Fprintf(c99, "%sif (setjmp(go_fr->jb)) { go_frame_unwind(go_fr); return%s; }", indent, c99.returnValues())
	}
	if closure { // (a scope of its own, as it may redeclare the variables it captures)
		fmt.Fprintf(c99, "%s{", indent)
	}
	for _, stmt := range body.Statements {
		if err := c99.Statement(stmt); err != nil {
			return err
		}
	}
	if closure {
		fmt.Fprintf(c99, "%s}", indent)
	}
	if c99.Frame && len(c99.Results) == 0 {
		fmt.Fprintf(c99, "%sgo_frame_return(go_fr);", indent)
	}
	if isMain {
		fmt.Fprintf(c99, "%sreturn 0;", indent) // the main goroutine's result, see go_main.
	}
	c99.Tabs--
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "}")
	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	if isMain { // (for calls of main)
		fmt.Fprintf(c99.Declarations, "\nvoid main_go_main_package(void);")
		fmt.Fprintf(c99, "void main_go_main_package(void) { go_main_goroutine(); }\n")
	}
	if isMethod {
		wrappers(true)
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

// returnValues returns the result variables of the function being compiled, for a return
// statement (with a leading space when there are any).
func (c99 Target) returnValues() string {
	switch len(c99.ResultVars) {
	case 0:
		return ""
	case 1:
		return " " + c99.ResultVars[0]
	default:
		return fmt.Sprintf(" (%s){ %s }", c99.TupleOf(c99.Results), strings.Join(c99.ResultVars, ", "))
	}
}

// derefType returns the element type of a pointer type, or t.
func derefType(t types.Type) types.Type {
	if pointer, ok := t.Underlying().(*types.Pointer); ok {
		return pointer.Elem()
	}
	return t
}

// shadowed reports whether body declares another variable named name.
func (c99 Target) shadowed(body ast.Node, name string) bool {
	info := c99.Closures.info
	if info == nil || body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && id.Name == name && info.Defs[id] != nil {
			found = true
		}
		return !found
	})
	return found
}
