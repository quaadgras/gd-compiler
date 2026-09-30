// Hooks of package reflect (see library/overlay/reflect), over gd's type descriptors.
#include <go/reflect.h>
#include <string.h>
#include <threads.h>

#define S(name) name##_go_reflect_package
#define T(p) ((const go_type*)(p).ptr)

// Descriptors of types that are not named are static in each file, so the same type may
// have several: canonical returns the first seen, so that reflect.Types can be compared.
typedef struct go_rtype_entry { const go_type* t; struct go_rtype_entry* next; } go_rtype_entry;
static go_rtype_entry* go_rtypes[1024];
static mtx_t go_rtypes_lock;
static once_flag go_rtypes_once = ONCE_FLAG_INIT;
static void go_rtypes_init(void) { mtx_init(&go_rtypes_lock, mtx_plain); }

static size_t go_rtype_hash(const char* name, go_kind kind) {
    size_t h = 1469598103934665603u ^ (size_t)kind;
    for (; name && *name; name++) h = (h ^ (unsigned char)*name) * 1099511628211u;
    return h % (sizeof go_rtypes / sizeof go_rtypes[0]);
}

// go_rtype_find returns the canonical descriptor of the type named name, or registers t
// (when t is not NULL).
static const go_type* go_rtype_find(const char* name, go_kind kind, const go_type* t) {
    call_once(&go_rtypes_once, go_rtypes_init);
    size_t h = go_rtype_hash(name, kind);
    mtx_lock(&go_rtypes_lock);
    for (go_rtype_entry* e = go_rtypes[h]; e; e = e->next) {
        if (e->t->kind == kind && strcmp(e->t->name, name) == 0) {
            mtx_unlock(&go_rtypes_lock);
            return e->t;
        }
    }
    if (t) {
        go_rtype_entry* e = go_new(sizeof(go_rtype_entry), NULL).ptr;
        e->t = t;
        e->next = go_rtypes[h];
        go_rtypes[h] = e;
    }
    mtx_unlock(&go_rtypes_lock);
    return t;
}

go_pt S(canonical)(go_pt t) {
    if (!t.ptr) return t;
    return (go_pt){ (void*)go_rtype_find(T(t)->name, T(t)->kind, T(t)) };
}

go_pt S(pointerTo)(go_pt t) {
    const char* elem = T(t)->name;
    size_t n = strlen(elem);
    char* name = go_new((go_ii)n + 2, NULL).ptr;
    name[0] = '*';
    memcpy(name + 1, elem, n + 1);
    const go_type* found = go_rtype_find(name, go_kind_pointer, NULL);
    if (found) return (go_pt){ (void*)found };
    go_type* p = go_new(sizeof(go_type), NULL).ptr;
    p->name = name;
    p->kind = go_kind_pointer;
    p->size = sizeof(go_pt);
    p->data.pointer.elem = T(t);
    return (go_pt){ (void*)go_rtype_find(name, go_kind_pointer, p) };
}

go_ss S(typeString)(go_pt t) { return go_string_new(T(t)->name); }
Kind_go_reflect_package S(typeKind)(go_pt t) { return (Kind_go_reflect_package)T(t)->kind; }
go_up S(typeSize)(go_pt t) { return (go_up)T(t)->size; }

go_pt S(typeElem)(go_pt t) {
    const go_type* typ = T(t);
    switch (typ->kind) {
    case go_kind_pointer: return (go_pt){ (void*)typ->data.pointer.elem };
    case go_kind_slice: return (go_pt){ (void*)typ->data.slice.elem };
    case go_kind_array: return (go_pt){ (void*)typ->data.array.elem };
    case go_kind_chan: return (go_pt){ (void*)typ->data.chan.elem };
    case go_kind_map: return (go_pt){ (void*)typ->data.map.elem };
    default: return (go_pt){0};
    }
}
go_pt S(typeKey)(go_pt t) { return (go_pt){ (void*)T(t)->data.map.key }; }
go_ii S(typeLen)(go_pt t) { return T(t)->data.array.len; }
go_ii S(typeChanDir)(go_pt t) { return T(t)->data.chan.dir; }

go_ii S(typeNumField)(go_pt t) { return T(t)->data.fields.count; }
static const go_field* go_rfield(go_pt t, go_ii i) {
    if (i < 0 || i >= T(t)->data.fields.count) go_panic_error("reflect: Field index out of bounds");
    return &T(t)->data.fields.field[i];
}
go_ss S(fieldName)(go_pt t, go_ii i) { return go_string_new(go_rfield(t, i)->name); }
go_pt S(fieldType)(go_pt t, go_ii i) { return (go_pt){ (void*)go_rfield(t, i)->type }; }
go_up S(fieldOffset)(go_pt t, go_ii i) { return (go_up)go_rfield(t, i)->offset; }
go_tf S(fieldExported)(go_pt t, go_ii i) { return go_rfield(t, i)->exported; }
go_tf S(fieldEmbedded)(go_pt t, go_ii i) { return go_rfield(t, i)->embedded; }

