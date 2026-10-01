//go:build ignore

#include <go.h>
#include <string.h>
#include <math.h>
#include <inttypes.h>
#include <stdarg.h>
#include <stdlib.h>
#include <threads.h>
#include "map.h"


go_pt go_new(go_ii size, const void* init) {
    go_pt p;
    p.ptr = malloc(size);
    if (init) {
        memcpy(p.ptr, init, size);
    } else {
        memset(p.ptr, 0, size);
    }
    return p;
}
go_ll go_slice_sparse(go_ii length, size_t size, go_ii n, const go_ii* indexes, const void* values) {
    go_ll s = { .ptr = go_new((go_ii)size * go_make_cap(length, length, size), NULL), .len = length, .cap = length };
    for (go_ii i = 0; i < n; i++) memcpy((char*)s.ptr.ptr + indexes[i] * (go_ii)size, (const char*)values + i * (go_ii)size, size);
    return s;
}
go_ll go_append(go_ll s, go_ii elem_size, const void* elem) {
    if (s.len >= s.cap) {
        go_ii new_cap = s.cap == 0 ? 1 : s.cap * 2;
        go_pt new_ptr = go_new(new_cap * elem_size, nil);
        if (s.len > 0) {
            memcpy(new_ptr.ptr, s.ptr.ptr, s.len * elem_size);
        }
        // the old array may be shared with other slices, it is garbage collected.
        s.ptr = new_ptr;
        s.cap = new_cap;
    }
    memcpy((char*)s.ptr.ptr + s.len * elem_size, elem, elem_size);
    s.len += 1;
    return s;
}
go_ll go_append_slice(go_ll s, go_ii elem_size, go_ll t) {
    if (t.len == 0) return s;
    if (s.len + t.len > s.cap) {
        go_ii new_cap = s.cap * 2;
        if (new_cap < s.len + t.len) new_cap = s.len + t.len;
        go_pt new_ptr = go_new(new_cap * elem_size, nil);
        if (s.len > 0) memcpy(new_ptr.ptr, s.ptr.ptr, s.len * elem_size);
        s.ptr = new_ptr;
        s.cap = new_cap;
    }
    memmove((char*)s.ptr.ptr + s.len * elem_size, t.ptr.ptr, t.len * elem_size); // may overlap.
    s.len += t.len;
    return s;
}

go_ll go_append_string(go_ll s, go_ss t) {
    go_ll bytes = { .ptr = { .ptr = (void*)t.ptr }, .len = go_string_len(t), .cap = go_string_len(t) };
    return go_append_slice(s, 1, bytes);
}

go_ii go_copy(go_ii elem_size, go_ll dst, go_ll src) {
    go_ii n = dst.len < src.len ? dst.len : src.len;
    memmove(dst.ptr.ptr, src.ptr.ptr, n * elem_size); // (they may overlap)
    return n;
}
void go_slice_clear(go_ll s, size_t elem_size) {
    if (s.len > 0) memset(s.ptr.ptr, 0, (size_t)s.len * elem_size);
}

go_ii go_string_len(go_ss s) {
    if (s.ptr == NULL) return 0;
    if (s.len == -1) return strlen(s.ptr);
    return s.len;
}
go_tf go_string_eq(go_ss a, go_ss b) {
    go_ii lena = go_string_len(a);
    go_ii lenb = go_string_len(b);
    if (lena != lenb) return false;
    return (memcmp(a.ptr, b.ptr, lena) == 0) ? true : false;
}

typedef struct {
    size_t key_size;
    size_t val_size;
    size_t val_offset; // aligned, so that pointers in values can be stored (Fil-C).
    go_hash key_hash;
    go_same key_same;
    go_ii clears; // (entries that iterators have yet to produce are gone)
    char staging[];
} map_metadata;

int map_compare(const void *a, const void *b, void *udata) {
    map_metadata *meta = (map_metadata*)udata;
    if (meta->key_same(a, b)) {
        return 0;
    }
    return 1;
}

uint64_t map_hash(const void *item, uint64_t seed0, uint64_t seed1, void *udata) {
    map_metadata *meta = (map_metadata*)udata;
    return meta->key_hash(item, seed0, seed1);
}

go_kv go_make(go_ii key_size, go_ii elem_size, go_hash hash_func, go_same same_func, go_ii hint, go_ii argc, const void* init, size_t stride, size_t val_offset) {
    size_t aligned = ((size_t)key_size + 7) & ~(size_t)7;
    map_metadata *meta = malloc(sizeof(map_metadata) + aligned + elem_size);
    meta->key_size = key_size;
    meta->val_size = elem_size;
    meta->val_offset = aligned;
    meta->key_hash = hash_func;
    meta->key_same = same_func;
    meta->clears = 0;
    go_kv map = (go_kv)hashmap_new(aligned + elem_size, hint > 0 ? (size_t)hint : 0, 0, 0,
        map_hash, map_compare, NULL, meta);
    for (go_ii i = 0; i < argc; i++) {
        const char* entry = (const char*)init + i * stride;
        go_map_set(map, entry, entry + val_offset);
    }
    return map;
}

