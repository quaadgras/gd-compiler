//go:build ignore

// Package reflect, for gd: it replaces the standard library's (which is coupled to the
// type descriptors of the Go runtime) with one over gd's type descriptors (go_type, see
// library/go.h), which it reads with the hooks in library/hooks/reflect.c.
package reflect

import (
	"strconv"
	"unsafe"
)

// Hooks, implemented in C.

func canonical(t unsafe.Pointer) unsafe.Pointer // the first descriptor seen of the type.
func typeString(t unsafe.Pointer) string
func typeKind(t unsafe.Pointer) Kind
func typeSize(t unsafe.Pointer) uintptr
func typeElem(t unsafe.Pointer) unsafe.Pointer
func typeKey(t unsafe.Pointer) unsafe.Pointer
func typeLen(t unsafe.Pointer) int
func typeChanDir(t unsafe.Pointer) int
func typeNumField(t unsafe.Pointer) int
func fieldName(t unsafe.Pointer, i int) string
func fieldType(t unsafe.Pointer, i int) unsafe.Pointer
func fieldOffset(t unsafe.Pointer, i int) uintptr
func fieldExported(t unsafe.Pointer, i int) bool
func fieldEmbedded(t unsafe.Pointer, i int) bool
func fieldTag(t unsafe.Pointer, i int) string
func methodType(t unsafe.Pointer, i int) unsafe.Pointer // (nil for the methods of interfaces)
func methodValue(t unsafe.Pointer, i int, recv, dst unsafe.Pointer)
func methodFuncType(t unsafe.Pointer, i int) unsafe.Pointer
func methodFunc(t unsafe.Pointer, i int, dst unsafe.Pointer)
func typeNumIn(t unsafe.Pointer) int
func typeNumOut(t unsafe.Pointer) int
func typeIn(t unsafe.Pointer, i int) unsafe.Pointer
func typeOut(t unsafe.Pointer, i int) unsafe.Pointer
func typeVariadic(t unsafe.Pointer) bool
func callFunc(t, fn, args, results unsafe.Pointer) // calls *fn, args and results point to pointers.
func makeFunc(t, env, dst unsafe.Pointer)           // *dst = a function of type t, that calls makeFuncCall(env, ...).
func registerMakeFunc()
func giveRecover(token bool) // lets the next function called recover, if token (see makeFuncCall)
func fieldPkgPath(t unsafe.Pointer, i int) string
func typePkgPath(t unsafe.Pointer) string
func typeNumMethod(t unsafe.Pointer) int
func methodName(t unsafe.Pointer, i int) string
func typeImplements(t, iface unsafe.Pointer) bool
func typeComparable(t unsafe.Pointer) bool
func pointerTo(t unsafe.Pointer) unsafe.Pointer
func efaceType(i any) unsafe.Pointer
func efaceData(i any) unsafe.Pointer
func packEface(t, p unsafe.Pointer) any               // a copy of the value (of type t) at p.
func storeInterface(iface, dst unsafe.Pointer, v any) // *dst = v, where *dst has the interface type iface.
func valuesEqual(t, a, b unsafe.Pointer) bool         // a == b, for comparable types.
func unsafeNew(t unsafe.Pointer) unsafe.Pointer       // a new zero value.
func memmove(dst, src unsafe.Pointer, size uintptr)
func mapLen(m unsafe.Pointer) int                                     // m points to the map.
func mapIndex(m, key unsafe.Pointer, t unsafe.Pointer) unsafe.Pointer // a copy of the element, or nil.
func mapAssign(m, key, elem unsafe.Pointer)
func mapDelete(m, key unsafe.Pointer)
func mapClear(m unsafe.Pointer)
func mapRange(m unsafe.Pointer) unsafe.Pointer // an iterator.
func mapNext(it, key, elem unsafe.Pointer) bool
func chanLen(c unsafe.Pointer) int // c points to the channel.
func chanCap(c unsafe.Pointer) int

// A Kind represents the specific kind of type that a Type represents.
type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Pointer
	Slice
	String
	Struct
	UnsafePointer
)

// Ptr is the old name for the Pointer kind.
const Ptr = Pointer

