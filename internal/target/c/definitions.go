package c

import (
	"fmt"
	"go/types"
	"path"
	"strconv"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) DefinedVariable(name source.DefinedVariable) error {
	return c99.definedVariable(false, name)
}

func (c99 Target) definedVariable(decl bool, name source.DefinedVariable) error {
	var prefix string
	if !decl && !c99.StackAllocated(name) {
		prefix = "*"
	}
	if name.String == "_" {
		_, err := c99.Write([]byte("_"))
		return err
	}
	if !decl && prefix != "" {
		fmt.Fprintf(c99, "%s", prefix)
	}
	_, err := c99.Write([]byte(name.String))
	if err != nil {
		return err
	}
	return err
}

func (c99 Target) DefinedFunction(name source.DefinedFunction) error {
	fmt.Fprintf(c99, "%s", name.String)
	return nil
}

func (c99 Target) DefinedConstant(name source.DefinedConstant) error {
	if name.Shadow > 0 {
		fmt.Fprintf(c99, `@"%s.%d"`, name.String, name.Shadow)
		return nil
	}
	_, err := c99.Write([]byte(name.String))
	return err
}

func (c99 Target) SpecificationImport(spec source.Import) error {
	return nil
	path, _ := strconv.Unquote(path.Base(spec.Path.Value))
	rename, ok := spec.Rename.Get()
	if ok {
		fmt.Fprintf(c99, "const %s = ", rename.String)
	} else {

		fmt.Fprintf(c99, "const %s = ", path)
	}
	fmt.Fprintf(c99, "@import(%q);\n", path+".zig")
	return nil
}

func (c99 Target) TypeDefinition(spec source.TypeDefinition) error {

	header := c99.Private
	suffix := ""
	if spec.Exported && spec.Global {
		header = c99.Exports
		suffix = "_go_" + c99.CurrentPackage + "_package"
	}
	if !spec.Global {
		header = c99
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	ftype, array := c99.ArrayStrippedTypeOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(header, "typedef %s %s%s%s;\n", c99.TypeOf(ftype), spec.Name.String, suffix, array)
	fmt.Fprintf(header, "extern struct go_type %s%s_type;\n", spec.Name.String, suffix)

	/*fmt.Fprintf(c99, "const @\"%s.(type)\" = go.rtype{", spec.Name.String)
	fmt.Fprintf(c99, ".name=%q,", spec.Name.String)
	kind := kindOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(c99, ".kind=go.rkind.%s, ", kind)
	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(c99, ".data=go.rdata{.%s=&[_]go.field{", kind)
		for i := range rtype.NumFields() {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			field := rtype.Field(i)
			fmt.Fprintf(c99, ".{.name=%q,.type=%s,.offset=@offsetOf(%s,\"%[1]s\"),.exported=%v,.embedded=%v}",
				field.Name(), c99.ReflectTypeOf(field.Type()), spec.Name.String, field.Exported(), field.Anonymous())
		}
		fmt.Fprintf(c99, "}}")
	default:
		fmt.Fprintf(c99, ".data=go.rdata{.%s=void{}}", kind)
	}
	fmt.Fprintf(c99, "}")
	if !spec.Global {
		fmt.Fprintf(c99, "; go.use(@\"%s.(type)\")", spec.Name.String)
	}
	fmt.Fprintf(c99, ";")*/
	return nil
}

func kindOf(t types.Type) string {
	switch t := t.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.UntypedBool:
			return "Bool"
		case types.Int, types.UntypedInt:
			return "Int"
		case types.Int8:
			return "Int8"
		case types.Int16:
			return "Int16"
		case types.Int32:
			return "Int32"
		case types.Int64:
			return "Int64"
		case types.Uint:
			return "Uint"
		case types.Uint8:
			return "Uint8"
		case types.Uint16:
			return "Uint16"
		case types.Uint32:
			return "Uint32"
		case types.Uint64:
			return "Uint64"
		case types.Uintptr:
			return "Uintptr"
		case types.Float32:
			return "Float32"
		case types.Float64, types.UntypedFloat:
			return "Float64"
		case types.Complex64:
			return "Complex64"
		case types.Complex128, types.UntypedComplex:
			return "Complex128"
		case types.String:
			return "String"
		case types.UnsafePointer:
			return "UnsafePointer"
		default:
			panic("unexpected kindOf: " + t.String())
		}
	case *types.Array:
		return "Array"
	case *types.Chan:
		return "Chan"
	case *types.Slice:
		return "Slice"
	case *types.Signature:
		return "Func"
	case *types.Interface:
		return "Interface"
	case *types.Map:
		return "Map"
	case *types.Pointer:
		return "Pointer"
	case *types.Struct:
		return "Struct"
	}
	panic("unexpected kindOf: " + t.String())
}