go_kv go_map_clone(go_kv m) {
    if (!m) return NULL;
    map_metadata *meta = hashmap_udata(m);
    go_kv clone = go_make(meta->key_size, meta->val_size, meta->key_hash, meta->key_same, go_map_len(m), 0, NULL, 0, 0);
    size_t i = 0;
    void* item;
    while (hashmap_iter(m, &i, &item)) hashmap_set(clone, item);
    return clone;
}

go_u8 go_hash_bytes(const void* p, size_t n, go_u8 seed0, go_u8 seed1) {
    return hashmap_xxhash3(p, n, seed0, seed1);
}
void go_map_set(go_kv m, const void *key, const void *val) {
    if (!m) go_panic_error("assignment to entry in nil map");
    map_metadata *meta = hashmap_udata(m);
    void* staging = meta->staging;
    memcpy(staging, key, meta->key_size);
    memcpy((char*)staging + meta->val_offset, val, meta->val_size);
    hashmap_set(m, staging);
}
go_ii go_map_len(go_kv m) {
    return m ? (go_ii)hashmap_count(m) : 0;
}

void go_map_delete(go_kv m, const void *key) {
    if (!m) return;
    map_metadata *meta = hashmap_udata(m);
    memcpy(meta->staging, key, meta->key_size); // hashmap compares whole items.
    hashmap_delete(m, meta->staging);
}

go_tf go_map_get(go_kv m, const void *key, void *val) {
    if (!m) return false; // val is zero initialized by the caller.
    map_metadata *meta = hashmap_udata(m);
    const void* ptr = hashmap_get(m, key);
    if (ptr) {
        memcpy(val, (char*)ptr + meta->val_offset, meta->val_size);
        return true;
    }
    memset(val, 0, meta->val_size);
    return false;
}

void go_map_clear(go_kv m) {
    if (!m) return;
    hashmap_clear(m, false);
    ((map_metadata*)hashmap_udata(m))->clears++;
}

go_map_iter go_map_range(go_kv m) {
    go_map_iter it = { m };
    go_ii n = go_map_len(m);
    if (n == 0) return it;
    map_metadata *meta = hashmap_udata(m);
    size_t size = meta->val_offset + meta->val_size;
    it.entries = go_new((go_ii)(n * size), NULL).ptr;
    size_t i = 0;
    void* item;
    while (hashmap_iter(m, &i, &item) && it.n < n) memcpy(it.entries + it.n++ * size, item, size);
    it.start = (go_ii)(go_rand() % (go_u8)it.n);
    it.clears = meta->clears;
    return it;
}

go_tf go_map_next(go_map_iter* it, void* key, void* val) {
    if (it->n == 0) return false;
    map_metadata *meta = hashmap_udata(it->m);
    size_t size = meta->val_offset + meta->val_size;
    if (meta->clears != it->clears) return false; // cleared since the range started.
    while (it->i < it->n) {
        const char* entry = it->entries + ((it->i++ + it->start) % it->n) * size;
        const char* current = hashmap_get(it->m, entry);
        if (!current) {
            if (meta->key_same(entry, entry)) continue; // deleted.
            current = entry; // NaN keys are never found.
        }
        memcpy(key, current, meta->key_size);
        if (val) memcpy(val, current + meta->val_offset, meta->val_size);
        return true;
    }
    return false;
}

go_u8 go_hash_ss(const void *item, go_u8 seed0, go_u8 seed1) {
    const go_ss *s = item;
    if (s->ptr == NULL) return 0;
    return hashmap_xxhash3(s->ptr, go_string_len(*s), seed0, seed1);
}
go_tf go_same_ss(const void *a, const void *b) {
    return go_string_eq(*(const go_ss*)a, *(const go_ss*)b);
}


void* go_index(go_ll s, go_ii elem_size, go_ii i) {
    go_index_check(i, s.len);
    return (char*)s.ptr.ptr + i * elem_size;
}

go_ll go_slice(go_ll s, go_ii elem_size, go_i8 low, go_i8 high, go_i8 max) { return go_slice_of(s, elem_size, low, high, max, false); }
go_ll go_sliceu(go_ll s, go_ii elem_size, go_i8 low, go_i8 high, go_i8 max, int uns) { return go_slice_ofu(s, elem_size, low, high, max, false, uns); }

// go_slice_of is go_slice, of an array (when array), as their messages are Go's (see
// runtime.boundsError).
// Slice bounds are int64s, or, when flagged in uns (go_slice_ulow and others), uint64s,
// which may not fit in an int64.
static go_tf go_bound_neg(go_i8 x, go_tf u) { return !u && x < 0; }
static go_tf go_bound_gt(go_i8 a, go_tf ua, go_i8 b, go_tf ub) {
    if (ua && a < 0) return ub && b < 0 ? (go_u8)a > (go_u8)b : true;
    if (ub && b < 0) return false;
    return a > b;
}
static const char* go_bound(char* buf, go_i8 x, go_tf u) {
    if (u) snprintf(buf, 24, "%llu", (unsigned long long)x); else snprintf(buf, 24, "%lld", (long long)x);
    return buf;
}

