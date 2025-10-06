#ifndef go_unsafe_package_imported
#define go_unsafe_package_imported
#include <go.h>
#include <stddef.h>

static go_pt Add_go_unsafe_package(go_pt ptr, go_ii len) { return (go_pt){ .ptr = (char*)ptr.ptr + len }; }
static go_pt SliceData_go_unsafe_package(go_ll s) { return s.ptr; }
static go_ll Slice_go_unsafe_package(go_pt ptr, go_ii len) { return (go_ll){ .ptr = ptr, .len = len, .cap = len }; }
static go_pt StringData_go_unsafe_package(go_ss s) { return (go_pt){ .ptr = s.ptr }; }
static go_ss String_go_unsafe_package(go_pt ptr, go_ii len) { return (go_ss){ .ptr = (char*)ptr.ptr, .len = len }; }
#define Sizeof_go_unsafe_package(x) { return sizeof(x); }
#define Alignof_go_unsafe_package(x) { return _Alignof(x); }

#endif // go_unsafe_package_imported
