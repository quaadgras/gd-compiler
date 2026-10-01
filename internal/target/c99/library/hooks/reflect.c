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
            if (t && kind == go_kind_func && !e->t->data.func.call && t->data.func.call) { // (made by FuncOf)
                ((go_type*)e->t)->data.func.call = t->data.func.call;
                ((go_type*)e->t)->data.func.makefunc = t->data.func.makefunc;
            }
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
    if (!t.ptr || T(t)->local) return t; // (local types are not the same as others of the same name)
    return (go_pt){ (void*)go_rtype_find(T(t)->name, T(t)->kind, T(t)) };
}

go_pt S(pointerTo)(go_pt t) {
    const char* elem = T(t)->name;
    size_t n = strlen(elem);
    char* name = go_new((go_ii)n + 2, NULL).ptr;
    name[0] = '*';
    memcpy(name + 1, elem, n + 1);
    if (T(t)->ptrto) return (go_pt){ (void*)go_rtype_find(name, go_kind_pointer, T(t)->ptrto) };
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
go_ss S(fieldPkgPath)(go_pt t, go_ii i) { const char* pkg = go_rfield(t, i)->pkg; return pkg ? go_string_new(pkg) : (go_ss){0}; }
go_ss S(typePkgPath)(go_pt t) { const char* pkg = T(t)->pkg; return pkg ? go_string_new(pkg) : (go_ss){0}; }
go_ss S(fieldTag)(go_pt t, go_ii i) { const char* tag = go_rfield(t, i)->tag; return tag ? go_string_new(tag) : (go_ss){0}; }

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

static const go_method* go_rmethod(go_pt t, go_ii i) {
    const go_type* typ = T(t);
    for (go_ii j = 0; j < typ->nmethods; j++) {
        if (go_rexported(typ->methods[j].name) && i-- == 0) return &typ->methods[j];
    }
    go_panic_error("reflect: Method index out of range");
}
go_pt S(methodType)(go_pt t, go_ii i) { return (go_pt){ (void*)go_rmethod(t, i)->mtype }; }
go_pt S(methodFuncType)(go_pt t, go_ii i) { return (go_pt){ (void*)go_rmethod(t, i)->ftype }; }
// methodFunc stores the method expression of the i'th method of t.
void S(methodFunc)(go_pt t, go_ii i, go_pt dst) { *(go_fn*)dst.ptr = (go_fn){ go_rmethod(t, i)->func, NULL }; }
// methodValue stores the method value of the i'th method of t, of the receiver at recv.
void S(methodValue)(go_pt t, go_ii i, go_pt recv, go_pt dst) {
    *(go_fn*)dst.ptr = (go_fn){ go_rmethod(t, i)->fn, recv.ptr };
}

// Functions: their parameters and results, calls (through the call function of the type)
// and making them (MakeFunc's functions call go_makefunc_call, with their environment).
go_ii S(typeNumIn)(go_pt t) { return T(t)->data.func.nin; }
go_ii S(typeNumOut)(go_pt t) { return T(t)->data.func.nout; }
go_pt S(typeIn)(go_pt t, go_ii i) { return (go_pt){ (void*)T(t)->data.func.in[i] }; }
go_pt S(typeOut)(go_pt t, go_ii i) { return (go_pt){ (void*)T(t)->data.func.out[i] }; }
go_tf S(typeVariadic)(go_pt t) { return T(t)->data.func.variadic; }
void S(callFunc)(go_pt t, go_pt fn, go_pt args, go_pt results) {
    if (!T(t)->data.func.call) go_panic_error("reflect: call of a function of type %s, which gd can't call", T(t)->name);
    T(t)->data.func.call(*(go_fn*)fn.ptr, args.ptr, results.ptr);
}
void S(makeFunc)(go_pt t, go_pt env, go_pt dst) {
    if (!T(t)->data.func.makefunc) go_panic_error("reflect.MakeFunc: gd can't make functions of type %s", T(t)->name);
    *(go_fn*)dst.ptr = (go_fn){ T(t)->data.func.makefunc, env.ptr };
}
static void go_reflect_makefunc(void* env, void** args, void** results) {
    go_tf token = go_take_recover(); // (for the function that MakeFunc wraps, if it is a deferred call)
    makeFuncCall_go_reflect_package((go_pt){ env }, (go_pt){ args }, (go_pt){ results }, token);
}
void S(giveRecover)(go_tf token) { go_give_recover(token); }
void S(registerMakeFunc)(void) { go_makefunc_call = go_reflect_makefunc; }

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

go_tf S(typeComparable)(go_pt t) { return go_type_comparable(T(t)); }

// Types made by reflection (SliceOf and others), canonical like the others, so that the
// same type made twice (or compiled) is the same reflect.Type.
static char* go_rconcat(const char* a, const char* b, const char* c, const char* d) {
    size_t n = strlen(a) + strlen(b) + strlen(c) + strlen(d);
    char* s = go_new((go_ii)n + 1, NULL).ptr;
    strcat(strcat(strcat(strcpy(s, a), b), c), d);
    return s;
}
static go_pt go_rmake(go_type* t) { return (go_pt){ (void*)go_rtype_find(t->name, t->kind, t) }; }
static go_type* go_rnew(char* name, go_kind kind, go_ii size) {
    const go_type* found = go_rtype_find(name, kind, NULL);
    if (found) return (go_type*)found;
    go_type* t = go_new(sizeof(go_type), NULL).ptr;
    t->name = name;
    t->kind = kind;
    t->size = size;
    return t;
}

// go_ralign returns the alignment of values of t, as C lays them out.
static go_ii go_ralign(const go_type* t) {
    switch (t->kind) {
    case go_kind_bool: case go_kind_int8: case go_kind_uint8: return 1;
    case go_kind_int16: case go_kind_uint16: return 2;
    case go_kind_int32: case go_kind_uint32: case go_kind_float32: case go_kind_complex64: return 4;
    case go_kind_array: return go_ralign(t->data.array.elem);
    case go_kind_struct: {
        go_ii align = 1;
        for (go_ii i = 0; i < t->data.fields.count; i++) {
            go_ii a = go_ralign(t->data.fields.field[i].type);
            if (a > align) align = a;
        }
        return align;
    }
    default: return 8;
    }
}

go_pt S(sliceOf)(go_pt elem) {
    go_type* t = go_rnew(go_rconcat("[]", T(elem)->name, "", ""), go_kind_slice, sizeof(go_ll));
    if (!t->data.slice.elem) t->data.slice.elem = T(elem);
    return go_rmake(t);
}
go_pt S(arrayOf)(go_ii n, go_pt elem) {
    char len[32];
    snprintf(len, sizeof len, "[%lld]", (long long)n);
    if (n < 0 || (T(elem)->size > 0 && n > ((go_ii)1 << 47) / T(elem)->size)) go_panic_error("reflect.ArrayOf: array size would exceed virtual address space");
    go_type* t = go_rnew(go_rconcat(len, T(elem)->name, "", ""), go_kind_array, n * T(elem)->size);
    if (!t->data.array.elem) t->data.array = (go_type_array){ T(elem), n };
    return go_rmake(t);
}
go_pt S(mapOf)(go_pt key, go_pt elem) {
    if (!go_type_comparable(T(key))) go_panic_error("reflect.MapOf: invalid key type %s", T(key)->name);
    go_type* t = go_rnew(go_rconcat("map[", T(key)->name, "]", T(elem)->name), go_kind_map, sizeof(go_kv));
    if (!t->data.map.key) t->data.map = (go_type_map){ T(key), T(elem) };
    return go_rmake(t);
}
go_pt S(chanOf)(go_ii dir, go_pt elem) {
    const char* prefix = dir == 1 ? "chan<- " : dir == 2 ? "<-chan " : "chan "; // (go/types' directions)
    go_type* t = go_rnew(go_rconcat(prefix, T(elem)->name, "", ""), go_kind_chan, sizeof(go_ch));
    if (!t->data.chan.elem) t->data.chan = (go_type_chan){ T(elem), dir };
    return go_rmake(t);
}
// structOf makes a struct type of n fields (with names, types, tags and whether they are
// exported or embedded), laid out as C does.
go_pt S(structOf)(go_ii n, go_ll names, go_ll types, go_ll tags, go_ll exported, go_ll embedded, go_ss pkg, go_ss typename) {
    go_field* fields = go_new(n > 0 ? n * (go_ii)sizeof(go_field) : 1, NULL).ptr;
    go_ii offset = 0, align = 1;
    for (go_ii i = 0; i < n; i++) {
        go_field* f = &fields[i];
        go_ss fname = ((go_ss*)names.ptr.ptr)[i], tag = ((go_ss*)tags.ptr.ptr)[i];
        f->name = go_new(go_string_len(fname) + 1, NULL).ptr;
        memcpy(f->name, fname.ptr, (size_t)go_string_len(fname));
        f->type = ((go_pt*)types.ptr.ptr)[i].ptr;
        f->exported = ((go_tf*)exported.ptr.ptr)[i];
        f->embedded = ((go_tf*)embedded.ptr.ptr)[i];
        if (go_string_len(tag) > 0) {
            f->tag = go_new(go_string_len(tag) + 1, NULL).ptr;
            memcpy(f->tag, tag.ptr, (size_t)go_string_len(tag));
        }
        if (!f->exported) {
            char* p = go_new(go_string_len(pkg) + 1, NULL).ptr;
            memcpy(p, pkg.ptr, (size_t)go_string_len(pkg));
            f->pkg = p;
        }
        go_ii a = go_ralign(f->type);
        if (a > align) align = a;
        offset = (offset + a - 1) / a * a;
        f->offset = offset;
        offset += f->type->size;
    }
    char* name = go_new(go_string_len(typename) + 1, NULL).ptr;
    memcpy(name, typename.ptr, (size_t)go_string_len(typename));
    go_type* t = go_rnew(name, go_kind_struct, (offset + align - 1) / align * align);
    if (!t->data.fields.field) t->data.fields = (go_type_struct){ fields, n };
    return go_rmake(t);
}
// funcOf makes a function type (named name), that reflection can't call or make functions
// of, unless the program has the type (whose descriptor has the functions to).
go_pt S(funcOf)(go_ss typename, go_ll in, go_ll out, go_tf variadic) {
    char* name = go_new(go_string_len(typename) + 1, NULL).ptr;
    memcpy(name, typename.ptr, (size_t)go_string_len(typename));
    go_type* t = go_rnew(name, go_kind_func, sizeof(go_fn));
    if (!t->data.func.call && !t->data.func.in && !t->data.func.out) {
        const go_type** ins = go_new(in.len > 0 ? in.len * (go_ii)sizeof(go_type*) : 1, NULL).ptr;
        const go_type** outs = go_new(out.len > 0 ? out.len * (go_ii)sizeof(go_type*) : 1, NULL).ptr;
        for (go_ii i = 0; i < in.len; i++) ins[i] = ((go_pt*)in.ptr.ptr)[i].ptr;
        for (go_ii i = 0; i < out.len; i++) outs[i] = ((go_pt*)out.ptr.ptr)[i].ptr;
        t->data.func = (go_type_func){ ins, in.len, outs, out.len, variadic, NULL, NULL };
    }
    return go_rmake(t);
}

go_pt S(makeMap)(go_pt t, go_ii n) { return (go_pt){ (void*)go_make_typed(T(t)->data.map.key, T(t)->data.map.elem, n > 0 ? n : 0) }; }

// Channels, c points to the channel.
go_pt S(makeChan)(go_pt t, go_ii buffer) { return (go_pt){ (void*)go_chan(T(t)->data.chan.elem->size, buffer) }; }
// chanSend sends the value at x, chanRecv receives into x, without waiting unless block,
// reporting whether they did (and for chanRecv, ok, whether the channel is open).
go_tf S(chanSend)(go_pt t, go_pt c, go_pt x, go_tf block) {
    go_ch ch = *(go_ch*)c.ptr;
    if (block) { go_send(ch, T(t)->data.chan.elem->size, x.ptr); return true; }
    go_select_case cases[] = { { ch, true, x.ptr, false } };
    return go_select(cases, 1, false) == 0;
}
go_tf S(chanRecv)(go_pt t, go_pt c, go_pt x, go_tf block, go_pt ok) {
    go_ch ch = *(go_ch*)c.ptr;
    if (block) { *(go_tf*)ok.ptr = go_recv(ch, T(t)->data.chan.elem->size, x.ptr); return true; }
    go_select_case cases[] = { { ch, false, x.ptr, false } };
    if (go_select(cases, 1, false) != 0) return false;
    *(go_tf*)ok.ptr = cases[0].ok;
    return true;
}
void S(chanClose)(go_pt c) { go_close(*(go_ch*)c.ptr); }
// selectCases selects between n cases: chans points to pointers to their channels (nil
// for none), elems to pointers to their values, and sends to whether they send.
go_ii S(selectCases)(go_ii n, go_pt chans, go_pt elems, go_pt sends, go_tf block, go_pt ok) {
    go_select_case* cases = go_new((n > 0 ? n : 1) * (go_ii)sizeof(go_select_case), NULL).ptr;
    for (go_ii i = 0; i < n; i++) {
        go_pt c = ((go_pt*)chans.ptr)[i];
        cases[i] = (go_select_case){ c.ptr ? *(go_ch*)c.ptr : NULL, ((go_tf*)sends.ptr)[i], ((go_pt*)elems.ptr)[i].ptr, false };
    }
    int chosen = go_select(cases, (int)n, block);
    if (chosen >= 0) *(go_tf*)ok.ptr = cases[chosen].ok;
    return chosen;
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
void S(mapClear)(go_pt m) { go_map_clear(*(go_kv*)m.ptr); }
go_pt S(mapRange)(go_pt m) {
    go_map_iter it = go_map_range(*(go_kv*)m.ptr);
    return go_new(sizeof it, &it);
}
go_tf S(mapNext)(go_pt it, go_pt key, go_pt elem) { return go_map_next(it.ptr, key.ptr, elem.ptr); }

go_ii S(chanLen)(go_pt c) { return go_chan_len(*(go_ch*)c.ptr); }
go_ii S(chanCap)(go_pt c) { return go_chan_cap(*(go_ch*)c.ptr); }
