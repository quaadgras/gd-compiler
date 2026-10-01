//go:build ignore

package reflect

import (
	"strconv"
	"unsafe"
)

// Value is the reflection interface to a Go value: its type, and the memory that holds it.
type Value struct {
	typ  *rtype
	ptr  unsafe.Pointer
	flag flag
}

type flag uint

const (
	flagAddr flag = 1 << iota // the memory is a variable (so it can be set).
	flagRO                    // obtained through unexported fields.
)

// A ValueError occurs when a Value method is invoked on a Value that does not support it.
type ValueError struct {
	Method string
	Kind   Kind
}

func (e *ValueError) Error() string {
	if e.Kind == 0 {
		return "reflect: call of " + e.Method + " on zero Value"
	}
	return "reflect: call of " + e.Method + " on " + e.Kind.String() + " Value"
}

func (v Value) mustBe(method string, kinds ...Kind) {
	k := v.Kind()
	for _, want := range kinds {
		if k == want {
			return
		}
	}
	panic(&ValueError{"reflect.Value." + method, k})
}

func (v Value) mustBeAssignable() {
	if v.flag&flagRO != 0 {
		panic("reflect: reflect.Value.Set using value obtained using unexported field")
	}
	if v.flag&flagAddr == 0 {
		panic("reflect: reflect.Value.Set using unaddressable value")
	}
}

// ValueOf returns a new Value initialized to the concrete value stored in i.
func ValueOf(i any) Value {
	t := efaceType(i)
	if t == nil {
		return Value{}
	}
	return Value{(*rtype)(canonical(t)), efaceData(i), 0}
}

// Zero returns a Value representing the zero value for the specified type.
func Zero(typ Type) Value {
	if typ == nil {
		panic("reflect: Zero(nil)")
	}
	t := typ.common()
	return Value{t, unsafeNew(t.ptr()), 0}
}

// New returns a Value representing a pointer to a new zero value for the specified type.
func New(typ Type) Value {
	if typ == nil {
		panic("reflect: New(nil)")
	}
	p := unsafeNew(typ.common().ptr())
	return Value{PointerTo(typ).common(), unsafe.Pointer(&p), 0}
}

// NewAt returns a Value representing a pointer to a value of the specified type at p.
func NewAt(typ Type, p unsafe.Pointer) Value {
	return Value{PointerTo(typ).common(), unsafe.Pointer(&p), 0}
}

// Indirect returns the value that v points to (or v, if it's not a pointer).
func Indirect(v Value) Value {
	if v.Kind() != Pointer {
		return v
	}
	return v.Elem()
}

func (v Value) IsValid() bool { return v.typ != nil }

func (v Value) Kind() Kind {
	if v.typ == nil {
		return Invalid
	}
	return v.typ.Kind()
}

func (v Value) Type() Type {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.Type", Invalid})
	}
	return v.typ
}

func (v Value) CanAddr() bool { return v.flag&flagAddr != 0 }
func (v Value) CanSet() bool  { return v.flag&(flagAddr|flagRO) == flagAddr }
func (v Value) CanInterface() bool {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.CanInterface", Invalid})
	}
	return v.flag&flagRO == 0
}

// Interface returns v's current value as an interface{}.
func (v Value) Interface() any {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.Interface", Invalid})
	}
	if v.flag&flagRO != 0 {
		panic("reflect.Value.Interface: cannot return value obtained from unexported field or method")
	}
	return packEface(v.typ.ptr(), v.ptr)
}

func (v Value) Addr() Value {
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.Addr of unaddressable value")
	}
	p := v.ptr
	return Value{PointerTo(v.typ).common(), unsafe.Pointer(&p), v.flag & flagRO}
}

func (v Value) UnsafeAddr() uintptr {
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.UnsafeAddr of unaddressable value")
	}
	return uintptr(v.ptr)
}

