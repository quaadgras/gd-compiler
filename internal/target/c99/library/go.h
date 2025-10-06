//go:build ignore
#ifndef GO
#define GO

#include <stdarg.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <stdbool.h>
#include <stddef.h>

#define true 1
#define false 0
#define nil NULL
#define go_ARCH "unknown"
#define go_OS "unknown"

typedef bool go_tf;
#ifdef __LP64__
typedef int64_t go_ii;
#else
typedef int32_t go_ii;
#endif
typedef int8_t go_i1;
typedef int16_t go_i2;
typedef int32_t go_i4;
typedef int64_t go_i8;
#ifdef __LP64__
typedef int64_t go_uu;
#else
typedef int32_t go_uu;
#endif
typedef uint8_t go_u1;
typedef uint16_t go_u2;
typedef uint32_t go_u4;
typedef uint64_t go_u8;
typedef uintptr_t go_up;
typedef float go_f4;
typedef double go_f8;
typedef struct{go_f4 f1; go_f4 f2;} go_aaf4f4zz;
typedef struct{go_f8 f1; go_f8 f2;} go_aaf8f8zz;
typedef void* go_ch;
typedef struct { void (*ptr)(void); char context[]; } go_fn;
struct go_if;
typedef void* go_kv;
typedef struct { void* ptr; /*size_t off;*/ } go_pt;
typedef struct { go_pt ptr; go_ii len; go_ii cap; } go_ll;
typedef const struct { const char *ptr; const go_ii len; } go_ss;

typedef struct {} go_az;

typedef enum {
    go_kind_invalid = 0,
    go_kind_bool = 1,
    go_kind_int = 2,
    go_kind_int8 = 3,
    go_kind_int16 = 4,
    go_kind_int32 = 5,
    go_kind_int64 = 6,
    go_kind_uint = 7,
    go_kind_uint8 = 8,
    go_kind_uint16 = 9,
    go_kind_uint32 = 10,
    go_kind_uint64 = 11,
    go_kind_uintptr = 12,
    go_kind_float32 = 13,
    go_kind_float64 = 15,
    go_kind_complex64 = 16,
    go_kind_complex128 = 17,
    go_kind_array = 18,
    go_kind_chan = 19,
    go_kind_func = 20,
    go_kind_interface = 21,
    go_kind_map = 22,
    go_kind_pointer = 23,
    go_kind_slice = 24,
    go_kind_string = 25,
    go_kind_struct = 26,
    go_kind_unsafe_pointer = 27,
} go_kind;

#define go_kind_byte go_kind_uint8
#define go_kind_rune go_kind_int32

typedef struct {
    char *name;
    const struct go_type* type;
    go_ii offset;
    go_tf exported;
    go_tf embedded;
} go_field;

typedef struct { const struct go_type* elem; go_ii len; } go_type_array;
typedef struct { const struct go_type* elem; go_ii dir; } go_type_chan;
typedef struct { go_ll ins; go_ll outs; } go_type_func;
typedef struct { go_ll methods; } go_type_interface;
typedef struct { const struct go_type* key; const struct go_type* elem; } go_type_map;
typedef struct { const struct go_type* elem; } go_type_pointer;
typedef struct { const struct go_type* elem; } go_type_slice;
typedef struct { const go_field* field; go_ii count; } go_type_struct;

typedef union {
    go_type_array array;
    go_type_chan chan;
    go_type_func func;
    go_type_interface interface;
    go_type_map map;
    go_type_pointer pointer;
    go_type_slice slice;
    go_type_struct fields;
} go_type_data;

typedef struct go_type {
    char *name;
    go_kind kind;
    go_type_data data;
} go_type;

typedef struct go_if { go_pt ptr; const go_type* go_type; void* vtable; } go_if;
typedef struct { go_pt ptr; const go_type* go_type; } go_vv;

static inline go_aaf4f4zz go_complex64(go_f4 real, go_f4 imag) { return (go_aaf4f4zz){real, imag}; }
static inline go_aaf8f8zz go_complex128(go_f8 real, go_f8 imag) { return (go_aaf8f8zz){real, imag}; }

typedef struct {} go_tuple;

#define go_ignore(x) (void)(x)
#define go_split() go_ll go_defers = {};
#define go_defer(fn, T, ...) do { \
    go_defers = go_append(go_defers, sizeof(fn), &fn); \
    go_defers = go_append(go_defers, sizeof(T), &(T){__VA_ARGS__}); \
} while(0)

void go_routine(int(trampoline)(void*), go_fn fn, size_t arg_size, void* arg);
#define go_call(fn, IN, OUT, ...) go_routine(go_call_##IN##OUT, fn, sizeof(go_##IN), &(go_##IN){__VA_ARGS__})

#define go_main() int main(int argc, char* argv[])
static inline void go_print(const char* format, ...) {
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
}

typedef go_u8 (*go_hash)(const void *item, go_u8 seed0, go_u8 seed1);
typedef go_tf (*go_same)(const void *a, const void *b);