go_ll go_slice_of(go_ll s, go_ii elem_size, go_i8 low, go_i8 high, go_i8 max, go_tf array) {
    return go_slice_ofu(s, elem_size, low, high, max, array, 0);
}

go_ll go_slice_ofu(go_ll s, go_ii elem_size, go_i8 low, go_i8 high, go_i8 max, go_tf array, int uns) {
    const char* of = array ? "length" : "capacity";
    go_tf ul = (uns & go_slice_ulow) != 0, uh = (uns & go_slice_uhigh) != 0, um = (uns & go_slice_umax) != 0;
    char a[24], b[24];
    if (low == go_slice_default) { low = 0; ul = false; }
    if (high == go_slice_default) { high = s.len; uh = false; }
    if (max == go_slice_default) { // s[low:high]
        max = s.cap;
        if (go_bound_neg(high, uh)) go_panic_error("runtime error: slice bounds out of range [:%s]", go_bound(a, high, uh));
        if (go_bound_gt(high, uh, max, false)) go_panic_error("runtime error: slice bounds out of range [:%s] with %s %lld", go_bound(a, high, uh), of, (long long)max);
        if (go_bound_neg(low, ul)) go_panic_error("runtime error: slice bounds out of range [%s:]", go_bound(a, low, ul));
        if (go_bound_gt(low, ul, high, uh)) go_panic_error("runtime error: slice bounds out of range [%s:%s]", go_bound(a, low, ul), go_bound(b, high, uh));
    } else { // s[low:high:max]
        if (go_bound_neg(max, um)) go_panic_error("runtime error: slice bounds out of range [::%s]", go_bound(a, max, um));
        if (go_bound_gt(max, um, s.cap, false)) go_panic_error("runtime error: slice bounds out of range [::%s] with %s %lld", go_bound(a, max, um), of, (long long)s.cap);
        if (go_bound_neg(high, uh)) go_panic_error("runtime error: slice bounds out of range [:%s:]", go_bound(a, high, uh));
        if (go_bound_gt(high, uh, max, um)) go_panic_error("runtime error: slice bounds out of range [:%s:%s]", go_bound(a, high, uh), go_bound(b, max, um));
        if (go_bound_neg(low, ul)) go_panic_error("runtime error: slice bounds out of range [%s::]", go_bound(a, low, ul));
        if (go_bound_gt(low, ul, high, uh)) go_panic_error("runtime error: slice bounds out of range [%s:%s:]", go_bound(a, low, ul), go_bound(b, high, uh));
    }
    // the result shares the backing array.
    go_pt ptr = { .ptr = s.ptr.ptr && max > low ? (char*)s.ptr.ptr + low * elem_size : s.ptr.ptr }; // (not past the end, as Go)
    return (go_ll){ .ptr = ptr, .len = (go_ii)(high - low), .cap = (go_ii)(max - low) };
}

go_vv go_any_new(size_t size, void* value, const go_type* go_type) {
    go_pt p = go_new(size, value);
    return (go_vv){ .ptr = p, .go_type = go_type };
}

// print and println, see runtime/print.go.

static void go_print_bytes(const char* p, size_t n) {
    if (n == 0) return;
    fwrite(p, 1, n, stderr);
}

void go_print_cstring(const char* s) { go_print_bytes(s, strlen(s)); }
void go_print_string(go_ss s) { go_print_bytes(s.ptr, (size_t)go_string_len(s)); }
void go_print_bool(go_tf v) { go_print_cstring(v ? "true" : "false"); }
void go_print_int(go_i8 v) { fprintf(stderr, "%" PRId64, v); }
void go_print_uint(go_u8 v) { fprintf(stderr, "%" PRIu64, v); }
void go_print_pointer(go_up p) { fprintf(stderr, "0x%" PRIxPTR, p); }