func (v Value) Bool() bool {
	v.mustBe("Bool", Bool)
	return *(*bool)(v.ptr)
}

func (v Value) Int() int64 {
	switch v.Kind() {
	case Int:
		return int64(*(*int)(v.ptr))
	case Int8:
		return int64(*(*int8)(v.ptr))
	case Int16:
		return int64(*(*int16)(v.ptr))
	case Int32:
		return int64(*(*int32)(v.ptr))
	case Int64:
		return *(*int64)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Int", v.Kind()})
}

func (v Value) Uint() uint64 {
	switch v.Kind() {
	case Uint:
		return uint64(*(*uint)(v.ptr))
	case Uint8:
		return uint64(*(*uint8)(v.ptr))
	case Uint16:
		return uint64(*(*uint16)(v.ptr))
	case Uint32:
		return uint64(*(*uint32)(v.ptr))
	case Uint64:
		return *(*uint64)(v.ptr)
	case Uintptr:
		return uint64(*(*uintptr)(v.ptr))
	}
	panic(&ValueError{"reflect.Value.Uint", v.Kind()})
}

func (v Value) Float() float64 {
	switch v.Kind() {
	case Float32:
		return float64(*(*float32)(v.ptr))
	case Float64:
		return *(*float64)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Float", v.Kind()})
}

func (v Value) Complex() complex128 {
	switch v.Kind() {
	case Complex64:
		return complex128(*(*complex64)(v.ptr))
	case Complex128:
		return *(*complex128)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.Complex", v.Kind()})
}

func (v Value) CanInt() bool     { k := v.Kind(); return k >= Int && k <= Int64 }
func (v Value) CanUint() bool    { k := v.Kind(); return k >= Uint && k <= Uintptr }
func (v Value) CanFloat() bool   { k := v.Kind(); return k == Float32 || k == Float64 }
func (v Value) CanComplex() bool { k := v.Kind(); return k == Complex64 || k == Complex128 }

// String returns the string v's underlying value, as a string (or "<T Value>" for other kinds).
func (v Value) String() string {
	switch v.Kind() {
	case Invalid:
		return "<invalid Value>"
	case String:
		return *(*string)(v.ptr)
	}
	return "<" + v.Type().String() + " Value>"
}

func (v Value) Bytes() []byte {
	switch v.Kind() {
	case Slice:
		if v.typ.Elem().Kind() == Uint8 {
			return *(*[]byte)(v.ptr)
		}
	case Array:
		if v.typ.Elem().Kind() == Uint8 {
			if v.flag&flagAddr == 0 {
				panic("reflect.Value.Bytes of unaddressable byte array")
			}
			return unsafe.Slice((*byte)(v.ptr), v.Len())
		}
	}
	panic(&ValueError{"reflect.Value.Bytes", v.Kind()})
}

func (v Value) Len() int {
	switch v.Kind() {
	case Slice:
		return len(*(*[]byte)(v.ptr)) // (the header is the same for all slices)
	case String:
		return len(*(*string)(v.ptr))
	case Array:
		return v.typ.Len()
	case Map:
		return mapLen(v.ptr)
	case Chan:
		return chanLen(v.ptr)
	case Pointer:
		if v.typ.Elem().Kind() == Array {
			return v.typ.Elem().Len()
		}
	}
	panic(&ValueError{"reflect.Value.Len", v.Kind()})
}

func (v Value) Cap() int {
	switch v.Kind() {
	case Slice:
		return cap(*(*[]byte)(v.ptr))
	case Array:
		return v.typ.Len()
	case Chan:
		return chanCap(v.ptr)
	case Pointer:
		if v.typ.Elem().Kind() == Array {
			return v.typ.Elem().Len()
		}
	}
	panic(&ValueError{"reflect.Value.Cap", v.Kind()})
}

func (v Value) IsNil() bool {
	switch v.Kind() {
	case Chan, Func, Map, Pointer, UnsafePointer:
		return *(*unsafe.Pointer)(v.ptr) == nil
	case Interface:
		return efaceType(*(*any)(v.ptr)) == nil
	case Slice:
		return *(*unsafe.Pointer)(v.ptr) == nil // (the data of the slice)
	}
	panic(&ValueError{"reflect.Value.IsNil", v.Kind()})
}

func (v Value) IsZero() bool {
	switch v.Kind() {
	case Invalid:
		panic(&ValueError{"reflect.Value.IsZero", Invalid})
	case Bool:
		return !v.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == 0
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == 0
	case Float32, Float64:
		return v.Float() == 0 && !signbit(v.Float())
	case Complex64, Complex128:
		c := v.Complex()
		return real(c) == 0 && imag(c) == 0 && !signbit(real(c)) && !signbit(imag(c))
	case String:
		return v.Len() == 0
	case Chan, Func, Interface, Map, Pointer, Slice, UnsafePointer:
		return v.IsNil()
	case Array:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).IsZero() {
				return false
			}
		}
		return true
	case Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsZero() {
				return false
			}
		}
		return true
	}
	return false
}

