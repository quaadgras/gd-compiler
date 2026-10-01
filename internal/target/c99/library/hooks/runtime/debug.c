// Hooks of runtime/debug, which the Go runtime implements: gd's runtime has none of the
// settings, so they report their defaults.
#include <go/runtime/debug.h>
#include <go/runtime/debug/private.h>

void readGCStats_go_runtime_debug_package(go_pt pauses) { (void)pauses; }
void freeOSMemory_go_runtime_debug_package(void) {}
go_ii setMaxStack_go_runtime_debug_package(go_ii n) { (void)n; return 1000000000; }
go_i4 setGCPercent_go_runtime_debug_package(go_i4 percent) { (void)percent; return 100; }
go_tf setPanicOnFault_go_runtime_debug_package(go_tf enabled) { (void)enabled; return false; }
go_ii setMaxThreads_go_runtime_debug_package(go_ii n) { (void)n; return 10000; }
go_i8 setMemoryLimit_go_runtime_debug_package(go_i8 limit) { (void)limit; return INT64_MAX; }
go_ss modinfo_go_runtime_debug_package(void) { return (go_ss){0}; }
void WriteHeapDump_go_runtime_debug_package(go_up fd) { (void)fd; }
void SetTraceback_go_runtime_debug_package(go_ss level) { (void)level; }