var kindNames = []string{
	Invalid: "invalid", Bool: "bool", Int: "int", Int8: "int8", Int16: "int16", Int32: "int32",
	Int64: "int64", Uint: "uint", Uint8: "uint8", Uint16: "uint16", Uint32: "uint32",
	Uint64: "uint64", Uintptr: "uintptr", Float32: "float32", Float64: "float64",
	Complex64: "complex64", Complex128: "complex128", Array: "array", Chan: "chan", Func: "func",
	Interface: "interface", Map: "map", Pointer: "ptr", Slice: "slice", String: "string",
	Struct: "struct", UnsafePointer: "unsafe.Pointer",
}

func (k Kind) String() string {
	if uint(k) < uint(len(kindNames)) {
		return kindNames[uint(k)]
	}
	return "kind" + strconv.Itoa(int(k))
}

// ChanDir represents a channel type's direction.
type ChanDir int

const (
	RecvDir ChanDir = 1 << iota
	SendDir
	BothDir = RecvDir | SendDir
)

func (d ChanDir) String() string {
	switch d {
	case SendDir:
		return "chan<-"
	case RecvDir:
		return "<-chan"
	case BothDir:
		return "chan"
	}
	return "ChanDir" + strconv.Itoa(int(d))
}

// Type is the representation of a Go type.
type Type interface {
	Align() int
	FieldAlign() int
	Method(int) Method
	MethodByName(string) (Method, bool)
	NumMethod() int
	Name() string
	PkgPath() string
	Size() uintptr
	String() string
	Kind() Kind
	Implements(u Type) bool
	AssignableTo(u Type) bool
	ConvertibleTo(u Type) bool
	Comparable() bool
	Bits() int
	ChanDir() ChanDir
	IsVariadic() bool
	Elem() Type
	Field(i int) StructField
	FieldByIndex(index []int) StructField
	FieldByName(name string) (StructField, bool)
	FieldByNameFunc(match func(string) bool) (StructField, bool)
	In(i int) Type
	Key() Type
	Len() int
	NumField() int
	NumIn() int
	NumOut() int
	Out(i int) Type
	OverflowComplex(x complex128) bool
	OverflowFloat(x float64) bool
	OverflowInt(x int64) bool
	OverflowUint(x uint64) bool
	CanSeq() bool
	CanSeq2() bool
	common() *rtype
}

// rtype is a type descriptor (a go_type, in C).
type rtype struct{ _ [0]func() }

func (t *rtype) ptr() unsafe.Pointer { return unsafe.Pointer(t) }

// toType returns the Type of the descriptor t (nil for nil).
func toType(t unsafe.Pointer) Type {
	if t == nil {
		return nil
	}
	return (*rtype)(canonical(t))
}

func (t *rtype) common() *rtype   { return t }
func (t *rtype) String() string   { return typeString(t.ptr()) }
func (t *rtype) Kind() Kind       { return typeKind(t.ptr()) }
func (t *rtype) Size() uintptr    { return typeSize(t.ptr()) }
func (t *rtype) Align() int       { return t.FieldAlign() }
func (t *rtype) NumMethod() int   { return typeNumMethod(t.ptr()) }
func (t *rtype) PkgPath() string  { return typePkgPath(t.ptr()) }
func (t *rtype) IsVariadic() bool { t.mustBeFunc("IsVariadic"); return typeVariadic(t.ptr()) }
func (t *rtype) CanSeq() bool     { return false }
func (t *rtype) CanSeq2() bool    { return false }
func (t *rtype) Comparable() bool { return typeComparable(t.ptr()) }

func (t *rtype) FieldAlign() int {
	switch t.Kind() {
	case Bool, Int8, Uint8:
		return 1
	case Int16, Uint16:
		return 2
	case Int32, Uint32, Float32, Complex64:
		return 4
	case Array:
		return t.Elem().FieldAlign()
	case Struct:
		align := 1
		for i := 0; i < t.NumField(); i++ {
			align = max(align, t.Field(i).Type.FieldAlign())
		}
		return align
	}
	return 8
}

// Name returns the name of a named type (the names of unnamed types start with *, [, map
// and so on, and those of named types with their package).
func (t *rtype) Name() string {
	s := t.String()
	if s == "" {
		return ""
	}
	if c := s[0]; !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_') {
		return ""
	}
	for _, prefix := range []string{"map[", "chan ", "chan<-", "func(", "struct{", "struct {", "interface {", "interface{"} {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return ""
		}
	}
	end := len(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			end = i
			break
		}
	}
	for i := end - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

func (t *rtype) Bits() int {
	switch k := t.Kind(); {
	case k >= Int && k <= Complex128:
		return int(t.Size()) * 8
	}
	panic("reflect: Bits of non-arithmetic Type " + t.String())
}