func signbit(x float64) bool { return 1/x < 0 || (x == 0 && 1/x < 0) }

// Pointer returns v's value as a uintptr.
func (v Value) Pointer() uintptr {
	switch v.Kind() {
	case Chan, Func, Map, Pointer, Slice, UnsafePointer:
		return uintptr(*(*unsafe.Pointer)(v.ptr))
	}
	panic(&ValueError{"reflect.Value.Pointer", v.Kind()})
}

func (v Value) UnsafePointer() unsafe.Pointer {
	switch v.Kind() {
	case Chan, Func, Map, Pointer, Slice, UnsafePointer:
		return *(*unsafe.Pointer)(v.ptr)
	}
	panic(&ValueError{"reflect.Value.UnsafePointer", v.Kind()})
}

// Elem returns the value that the interface v contains or that the pointer v points to.
func (v Value) Elem() Value {
	switch v.Kind() {
	case Interface:
		x := ValueOf(*(*any)(v.ptr)) // (non-empty interfaces start like empty ones)
		x.flag |= v.flag & flagRO
		return x
	case Pointer:
		p := *(*unsafe.Pointer)(v.ptr)
		if p == nil {
			return Value{}
		}
		return Value{v.typ.Elem().common(), p, flagAddr | v.flag&flagRO}
	}
	panic(&ValueError{"reflect.Value.Elem", v.Kind()})
}

func (v Value) NumField() int {
	v.mustBe("NumField", Struct)
	return v.typ.NumField()
}

func (v Value) Field(i int) Value {
	v.mustBe("Field", Struct)
	if i < 0 || i >= v.typ.NumField() {
		panic("reflect: Field index out of range")
	}
	t := v.typ.ptr()
	fl := v.flag
	if !fieldExported(t, i) {
		fl |= flagRO
	}
	return Value{(*rtype)(canonical(fieldType(t, i))), unsafe.Add(v.ptr, fieldOffset(t, i)), fl}
}