// go_format_float formats v like strconv.FormatFloat(v, 'g', -1, bits), into buf, which
// must have room for 32 bytes.
static void go_format_float(char* buf, double v, int bits) {
    if (isnan(v)) { strcpy(buf, "NaN"); return; }
    if (isinf(v)) { strcpy(buf, v > 0 ? "+Inf" : "-Inf"); return; }
    char* out = buf;
    if (signbit(v)) *out++ = '-';
    if (v == 0) { strcpy(out, "0"); return; }
    // The shortest decimal that parses back to v.
    char tmp[40];
    for (int p = 1; p <= 17; p++) {
        snprintf(tmp, sizeof tmp, "%.*e", p - 1, v);
        if (bits == 32 ? strtof(tmp, NULL) == (float)v : strtod(tmp, NULL) == v) break;
    }
    char d[20];
    int nd = 0;
    const char* c = tmp;
    if (*c == '-') c++;
    for (; *c && *c != 'e'; c++) {
        if (*c != '.') d[nd++] = *c;
    }
    while (nd > 1 && d[nd-1] == '0') nd--;
    int exp = atoi(c + 1);
    int dp = exp + 1; // position of the decimal point, relative to the digits.
    if (exp < -4 || exp >= 6) { // %e, with as many digits as needed.
        *out++ = d[0];
        if (nd > 1) {
            *out++ = '.';
            memcpy(out, d + 1, nd - 1);
            out += nd - 1;
        }
        *out++ = 'e';
        *out++ = exp < 0 ? '-' : '+';
        if (exp < 0) exp = -exp;
        if (exp >= 100) *out++ = '0' + exp / 100;
        *out++ = '0' + (exp / 10) % 10;
        *out++ = '0' + exp % 10;
    } else { // %f
        if (dp > 0) {
            for (int i = 0; i < dp; i++) *out++ = i < nd ? d[i] : '0';
        } else {
            *out++ = '0';
        }
        int prec = nd - dp > 0 ? nd - dp : 0;
        if (prec > 0) {
            *out++ = '.';
            for (int i = 1; i <= prec; i++) {
                int j = dp + i - 1;
                *out++ = (j >= 0 && j < nd) ? d[j] : '0';
            }
        }
    }
    *out = 0;
}

void go_print_float64(go_f8 v) {
    char buf[32];
    go_format_float(buf, v, 64);
    go_print_cstring(buf);
}

void go_print_float32(go_f4 v) {
    char buf[32];
    go_format_float(buf, v, 32);
    go_print_cstring(buf);
}

static void go_print_complex(double re, double im, int bits) {
    char buf[32];
    go_print_cstring("(");
    go_format_float(buf, re, bits);
    go_print_cstring(buf);
    go_format_float(buf, im, bits);
    if (buf[0] != '+' && buf[0] != '-') go_print_cstring("+");
    go_print_cstring(buf);
    go_print_cstring("i)");
}

// go_aaf8f8zz_quo divides complex numbers, as Go does, see runtime/complex.go.
go_aaf8f8zz go_aaf8f8zz_quo(go_aaf8f8zz n, go_aaf8f8zz m) {
    double e, f;
    if (fabs(m.f1) >= fabs(m.f2)) {
        double ratio = m.f2 / m.f1, denom = m.f1 + ratio * m.f2;
        e = (n.f1 + n.f2 * ratio) / denom;
        f = (n.f2 - n.f1 * ratio) / denom;
    } else {
        double ratio = m.f1 / m.f2, denom = m.f2 + ratio * m.f1;
        e = (n.f1 * ratio + n.f2) / denom;
        f = (n.f2 * ratio - n.f1) / denom;
    }
    if (isnan(e) && isnan(f)) { // correct the result to infinities and zeros, as C99 G.5.1.
        double a = n.f1, b = n.f2, c = m.f1, d = m.f2;
        if (c == 0 && d == 0 && (!isnan(a) || !isnan(b))) {
            e = copysign(INFINITY, c) * a;
            f = copysign(INFINITY, c) * b;
        } else if ((isinf(a) || isinf(b)) && isfinite(c) && isfinite(d)) {
            a = copysign(isinf(a) ? 1.0 : 0.0, a);
            b = copysign(isinf(b) ? 1.0 : 0.0, b);
            e = INFINITY * (a * c + b * d);
            f = INFINITY * (b * c - a * d);
        } else if ((isinf(c) || isinf(d)) && isfinite(a) && isfinite(b)) {
            c = copysign(isinf(c) ? 1.0 : 0.0, c);
            d = copysign(isinf(d) ? 1.0 : 0.0, d);
            e = 0 * (a * c + b * d);
            f = 0 * (b * d - a * c);
        }
    }
    return (go_aaf8f8zz){e, f};
}

void go_print_complex128(go_aaf8f8zz v) { go_print_complex(v.f1, v.f2, 64); }
void go_print_complex64(go_aaf4f4zz v) { go_print_complex(v.f1, v.f2, 32); }

void go_print_slice(go_ll s) {
    go_print_cstring("[");
    go_print_int(s.len);
    go_print_cstring("/");
    go_print_int(s.cap);
    go_print_cstring("]");
    go_print_pointer((go_up)s.ptr.ptr);
}

void go_print_iface(go_up type, go_up data) {
    go_print_cstring("(");
    go_print_pointer(type);
    go_print_cstring(",");
    go_print_pointer(data);
    go_print_cstring(")");
}

// defer, panic and recover, see go.h.

// A go_panicking is a panic in progress. Panics started by deferred calls (while another
// is in progress) are pushed on top of it.
typedef struct go_panicking {
    go_vv value;
    go_tf recovered;
    go_frame* frame; // the innermost frame with deferred calls when it started.
    struct go_panicking* prev;
} go_panicking;

static go_thread_local struct {
    go_frame* top;      // innermost frame with deferred calls.
    go_panicking* panic; // the current panic, if any.
    go_tf token;        // set while a deferred call is starting, see go_take_recover.
} go_g;

