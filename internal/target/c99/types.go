package c99

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
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

/*
Mangle a type into a string suitable for use for use as an
identifier in generic instantiations.

	Type       Mangled Type
	----	   -------------
	bool       tf
	int        ii
	int8       i1
	int16      i2
	int32      i4
	int64      i8
	uint       uu
	uint8      u1
	uint16     u2
	uint32     u4
	uint64     u8
	uintptr    pt
	float32    f4
	float64    f8
	complex64  aaf4f4zz
	complex128 aaf8f8zz
	array 	   do...00
	chan       ch
	func       fn
	any		   vv
	interface  if...00
	map        kv
	pointer    pt
	slice	   ll
	string     ss
*/
func (c99 Target) Mangle(t types.Type) string {
	t = t.Underlying()
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool, types.UntypedBool:
			return "tf"
		case types.Int, types.UntypedInt:
			return "ii"
		case types.Int8:
			return "i1"
		case types.Int16:
			return "i2"
		case types.Int32, types.UntypedRune:
			return "i4"
		case types.Int64:
			return "i8"
		case types.Uint:
			return "uu"
		case types.Uint8:
			return "u1"
		case types.Uint16:
			return "u2"
		case types.Uint32:
			return "u4"
		case types.Uint64:
			return "u8"
		case types.Uintptr:
			return "pt"
		case types.Float32:
			return "f4"
		case types.Float64, types.UntypedFloat:
			return "f8"
		case types.String, types.UntypedString:
			return "ss"
		case types.Complex64:
			return "aaf4f4zz"
		case types.Complex128, types.UntypedComplex:
			return "aaf8f8zz"
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Array:
		repeat := fmt.Sprintf("%d", typ.Len())
		if len(repeat)%2 == 1 {
			repeat = "0" + repeat
		}
		return "do" + repeat + c99.Mangle(typ.Elem())
	case *types.Signature:
		return "fn"
	case *types.Pointer:
		return "pt"
	case *types.Slice:
		return "ll"
	case *types.Chan:
		return "ch"
	case *types.Map:
		return "kv"
	case *types.Interface:
		if typ.NumMethods() == 0 {
			return "vv"
		}
		var length = fmt.Sprintf("%d", typ.NumMethods())
		if len(length)%2 == 1 {
			length = "0" + length
		}
		return "if" + length
	case *types.Struct:
		var builder strings.Builder
		builder.WriteString("aa")
		for i := 0; i < typ.NumFields(); i++ {
			builder.WriteString(c99.Mangle(typ.Field(i).Type()))
		}
		builder.WriteString("zz")
		return builder.String()
	case nil:
		return ""
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

func (c99 Target) InterfaceTypeOf(t types.Type) string {
	typ, ok := t.(*types.Named)
	if !ok {
		return c99.TypeOf(t.Underlying())
	}
	if typ.Obj().Pkg() == nil {
		return "go_" + typ.Obj().Name()
	}
	if typ.Obj().Pkg().Name() == c99.CurrentPackage {
		if !ast.IsExported(typ.Obj().Name()) {
			return typ.Obj().Name()
		}
	}
	return typ.Obj().Name() + "_go_" + typ.Obj().Pkg().Name() + "_package"
}

func (c99 Target) TupleTypeOf(t *types.Tuple) string {
	var builder strings.Builder
	builder.WriteString("aa")
	for arg := range t.Variables() {
		fmt.Fprintf(&builder, "%s", c99.Mangle(arg.Type()))
	}
	builder.WriteString("zz")
	symbol := builder.String()
	if symbol == "aazz" {
		return "az"
	}
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "typedef struct { ")
		var i int
		for arg := range t.Variables() {
			fmt.Fprintf(w, "%s f%d; ", c99.TypeOf(arg.Type()), i)
			i++
		}
		fmt.Fprintf(w, "} go_%s;\n", symbol)
		return nil
	})
	return symbol
}

func (c99 Target) TypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		return "go_" + c99.Mangle(typ)
	case *types.Array:
		return fmt.Sprintf("%s[%d]", c99.TypeOf(typ.Elem()), typ.Len())
	case *types.Signature:
		return "go_fn"
	case *types.Named:
		if _, ok := typ.Underlying().(*types.Interface); ok {
			return "go_if"
		}
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
		return "go_pt"
	case *types.Slice:
		return "go_ll"
	case *types.Chan:
		return "go_ch"
	case *types.Map:
		return "go_kv"
	case *types.Interface:
		if typ.NumMethods() == 0 {
			return "go_vv"
		}
		var builder strings.Builder
		builder.WriteString("struct { ")
		for i := 0; i < typ.NumMethods(); i++ {
			method := typ.Method(i)
			sig := method.Type().(*types.Signature)
			switch sig.Results().Len() {
			case 0:
				builder.WriteString("void")
			case 1:
				builder.WriteString(c99.TypeOf(sig.Results().At(0).Type()))
			default:
				panic("interface methods with multiple return values are not supported")
			}
			builder.WriteString("(*")
			builder.WriteString(method.Name())
			builder.WriteString(")(void*")
			for j := 0; j < sig.Params().Len(); j++ {
				builder.WriteString(", ")
				builder.WriteString(c99.TypeOf(sig.Params().At(j).Type()))
			}
			builder.WriteString(");")
		}
		builder.WriteString("}")
		return builder.String()
	case *types.Struct:
		if typ.NumFields() == 0 {
			return "go_az"
		}
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
		return "&go_type_" + typ.Name()
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "&@\"go." + typ.Obj().Name() + ".(type)\""
		}
		return "&go_type_" + typ.Obj().Name() + "_go_" + typ.Obj().Pkg().Name() + "_package"
	case *types.Pointer:
		return "go_type_pointer_to(" + c99.ReflectTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