func (c99 Target) VariableDefinition(spec source.VariableDefinition) error {
	if c99.Tabs > 0 {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	var name = spec.Name
	var value func() error
	var rtype types.Type
	var ztype string
	vtype, ok := spec.Type.Get()
	assignValue, hasValue := spec.Value.Get()
	if !ok && !hasValue {
		return fmt.Errorf("missing type for value %s", name.String)
	}
	if ok {
		rtype = vtype.TypeAndValue().Type
		ztype = c99.TypeOf(vtype.TypeAndValue().Type)
	} else {
		rtype = assignValue.TypeAndValue().Type
		ztype = c99.TypeOf(assignValue.TypeAndValue().Type)
	}
	if !hasValue {
		value = func() error {
			if ztype[0] == '*' {
				fmt.Fprintf(c99, "null")
				return nil
			}
			fmt.Fprintf(c99, "go.zero(%s)", ztype)
			return nil
		}
	} else {
		value = func() error {
			return c99.Expression(assignValue)
		}
		_, isInterface := rtype.Underlying().(*types.Interface)
		if isInterface {
			value = func() error {
				return c99.FunctionCall(source.FunctionCall{
					Location:  spec.Location,
					Function:  source.Expressions.Type.As(vtype),
					Arguments: []source.Expression{assignValue},
				})
			}
		}
	}
	if name.String == "_" {
		fmt.Fprintf(c99, "go_ignore(")
		if err := value(); err != nil {
			return err
		}
		fmt.Fprintf(c99, ")")
	} else {
		ftype, array := c99.ArrayStrippedTypeOf(rtype)
		fmt.Fprintf(c99, "%s ", c99.TypeOf(ftype))
		if err := c99.definedVariable(true, name); err != nil {
			return err
		}
		fmt.Fprint(c99, array)
		stackAllocated := c99.StackAllocated(name)
		stackAllocated = true
		fmt.Fprint(c99, " = ")
		if !stackAllocated {
			fmt.Fprintf(c99, "new(%s); *", c99.TypeOf(assignValue.TypeAndValue().Type))
			if err := c99.definedVariable(true, name); err != nil {
				return err
			}
			fmt.Fprint(c99, " = ")
		}
		if err := value(); err != nil {
			return err
		}
	}
	if c99.Tabs > 0 || spec.Global {
		fmt.Fprintf(c99, ";")
	}
	return nil
}

func (c99 Target) ConstantDefinition(def source.ConstantDefinition) error {
	if def.Name.String == "_" {
		return nil
	}
	if c99.Tabs > 0 {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	fmt.Fprintf(c99, "const ")
	if err := c99.DefinedConstant(def.Name); err != nil {
		return err
	}
	fmt.Fprintf(c99, ": %s = ", c99.TypeOf(def.TypeAndValue().Type))
	if err := c99.Expression(def.Value); err != nil {
		return err
	}
	if c99.Tabs > 0 || def.Global {
		fmt.Fprintf(c99, ";")
	}
	return nil
}