func (v Value) FieldByIndex(index []int) Value {
	for i, x := range index {
		if i > 0 && v.Kind() == Pointer {
			if v.IsNil() {
				panic("reflect: indirection through nil pointer to embedded struct")
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v
}

func (v Value) FieldByName(name string) Value {
	if f, ok := v.typ.FieldByName(name); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

func (v Value) FieldByNameFunc(match func(string) bool) Value {
	if f, ok := v.typ.FieldByNameFunc(match); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// Index returns v's i'th element.
func (v Value) Index(i int) Value {
	switch v.Kind() {
	case Array:
		if i < 0 || i >= v.typ.Len() {
			panic("reflect: array index out of range")
		}
		elem := v.typ.Elem().common()
		return Value{elem, unsafe.Add(v.ptr, uintptr(i)*elem.Size()), v.flag}
	case Slice:
		s := *(*[]byte)(v.ptr)
		if i < 0 || i >= len(s) {
			panic("reflect: slice index out of range")
		}
		elem := v.typ.Elem().common()
		return Value{elem, unsafe.Add(unsafe.Pointer(unsafe.SliceData(s)), uintptr(i)*elem.Size()), flagAddr | v.flag&flagRO}
	case String:
		s := *(*string)(v.ptr)
		if i < 0 || i >= len(s) {
			panic("reflect: string index out of range")
		}
		b := s[i]
		return Value{TypeOf(b).common(), unsafe.Pointer(&b), v.flag & flagRO}
	}
	panic(&ValueError{"reflect.Value.Index", v.Kind()})
}

// Slice returns v[i:j].
func (v Value) Slice(i, j int) Value {
	switch v.Kind() {
	case Slice:
		s := *(*[]byte)(v.ptr)
		if i < 0 || j < i || j > cap(s) {
			panic("reflect.Value.Slice: slice index out of bounds")
		}
		size := v.typ.Elem().Size()
		r := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(s)), uintptr(i)*size)), 0)
		h := (*[3]int)(unsafe.Pointer(&r))
		h[1], h[2] = j-i, cap(s)-i
		return Value{v.typ, unsafe.Pointer(&r), v.flag & flagRO}
	case String:
		s := (*(*string)(v.ptr))[i:j]
		return Value{v.typ, unsafe.Pointer(&s), v.flag & flagRO}
	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice: slice of unaddressable array")
		}
		panic("reflect.Value.Slice of arrays is not supported by gd")
	}
	panic(&ValueError{"reflect.Value.Slice", v.Kind()})
}

// Set assigns x to the value v.
func (v Value) Set(x Value) {
	v.mustBeAssignable()
	if x.typ == nil {
		panic("reflect: Set with invalid Value")
	}
	if v.Kind() == Interface {
		storeInterface(v.typ.ptr(), v.ptr, packEface(x.typ.ptr(), x.ptr))
		return
	}
	if x.typ != v.typ {
		panic("reflect.Set: value of type " + x.typ.String() + " is not assignable to type " + v.typ.String())
	}
	memmove(v.ptr, x.ptr, v.typ.Size())
}

func (v Value) SetBool(x bool) {
	v.mustBeAssignable()
	v.mustBe("SetBool", Bool)
	*(*bool)(v.ptr) = x
}

func (v Value) SetInt(x int64) {
	v.mustBeAssignable()
	switch v.Kind() {
	case Int:
		*(*int)(v.ptr) = int(x)
	case Int8:
		*(*int8)(v.ptr) = int8(x)
	case Int16:
		*(*int16)(v.ptr) = int16(x)
	case Int32:
		*(*int32)(v.ptr) = int32(x)
	case Int64:
		*(*int64)(v.ptr) = x
	default:
		panic(&ValueError{"reflect.Value.SetInt", v.Kind()})
	}
}

func (v Value) SetUint(x uint64) {
	v.mustBeAssignable()
	switch v.Kind() {
	case Uint:
		*(*uint)(v.ptr) = uint(x)
	case Uint8:
		*(*uint8)(v.ptr) = uint8(x)
	case Uint16:
		*(*uint16)(v.ptr) = uint16(x)
	case Uint32:
		*(*uint32)(v.ptr) = uint32(x)
	case Uint64:
		*(*uint64)(v.ptr) = x
	case Uintptr:
		*(*uintptr)(v.ptr) = uintptr(x)
	default:
		panic(&ValueError{"reflect.Value.SetUint", v.Kind()})
	}
}

func (v Value) SetFloat(x float64) {
	v.mustBeAssignable()
	switch v.Kind() {
	case Float32:
		*(*float32)(v.ptr) = float32(x)
	case Float64:
		*(*float64)(v.ptr) = x
	default:
		panic(&ValueError{"reflect.Value.SetFloat", v.Kind()})
	}
}

