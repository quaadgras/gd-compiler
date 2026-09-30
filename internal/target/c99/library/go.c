//go:build ignore

#include <go.h>
#include <string.h>
#include <math.h>
#include <inttypes.h>
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
go_ll go_append(go_ll s, go_ii elem_size, const void* elem) {
    if (s.len >= s.cap) {
        go_ii new_cap = s.cap == 0 ? 1 : s.cap * 2;
        go_pt new_ptr = go_new(new_cap * elem_size, nil);
        if (s.len > 0) {
            memcpy(new_ptr.ptr, s.ptr.ptr, s.len * elem_size);
        }
        free(s.ptr.ptr);
        s.ptr = new_ptr;
        s.cap = new_cap;
    }
    memcpy((char*)s.ptr.ptr + s.len * elem_size, elem, elem_size);
    s.len += 1;
    return s;
}
go_ii go_copy(go_ii elem_size, go_ll dst, go_ll src) {
    go_ii n = dst.len < src.len ? dst.len : src.len;
    memcpy(dst.ptr.ptr, src.ptr.ptr, n * elem_size);
    return n;
}
void go_slice_clear(go_ll s) {
    memset(s.ptr.ptr, 0, s.cap * sizeof(s.ptr));
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
    go_hash key_hash;
    go_same key_same;
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

go_kv go_make(go_ii key_size, go_ii elem_size, go_hash hash_func, go_same same_func, go_ii hint, go_ii argc, void* init) {
    map_metadata *meta = malloc(sizeof(map_metadata) + key_size + elem_size);
    meta->key_size = key_size;
    meta->val_size = elem_size;
    meta->key_hash = hash_func;
    meta->key_same = same_func;
    go_kv map = (go_kv)hashmap_new(key_size+elem_size, 0, 0, 0,
        map_hash, map_compare, NULL, meta);
    for (go_ii i = 0; i < argc; i++) {
        hashmap_set(map, (char*)init + i * (key_size + elem_size));
    }
    return map;
}
void go_map_set(go_kv m, const void *key, const void *val) {
    map_metadata *meta = hashmap_udata(m);
    void* staging = meta->staging;
    memcpy(staging, key, meta->key_size);
    memcpy((char*)staging + meta->key_size, val, meta->val_size);
    hashmap_set(m, staging);
}
go_tf go_map_get(go_kv m, const void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    const void* ptr = hashmap_get(m, key);
    if (ptr) {
        memcpy(val, (char*)ptr + meta->key_size, meta->val_size);
        return true;
    }
    memset(val, 0, meta->val_size);
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

void go_routine(int(trampoline)(void*), go_fn fn, size_t arg_size, void* arg) {
    thrd_t thread;
    void *data = malloc(sizeof(go_fn) + arg_size);
    memcpy(data, &fn, sizeof(go_fn));
    memcpy((char*)data + sizeof(go_fn), arg, arg_size);
    thrd_create(&thread, trampoline, data);
}

void* go_index(go_ll s, go_ii elem_size, go_ii i) {
    if (i < 0 || i >= s.len) {
        go_panic("index out of range");
    }
    return (char*)s.ptr.ptr + i * elem_size;
}

go_ll go_slice(go_ll s, go_ii elem_size, go_ii low, go_ii high, go_ii cap) {
    if (low < 0 || high < low || high > s.len) {
        go_panic("slice bounds out of range");
    }
    if (cap < high - low) {
        cap = high - low;
    }
    go_pt new_ptr = go_new(cap * elem_size, nil);
    memcpy(new_ptr.ptr, (char*)s.ptr.ptr + low * elem_size, (high - low) * elem_size);
    return (go_ll){ .ptr = new_ptr, .len = high - low, .cap = cap };
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
