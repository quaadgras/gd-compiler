//go:build ignore

#include <go.h>
#include <string.h>
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
        hashmap_set(map, init + i * (key_size + elem_size));
    }
    return map;
}
void go_map_set(go_kv m, void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    void* staging = meta->staging;
    memcpy(staging, key, meta->key_size);
    memcpy(staging + meta->key_size, val, meta->val_size);
    hashmap_set(m, staging);
}
go_tf go_map_get(go_kv m, void *key, void *val) {
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
    memcpy(data + sizeof(go_fn), arg, arg_size);
    thrd_create(&thread, trampoline, data);
}

void* go_index(go_ll s, go_ii elem_size, go_ii i) {
    if (i < 0 || i >= s.len) {
        go_panic("index out of range");
    }
    return (void*)s.ptr.ptr + i * elem_size;
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