func (v Value) SetComplex(x complex128) {
	v.mustBeAssignable()
	switch v.Kind() {
	case Complex64:
		*(*complex64)(v.ptr) = complex64(x)
	case Complex128:
		*(*complex128)(v.ptr) = x
	default:
		panic(&ValueError{"reflect.Value.SetComplex", v.Kind()})
	}
}

func (v Value) SetString(x string) {
	v.mustBeAssignable()
	v.mustBe("SetString", String)
	*(*string)(v.ptr) = x
}

func (v Value) SetBytes(x []byte) {
	v.mustBeAssignable()
	v.mustBe("SetBytes", Slice)
	*(*[]byte)(v.ptr) = x
}

func (v Value) SetLen(n int) {
	v.mustBeAssignable()
	v.mustBe("SetLen", Slice)
	h := (*[3]int)(v.ptr)
	if n < 0 || n > h[2] {
		panic("reflect: slice length out of range in SetLen")
	}
	h[1] = n
}

func (v Value) SetPointer(x unsafe.Pointer) {
	v.mustBeAssignable()
	v.mustBe("SetPointer", UnsafePointer)
	*(*unsafe.Pointer)(v.ptr) = x
}

// Clear clears the contents of a map or zeros the contents of a slice.
func (v Value) Clear() {
	switch v.Kind() {
	case Map:
		mapClear(v.ptr)
	case Slice:
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			memmove(e.ptr, unsafeNew(e.typ.ptr()), e.typ.Size())
		}
	default:
		panic(&ValueError{"reflect.Value.Clear", v.Kind()})
	}
}

// Grow increases the slice's capacity, if necessary, to guarantee space for another n
// elements.
func (v Value) Grow(n int) {
	v.mustBeAssignable()
	v.mustBe("Grow", Slice)
	if n < 0 {
		panic("reflect.Value.Grow: negative len")
	}
	if v.Len()+n > v.Cap() {
		g := grow(v, n)
		h := (*[3]int)(g.ptr)
		h[1] = v.Len()
		memmove(v.ptr, g.ptr, unsafe.Sizeof([]byte(nil)))
	}
}

func (v Value) SetZero() {
	v.mustBeAssignable()
	memmove(v.ptr, unsafeNew(v.typ.ptr()), v.typ.Size())
}

func (v Value) OverflowInt(x int64) bool     { return v.Type().OverflowInt(x) }
func (v Value) OverflowUint(x uint64) bool   { return v.Type().OverflowUint(x) }
func (v Value) OverflowFloat(x float64) bool { return v.Type().OverflowFloat(x) }
func (v Value) OverflowComplex(x complex128) bool {
	return v.Type().OverflowComplex(x)
}

func (v Value) Comparable() bool {
	if v.typ == nil {
		return true
	}
	return v.typ.Comparable()
}

// Equal reports whether v is equal to u (as with ==, panicking for uncomparable types).
func (v Value) Equal(u Value) bool {
	if v.Kind() == Interface {
		v = v.Elem()
	}
	if u.Kind() == Interface {
		u = u.Elem()
	}
	if !v.IsValid() || !u.IsValid() {
		return v.IsValid() == u.IsValid()
	}
	if v.typ != u.typ {
		return false
	}
	return valuesEqual(v.typ.ptr(), v.ptr, u.ptr)
}

func (v Value) NumMethod() int {
	if v.typ == nil {
		panic(&ValueError{"reflect.Value.NumMethod", Invalid})
	}
	return v.typ.NumMethod()
}