go_pt go_new(go_ii size,  const void* init);
#define go_pointer_new(t) go_new(sizeof(t), nil)
#define go_pointer_set(p, t, v) *(t*)((p).ptr) = (v)
#define go_pointer_get(p, t) (*(t*)((p).ptr))
#define go_pointer_slice(p, S, T, lo, hi, cap) go_slice((go_ll){p,S,S}, sizeof(T), lo, hi, cap)

go_ii go_copy(go_ii elem_size, go_ll dst, go_ll src);
go_ll go_append(go_ll s, go_ii elem_size, const void* elem);
go_ll go_slice(go_ll s, go_ii elem_size, go_ii low, go_ii high, go_ii cap);
void* go_index(go_ll s, go_ii elem_size, go_ii i);

#define go_slice_make(T, length, capacity) (go_ll){ .ptr = go_new(sizeof(T)*capacity, nil), .len = length, .cap = capacity }
#define go_slice_index(s, T, i) (*(T*)go_index(s, sizeof(T), i))
#define go_slice_copy(T, dst, src) go_copy(sizeof(T), dst, src)
#define go_slice_literal(length, T, ...) (go_ll){ .ptr = go_new(sizeof(T)*length, &(T[]){__VA_ARGS__}), .len = length, .cap = length }
#define go_variadic(length, T, ...) (go_ll){ .ptr = &(T[]){__VA_ARGS__}, .len = length, .cap = length }
static inline go_ii go_slice_len(go_ll s) { return s.len; }
void go_slice_clear(go_ll s);

go_kv go_make(go_ii key_size, go_ii elem_size, go_hash hash_func, go_same same_func, go_ii hint, go_ii argc, void* init);
#define go_map_make(K, V, hint) go_make(sizeof(go_##K), sizeof(go_##V), go_hash_##K, go_same_##K, hint, 0, nil)
#define go_map_literal(K, V, count, ...) go_make(sizeof(go_##K), sizeof(go_##V), go_hash_##K, go_same_##K, count, count, &(go_map_entry__##K##__##V[]){__VA_ARGS__})
void go_map_set(go_kv m, void* key, void* val);
go_tf go_map_get(go_kv m, void* key, void* val);

#define go_string_new(str) (go_ss){ .ptr = str, .len = -1 }
go_ii go_string_len(go_ss s);
go_tf go_string_eq(go_ss a, go_ss b);

#define go_chan_make(T, length) ((go_ch){})
go_ch go_chan(go_ii elem_size, go_ii cap);
void go_send(go_ch c, go_ii size, const void* v);
go_tf go_recv(go_ch c, go_ii size, void* v);

#define go_make_func(fn) ((go_fn){ .ptr = (void(*)(void))(fn) })
#define go_func_get(f, T) (T)(f.ptr)

static inline go_if go_interface_new(size_t size, const void* value, const go_type* go_type, void* vtable) {
    go_pt p = go_new(size, value);
    return (go_if){ .ptr = p, .go_type = go_type, .vtable = vtable };
}
#define go_interface_methods(T, v) ((T*)v.vtable)

go_vv go_any_new(size_t size, void* value, const go_type* go_type);

go_u8 go_hash_ss(const void* item, go_u8 seed0, go_u8 seed1);
go_tf go_same_ss(const void* a, const void* b);

static inline go_type* go_type_pointer_to(const go_type* to) {
    return go_new(sizeof(go_type), &(go_type){.name="*", .kind=go_kind_pointer, .data={.pointer={.elem=to}}}).ptr;
}

static inline void go_panic(const char* msg) {
    fprintf(stderr, "panic: %s\n", msg);
    abort();
}

static const go_type go_type_bool = {.name="bool", .kind=go_kind_bool};
static const go_type go_type_int = {.name="int", .kind=go_kind_int};
static const go_type go_type_int8 = {.name="int8", .kind=go_kind_int8};
static const go_type go_type_int16 = {.name="int16", .kind=go_kind_int16};
static const go_type go_type_int32 = {.name="int32", .kind=go_kind_int32};
static const go_type go_type_int64 = {.name="int64", .kind=go_kind_int64};
static const go_type go_type_uint = {.name="uint", .kind=go_kind_uint};
static const go_type go_type_uint8 = {.name="uint8", .kind=go_kind_uint8};
static const go_type go_type_uint16 = {.name="uint16", .kind=go_kind_uint16};
static const go_type go_type_uint32 = {.name="uint32", .kind=go_kind_uint32};
static const go_type go_type_uint64 = {.name="uint64", .kind=go_kind_uint64};
static const go_type go_type_uintptr = {.name="uintptr", .kind=go_kind_uintptr};
static const go_type go_type_float32 = {.name="float32", .kind=go_kind_float32};
static const go_type go_type_float64 = {.name="float64", .kind=go_kind_float64};
static const go_type go_type_complex64 = {.name="complex64", .kind=go_kind_complex64};
static const go_type go_type_complex128 = {.name="complex128", .kind=go_kind_complex128};
static const go_type go_type_byte = go_type_uint8;
static const go_type go_type_rune = go_type_int32;
static const go_type go_type_string = {.name="string", .kind=go_kind_string};

typedef struct { go_ss(*Error)(void*);} go_error;

#endif
