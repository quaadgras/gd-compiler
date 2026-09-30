// Hooks of internal/sync, which the Go runtime implements.
#include <go/internal/sync.h>

go_u8 runtime_rand_go_internal_sync_package(void) { return go_rand(); }
void runtime_SemacquireMutex_go_internal_sync_package(go_pt s, go_tf lifo, go_ii skipframes) { go_semacquire(s); }
void runtime_Semrelease_go_internal_sync_package(go_pt s, go_tf handoff, go_ii skipframes) { go_semrelease(s); }
go_tf runtime_canSpin_go_internal_sync_package(go_ii i) { return false; }
void runtime_doSpin_go_internal_sync_package(void) {}
go_i8 runtime_nanotime_go_internal_sync_package(void) { return go_nanotime(); }
void throw_go_internal_sync_package(go_ss msg) { go_fatal(msg); }
void fatal_go_internal_sync_package(go_ss msg) { go_fatal(msg); }
