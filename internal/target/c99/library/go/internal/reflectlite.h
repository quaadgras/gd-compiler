#ifndef go_internal_reflectlite_package_imported
#define go_internal_reflectlite_package_imported
// Package internal/reflectlite, implemented natively (with the type descriptors of gd,
// rather than those of the Go runtime), for the packages that use it: sort, errors...
#include <go.h>
#include <string.h>

#define go_rl(name) name##_go_internal_reflectlite_package

// A Value is the type and data of a value, which can be set when it's addressable.
typedef struct { const go_type* typ; void* ptr; go_tf addressable; } go_rl(Value);

// A Type is an interface value, whose data is its type descriptor (the dynamic type being
// *reflectlite.rtype), with this table of methods (in the order of go/types).
typedef struct {
    go_tf (*AssignableTo)(void*, go_if);
    go_tf (*Comparable)(void*);
    go_if (*Elem)(void*);
    go_tf (*Implements)(void*, go_if);
    go_u1 (*Kind)(void*);
    go_ss (*Name)(void*);
    go_ss (*PkgPath)(void*);
    go_up (*Size)(void*);
    go_ss (*String)(void*);
    go_pt (*common)(void*);
    go_pt (*uncommon)(void*);
} go_rl(Type);

static const go_type go_rl(go_type_rtype) = {.name="*reflectlite.rtype", .kind=go_kind_pointer, .size=sizeof(go_pt)};
static const go_type go_type_Value_go_internal_reflectlite_package = {.name="reflectlite.Value", .kind=go_kind_struct, .size=sizeof(go_rl(Value))};
static const go_type go_type_Type_go_internal_reflectlite_package = {.name="reflectlite.Type", .kind=go_kind_interface, .size=sizeof(go_if)};

static inline go_if go_rl(TypeOf_desc)(const go_type* t);

static inline const go_type* go_rl(desc)(go_if t) { return t.ptr.ptr; }

