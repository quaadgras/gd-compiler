//go:build ignore
#include <stdarg.h>
#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>

typedef uint64_t go_uint64;
typedef uint32_t go_uint32;
typedef uint8_t go_bool;

#define go_true 1
#define go_false 0

#define main() int main(int argc, char* argv[])

#ifdef __LP64__
    typedef int64_t go_int;
#else
    typedef int32_t go_int;
#endif

typedef struct {
    void* ptr;
    go_int len;
    go_int cap;
} go_slice;

typedef void* go_map;

typedef struct {
    char  *ptr;
    go_int len;
} go_string;

static inline void go_print(const char* format, ...) {
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
}

typedef go_uint64 (*go_hash)(const void *item, go_uint64 seed0, go_uint64 seed1);
typedef go_bool (*go_same)(const void *a, const void *b);

void* go_new(go_int size);
go_slice go_append(go_int elem_size, go_slice s, const int argc, ...);
go_slice go_slice_make(go_int elem_size, go_int length, go_int capacity);
go_int go_slice_copy(go_int elem_size, go_slice dst, go_slice src);
void go_slice_clear(go_slice s);
go_map go_map_make(go_int key_size, go_int elem_size, go_hash hash_func, go_same same_func, go_int hint);
void go_map_set(go_map m, void *key, void *val);
go_bool go_map_get(go_map m, void *key, void *val);
go_string go_string_new(const char* str);
go_int go_string_len(go_string s);


go_uint64 go_hash_go_string(const void *item, go_uint64 seed0, go_uint64 seed1);
go_bool go_same_go_string(const void *a, const void *b);