go_frame* go_frame_push(void) {
    go_frame* f = go_new(sizeof(go_frame), NULL).ptr;
    f->prev = go_g.top;
    go_g.top = f;
    return f;
}

void go_defer_push(go_frame* f, go_fn fn) {
    go_deferred* d = go_new(sizeof(go_deferred), NULL).ptr;
    d->fn = fn;
    d->next = f->defers;
    f->defers = d;
}

// go_run_defers runs the deferred calls of f, in last-in-first-out order. Each is removed
// before it is called, so that if it panics, the frame continues with the rest.
static void go_run_defers(go_frame* f) {
    while (f->defers) {
        go_deferred* d = f->defers;
        f->defers = d->next;
        go_g.token = true;
        ((void(*)(void*))d->fn.ptr)(d->fn.env);
        go_g.token = false;
    }
}

void go_frame_return(go_frame* f) {
    go_run_defers(f);
    go_g.top = f->prev;
}

// go_take_recover is called on entry to every function: only a function called directly
// as a deferred call receives the token that allows it to recover.
go_tf go_take_recover(void) {
    go_tf token = go_g.token;
    go_g.token = false;
    return token;
}

go_vv go_recover(go_tf can_recover) {
    go_panicking* p = go_g.panic;
    if (!can_recover || !p || p->recovered) return (go_vv){0};
    p->recovered = true;
    return p->value;
}

static void go_print_panic_value(go_vv v) {
    const go_type* t = v.go_type;
    void* p = v.ptr.ptr;
    if (!t) { go_print_cstring("nil"); return; }
    switch (t->kind) {
    case go_kind_string: go_print_string(*(go_ss*)p); return;
    case go_kind_bool: go_print_bool(*(go_tf*)p); return;
    case go_kind_int: go_print_int(*(go_ii*)p); return;
    case go_kind_int8: go_print_int(*(go_i1*)p); return;
    case go_kind_int16: go_print_int(*(go_i2*)p); return;
    case go_kind_int32: go_print_int(*(go_i4*)p); return;
    case go_kind_int64: go_print_int(*(go_i8*)p); return;
    case go_kind_uint: go_print_uint((go_u8)*(go_uu*)p); return;
    case go_kind_uint8: go_print_uint(*(go_u1*)p); return;
    case go_kind_uint16: go_print_uint(*(go_u2*)p); return;
    case go_kind_uint32: go_print_uint(*(go_u4*)p); return;
    case go_kind_uint64: go_print_uint(*(go_u8*)p); return;
    case go_kind_uintptr: go_print_uint(*(go_up*)p); return;
    case go_kind_float32: go_print_float32(*(go_f4*)p); return;
    case go_kind_float64: go_print_float64(*(go_f8*)p); return;
    default:
        go_print_cstring("(");
        go_print_cstring(t->name);
        go_print_cstring(") ");
        go_print_pointer((go_up)p);
    }
}

// go_panic_continue unwinds to the innermost frame, or if there are none, exits the
// program like gc does, after printing the panic value.
static _Noreturn void go_panic_continue(void) {
    if (go_g.top) longjmp(go_g.top->jb, 1);
    go_print_cstring("panic: ");
    go_print_panic_value(go_g.panic->value);
    go_print_cstring("\n\ngoroutine 1 [running]:\n");
    exit(2);
}

// go_started_in reports whether frame is f, or a frame that f's function called.
static go_tf go_started_in(go_frame* frame, go_frame* f) {
    for (; frame; frame = frame->prev) {
        if (frame == f) return true;
    }
    return false;
}

void go_frame_unwind(go_frame* f) {
    go_run_defers(f);
    go_g.top = f->prev;
    if (!go_g.panic->recovered) go_panic_continue();
    // f's function returns normally: the recovered panic is over, and so are those that
    // were started (by deferred calls) in f, or in the functions it called.
    go_g.panic = go_g.panic->prev;
    while (go_g.panic && go_started_in(go_g.panic->frame, f)) go_g.panic = go_g.panic->prev;
}

// panic(nil) panics with a *runtime.PanicNilError (since Go 1.21).
static go_ss go_panic_nil_Error(void* e) { (void)e; return go_string_new("panic called with nil argument (use runtime.PanicNilError)"); }
static void go_panic_nil_RuntimeError(void* e) { (void)e; }
static const go_method go_panic_nil_methods[] = {
    { "Error", "func() string", (void(*)(void))go_panic_nil_Error },
    { "RuntimeError", "func()", (void(*)(void))go_panic_nil_RuntimeError },
};
static const go_type go_type_panic_nil = {.name="*runtime.PanicNilError", .kind=go_kind_pointer, .size=sizeof(go_pt), .methods=go_panic_nil_methods, .nmethods=2};

void go_panic_any(go_vv v) {
    if (!v.go_type) {
        static go_pt nil_error;
        v = (go_vv){ (go_pt){ &nil_error }, &go_type_panic_nil };
    }
    go_panicking* p = go_new(sizeof(go_panicking), NULL).ptr;
    p->value = v;
    p->frame = go_g.top;
    p->prev = go_g.panic;
    go_g.panic = p;
    go_panic_continue();
}

