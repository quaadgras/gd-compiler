//go:build ignore
#ifndef GO
#define GO

#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdbool.h>

typedef bool go_bool;
typedef int8_t go_int8;
typedef int16_t go_int16;
typedef int32_t go_int32;
typedef int64_t go_int64;
typedef uint8_t go_uint8;
typedef uint16_t go_uint16;
typedef uint32_t go_uint32;
typedef uint64_t go_uint64;
typedef uintptr_t go_uintptr;
typedef float go_float32;
typedef double go_float64;
typedef struct { float real; float imag; } go_complex64;
typedef struct { double real; double imag; } go_complex128;

#define true 1
#define false 0
#define nil NULL

#define go_main() int main(int argc, char* argv[])

#ifdef __LP64__
    typedef int64_t go_int;
    typedef uint64_t go_uint;
#else
    typedef int32_t go_int;
    typedef uint32_t go_uint;
#endif

typedef struct {
    void* ptr;
    //size_t off; // (for GCs without support for interior pointers, an offset is needed)
} go_pointer;

typedef struct {
    void (*ptr)(void);
    char context[];
} go_func;

typedef struct {

} go_chan;

typedef struct {
    go_pointer ptr;
    go_int len;
    go_int cap;
} go_slice;

typedef void* go_map;

typedef struct {
    char  *ptr;
    go_int len;
} go_string;

typedef struct {
    go_pointer ptr;
    go_int typ; // type descriptor index
} go_interface;

static inline go_complex64 go_complex64_new(float real, float imag) {
    return (go_complex64){real, imag};
}
static inline go_complex128 go_complex128_new(double real, double imag) {
    return (go_complex128){real, imag};
}

#define go_ignore(x) (void)(x)

static inline void go_print(const char* format, ...) {
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
}

typedef go_uint64 (*go_hash)(const void *item, go_uint64 seed0, go_uint64 seed1);
typedef go_bool (*go_same)(const void *a, const void *b);

go_pointer go_new(go_int size, void* init);
#define go_pointer_new(t) go_new(sizeof(t), nil)
#define go_pointer_set(p, t, v) *(t*)((p).ptr) = (v)
#define go_pointer_get(p, t) (*(t*)((p).ptr))

go_int go_copy(go_int elem_size, go_slice dst, go_slice src);
go_slice go_append(go_slice s, go_int elem_size, const void* elem);
#define go_slice_make(T, length, capacity) (go_slice){ .ptr = go_new(sizeof(T)*capacity, nil), .len = length, .cap = capacity }
#define go_slice_index(s, T, i) (((T*)(s).ptr.ptr)[i])
#define go_slice_copy(T, dst, src) go_copy(sizeof(T), dst, src)
#define go_slice_literal(length, T, ...) (go_slice){ .ptr = go_new(sizeof(T)*length, &(T[]){__VA_ARGS__}), .len = length, .cap = length }
static inline go_int go_slice_len(go_slice s) { return s.len; }
void go_slice_clear(go_slice s);

go_map go_make(go_int key_size, go_int elem_size, go_hash hash_func, go_same same_func, go_int hint, go_int argc, void* init);
#define go_map_make(K, V, hint) go_make(sizeof(K), sizeof(V), go_hash_##K, go_same_##K, hint, 0)
#define go_map_literal(K, V, count, ...) go_make(sizeof(K), sizeof(V), go_hash_##K, go_same_##K, count, count, &(go_map_entry__##K##__##V[]){__VA_ARGS__})
void go_map_set(go_map m, void* key, void* val);
go_bool go_map_get(go_map m, void* key, void* val);

go_string go_string_new(const char* str);
go_int go_string_len(go_string s);
go_bool go_string_eq(go_string a, go_string b);

#define go_chan_make(T, length) ((go_chan){})
void go_chan_send(go_chan c, const void* v);
void go_chan_recv(go_chan c, void* v);

#define go_make_func(fn) ((go_func){ .ptr = (void(*)(void))(fn) })
#define go_func_get(f, T) (T)(f.ptr)

go_uint64 go_hash_go_string(const void* item, go_uint64 seed0, go_uint64 seed1);
go_bool go_same_go_string(const void* a, const void* b);
#endif