func (t *rtype) ChanDir() ChanDir {
	if t.Kind() != Chan {
		panic("reflect: ChanDir of non-chan type " + t.String())
	}
	switch typeChanDir(t.ptr()) { // (go/types' ChanDir)
	case 1:
		return SendDir
	case 2:
		return RecvDir
	}
	return BothDir
}

func (t *rtype) Elem() Type {
	switch t.Kind() {
	case Array, Chan, Map, Pointer, Slice:
		return toType(typeElem(t.ptr()))
	}
	panic("reflect: Elem of invalid type " + t.String())
}

func (t *rtype) Key() Type {
	if t.Kind() != Map {
		panic("reflect: Key of non-map type " + t.String())
	}
	return toType(typeKey(t.ptr()))
}

func (t *rtype) Len() int {
	if t.Kind() != Array {
		panic("reflect: Len of non-array type " + t.String())
	}
	return typeLen(t.ptr())
}

func (t *rtype) NumField() int {
	if t.Kind() != Struct {
		panic("reflect: NumField of non-struct type " + t.String())
	}
	return typeNumField(t.ptr())
}

func (t *rtype) Field(i int) StructField {
	if t.Kind() != Struct {
		panic("reflect: Field of non-struct type " + t.String())
	}
	if i < 0 || i >= t.NumField() {
		panic("reflect: Field index out of bounds")
	}
	f := StructField{
		Name:      fieldName(t.ptr(), i),
		Type:      toType(fieldType(t.ptr(), i)),
		Offset:    fieldOffset(t.ptr(), i),
		Index:     []int{i},
		Anonymous: fieldEmbedded(t.ptr(), i),
		Tag:       StructTag(fieldTag(t.ptr(), i)),
	}
	if !fieldExported(t.ptr(), i) {
		f.PkgPath = fieldPkgPath(t.ptr(), i)
	}
	return f
}

func (t *rtype) FieldByIndex(index []int) StructField {
	var f StructField
	var typ Type = t
	for i, x := range index {
		if i > 0 && typ.Kind() == Pointer {
			typ = typ.Elem()
		}
		f = typ.Field(x)
		typ = f.Type
	}
	f.Index = index
	return f
}

func (t *rtype) FieldByName(name string) (StructField, bool) {
	return t.FieldByNameFunc(func(s string) bool { return s == name })
}