// go_type_eq reports whether a and b describe the same type. Descriptors of predeclared
// types are defined in each file that uses them, so they are compared by kind and name.
go_tf go_type_eq(const go_type* a, const go_type* b) {
    if (a == b) return true;
    if (!a || !b || a->local || b->local) return false; // (local types have one descriptor)
    return a->kind == b->kind && strcmp(a->name, b->name) == 0;
}

char go_zerobase[8];
void (*go_makefunc_call)(void* env, void** args, void** results);
int go_argc;
char** go_argv;

go_tf go_vv_eq(go_vv a, go_vv b) {
    if (!a.go_type || !b.go_type) return a.go_type == b.go_type;
    if (!go_type_eq(a.go_type, b.go_type)) return false;
    const void *x = a.ptr.ptr, *y = b.ptr.ptr;
    switch (a.go_type->kind) {
    case go_kind_bool: return *(const go_tf*)x == *(const go_tf*)y;
    case go_kind_int8: case go_kind_uint8: return *(const go_u1*)x == *(const go_u1*)y;
    case go_kind_int16: case go_kind_uint16: return *(const go_u2*)x == *(const go_u2*)y;
    case go_kind_int32: case go_kind_uint32: return *(const go_u4*)x == *(const go_u4*)y;
    case go_kind_int64: case go_kind_uint64: return *(const go_u8*)x == *(const go_u8*)y;
    case go_kind_int: case go_kind_uint: return *(const go_ii*)x == *(const go_ii*)y;
    case go_kind_uintptr: return *(const go_up*)x == *(const go_up*)y;
    case go_kind_float32: return *(const go_f4*)x == *(const go_f4*)y;
    case go_kind_float64: return *(const go_f8*)x == *(const go_f8*)y;
    case go_kind_complex64: return ((const go_aaf4f4zz*)x)->f1 == ((const go_aaf4f4zz*)y)->f1 && ((const go_aaf4f4zz*)x)->f2 == ((const go_aaf4f4zz*)y)->f2;
    case go_kind_complex128: return ((const go_aaf8f8zz*)x)->f1 == ((const go_aaf8f8zz*)y)->f1 && ((const go_aaf8f8zz*)x)->f2 == ((const go_aaf8f8zz*)y)->f2;
    case go_kind_string: return go_string_eq(*(const go_ss*)x, *(const go_ss*)y);
    case go_kind_pointer: case go_kind_unsafe_pointer: return ((const go_pt*)x)->ptr == ((const go_pt*)y)->ptr;
    case go_kind_chan: return *(const go_ch*)x == *(const go_ch*)y;
    case go_kind_struct: case go_kind_array:
        if (a.go_type->equal) return a.go_type->equal(x, y);
        break;
    default: break;
    }
    go_panic_error("runtime error: comparing uncomparable type %s", a.go_type->name);
}

go_u8 go_vv_hash(go_vv v, go_u8 seed0, go_u8 seed1) {
    if (!v.go_type) return go_hash_bytes("", 0, seed0, seed1);
    const void* x = v.ptr.ptr;
    size_t size;
    switch (v.go_type->kind) {
    case go_kind_bool: case go_kind_int8: case go_kind_uint8: size = 1; break;
    case go_kind_int16: case go_kind_uint16: size = 2; break;
    case go_kind_int32: case go_kind_uint32: size = 4; break;
    case go_kind_int64: case go_kind_uint64: size = 8; break;
    case go_kind_int: case go_kind_uint: size = sizeof(go_ii); break;
    case go_kind_uintptr: size = sizeof(go_up); break;
    case go_kind_float32: { go_f4 f = *(const go_f4*)x; if (f == 0) f = 0; return go_hash_bytes(&f, sizeof f, seed0, seed1); }
    case go_kind_float64: { go_f8 f = *(const go_f8*)x; if (f == 0) f = 0; return go_hash_bytes(&f, sizeof f, seed0, seed1); }
    case go_kind_complex64: { go_aaf4f4zz c = *(const go_aaf4f4zz*)x; if (c.f1 == 0) c.f1 = 0; if (c.f2 == 0) c.f2 = 0; return go_hash_bytes(&c, sizeof c, seed0, seed1); }
    case go_kind_complex128: { go_aaf8f8zz c = *(const go_aaf8f8zz*)x; if (c.f1 == 0) c.f1 = 0; if (c.f2 == 0) c.f2 = 0; return go_hash_bytes(&c, sizeof c, seed0, seed1); }
    case go_kind_string: return go_hash_ss(x, seed0, seed1);
    case go_kind_pointer: case go_kind_unsafe_pointer: { void* p = ((const go_pt*)x)->ptr; return go_hash_bytes(&p, sizeof p, seed0, seed1); }
    case go_kind_chan: { go_ch c = *(const go_ch*)x; return go_hash_bytes(&c, sizeof c, seed0, seed1); }
    case go_kind_struct: case go_kind_array:
        if (v.go_type->hash) return v.go_type->hash(x, seed0, seed1);
        /* fallthrough */
    default:
        go_panic_error("runtime error: hash of unhashable type %s", v.go_type->name);
    }
    return go_hash_bytes(x, size, seed0, seed1);
}

