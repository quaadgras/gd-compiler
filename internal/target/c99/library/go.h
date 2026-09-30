//go:build ignore
#ifndef GO
#define GO

#include <stdarg.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <stdbool.h>
#include <stddef.h>
#include <setjmp.h>
#include <string.h>
#include <math.h>

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
// A func value: code, and the environment of a closure (NULL for other functions). Code
// called through a func value takes the environment as its first argument.
typedef struct { void (*ptr)(void); void* env; } go_fn;
struct go_if;
typedef void* go_kv;
typedef struct { void* ptr; /*size_t off;*/ } go_pt;
typedef struct { go_pt ptr; go_ii len; go_ii cap; } go_ll;
typedef struct { const char *ptr; go_ii len; } go_ss;

// C has no empty structs.
typedef struct { char _; } go_az;

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

// A method of a type (its method set): name, signature (like func(int) string) and the
// function that calls it with the data of an interface value (the I_ and IP_ wrappers).
typedef struct go_method { const char* name; const char* type; void (*fn)(void); } go_method;

typedef struct go_type {
    char *name;
    go_kind kind;
    go_type_data data;
    const go_method* methods; // sorted by name, like the methods of interfaces.
    go_ii nmethods;
    go_tf (*equal)(const void*, const void*); // of structs and arrays, NULL if not comparable.
    go_u8 (*hash)(const void*, go_u8, go_u8); // of structs and arrays, for map keys.
} go_type;

// The methods an interface requires, in the order of its table of methods.
typedef struct { const char* name; const char* type; } go_imethod;

typedef struct go_if { go_pt ptr; const go_type* go_type; void* vtable; } go_if;
typedef struct { go_pt ptr; const go_type* go_type; } go_vv;

static inline go_aaf4f4zz go_complex64(go_f4 real, go_f4 imag) { return (go_aaf4f4zz){real, imag}; }
static inline go_aaf8f8zz go_complex128(go_f8 real, go_f8 imag) { return (go_aaf8f8zz){real, imag}; }
#define go_complex_ops(T, F) \
    static inline T T##_add(T a, T b) { return (T){a.f1 + b.f1, a.f2 + b.f2}; } \
    static inline T T##_sub(T a, T b) { return (T){a.f1 - b.f1, a.f2 - b.f2}; } \
    static inline T T##_mul(T a, T b) { return (T){a.f1 * b.f1 - a.f2 * b.f2, a.f1 * b.f2 + a.f2 * b.f1}; } \
    static inline T T##_neg(T a) { return (T){-a.f1, -a.f2}; }
static inline go_aaf4f4zz go_aaf4f4zz_convert(go_aaf8f8zz a) { return (go_aaf4f4zz){(go_f4)a.f1, (go_f4)a.f2}; }
static inline go_aaf8f8zz go_aaf8f8zz_convert(go_aaf4f4zz a) { return (go_aaf8f8zz){a.f1, a.f2}; }
go_complex_ops(go_aaf4f4zz, go_f4)
go_complex_ops(go_aaf8f8zz, go_f8)
go_aaf8f8zz go_aaf8f8zz_quo(go_aaf8f8zz n, go_aaf8f8zz m);
static inline go_aaf4f4zz go_aaf4f4zz_quo(go_aaf4f4zz n, go_aaf4f4zz m) {
    go_aaf8f8zz q = go_aaf8f8zz_quo((go_aaf8f8zz){n.f1, n.f2}, (go_aaf8f8zz){m.f1, m.f2});
    return (go_aaf4f4zz){(go_f4)q.f1, (go_f4)q.f2};
}

typedef struct { char _; } go_tuple;

#define go_ignore(x) (void)(x)
// Every function starts with go_split, which takes the token that allows recover to
// stop a panic, see go_recover.
#define go_split() go_tf go_can_recover = go_take_recover(); (void)go_can_recover;


// go_main defines C's main, that runs the Go main function as the main goroutine (with a
// larger stack than C's main thread may have, as Go's stacks grow), then exits.
int go_run_main(int (*main)(void));
#define go_main() static int go_main_goroutine(void); \
    int main(int argc, char* argv[]) { (void)argc; (void)argv; return go_run_main(go_main_goroutine); } \
    static int go_main_goroutine(void)
static inline void go_print(const char* format, ...) {
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
}

