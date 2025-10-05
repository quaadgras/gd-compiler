//go:build ignore

#include <go.h>
#include <string.h>
#include <stdlib.h>
#include "map.h"

go_pointer go_new(go_int size, void* init) {
    go_pointer p;
    p.ptr = malloc(size);
    if (init) {
        memcpy(p.ptr, init, size);
    } else {
        memset(p.ptr, 0, size);
    }
    return p;
}
go_slice go_append(go_slice s, go_int elem_size, const void* elem) {
    if (s.len >= s.cap) {
        go_int new_cap = s.cap == 0 ? 1 : s.cap * 2;
        go_pointer new_ptr = go_new(new_cap * elem_size, nil);
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
go_int go_copy(go_int elem_size, go_slice dst, go_slice src) {
    go_int n = dst.len < src.len ? dst.len : src.len;
    memcpy(dst.ptr.ptr, src.ptr.ptr, n * elem_size);
    return n;
}
void go_slice_clear(go_slice s) {
    memset(s.ptr.ptr, 0, s.cap * sizeof(s.ptr));
}
go_string go_string_new(const char* str) {
    go_string s;
    s.len = -1;
    s.ptr = (char*)str;
    return s;
}
go_int go_string_len(go_string s) {
    if (s.ptr == NULL) return 0;
    if (s.len == -1) return strlen(s.ptr);
    return s.len;
}
go_bool go_string_eq(go_string a, go_string b) {
    go_int lena = go_string_len(a);
    go_int lenb = go_string_len(b);
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

go_map go_make(go_int key_size, go_int elem_size, go_hash hash_func, go_same same_func, go_int hint, go_int argc, void* init) {
    map_metadata *meta = malloc(sizeof(map_metadata) + key_size + elem_size);
    meta->key_size = key_size;
    meta->val_size = elem_size;
    meta->key_hash = hash_func;
    meta->key_same = same_func;
    go_map map = (go_map)hashmap_new(key_size+elem_size, 0, 0, 0,
        map_hash, map_compare, NULL, meta);
    for (go_int i = 0; i < argc; i++) {
        hashmap_set(map, init + i * (key_size + elem_size));
    }
    return map;
}
void go_map_set(go_map m, void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    void* staging = meta->staging;
    memcpy(staging, key, meta->key_size);
    memcpy(staging + meta->key_size, val, meta->val_size);
    hashmap_set(m, staging);
}
go_bool go_map_get(go_map m, void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    const void* ptr = hashmap_get(m, key);
    if (ptr) {
        memcpy(val, (char*)ptr + meta->key_size, meta->val_size);
        return true;
    }
    memset(val, 0, meta->val_size);
    return false;
}


go_uint64 go_hash_go_string(const void *item, go_uint64 seed0, go_uint64 seed1) {
    const go_string *s = item;
    if (s->ptr == NULL) return 0;
    return hashmap_xxhash3(s->ptr, go_string_len(*s), seed0, seed1);
}
go_bool go_same_go_string(const void *a, const void *b) {
    return go_string_eq(*(const go_string*)a, *(const go_string*)b);
}