static inline go_ss go_rl(rtype_String)(void* t) { return go_string_new(((const go_type*)t)->name); }
static inline go_ss go_rl(rtype_Name)(void* t) { // the name of a named type (unnamed types' names start with *, [, map...).
    const char* name = ((const go_type*)t)->name;
    if (!name || !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z') || name[0] == '_')) return go_string_new("");
    if (!strncmp(name, "map[", 4) || !strncmp(name, "chan ", 5) || !strncmp(name, "func(", 5) || !strncmp(name, "struct{", 7) || !strncmp(name, "interface {", 11)) return go_string_new("");
    const char* dot = strrchr(name, '.');
    const char* bracket = strchr(name, '[');
    if (dot && (!bracket || dot < bracket)) name = dot + 1;
    return go_string_new(name);
}
static inline go_ss go_rl(rtype_PkgPath)(void* t) { (void)t; return go_string_new(""); }
static inline go_up go_rl(rtype_Size)(void* t) { return (go_up)((const go_type*)t)->size; }
static inline go_u1 go_rl(rtype_Kind)(void* t) { return (go_u1)((const go_type*)t)->kind; }
static inline go_tf go_rl(rtype_Implements)(void* t, go_if u) {
    const go_type* i = go_rl(desc)(u);
    if (!i || i->kind != go_kind_interface) go_panic_error("reflect: non-interface type passed to Type.Implements");
    return go_implements(t, i->data.interface.methods, i->data.interface.count, NULL);
}
static inline go_tf go_rl(rtype_AssignableTo)(void* t, go_if u) {
    const go_type* to = go_rl(desc)(u);
    if (!to) go_panic_error("reflect: nil type passed to Type.AssignableTo");
    return go_type_eq(t, to) || (to->kind == go_kind_interface && go_implements(t, to->data.interface.methods, to->data.interface.count, NULL));
}
static inline go_tf go_rl(rtype_Comparable)(void* t) {
    const go_type* typ = t;
    switch (typ->kind) {
    case go_kind_slice: case go_kind_map: case go_kind_func: return false;
    case go_kind_struct: case go_kind_array: return typ->equal != NULL;
    default: return true;
    }
}
static inline const go_type* go_rl(elem)(const go_type* t) {
    switch (t->kind) {
    case go_kind_pointer: return t->data.pointer.elem;
    case go_kind_slice: return t->data.slice.elem;
    case go_kind_array: return t->data.array.elem;
    case go_kind_chan: return t->data.chan.elem;
    case go_kind_map: return t->data.map.elem;
    default: go_panic_error("reflect: Elem of invalid type %s", t->name);
    }
}
static inline go_if go_rl(rtype_Elem)(void* t) { return go_rl(TypeOf_desc)(go_rl(elem)(t)); }
static inline go_pt go_rl(rtype_common)(void* t) { return (go_pt){ t }; }
static inline go_pt go_rl(rtype_uncommon)(void* t) { (void)t; return (go_pt){0}; }

static go_rl(Type) go_rl(rtype_methods) = {
    go_rl(rtype_AssignableTo), go_rl(rtype_Comparable), go_rl(rtype_Elem), go_rl(rtype_Implements),
    go_rl(rtype_Kind), go_rl(rtype_Name), go_rl(rtype_PkgPath), go_rl(rtype_Size), go_rl(rtype_String),
    go_rl(rtype_common), go_rl(rtype_uncommon),
};

static inline go_if go_rl(TypeOf_desc)(const go_type* t) {
    if (!t) return (go_if){0};
    return (go_if){ .ptr = (go_pt){ (void*)t }, .go_type = &go_rl(go_type_rtype), .vtable = &go_rl(rtype_methods) };
}
static inline go_if go_rl(TypeOf)(go_vv i) { return go_rl(TypeOf_desc)(i.go_type); }

static inline go_rl(Value) go_rl(ValueOf)(go_vv i) { return (go_rl(Value)){ i.go_type, i.ptr.ptr, false }; }

static inline go_u1 go_rl(Value_Kind)(go_rl(Value) v) { return v.typ ? (go_u1)v.typ->kind : 0; }
static inline go_tf go_rl(Value_IsValid)(go_rl(Value) v) { return v.typ != NULL; }
static inline go_tf go_rl(Value_CanSet)(go_rl(Value) v) { return v.addressable; }
static inline go_if go_rl(Value_Type)(go_rl(Value) v) {
    if (!v.typ) go_panic_error("reflect: call of reflect.Value.Type on zero Value");
    return go_rl(TypeOf_desc)(v.typ);
}
static inline go_ii go_rl(Value_Len)(go_rl(Value) v) {
    switch (v.typ ? v.typ->kind : go_kind_invalid) {
    case go_kind_slice: return ((go_ll*)v.ptr)->len;
    case go_kind_string: return ((go_ss*)v.ptr)->len;
    case go_kind_array: return v.typ->data.array.len;
    case go_kind_map: return go_map_len(*(go_kv*)v.ptr);
    case go_kind_chan: return go_chan_len(*(go_ch*)v.ptr);
    case go_kind_pointer:
        if (v.typ->data.pointer.elem->kind == go_kind_array) return v.typ->data.pointer.elem->data.array.len;
        /* fallthrough */
    default: go_panic_error("reflect: call of reflect.Value.Len on %s value", v.typ ? v.typ->name : "zero");
    }
}
static inline go_tf go_rl(Value_IsNil)(go_rl(Value) v) {
    switch (v.typ ? v.typ->kind : go_kind_invalid) {
    case go_kind_pointer: case go_kind_unsafe_pointer: return ((go_pt*)v.ptr)->ptr == NULL;
    case go_kind_map: case go_kind_chan: return *(void**)v.ptr == NULL;
    case go_kind_slice: return ((go_ll*)v.ptr)->ptr.ptr == NULL;
    case go_kind_func: return ((go_fn*)v.ptr)->ptr == NULL;
    case go_kind_interface: return ((go_vv*)v.ptr)->go_type == NULL;
    default: go_panic_error("reflect: call of reflect.Value.IsNil on %s value", v.typ ? v.typ->name : "zero");
    }
}
static inline go_rl(Value) go_rl(Value_Elem)(go_rl(Value) v) {
    switch (v.typ ? v.typ->kind : go_kind_invalid) {
    case go_kind_interface: { // go_vv and go_if start with the data and dynamic type.
        go_vv x = *(go_vv*)v.ptr;
        return (go_rl(Value)){ x.go_type, x.ptr.ptr, false };
    }
    case go_kind_pointer: {
        void* p = ((go_pt*)v.ptr)->ptr;
        if (!p) return (go_rl(Value)){0};
        return (go_rl(Value)){ v.typ->data.pointer.elem, p, true };
    }
    default: go_panic_error("reflect: call of reflect.Value.Elem on %s value", v.typ ? v.typ->name : "zero");
    }
}
static inline void go_rl(Value_Set)(go_rl(Value) v, go_rl(Value) x) {
    if (!v.addressable) go_panic_error("reflect: reflect.Value.Set using unaddressable value");
    if (v.typ->kind == go_kind_interface) {
        go_vv value = { .go_type = x.typ };
        if (x.typ) value.ptr = go_new(x.typ->size, x.ptr);
        if (v.typ->data.interface.count == 0) *(go_vv*)v.ptr = value;
        else *(go_if*)v.ptr = go_to_iface(value, v.typ->name, v.typ->data.interface.methods, v.typ->data.interface.count, true, NULL);
        return;
    }
    if (!go_type_eq(v.typ, x.typ)) go_panic_error("reflect.Set: value of type %s is not assignable to type %s", x.typ ? x.typ->name : "nil", v.typ->name);
    if (v.typ->size > 0) memmove(v.ptr, x.ptr, (size_t)v.typ->size);
}

// Swapper returns a function that swaps the elements of the slice.
typedef struct { go_ll slice; go_ii size; } go_rl(swapper);
static void go_rl(swap)(void* env, go_ii i, go_ii j) {
    go_rl(swapper)* s = env;
    if ((go_u8)i >= (go_u8)s->slice.len || (go_u8)j >= (go_u8)s->slice.len) go_panic_error("reflect: slice index out of range");
    char* a = (char*)s->slice.ptr.ptr + i * s->size;
    char* b = (char*)s->slice.ptr.ptr + j * s->size;
    char tmp[64];
    for (go_ii off = 0; off < s->size; off += (go_ii)sizeof tmp) { // (in chunks, as C11 may not have VLAs)
        size_t n = (size_t)(s->size - off) < sizeof tmp ? (size_t)(s->size - off) : sizeof tmp;
        memcpy(tmp, a + off, n);
        memcpy(a + off, b + off, n);
        memcpy(b + off, tmp, n);
    }
}
static inline go_fn go_rl(Swapper)(go_vv slice) {
    if (!slice.go_type || slice.go_type->kind != go_kind_slice) go_panic_error("reflect: call of Swapper on %s value", slice.go_type ? slice.go_type->name : "nil");
    go_rl(swapper) s = { *(go_ll*)slice.ptr.ptr, slice.go_type->data.slice.elem->size };
    return go_make_closure(go_rl(swap), go_new(sizeof s, &s).ptr);
}

#undef go_rl
#endif // go_internal_reflectlite_package_imported