// FieldByNameFunc finds the shallowest field whose name matches (breadth first).
func (t *rtype) FieldByNameFunc(match func(string) bool) (StructField, bool) {
	type candidate struct {
		typ   Type
		index []int
	}
	level := []candidate{{t, nil}}
	for len(level) > 0 {
		var next []candidate
		var found []StructField
		for _, c := range level {
			typ := c.typ
			if typ.Kind() == Pointer {
				typ = typ.Elem()
			}
			if typ.Kind() != Struct {
				continue
			}
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				index := append(append([]int{}, c.index...), i)
				if match(f.Name) {
					f.Index = index
					found = append(found, f)
				} else if f.Anonymous {
					next = append(next, candidate{f.Type, index})
				}
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		if len(found) > 1 {
			return StructField{}, false // ambiguous
		}
		level = next
	}
	return StructField{}, false
}

func (t *rtype) Method(i int) Method {
	if i < 0 || i >= t.NumMethod() {
		panic("reflect: Method index out of range")
	}
	m := Method{Name: methodName(t.ptr(), i), Index: i}
	if t.Kind() == Interface {
		return m // (gd records the signatures of the methods of interfaces as strings)
	}
	if ft := methodFuncType(t.ptr(), i); ft != nil { // (the method expression, with the receiver first)
		m.Type = toType(ft)
		fn := unsafeNew(ft)
		methodFunc(t.ptr(), i, fn)
		m.Func = Value{(*rtype)(canonical(ft)), fn, 0}
	}
	return m
}

func (t *rtype) MethodByName(name string) (Method, bool) {
	for i := 0; i < t.NumMethod(); i++ {
		if methodName(t.ptr(), i) == name {
			return t.Method(i), true
		}
	}
	return Method{}, false
}

func (t *rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return typeImplements(t.ptr(), u.common().ptr())
}

func (t *rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	return u.common() == t || (u.Kind() == Interface && t.Implements(u))
}

func (t *rtype) ConvertibleTo(u Type) bool {
	if t.AssignableTo(u) {
		return true
	}
	a, b := t.Kind(), u.Kind()
	numeric := func(k Kind) bool { return k >= Int && k <= Complex128 }
	switch {
	case numeric(a) && numeric(b):
		return (a >= Complex64) == (b >= Complex64)
	case b == String && (a >= Int && a <= Uintptr || a == Slice && (t.Elem().Kind() == Uint8 || t.Elem().Kind() == Int32)):
		return true
	case a == String && b == Slice && (u.Elem().Kind() == Uint8 || u.Elem().Kind() == Int32):
		return true
	}
	return false
}

func (t *rtype) mustBeFunc(method string) {
	if t.Kind() != Func {
		panic("reflect: " + method + " of non-func type " + t.String())
	}
}

func (t *rtype) In(i int) Type {
	t.mustBeFunc("In")
	if i < 0 || i >= t.NumIn() {
		panic("reflect: Function index out of range")
	}
	return toType(typeIn(t.ptr(), i))
}

func (t *rtype) Out(i int) Type {
	t.mustBeFunc("Out")
	if i < 0 || i >= t.NumOut() {
		panic("reflect: Function index out of range")
	}
	return toType(typeOut(t.ptr(), i))
}

func (t *rtype) NumIn() int  { t.mustBeFunc("NumIn"); return typeNumIn(t.ptr()) }
func (t *rtype) NumOut() int { t.mustBeFunc("NumOut"); return typeNumOut(t.ptr()) }

func (t *rtype) OverflowComplex(x complex128) bool {
	k := t.Kind()
	switch k {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic("reflect: OverflowComplex of non-complex type " + t.String())
}

func (t *rtype) OverflowFloat(x float64) bool {
	k := t.Kind()
	switch k {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic("reflect: OverflowFloat of non-float type " + t.String())
}

func (t *rtype) OverflowInt(x int64) bool {
	k := t.Kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowInt of non-int type " + t.String())
}

func (t *rtype) OverflowUint(x uint64) bool {
	k := t.Kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowUint of non-uint type " + t.String())
}

func overflowFloat32(x float64) bool {
	if x < 0 {
		x = -x
	}
	return 3.4028234663852886e38 < x && x <= 1.7976931348623157e308
}

// TypeOf returns the dynamic type of i (nil for nil).
func TypeOf(i any) Type { return toType(efaceType(i)) }

// TypeFor returns the Type that represents the type argument T.
func TypeFor[T any]() Type {
	var v T
	if t := TypeOf(v); t != nil {
		return t
	}
	return TypeOf((*T)(nil)).Elem()
}

// PtrTo returns the pointer type with element t.
func PtrTo(t Type) Type { return PointerTo(t) }

// PointerTo returns the pointer type with element t.
func PointerTo(t Type) Type { return toType(pointerTo(t.common().ptr())) }

func SliceOf(t Type) Type                       { panic("reflect.SliceOf is not supported by gd") }
func MapOf(key, elem Type) Type                 { panic("reflect.MapOf is not supported by gd") }
func ArrayOf(length int, elem Type) Type        { panic("reflect.ArrayOf is not supported by gd") }
func ChanOf(dir ChanDir, t Type) Type           { panic("reflect.ChanOf is not supported by gd") }
func StructOf(fields []StructField) Type        { panic("reflect.StructOf is not supported by gd") }
func FuncOf(in, out []Type, variadic bool) Type { panic("reflect.FuncOf is not supported by gd") }

// Method represents a single method.
type Method struct {
	Name    string
	PkgPath string
	Type    Type
	Func    Value
	Index   int
}

// IsExported reports whether the method is exported.
func (m Method) IsExported() bool { return m.PkgPath == "" }

// A StructField describes a single field in a struct.
type StructField struct {
	Name      string
	PkgPath   string
	Type      Type
	Tag       StructTag
	Offset    uintptr
	Index     []int
	Anonymous bool
}

// IsExported reports whether the field is exported.
func (f StructField) IsExported() bool { return f.PkgPath == "" }

// A StructTag is the tag string in a struct field.
type StructTag string

// Get returns the value associated with key in the tag string.
func (tag StructTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value associated with key in the tag string.
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]
		if key == name {
			value, err := strconv.Unquote(qvalue)
			if err != nil {
				break
			}
			return value, true
		}
	}
	return "", false
}

// StringHeader is the runtime representation of a string.
type StringHeader struct {
	Data uintptr
	Len  int
}

// SliceHeader is the runtime representation of a slice.
type SliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}