// Runtime errors are strings, with the methods of runtime.Error (sorted by name).
static go_ss go_runtime_error_Error(void* e) { return *(go_ss*)e; }
static void go_runtime_error_RuntimeError(void* e) { (void)e; }
static const go_method go_runtime_error_methods[] = {
    { "Error", "func() string", (void(*)(void))go_runtime_error_Error },
    { "RuntimeError", "func()", (void(*)(void))go_runtime_error_RuntimeError },
};
const go_type go_type_runtime_error = {.name="runtime.Error", .kind=go_kind_string, .methods=go_runtime_error_methods, .nmethods=2};

void go_panic_error(const char* format, ...) {
    va_list args;
    va_start(args, format);
    int n = vsnprintf(NULL, 0, format, args);
    va_end(args);
    char* msg = go_new(n + 1, NULL).ptr;
    va_start(args, format);
    vsnprintf(msg, (size_t)n + 1, format, args);
    va_end(args);
    go_ss s = { .ptr = msg, .len = n };
    go_panic_any(go_any_new(sizeof(go_ss), &s, &go_type_runtime_error));
}

void go_panic_assertion(const go_type* want, go_vv have, const char* from) {
    const char* scopes = have.go_type && strcmp(have.go_type->name, want->name) == 0 ? " (types from different scopes)" : "";
    go_panic_error("interface conversion: %s is %s, not %s%s", from, have.go_type ? have.go_type->name : "nil", want->name, scopes);
}

// strings, see also runtime/string.go and unicode/utf8.

static go_ss go_string_alloc(go_ii n, char** data) {
    *data = go_new(n + 1, NULL).ptr; // NUL terminated, for C.
    return (go_ss){ .ptr = *data, .len = n };
}

go_ii go_string_cmp(go_ss a, go_ss b) {
    go_ii la = go_string_len(a), lb = go_string_len(b);
    go_ii n = la < lb ? la : lb;
    int c = n > 0 ? memcmp(a.ptr, b.ptr, (size_t)n) : 0;
    if (c != 0) return c < 0 ? -1 : 1;
    return la < lb ? -1 : (la > lb ? 1 : 0);
}

go_ss go_string_concat(go_ss a, go_ss b) {
    go_ii la = go_string_len(a), lb = go_string_len(b);
    if (lb == 0) return a;
    if (la == 0) return b;
    char* data;
    go_ss s = go_string_alloc(la + lb, &data);
    memcpy(data, a.ptr, (size_t)la);
    memcpy(data + la, b.ptr, (size_t)lb);
    return s;
}

go_u1 go_string_index(go_ss s, go_ii i) {
    return (go_u1)s.ptr[go_index_check(i, go_string_len(s))];
}

go_ss go_string_slice(go_ss s, go_i8 low, go_i8 high) { return go_string_sliceu(s, low, high, 0); }

go_ss go_string_sliceu(go_ss s, go_i8 low, go_i8 high, int uns) {
    go_ii n = go_string_len(s);
    go_tf ul = (uns & go_slice_ulow) != 0, uh = (uns & go_slice_uhigh) != 0;
    char a[24], b[24];
    if (low == go_slice_default) { low = 0; ul = false; }
    if (high == go_slice_default) { high = n; uh = false; }
    if (go_bound_neg(high, uh)) go_panic_error("runtime error: slice bounds out of range [:%s]", go_bound(a, high, uh));
    if (go_bound_gt(high, uh, n, false)) go_panic_error("runtime error: slice bounds out of range [:%s] with length %lld", go_bound(a, high, uh), (long long)n);
    if (go_bound_neg(low, ul)) go_panic_error("runtime error: slice bounds out of range [%s:]", go_bound(a, low, ul));
    if (go_bound_gt(low, ul, high, uh)) go_panic_error("runtime error: slice bounds out of range [%s:%s]", go_bound(a, low, ul), go_bound(b, high, uh));
    if (high == low) return (go_ss){ .ptr = s.ptr, .len = 0 }; // (not past the end, as Go)
    return (go_ss){ .ptr = s.ptr ? s.ptr + low : NULL, .len = (go_ii)(high - low) };
}

