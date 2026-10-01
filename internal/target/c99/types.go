package c99

import (
	"fmt"
	"go/types"
	"hash/fnv"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
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
			return "up"
		case types.UnsafePointer:
			return "pt" // the same C type as other pointers.
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

// ArrayTypeOf returns the C type of a Go array, which is wrapped in a struct, as C arrays
// can't be assigned, passed or returned by value. The typedef is written to the package's
// private header, so that it is visible to every file in the package.
func (c99 Target) ArrayTypeOf(typ *types.Array) string {
	elem := c99.TypeOf(typ.Elem())
	symbol := fmt.Sprintf("go_arr%d_%s", typ.Len(), identifier.ReplaceAllString(elem, "_"))
	c99.defineType(symbol, []types.Type{typ.Elem()}, func(w io.Writer) {
		// C has no zero length arrays, so [0]T has room for one element.
		fmt.Fprintf(w, "\n#ifndef %[1]s_defined\n#define %[1]s_defined\ntypedef struct { %[2]s a[%[3]d]; } %[1]s;\n#endif\n",
			symbol, elem, max(typ.Len(), 1))
	})
	return symbol
}

var identifier = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func (c99 Target) InterfaceTypeOf(t types.Type) string {
	typ, ok := t.(*types.Named)
	if !ok { // a C struct type written out: the table of methods is a pointer to it.
		iface := t.Underlying().(*types.Interface)
		if iface.NumMethods() == 0 {
			return "go_az"
		}
		var builder strings.Builder
		builder.WriteString("struct { ")
		for i := 0; i < iface.NumMethods(); i++ {
			method := iface.Method(i)
			sig := method.Type().(*types.Signature)
			builder.WriteString(c99.TupleOfResults(sig))
			builder.WriteString("(*")
			builder.WriteString(source.CIdent(method.Name()))
			builder.WriteString(")(void*")
			for j := 0; j < sig.Params().Len(); j++ {
				builder.WriteString(", ")
				builder.WriteString(c99.TypeOf(sig.Params().At(j).Type()))
			}
			builder.WriteString(");")
		}
		builder.WriteString("}")
		return builder.String()
	}
	if typ.Obj().Pkg() == nil {
		return "go_" + typ.Obj().Name()
	}
	name := c99.typeCName(typ) + "_go_" + source.PackageIdent(typ.Obj().Pkg()) + "_package"
	if typ.TypeArgs().Len() > 0 { // an instance of a generic interface, defined where it's used.
		var deps []types.Type // (the types of its methods)
		for m := range typ.Underlying().(*types.Interface).Methods() {
			sig := m.Type().(*types.Signature)
			deps = append(deps, slicesOfTypes(sig.Params())...)
			deps = append(deps, slicesOfTypes(sig.Results())...)
		}
		c99.defineType(name, deps, func(w io.Writer) {
			fmt.Fprintf(w, "\n#ifndef %[1]s_defined\n#define %[1]s_defined\ntypedef %[2]s %[1]s;\n#endif\n", name, c99.InterfaceTypeOf(typ.Underlying()))
		})
	}
	return name
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
		return c99.ArrayTypeOf(typ)
	case *types.Signature:
		return "go_fn"
	case *types.Named:
		if iface, ok := typ.Underlying().(*types.Interface); ok {
			if iface.Empty() {
				return "go_vv"
			}
			return "go_if"
		}
		if typ.Obj().Pkg() == nil {
			return "go_" + typ.Obj().Name()
		}
		// named types are always qualified by their package, as headers are shared.
		name := c99.typeCName(typ) + "_go_" + source.PackageIdent(typ.Obj().Pkg()) + "_package"
		if typ.TypeArgs().Len() > 0 {
			c99.instanceType(typ, name)
		}
		return name
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
		return "go_if"
	case *types.Struct:
		if typ.NumFields() == 0 {
			return "go_az"
		}
		// Each struct type is a single C type (C struct types written out are distinct),
		// named after (a hash of) its fields, defined in the package's private header.
		hash := fnv.New64a()
		hash.Write([]byte(types.TypeString(typ, func(pkg *types.Package) string { return pkg.Path() })))
		symbol := fmt.Sprintf("go_struct_%x", hash.Sum64())
		var fieldTypes []types.Type
		for field := range typ.Fields() {
			fieldTypes = append(fieldTypes, field.Type())
		}
		c99.defineType(symbol, fieldTypes, func(w io.Writer) {
			var fields strings.Builder
			for i := 0; i < typ.NumFields(); i++ {
				field := typ.Field(i)
				fmt.Fprintf(&fields, "%s %s; ", c99.TypeOf(field.Type()), fieldName(field, i))
			}
			fmt.Fprintf(w, "\n#ifndef %[1]s_defined\n#define %[1]s_defined\ntypedef struct { %[2]s} %[1]s;\n#endif\n", symbol, fields.String())
		})
		return symbol
	case *types.Tuple:
		return ".{}"
	case nil:
		return "void"
	case *types.Alias:
		return c99.TypeOf(typ.Rhs())
	case *types.TypeParam:
		panic("type parameter " + typ.String() + " was not substituted")
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

// ReflectTypeOf returns a C expression for a pointer to the type descriptor (go_type) of t,
// a constant expression. Descriptors of types that are not named are static in each file
// that uses them, so descriptors are compared by kind and name, see go_type_eq.
func (c99 Target) ReflectTypeOf(t types.Type) string {
	t = types.Unalias(t)
	switch typ := t.(type) {
	case *types.Basic:
		if typ.Kind() == types.UnsafePointer {
			return "&go_type_unsafe_pointer"
		}
		return "&go_type_" + types.Default(typ).(*types.Basic).Name()
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "&go_type_" + typ.Obj().Name() // error
		}
		if typ.TypeArgs().Len() > 0 {
			return c99.instanceDescriptor(typ)
		}
		return "&go_type_" + c99.typeCName(typ) + "_go_" + source.PackageIdent(typ.Obj().Pkg()) + "_package"
	case *types.TypeParam:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
	name := typeName(t)
	symbol := "go_rtype_" + mangle(name)
	c99.staticDescriptor(symbol, name, t)
	return "&" + symbol
}

// descriptorFields returns the fields of the type descriptor (go_type) of t, other than
// its name: its kind and size, the types it's made of, its methods, and how its values are
// compared and hashed (when they are the dynamic values of interfaces).
func (c99 Target) descriptorFields(t types.Type) string {
	var b strings.Builder
	fmt.Fprintf(&b, ", .kind=go_kind_%s", kindOf(t))
	if zeroSize(t) {
		fmt.Fprintf(&b, ", .size=0") // C has no empty types.
	} else {
		fmt.Fprintf(&b, ", .size=sizeof(%s)", c99.TypeOf(t))
	}
	switch typ := t.Underlying().(type) {
	case *types.Pointer:
		fmt.Fprintf(&b, ", .data={.pointer={.elem=%s}}", c99.ReflectTypeOf(typ.Elem()))
	case *types.Slice:
		fmt.Fprintf(&b, ", .data={.slice={.elem=%s}}", c99.ReflectTypeOf(typ.Elem()))
	case *types.Array:
		fmt.Fprintf(&b, ", .data={.array={.elem=%s, .len=%d}}", c99.ReflectTypeOf(typ.Elem()), typ.Len())
	case *types.Map:
		fmt.Fprintf(&b, ", .data={.map={.key=%s, .elem=%s}}", c99.ReflectTypeOf(typ.Key()), c99.ReflectTypeOf(typ.Elem()))
	case *types.Chan:
		fmt.Fprintf(&b, ", .data={.chan={.elem=%s, .dir=%d}}", c99.ReflectTypeOf(typ.Elem()), typ.Dir())
	case *types.Interface:
		if methods, n := c99.interfaceMethods(t); n > 0 {
			fmt.Fprintf(&b, ", .data={.interface={%s, %d}}", methods, n)
		}
	case *types.Struct:
		if typ.NumFields() > 0 {
			ctype := c99.TypeOf(t)
			var fields []string
			for i := range typ.NumFields() {
				field := typ.Field(i)
				fields = append(fields, fmt.Sprintf("{.name=%s, .type=%s, .offset=offsetof(%s, %s), .exported=%t, .embedded=%t}",
					cString(field.Name()), c99.ReflectTypeOf(field.Type()), ctype, fieldName(field, i), field.Exported(), field.Anonymous()))
			}
			symbol := "go_fields_" + mangle(typeName(t))
			c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
				fmt.Fprintf(w, "static const go_field %s[] = {%s};\n", symbol, strings.Join(fields, ", "))
				return nil
			})
			fmt.Fprintf(&b, ", .data={.fields={%s, %d}}", symbol, typ.NumFields())
		}
	}
	b.WriteString(c99.methodTable(t))
	b.WriteString(c99.equalField(t))
	return b.String()
}

