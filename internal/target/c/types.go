package c

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) Type(e source.Type) string {
	value, _ := e.Get()
	return c99.TypeOf(value.TypeAndValue().Type)
}

func (c99 Target) ReflectType(e source.Type) string {
	value, _ := e.Get()
	return c99.ReflectTypeOf(value.TypeAndValue().Type)
}

func (c99 Target) TypeUnknown(source.TypeUnknown) error {
	fmt.Fprintf(c99, "unknown")
	return nil
}

func (c99 Target) Mangle(s string) string {
	var replacer = strings.NewReplacer(
		".", "_",
		"/", "_",
		"(", "_",
		")", "_",
		"*", "_",
	)
	return replacer.Replace(s)
}

func (c99 Target) TypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool, types.UntypedBool:
			return "go_bool"
		case types.Int, types.UntypedInt:
			return "go_int"
		case types.Int8:
			return "go_int8"
		case types.Int16:
			return "go_int16"
		case types.Int32, types.UntypedRune:
			return "go_int32"
		case types.Int64:
			return "go_int64"
		case types.Uint:
			return "go_uint"
		case types.Uint8:
			return "go_uint8"
		case types.Uint16:
			return "go_uint16"
		case types.Uint32:
			return "go_uint32"
		case types.Uint64:
			return "go_uint64"
		case types.Uintptr:
			return "go_uintptr"
		case types.Float32:
			return "go_float32"
		case types.Float64, types.UntypedFloat:
			return "go_float64"
		case types.String, types.UntypedString:
			return "go_string"
		case types.Complex64:
			return "go_complex64"
		case types.Complex128, types.UntypedComplex:
			return "go_complex128"
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Array:
		return fmt.Sprintf("%s[%d]", c99.TypeOf(typ.Elem()), typ.Len())
	case *types.Signature:
		return "go_func"
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "go_" + typ.Obj().Name()
		}
		if typ.Obj().Pkg().Name() == c99.CurrentPackage {
			if !ast.IsExported(typ.Obj().Name()) {
				return typ.Obj().Name()
			}
		}
		return typ.Obj().Name() + "_go_" + typ.Obj().Pkg().Name() + "_package"
	case *types.Pointer:
		return "go_pointer"
	case *types.Slice:
		return "go_slice"
	case *types.Chan:
		return "go_chan"
	case *types.Map:
		return "go_map"
	case *types.Interface:
		return "go_interface"
	case *types.Struct:
		var builder strings.Builder
		builder.WriteString("struct { ")
		for i := 0; i < typ.NumFields(); i++ {
			field := typ.Field(i)
			ftype, array := c99.ArrayStrippedTypeOf(field.Type())
			builder.WriteString(c99.TypeOf(ftype))
			builder.WriteString(" ")
			builder.WriteString(field.Name())
			builder.WriteString(array)
			builder.WriteString("; ")
		}
		builder.WriteString("}")
		return builder.String()
	case *types.Tuple:
		return ".{}"
	case nil:
		return "void"
	case *types.Alias:
		return c99.TypeOf(typ.Rhs())
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

func (c99 Target) ReflectTypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			return "&go.@\"bool.(type)\""
		case types.Int, types.UntypedInt:
			return "&go.@\"int.(type)\""
		case types.Int8:
			return "&go.@\"int8.(type)\""
		case types.Int16:
			return "&go.@\"int16.(type)\""
		case types.Int32:
			return "&go.@\"int32.(type)\""
		case types.Int64:
			return "&go.@\"int64.(type)\""
		case types.Uint:
			return "&go.@\"uint.(type)\""
		case types.Uint8:
			return "&go.@\"uint8.(type)\""
		case types.Uint16:
			return "&go.@\"uint16.(type)\""
		case types.Uint32:
			return "&go.@\"uint32.(type)\""
		case types.Uint64:
			return "&go.@\"uint64.(type)\""
		case types.Uintptr:
			return "&go.@\"uintptr.(type)\""
		case types.Float32:
			return "&go.@\"float32.(type)\""
		case types.Float64:
			return "&go.@\"float64.(type)\""
		case types.String:
			return "string"
		case types.Complex64:
			return "&go.@\"complex64)\""
		case types.Complex128:
			return "&go.@\"complex128)\""
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "&@\"go." + typ.Obj().Name() + ".(type)\""
		}
		if typ.Obj().Pkg().Name() == c99.CurrentPackage {
			return "&@\"" + typ.Obj().Name() + ".(type)\""
		}
		return "&@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + ".(type)\""
	case *types.Pointer:
		return "go.rptr(goto, " + c99.ReflectTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