go_ii go_string_decode(go_ss s, go_ii i, go_i4* r) {
    const unsigned char* p = (const unsigned char*)s.ptr + i;
    go_ii n = go_string_len(s) - i;
    *r = 0xFFFD;
    if (n <= 0) return 0;
    unsigned char c = p[0];
    if (c < 0x80) { *r = c; return 1; }
    go_ii width; go_i4 min, v;
    if (c >= 0xC2 && c <= 0xDF) { width = 2; min = 0x80; v = c & 0x1F; }
    else if (c >= 0xE0 && c <= 0xEF) { width = 3; min = 0x800; v = c & 0x0F; }
    else if (c >= 0xF0 && c <= 0xF4) { width = 4; min = 0x10000; v = c & 0x07; }
    else return 1;
    if (n < width) return 1;
    for (go_ii k = 1; k < width; k++) {
        if ((p[k] & 0xC0) != 0x80) return 1;
        v = (v << 6) | (p[k] & 0x3F);
    }
    if (v < min || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF)) return 1;
    *r = v;
    return width;
}

static go_ii go_rune_encode(char* buf, go_i8 r) {
    if (r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) r = 0xFFFD;
    if (r < 0x80) { buf[0] = (char)r; return 1; }
    if (r < 0x800) { buf[0] = (char)(0xC0 | (r >> 6)); buf[1] = (char)(0x80 | (r & 0x3F)); return 2; }
    if (r < 0x10000) {
        buf[0] = (char)(0xE0 | (r >> 12)); buf[1] = (char)(0x80 | ((r >> 6) & 0x3F)); buf[2] = (char)(0x80 | (r & 0x3F));
        return 3;
    }
    buf[0] = (char)(0xF0 | (r >> 18)); buf[1] = (char)(0x80 | ((r >> 12) & 0x3F));
    buf[2] = (char)(0x80 | ((r >> 6) & 0x3F)); buf[3] = (char)(0x80 | (r & 0x3F));
    return 4;
}

go_ss go_string_from_rune(go_i8 r) {
    char buf[4], *data;
    go_ii n = go_rune_encode(buf, r);
    go_ss s = go_string_alloc(n, &data);
    memcpy(data, buf, (size_t)n);
    return s;
}

go_ss go_string_from_bytes(go_ll b) {
    char* data;
    go_ss s = go_string_alloc(b.len, &data);
    if (b.len > 0) memcpy(data, b.ptr.ptr, (size_t)b.len);
    return s;
}

go_ll go_bytes_from_string(go_ss s) {
    go_ii n = go_string_len(s);
    go_ll b = { .ptr = go_new(n, NULL), .len = n, .cap = n };
    if (n > 0) memcpy(b.ptr.ptr, s.ptr, (size_t)n);
    return b;
}

go_ss go_string_from_runes(go_ll r) {
    const go_i4* runes = r.ptr.ptr;
    go_ii n = 0;
    char buf[4], *data;
    for (go_ii i = 0; i < r.len; i++) n += go_rune_encode(buf, runes[i]);
    go_ss s = go_string_alloc(n, &data);
    for (go_ii i = 0, at = 0; i < r.len; i++) at += go_rune_encode(data + at, runes[i]);
    return s;
}

go_ll go_runes_from_string(go_ss s) {
    go_ii len = go_string_len(s), n = 0;
    go_i4 r;
    for (go_ii i = 0; i < len; i += go_string_decode(s, i, &r)) n++;
    go_ll out = { .ptr = go_new(n * (go_ii)sizeof(go_i4), NULL), .len = n, .cap = n };
    go_i4* runes = out.ptr.ptr;
    for (go_ii i = 0, k = 0; i < len; k++) i += go_string_decode(s, i, &runes[k]);
    return out;
}

// method sets, see go.h.

static const go_method* go_find_method(const go_type* t, const char* name) {
    if (!t) return NULL;
    for (go_ii i = 0; i < t->nmethods; i++) { // TODO: binary search (they're sorted).
        if (strcmp(t->methods[i].name, name) == 0) return &t->methods[i];
    }
    return NULL;
}

go_tf go_implements(const go_type* t, const go_imethod* methods, go_ii n, void (**table)(void)) {
    for (go_ii i = 0; i < n; i++) {
        const go_method* m = go_find_method(t, methods[i].name);
        if (!m || strcmp(m->type, methods[i].type) != 0) return false;
        if (table) table[i] = m->fn;
    }
    return true;
}

go_if go_to_iface(go_vv v, const char* iface, const go_imethod* methods, go_ii n, go_tf assert, go_tf* ok) {
    if (ok) *ok = false;
    if (!v.go_type) {
        if (assert) go_panic_error("interface conversion: interface is nil, not %s", iface);
        return (go_if){0};
    }
    void (**table)(void) = go_new(n > 0 ? n * (go_ii)sizeof(void (*)(void)) : 1, NULL).ptr;
    if (!go_implements(v.go_type, methods, n, table)) {
        if (assert) {
            const char* missing = "";
            for (go_ii i = 0; i < n; i++) {
                const go_method* m = go_find_method(v.go_type, methods[i].name);
                if (!m || strcmp(m->type, methods[i].type) != 0) { missing = methods[i].name; break; }
            }
            go_panic_error("interface conversion: %s is not %s: missing method %s", v.go_type->name, iface, missing);
        }
        return (go_if){0};
    }
    if (ok) *ok = true;
    return (go_if){ .ptr = v.ptr, .go_type = v.go_type, .vtable = table };
}
