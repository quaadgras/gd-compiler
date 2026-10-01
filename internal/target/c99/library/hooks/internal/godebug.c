// Hooks of internal/godebug, which the Go runtime implements: settings are read from the
// GODEBUG environment variable once, and metrics are not recorded.
#include <go/internal/godebug.h>
#include <stdio.h>
#include <stdlib.h>

#define S(name) name##_go_internal_godebug_package

void S(setUpdate)(go_fn update) {
    const char* env = getenv("GODEBUG");
    ((void(*)(void*, go_ss, go_ss))update.ptr)(update.env, go_string_new(""), go_string_new(env ? env : ""));
}
void S(registerMetric)(go_ss name, go_fn read) {}
void S(setNewIncNonDefault)(go_fn newIncNonDefault) {}
go_i4 S(write)(go_up fd, go_pt p, go_i4 n) {
    FILE* f = fd == 2 ? stderr : stdout;
    return (go_i4)fwrite(p.ptr, 1, (size_t)n, f);
}