func (v Value) Method(i int) Value { panic("reflect.Value.Method is not supported by gd") }
func (v Value) MethodByName(name string) Value {
	panic("reflect.Value.MethodByName is not supported by gd")
}
func (v Value) Call(in []Value) []Value      { panic("reflect.Value.Call is not supported by gd") }
func (v Value) CallSlice(in []Value) []Value { panic("reflect.Value.CallSlice is not supported by gd") }
func (v Value) Send(x Value)                 { panic("reflect.Value.Send is not supported by gd") }
func (v Value) Recv() (Value, bool)          { panic("reflect.Value.Recv is not supported by gd") }
func (v Value) TrySend(x Value) bool         { panic("reflect.Value.TrySend is not supported by gd") }
func (v Value) TryRecv() (Value, bool)       { panic("reflect.Value.TryRecv is not supported by gd") }
func (v Value) Close()                       { panic("reflect.Value.Close is not supported by gd") }
func (v Value) Convert(t Type) Value         { return convert(v, t) }
func (v Value) CanConvert(t Type) bool       { return v.Type().ConvertibleTo(t) }
func (v Value) Seq() func(func(Value) bool)  { panic("reflect.Value.Seq is not supported by gd") }
func (v Value) Seq2() func(func(Value, Value) bool) {
	panic("reflect.Value.Seq2 is not supported by gd")
}

// convert converts v to t, for numeric types, and strings.
func convert(v Value, t Type) Value {
	out := New(t).Elem()
	src, dst := v.Kind(), t.Kind()
	switch {
	case v.Type().AssignableTo(t):
		out.Set(v)
	case v.CanInt() && out.CanInt():
		out.SetInt(v.Int())
	case v.CanInt() && out.CanUint():
		out.SetUint(uint64(v.Int()))
	case v.CanUint() && out.CanInt():
		out.SetInt(int64(v.Uint()))
	case v.CanUint() && out.CanUint():
		out.SetUint(v.Uint())
	case v.CanInt() && out.CanFloat():
		out.SetFloat(float64(v.Int()))
	case v.CanUint() && out.CanFloat():
		out.SetFloat(float64(v.Uint()))
	case v.CanFloat() && out.CanInt():
		out.SetInt(int64(v.Float()))
	case v.CanFloat() && out.CanUint():
		out.SetUint(uint64(v.Float()))
	case v.CanFloat() && out.CanFloat():
		out.SetFloat(v.Float())
	case v.CanComplex() && out.CanComplex():
		out.SetComplex(v.Complex())
	case src == String && dst == String:
		out.SetString(v.String())
	case (v.CanInt() || v.CanUint()) && dst == String:
		r := rune(0xFFFD)
		if v.CanInt() {
			r = rune(v.Int())
		} else if v.Uint() <= 0x10FFFF {
			r = rune(v.Uint())
		}
		out.SetString(string(r))
	case src == Slice && dst == String && v.Type().Elem().Kind() == Uint8:
		out.SetString(string(v.Bytes()))
	case src == String && dst == Slice && t.Elem().Kind() == Uint8:
		b := []byte(v.String())
		memmove(out.ptr, unsafe.Pointer(&b), unsafe.Sizeof(b))
	default:
		panic("reflect.Value.Convert: value of type " + v.Type().String() + " cannot be converted to type " + t.String())
	}
	out.flag = v.flag & flagRO
	return out
}

// MakeSlice creates a new zero-initialized slice value for the specified slice type.
func MakeSlice(typ Type, len, cap int) Value {
	if typ.Kind() != Slice {
		panic("reflect.MakeSlice of non-slice type")
	}
	if len < 0 || cap < len {
		panic("reflect.MakeSlice: len out of range")
	}
	size := typ.Elem().Size()
	data := make([]byte, uintptr(cap)*size+1) // (+1, for zero-sized elements)
	s := unsafe.Slice(unsafe.SliceData(data), 0)
	h := (*[3]int)(unsafe.Pointer(&s))
	h[1], h[2] = len, cap
	return Value{typ.common(), unsafe.Pointer(&s), 0}
}