// print and println, see runtime/print.go. Output goes to stderr, as with gc.
void go_print_string(go_ss s);
void go_print_cstring(const char* s);
void go_print_bool(go_tf v);
void go_print_int(go_i8 v);
void go_print_uint(go_u8 v);
void go_print_float64(go_f8 v);
void go_print_float32(go_f4 v);
void go_print_complex128(go_aaf8f8zz v);
void go_print_complex64(go_aaf4f4zz v);
void go_print_pointer(go_up p);
void go_print_slice(go_ll s);
void go_print_iface(go_up type, go_up data);

// defer, panic and recover. A function with deferred calls pushes a frame, and sets its
// jmp_buf with setjmp, panics longjmp to the innermost frame, which runs its deferred calls,
// either returning normally if one of them recovered, or continuing the panic.
#if defined(_MSC_VER) && !defined(__clang__)
#define go_thread_local __declspec(thread)
#else
#define go_thread_local _Thread_local
#endif
typedef struct go_deferred { go_fn fn; struct go_deferred* next; } go_deferred;
typedef struct go_frame { jmp_buf jb; struct go_frame* prev; go_deferred* defers; } go_frame;
go_frame* go_frame_push(void);
void go_defer_push(go_frame* f, go_fn fn);
void go_frame_return(go_frame* f);
void go_frame_unwind(go_frame* f);
go_tf go_take_recover(void);
go_vv go_recover(go_tf can_recover);
_Noreturn void go_panic_any(go_vv v);
go_tf go_type_eq(const go_type* a, const go_type* b);
go_tf go_vv_eq(go_vv a, go_vv b); // panics if the dynamic type is not comparable.
go_u8 go_vv_hash(go_vv v, go_u8 seed0, go_u8 seed1); // panics if the dynamic type is not hashable.

// The runtime, for the hooks of packages (see library/hooks), see sema.c.
void go_semacquire(go_pt addr);
void go_semrelease(go_pt addr);
go_u4 go_notify_add(go_pt wait);
void go_notify_wait(go_pt notify, go_u4 ticket);
void go_notify(go_pt wait, go_pt notify, go_tf all);
go_i8 go_nanotime(void);
go_u8 go_rand(void);
_Noreturn void go_fatal(go_ss msg);
static inline go_vv go_if_to_vv(go_if v) { return (go_vv){ .ptr = v.ptr, .go_type = v.go_type }; }
_Noreturn void go_panic_assertion(const go_type* want, go_vv have);
// go_implements reports whether t has the methods, filling table (if not NULL) with them.
go_tf go_implements(const go_type* t, const go_imethod* methods, go_ii n, void (**table)(void));
// go_to_iface converts v to an interface with the methods (a type assertion when assert,
// which panics, otherwise *ok reports whether it could). nil converts to nil.
go_if go_to_iface(go_vv v, const char* iface, const go_imethod* methods, go_ii n, go_tf assert, go_tf* ok);

typedef go_u8 (*go_hash)(const void *item, go_u8 seed0, go_u8 seed1);
typedef go_tf (*go_same)(const void *a, const void *b);

go_pt go_new(go_ii size,  const void* init);
#define go_pointer_new(t) go_new(sizeof(t), nil)
#define go_pointer_set(p, t, ...) *(t*)go_nil_check((p).ptr) = (__VA_ARGS__) // values may have commas.
#define go_pointer_get(p, t) (*(t*)go_nil_check((p).ptr))
// go_slice slices s[low:high:max], omitted bounds are go_slice_default.
#define go_slice_default INT64_MIN
#define go_pointer_slice(p, S, T, lo, hi, max) go_slice((go_ll){ (go_pt){ go_nil_check((p).ptr) }, S, S }, sizeof(T), lo, hi, max)

go_ii go_copy(go_ii elem_size, go_ll dst, go_ll src);
go_ll go_append(go_ll s, go_ii elem_size, const void* elem);
go_ll go_append_slice(go_ll s, go_ii elem_size, go_ll t);
go_ll go_append_string(go_ll s, go_ss t);
go_ll go_slice(go_ll s, go_ii elem_size, go_i8 low, go_i8 high, go_i8 max);
void* go_index(go_ll s, go_ii elem_size, go_ii i);

#define go_slice_make(T, length, capacity) (go_ll){ .ptr = go_new(sizeof(T)*capacity, nil), .len = length, .cap = capacity }
#define go_slice_index(s, T, i) (*(T*)go_index(s, sizeof(T), i))
#define go_slice_copy(T, dst, src) go_copy(sizeof(T), dst, src)
#define go_slice_literal(length, T, ...) (go_ll){ .ptr = go_new(sizeof(T)*length, &(T[]){__VA_ARGS__}), .len = length, .cap = length }
#define go_variadic(length, T, ...) go_slice_literal(length, T, __VA_ARGS__) // may be kept by the callee.
static inline go_ii go_slice_len(go_ll s) { return s.len; }
void go_slice_clear(go_ll s);

