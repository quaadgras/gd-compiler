// Hooks of internal/cpu, which Go implements in assembly: no CPU features are reported, so
// that packages use their portable code.
#include <go/internal/cpu.h>

go_tuple_go_u4_go_u4_go_u4_go_u4 cpuid_go_internal_cpu_package(go_u4 eaxArg, go_u4 ecxArg) { return (go_tuple_go_u4_go_u4_go_u4_go_u4){0}; }
go_tuple_go_u4_go_u4 xgetbv_go_internal_cpu_package(void) { return (go_tuple_go_u4_go_u4){0}; }
go_i4 getGOAMD64level_go_internal_cpu_package(void) { return 1; }