// Append appends the values x to a slice s and returns the resulting slice.
func Append(s Value, x ...Value) Value {
	s.mustBe("Append", Slice)
	n := s.Len()
	out := grow(s, len(x))
	for i, v := range x {
		out.Index(n + i).Set(v)
	}
	return out
}

// AppendSlice appends a slice t to a slice s and returns the resulting slice.
func AppendSlice(s, t Value) Value {
	s.mustBe("AppendSlice", Slice)
	t.mustBe("AppendSlice", Slice)
	n := s.Len()
	out := grow(s, t.Len())
	for i := 0; i < t.Len(); i++ {
		out.Index(n + i).Set(t.Index(i))
	}
	return out
}

// grow returns a copy of the slice s, with n more elements.
func grow(s Value, n int) Value {
	length := s.Len()
	out := MakeSlice(s.typ, length+n, max(length+n, 2*s.Cap()))
	Copy(out, s)
	return out
}

// Copy copies the contents of src into dst until either dst has been filled or src has
// been exhausted, and returns the number of elements copied.
func Copy(dst, src Value) int {
	n := min(dst.Len(), src.Len())
	if src.Kind() == String {
		s := src.String()
		for i := 0; i < n; i++ {
			*(*byte)(dst.Index(i).ptr) = s[i]
		}
		return n
	}
	size := dst.typ.Elem().Size()
	if n > 0 {
		memmove(dst.Index(0).ptr, src.Index(0).ptr, uintptr(n)*size)
	}
	return n
}

// Swapper returns a function that swaps the elements in the provided slice.
func Swapper(slice any) func(i, j int) {
	v := ValueOf(slice)
	if v.Kind() != Slice {
		panic(&ValueError{Method: "Swapper", Kind: v.Kind()})
	}
	size := v.typ.Elem().Size()
	tmp := make([]byte, size)
	return func(i, j int) {
		a, b := v.Index(i).ptr, v.Index(j).ptr
		memmove(unsafe.Pointer(unsafe.SliceData(tmp)), a, size)
		memmove(a, b, size)
		memmove(b, unsafe.Pointer(unsafe.SliceData(tmp)), size)
	}
}

// Maps.

func (v Value) MapIndex(key Value) Value {
	v.mustBe("MapIndex", Map)
	k := v.keyOf(key)
	elem := mapIndex(v.ptr, k, v.typ.Elem().common().ptr())
	if elem == nil {
		return Value{}
	}
	return Value{v.typ.Elem().common(), elem, v.flag & flagRO}
}

// keyOf returns a pointer to key, as a key of the map v (converted to its interface type).
func (v Value) keyOf(key Value) unsafe.Pointer {
	kt := v.typ.Key().common()
	if kt.Kind() == Interface && key.typ != kt {
		k := New(kt).Elem()
		k.Set(key)
		return k.ptr
	}
	return key.ptr
}

func (v Value) MapKeys() []Value {
	v.mustBe("MapKeys", Map)
	var keys []Value
	it := v.MapRange()
	for it.Next() {
		keys = append(keys, it.Key())
	}
	return keys
}

func (v Value) SetMapIndex(key, elem Value) {
	v.mustBe("SetMapIndex", Map)
	k := v.keyOf(key)
	if !elem.IsValid() {
		mapDelete(v.ptr, k)
		return
	}
	e := elem.ptr
	if et := v.typ.Elem().common(); et.Kind() == Interface && elem.typ != et {
		x := New(et).Elem()
		x.Set(elem)
		e = x.ptr
	}
	mapAssign(v.ptr, k, e)
}

func MakeMap(typ Type) Value                { panic("reflect.MakeMap is not supported by gd") }
func MakeMapWithSize(typ Type, n int) Value { panic("reflect.MakeMapWithSize is not supported by gd") }
func MakeChan(typ Type, buffer int) Value   { panic("reflect.MakeChan is not supported by gd") }
func MakeFunc(typ Type, fn func(args []Value) (results []Value)) Value {
	panic("reflect.MakeFunc is not supported by gd")
}