// go_make makes a map, with argc entries from init, each stride bytes, with the value at
// val_offset (for map literals).
go_kv go_make(go_ii key_size, go_ii elem_size, go_hash hash_func, go_same same_func, go_ii hint, go_ii argc, const void* init, size_t stride, size_t val_offset);
go_u8 go_hash_bytes(const void* p, size_t n, go_u8 seed0, go_u8 seed1);
void go_map_set(go_kv m, const void* key, const void* val);
go_ii go_map_len(go_kv m);
void go_map_delete(go_kv m, const void* key);
go_tf go_map_get(go_kv m, const void* key, void* val);

#define go_string_new(str) (go_ss){ .ptr = str, .len = -1 }
#define go_string_const(str) { .ptr = str, .len = -1 } // for static initializers.
go_ii go_string_len(go_ss s);
// go_bytes_of_string is a byte slice that shares the bytes of s (for reading them).
static inline go_ll go_bytes_of_string(go_ss s) {
    go_ii n = go_string_len(s);
    return (go_ll){ .ptr = { .ptr = (void*)s.ptr }, .len = n, .cap = n };
}
go_tf go_string_eq(go_ss a, go_ss b);
go_ii go_string_cmp(go_ss a, go_ss b);
go_ss go_string_concat(go_ss a, go_ss b);
go_u1 go_string_index(go_ss s, go_ii i);
go_ss go_string_slice(go_ss s, go_i8 low, go_i8 high);
go_ii go_string_decode(go_ss s, go_ii i, go_i4* r); // UTF-8, returns the width.
go_ss go_string_from_rune(go_i8 r);
go_ss go_string_from_bytes(go_ll b);
go_ll go_bytes_from_string(go_ss s);
go_ss go_string_from_runes(go_ll r);
go_ll go_runes_from_string(go_ss s);

// Goroutines and channels, see chan.c.
go_ch go_chan(go_ii elem_size, go_ii cap);
void go_send(go_ch c, go_ii size, const void* v);
go_tf go_recv(go_ch c, go_ii size, void* v); // false when the channel is closed.
void go_close(go_ch c);
go_ii go_chan_len(go_ch c);
go_ii go_chan_cap(go_ch c);
typedef struct { go_ch c; go_tf send; void* elem; go_tf ok; } go_select_case;
int go_select(go_select_case* cases, int n, go_tf block); // the case, or -1 (default).
void go_start(go_fn fn); // a goroutine.

#define go_make_func(fn) ((go_fn){ .ptr = (void(*)(void))(fn), .env = NULL })
#define go_make_closure(fn, environment) ((go_fn){ .ptr = (void(*)(void))(fn), .env = (environment) })
#define go_func_get(f, T) (T)(f.ptr)

static inline go_if go_interface_new(size_t size, const void* value, const go_type* go_type, void* vtable) {
    go_pt p = go_new(size, value);
    return (go_if){ .ptr = p, .go_type = go_type, .vtable = vtable };
}
#define go_interface_methods(T, v) ((T*)v.vtable)

go_vv go_any_new(size_t size, void* value, const go_type* go_type);

go_u8 go_hash_ss(const void* item, go_u8 seed0, go_u8 seed1);
go_tf go_same_ss(const void* a, const void* b);

// Runtime errors panic with a value of type runtime.Error, holding the message (a string).
extern const go_type go_type_runtime_error;
_Noreturn void go_panic_error(const char* format, ...);
static inline void go_panic(const char* msg) { go_panic_error("%s", msg); }
static inline void* go_nil_check(void* p) {
    if (!p) go_panic_error("runtime error: invalid memory address or nil pointer dereference");
    return p;
}
static inline go_ii go_index_check(go_ii i, go_ii length) {
    if (i < 0) go_panic_error("runtime error: index out of range [%lld]", (long long)i);
    if (i >= length) go_panic_error("runtime error: index out of range [%lld] with length %lld", (long long)i, (long long)length);
    return i;
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
static const go_type go_type_unsafe_pointer = {.name="unsafe.Pointer", .kind=go_kind_unsafe_pointer};
static const go_type go_type_error = {.name="error", .kind=go_kind_interface};

typedef struct { go_ss(*Error)(void*);} go_error;

#endif
