package c

import (
	"fmt"
	"go/types"
	"path"
	"strconv"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) DefinedVariable(name source.DefinedVariable) error {
	return cc.definedVariable(false, name)
}

func (cc Target) definedVariable(decl bool, name source.DefinedVariable) error {
	var prefix string
	if !decl && !cc.StackAllocated(name) {
		prefix = "*"
	}
	if name.String == "_" {
		_, err := cc.Write([]byte("_"))
		return err
	}
	if !decl && prefix != "" {
		fmt.Fprintf(cc, "%s", prefix)
	}
	_, err := cc.Write([]byte(name.String))
	if err != nil {
		return err
	}
	return err
}

func (cc Target) DefinedFunction(name source.DefinedFunction) error {
	fmt.Fprintf(cc, "%s", name.String)
	return nil
}

func (cc Target) DefinedConstant(name source.DefinedConstant) error {
	if name.Shadow > 0 {
		fmt.Fprintf(cc, `@"%s.%d"`, name.String, name.Shadow)
		return nil
	}
	_, err := cc.Write([]byte(name.String))
	return err
}

func (cc Target) SpecificationImport(spec source.Import) error {
	return nil
	path, _ := strconv.Unquote(path.Base(spec.Path.Value))
	rename, ok := spec.Rename.Get()
	if ok {
		fmt.Fprintf(cc, "const %s = ", rename.String)
	} else {

		fmt.Fprintf(cc, "const %s = ", path)
	}
	fmt.Fprintf(cc, "@import(%q);\n", path+".zig")
	return nil
}

func (cc Target) TypeDefinition(spec source.TypeDefinition) error {
	fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	fmt.Fprintf(cc, "const %s = %s;", spec.Name.String, cc.Type(spec.Type))
	if !spec.Global {
		fmt.Fprintf(cc, "go.use(%s);", spec.Name.String)
	}
	fmt.Fprintf(cc, "const @\"%s.(type)\" = go.rtype{", spec.Name.String)
	fmt.Fprintf(cc, ".name=%q,", spec.Name.String)
	kind := kindOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(cc, ".kind=go.rkind.%s, ", kind)
	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(cc, ".data=go.rdata{.%s=&[_]go.field{", kind)
		for i := range rtype.NumFields() {
			if i > 0 {
				fmt.Fprintf(cc, ", ")
			}
			field := rtype.Field(i)
			fmt.Fprintf(cc, ".{.name=%q,.type=%s,.offset=@offsetOf(%s,\"%[1]s\"),.exported=%v,.embedded=%v}",
				field.Name(), cc.ReflectTypeOf(field.Type()), spec.Name.String, field.Exported(), field.Anonymous())
		}
		fmt.Fprintf(cc, "}}")
	default:
		fmt.Fprintf(cc, ".data=go.rdata{.%s=void{}}", kind)
	}
	fmt.Fprintf(cc, "}")
	if !spec.Global {
		fmt.Fprintf(cc, "; go.use(@\"%s.(type)\")", spec.Name.String)
	}
	fmt.Fprintf(cc, ";")
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

func (cc Target) VariableDefinition(spec source.VariableDefinition) error {
	if cc.Tabs > 0 {
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
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
		ztype = cc.TypeOf(vtype.TypeAndValue().Type)
	} else {
		rtype = assignValue.TypeAndValue().Type
		ztype = cc.TypeOf(assignValue.TypeAndValue().Type)
	}
	if !hasValue {
		value = func() error {
			if ztype[0] == '*' {
				fmt.Fprintf(cc, "null")
				return nil
			}
			fmt.Fprintf(cc, "go.zero(%s)", ztype)
			return nil
		}
	} else {
		value = func() error {
			return cc.Expression(assignValue)
		}
		_, isInterface := rtype.Underlying().(*types.Interface)
		if isInterface {
			value = func() error {
				return cc.FunctionCall(source.FunctionCall{
					Location:  spec.Location,
					Function:  source.Expressions.Type.As(vtype),
					Arguments: []source.Expression{assignValue},
				})
			}
		}
	}
	if name.String == "_" {
		fmt.Fprintf(cc, "_ = ")
		if err := value(); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cc, "%s ", ztype)
		if err := cc.definedVariable(true, name); err != nil {
			return err
		}
		stackAllocated := cc.StackAllocated(name)
		stackAllocated = true
		fmt.Fprint(cc, " = ")
		if !stackAllocated {
			fmt.Fprintf(cc, "new(%s); *", cc.TypeOf(assignValue.TypeAndValue().Type))
			if err := cc.definedVariable(true, name); err != nil {
				return err
			}
			fmt.Fprint(cc, " = ")
		}
		if err := value(); err != nil {
			return err
		}
	}
	if cc.Tabs > 0 || spec.Global {
		fmt.Fprintf(cc, ";")
	}
	return nil
}

func (cc Target) ConstantDefinition(def source.ConstantDefinition) error {
	if cc.Tabs > 0 {
		fmt.Fprintf(cc, "\n%s", strings.Repeat("\t", cc.Tabs))
	}
	if def.Name.String != "_" {
		fmt.Fprintf(cc, "const ")
		if err := cc.DefinedConstant(def.Name); err != nil {
			return err
		}
		fmt.Fprintf(cc, ": %s = ", cc.TypeOf(def.TypeAndValue().Type))
	} else {
		if err := cc.DefinedConstant(def.Name); err != nil {
			return err
		}
		fmt.Fprintf(cc, " = ")
	}
	if err := cc.Expression(def.Value); err != nil {
		return err
	}
	if cc.Tabs > 0 || def.Global {
		fmt.Fprintf(cc, ";")
	}
	return nil
}