// The methods of types (other than interfaces) are their exported methods.
static go_tf go_rexported(const char* name) { return name[0] >= 'A' && name[0] <= 'Z'; }
go_ii S(typeNumMethod)(go_pt t) {
    const go_type* typ = T(t);
    if (typ->kind == go_kind_interface) return typ->data.interface.count;
    go_ii n = 0;
    for (go_ii i = 0; i < typ->nmethods; i++) n += go_rexported(typ->methods[i].name);
    return n;
}
go_ss S(methodName)(go_pt t, go_ii i) {
    const go_type* typ = T(t);
    if (typ->kind == go_kind_interface) return go_string_new(typ->data.interface.methods[i].name);
    for (go_ii j = 0; j < typ->nmethods; j++) {
        if (go_rexported(typ->methods[j].name) && i-- == 0) return go_string_new(typ->methods[j].name);
    }
    go_panic_error("reflect: Method index out of range");
}

go_tf S(typeImplements)(go_pt t, go_pt iface) {
    const go_type* i = T(iface);
    if (T(t)->kind == go_kind_interface) { // every method of iface is one of t's.
        for (go_ii m = 0; m < i->data.interface.count; m++) {
            go_tf found = false;
            for (go_ii n = 0; n < T(t)->data.interface.count; n++) {
                if (!strcmp(i->data.interface.methods[m].name, T(t)->data.interface.methods[n].name)) found = true;
            }
            if (!found) return false;
        }
        return true;
    }
    return go_implements(T(t), i->data.interface.methods, i->data.interface.count, NULL);
}

go_tf S(typeComparable)(go_pt t) {
    switch (T(t)->kind) {
    case go_kind_slice: case go_kind_map: case go_kind_func: return false;
    case go_kind_struct: case go_kind_array: return T(t)->equal != NULL || T(t)->size == 0;
    default: return true;
    }
}

go_pt S(efaceType)(go_vv i) { return (go_pt){ (void*)i.go_type }; }
go_pt S(efaceData)(go_vv i) { return i.ptr; }

go_vv S(packEface)(go_pt t, go_pt p) {
    if (T(t)->kind == go_kind_interface) return *(go_vv*)p.ptr; // (go_if starts like go_vv)
    return (go_vv){ go_new(T(t)->size, p.ptr), T(t) };
}

void S(storeInterface)(go_pt iface, go_pt dst, go_vv v) {
    const go_type* i = T(iface);
    if (i->data.interface.count == 0) { *(go_vv*)dst.ptr = v; return; }
    if (!v.go_type) { *(go_if*)dst.ptr = (go_if){0}; return; }
    *(go_if*)dst.ptr = go_to_iface(v, i->name, i->data.interface.methods, i->data.interface.count, true, NULL);
}

go_tf S(valuesEqual)(go_pt t, go_pt a, go_pt b) {
    if (T(t)->kind == go_kind_interface) return go_vv_eq(*(go_vv*)a.ptr, *(go_vv*)b.ptr);
    return go_vv_eq((go_vv){ a, T(t) }, (go_vv){ b, T(t) });
}

go_pt S(unsafeNew)(go_pt t) { return go_new(T(t)->size > 0 ? T(t)->size : 1, NULL); }
void S(memmove)(go_pt dst, go_pt src, go_up size) { if (size > 0) memmove(dst.ptr, src.ptr, (size_t)size); }

go_ii S(mapLen)(go_pt m) { return go_map_len(*(go_kv*)m.ptr); }
go_pt S(mapIndex)(go_pt m, go_pt key, go_pt t) {
    go_pt elem = go_new(T(t)->size > 0 ? T(t)->size : 1, NULL);
    return go_map_get(*(go_kv*)m.ptr, key.ptr, elem.ptr) ? elem : (go_pt){0};
}
void S(mapAssign)(go_pt m, go_pt key, go_pt elem) { go_map_set(*(go_kv*)m.ptr, key.ptr, elem.ptr); }
void S(mapDelete)(go_pt m, go_pt key) { go_map_delete(*(go_kv*)m.ptr, key.ptr); }
go_pt S(mapRange)(go_pt m) {
    go_map_iter it = go_map_range(*(go_kv*)m.ptr);
    return go_new(sizeof it, &it);
}
go_tf S(mapNext)(go_pt it, go_pt key, go_pt elem) { return go_map_next(it.ptr, key.ptr, elem.ptr); }

go_ii S(chanLen)(go_pt c) { return go_chan_len(*(go_ch*)c.ptr); }
go_ii S(chanCap)(go_pt c) { return go_chan_cap(*(go_ch*)c.ptr); }
