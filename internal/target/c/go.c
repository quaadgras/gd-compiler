//go:build ignore

#include <go.h>
#include <string.h>
#include "map.h"

void* go_new(go_int size) { return malloc(size); }
go_slice go_append(go_int elem_size, go_slice s, const int argc, ...) {
    if (s.len + argc > s.cap) {
        s.cap = (s.len + argc) * 2;
        s.ptr = realloc(s.ptr, s.cap * elem_size);
    }
    va_list args;
    va_start(args, argc);
    for (int i = 0; i < argc; i++) {
        void* elem = va_arg(args, void*);
        memcpy((char*)s.ptr + s.len * elem_size, elem, elem_size);
        s.len++;
    }
    va_end(args);
    return s;
}
go_slice go_slice_make(go_int elem_size, go_int length, go_int capacity) {
    go_slice s;
    s.len = length;
    s.cap = capacity;
    s.ptr = malloc(capacity * elem_size);
    return s;
}
go_int go_slice_copy(go_int elem_size, go_slice dst, go_slice src) {
    go_int n = dst.len < src.len ? dst.len : src.len;
    memcpy(dst.ptr, src.ptr, n * elem_size);
    return n;
}
void go_slice_clear(go_slice s) {
    memset(s.ptr, 0, s.cap * sizeof(s.ptr));
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

typedef struct {
    size_t key_size;
    size_t val_size;
    go_hash key_hash;
    go_same val_same;
    char staging[];
} map_metadata;

int map_compare(const void *a, const void *b, void *udata) {
    map_metadata *meta = (map_metadata*)udata;
    return meta->val_same(a+meta->key_size, b+meta->key_size);
}

uint64_t map_hash(const void *item, uint64_t seed0, uint64_t seed1, void *udata) {
    map_metadata *meta = (map_metadata*)udata;
    return meta->key_hash(item, seed0, seed1);
}

go_map go_map_make(go_int key_size, go_int elem_size, go_hash hash_func, go_same same_func, go_int hint) {
    map_metadata *meta = malloc(sizeof(map_metadata));
    meta->key_size = key_size;
    meta->val_size = elem_size;
    meta->key_hash = hash_func;
    meta->val_same = same_func;
    return hashmap_new(key_size+elem_size, 0, 0, 0,
        map_hash, map_compare, NULL, meta);
}
void go_map_set(go_map m, void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    void* staging = malloc(meta->key_size + meta->val_size);
    memcpy(staging, key, meta->key_size);
    memcpy(staging + meta->key_size, val, meta->val_size);
    hashmap_set(m, staging);
}
go_bool go_map_get(go_map m, void *key, void *val) {
    map_metadata *meta = hashmap_udata(m);
    const void* ptr = hashmap_get(m, key);
    if (ptr) {
        memcpy(val, ptr + meta->key_size, meta->val_size);
        return go_true;
    }
    memset(val, 0, meta->val_size);
    return go_false;
}


go_uint64 go_hash_go_string(const void *item, go_uint64 seed0, go_uint64 seed1) {
    const go_string *s = item;
    if (s->ptr == NULL) return 0;
    return hashmap_xxhash3(s->ptr, go_string_len(*s), seed0, seed1);
}
go_bool go_same_go_string(const void *a, const void *b) {
    const go_string *sa = a;
    const go_string *sb = b;
    go_int lena = go_string_len(*sa);
    go_int lenb = go_string_len(*sb);
    if (lena != lenb) return go_false;
    return (memcmp(sa->ptr, sb->ptr, lena) == 0) ? go_true : go_false;
}