// zeroSize reports whether values of t take no memory (in Go: C has no empty types).
func zeroSize(t types.Type) bool {
	switch typ := t.Underlying().(type) {
	case *types.Struct:
		for field := range typ.Fields() {
			if !zeroSize(field.Type()) {
				return false
			}
		}
		return true
	case *types.Array:
		return typ.Len() == 0 || zeroSize(typ.Elem())
	}
	return false
}

// staticDescriptor defines (if it hasn't been) the type descriptor of t, static in the file,
// declared first, as it may be used by the descriptors it uses (of recursive types).
func (c99 Target) staticDescriptor(symbol, name string, t types.Type) {
	c99.Requires("declare "+symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static const go_type %s;\n", symbol)
		return nil
	})
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fields := c99.descriptorFields(t) // first, as it may write the descriptors it uses.
		fmt.Fprintf(w, "static const go_type %s = {.name=%s%s};\n", symbol, cString(name), fields)
		return nil
	})
}

// typeName returns the name of t for its type descriptor, which identifies the type: like
// types.TypeString, but without the names of parameters and results.
func typeName(t types.Type) string {
	t = types.Unalias(t) // (aliases are the same type)
	qualifier := func(pkg *types.Package) string { return pkg.Name() }
	switch typ := types.Unalias(t).(type) {
	case *types.Pointer:
		return "*" + typeName(typ.Elem())
	case *types.Slice:
		return "[]" + typeName(typ.Elem())
	case *types.Array:
		return fmt.Sprintf("[%d]%s", typ.Len(), typeName(typ.Elem()))
	case *types.Map:
		return "map[" + typeName(typ.Key()) + "]" + typeName(typ.Elem())
	case *types.Chan:
		switch typ.Dir() {
		case types.SendOnly:
			return "chan<- " + typeName(typ.Elem())
		case types.RecvOnly:
			return "<-chan " + typeName(typ.Elem())
		}
		return "chan " + typeName(typ.Elem())
	case *types.Signature:
		var params, results []string
		for i := range typ.Params().Len() {
			p := typeName(typ.Params().At(i).Type())
			if typ.Variadic() && i == typ.Params().Len()-1 {
				p = "..." + strings.TrimPrefix(p, "[]")
			}
			params = append(params, p)
		}
		for v := range typ.Results().Variables() {
			results = append(results, typeName(v.Type()))
		}
		name := "func(" + strings.Join(params, ", ") + ")"
		switch len(results) {
		case 0:
		case 1:
			name += " " + results[0]
		default:
			name += " (" + strings.Join(results, ", ") + ")"
		}
		return name
	case *types.Interface: // (as the Go runtime writes them: interface { M() int })
		if typ.NumMethods() == 0 && typ.NumEmbeddeds() == 0 {
			return "interface {}"
		}
		if typ.IsMethodSet() {
			var methods []string
			for m := range typ.Methods() {
				methods = append(methods, m.Name()+strings.TrimPrefix(typeName(m.Type()), "func"))
			}
			return "interface { " + strings.Join(methods, "; ") + " }"
		}
	case *types.Struct: // (struct { a int; B })
		if typ.NumFields() == 0 {
			return "struct {}"
		}
		var fields []string
		for i := range typ.NumFields() {
			f := typ.Field(i)
			field := typeName(f.Type())
			if !f.Embedded() {
				field = f.Name() + " " + field
			}
			if tag := typ.Tag(i); tag != "" { // (tags are part of the type)
				field += " " + strconv.Quote(tag)
			}
			fields = append(fields, field)
		}
		return "struct { " + strings.Join(fields, "; ") + " }"
	}
	return types.TypeString(t, qualifier)
}

// fieldName returns the C name of the i'th field of a struct (blank fields, which may be
// repeated, are numbered).
func fieldName(field *types.Var, i int) string {
	if field.Name() == "_" {
		return fmt.Sprintf("go_blank_%d", i)
	}
	return source.CIdent(field.Name())
}

func fieldIndex(typ *types.Struct, field *types.Var) int {
	for i := range typ.NumFields() {
		if typ.Field(i) == field {
			return i
		}
	}
	return -1
}

// mangle returns a C identifier for s (a type name), different for different names:
// letters and digits are kept, and other characters are escaped (as _XX, _ as __).
func mangle(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '_':
			b.WriteString("__")
		default:
			fmt.Fprintf(&b, "_%02x", c)
		}
	}
	return b.String()
}