// A MapIter is an iterator for ranging over a map.
type MapIter struct {
	m        Value
	it       unsafe.Pointer
	key, val Value
}

func (v Value) MapRange() *MapIter {
	v.mustBe("MapRange", Map)
	return &MapIter{m: v}
}

func (iter *MapIter) Next() bool {
	if iter.it == nil {
		iter.it = mapRange(iter.m.ptr)
	}
	k := New(iter.m.typ.Key()).Elem()
	e := New(iter.m.typ.Elem()).Elem()
	if !mapNext(iter.it, k.ptr, e.ptr) {
		iter.key, iter.val = Value{}, Value{}
		return false
	}
	k.flag, e.flag = iter.m.flag&flagRO, iter.m.flag&flagRO
	iter.key, iter.val = k, e
	return true
}

func (iter *MapIter) Key() Value {
	if !iter.key.IsValid() {
		panic("MapIter.Key called before Next")
	}
	return iter.key
}

func (iter *MapIter) Value() Value {
	if !iter.val.IsValid() {
		panic("MapIter.Value called before Next")
	}
	return iter.val
}

func (iter *MapIter) Reset(v Value) {
	iter.m, iter.it = v, nil
	iter.key, iter.val = Value{}, Value{}
}

func (v Value) SetIterKey(iter *MapIter)   { v.Set(iter.Key()) }
func (v Value) SetIterValue(iter *MapIter) { v.Set(iter.Value()) }

// Select.

type SelectDir int

const (
	_ SelectDir = iota
	SelectSend
	SelectRecv
	SelectDefault
)

type SelectCase struct {
	Dir  SelectDir
	Chan Value
	Send Value
}

func Select(cases []SelectCase) (chosen int, recv Value, recvOK bool) {
	panic("reflect.Select is not supported by gd")
}

// DeepEqual reports whether x and y are "deeply equal".
func DeepEqual(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1, v2 := ValueOf(x), ValueOf(y)
	if v1.typ != v2.typ {
		return false
	}
	return deepValueEqual(v1, v2, make(map[visit]bool))
}

type visit struct {
	a1, a2 unsafe.Pointer
	typ    *rtype
}

func deepValueEqual(v1, v2 Value, visited map[visit]bool) bool {
	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}
	if v1.typ != v2.typ {
		return false
	}
	switch v1.Kind() {
	case Map, Slice, Pointer, Interface:
		if v1.Kind() != Interface {
			p1, p2 := *(*unsafe.Pointer)(v1.ptr), *(*unsafe.Pointer)(v2.ptr)
			if p1 != nil && p2 != nil {
				key := visit{p1, p2, v1.typ}
				if visited[key] {
					return true
				}
				visited[key] = true
			}
		}
	}
	switch v1.Kind() {
	case Array:
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited) {
				return false
			}
		}
		return true
	case Slice:
		if v1.IsNil() != v2.IsNil() || v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited) {
				return false
			}
		}
		return true
	case Interface:
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited)
	case Pointer:
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited)
	case Struct:
		for i := 0; i < v1.NumField(); i++ {
			if !deepValueEqual(v1.Field(i), v2.Field(i), visited) {
				return false
			}
		}
		return true
	case Map:
		if v1.IsNil() != v2.IsNil() || v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		it := v1.MapRange()
		for it.Next() {
			val1 := it.Value()
			val2 := v2.MapIndex(it.Key())
			if !val2.IsValid() || !deepValueEqual(val1, val2, visited) {
				return false
			}
		}
		return true
	case Func:
		return v1.IsNil() && v2.IsNil()
	case Float32, Float64:
		return v1.Float() == v2.Float()
	case Complex64, Complex128:
		return v1.Complex() == v2.Complex()
	}
	return valuesEqual(v1.typ.ptr(), v1.ptr, v2.ptr)
}

var _ = strconv.Itoa
