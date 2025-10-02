package c

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (cc Target) Type(e source.Type) string {
	value, _ := e.Get()
	return cc.TypeOf(value.TypeAndValue().Type)
}

func (cc Target) ReflectType(e source.Type) string {
	value, _ := e.Get()
	return cc.ReflectTypeOf(value.TypeAndValue().Type)
}

func (cc Target) TypeUnknown(source.TypeUnknown) error {
	fmt.Fprintf(cc, "unknown")
	return nil
}

func (cc Target) Mangle(s string) string {
	var replacer = strings.NewReplacer(
		".", "_",
		"/", "_",
		"(", "_",
		")", "_",
		"*", "_",
	)
	return replacer.Replace(s)
}

func (cc Target) TypeOf(t types.Type) string {
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
		return fmt.Sprintf("[%d]%s", typ.Len(), cc.TypeOf(typ.Elem()))
	case *types.Signature:
		var builder strings.Builder
		builder.WriteString("go.func(fn(*const anyopaque,?*go.routine")
		for i := 0; i < typ.Params().Len(); i++ {
			param := typ.Params().At(i)
			builder.WriteString(", ")
			builder.WriteString(cc.TypeOf(param.Type()))
		}
		builder.WriteString(") ")
		if typ.Results().Len() == 0 {
			builder.WriteString("void")
		} else if typ.Results().Len() == 1 {
			builder.WriteString(cc.TypeOf(typ.Results().At(0).Type()))
		} else {
			panic("unsupported function type with multiple results")
		}
		builder.WriteString(")")
		return builder.String()
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "@\"go." + typ.Obj().Name() + "\""
		}
		switch typ.Obj().Pkg().Name() {
		case "testing":
			return "go." + typ.Obj().Pkg().Name() + "." + typ.Obj().Name()
		case cc.CurrentPackage:
			return typ.Obj().Name()
		}
		return "@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + "\""
	case *types.Pointer:
		return cc.TypeOf(typ.Elem()) + "*"
	case *types.Slice:
		return "go_slice"
	case *types.Chan:
		return "go_chan"
	case *types.Map:
		return "go_map"
	case *types.Interface:
		return "go.interface"
	case *types.Struct:
		var builder strings.Builder
		builder.WriteString("struct {")
		for i := 0; i < typ.NumFields(); i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			field := typ.Field(i)
			builder.WriteString(field.Name())
			builder.WriteString(": ")
			builder.WriteString(cc.TypeOf(field.Type()))
		}
		builder.WriteString("}")
		return builder.String()
	case *types.Tuple:
		return ".{}"
	case nil:
		return "void"
	case *types.Alias:
		return cc.TypeOf(typ.Rhs())
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

func (cc Target) ReflectTypeOf(t types.Type) string {
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
		if typ.Obj().Pkg().Name() == cc.CurrentPackage {
			return "&@\"" + typ.Obj().Name() + ".(type)\""
		}
		return "&@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + ".(type)\""
	case *types.Pointer:
		return "go.rptr(goto, " + cc.ReflectTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
