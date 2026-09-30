// Hooks of sync/atomic, which the Go runtime implements in assembly, with C11 atomics.
#include <go/sync/atomic.h>
#include <stdatomic.h>

#define go_atomic(T, p) ((_Atomic(T)*)go_nil_check((p).ptr))
go_i4 SwapInt32_go_sync_atomic_package(go_pt p, go_i4 v) { return atomic_exchange(go_atomic(go_i4, p), v); }
go_tf CompareAndSwapInt32_go_sync_atomic_package(go_pt p, go_i4 old, go_i4 v) { return atomic_compare_exchange_strong(go_atomic(go_i4, p), &old, v); }
go_i4 AddInt32_go_sync_atomic_package(go_pt p, go_i4 d) { return atomic_fetch_add(go_atomic(go_i4, p), d) + d; }
go_i4 AndInt32_go_sync_atomic_package(go_pt p, go_i4 m) { return atomic_fetch_and(go_atomic(go_i4, p), m); }
go_i4 OrInt32_go_sync_atomic_package(go_pt p, go_i4 m) { return atomic_fetch_or(go_atomic(go_i4, p), m); }
go_i4 LoadInt32_go_sync_atomic_package(go_pt p) { return atomic_load(go_atomic(go_i4, p)); }
void StoreInt32_go_sync_atomic_package(go_pt p, go_i4 v) { atomic_store(go_atomic(go_i4, p), v); }
go_u4 SwapUint32_go_sync_atomic_package(go_pt p, go_u4 v) { return atomic_exchange(go_atomic(go_u4, p), v); }
go_tf CompareAndSwapUint32_go_sync_atomic_package(go_pt p, go_u4 old, go_u4 v) { return atomic_compare_exchange_strong(go_atomic(go_u4, p), &old, v); }
go_u4 AddUint32_go_sync_atomic_package(go_pt p, go_u4 d) { return atomic_fetch_add(go_atomic(go_u4, p), d) + d; }
go_u4 AndUint32_go_sync_atomic_package(go_pt p, go_u4 m) { return atomic_fetch_and(go_atomic(go_u4, p), m); }
go_u4 OrUint32_go_sync_atomic_package(go_pt p, go_u4 m) { return atomic_fetch_or(go_atomic(go_u4, p), m); }
go_u4 LoadUint32_go_sync_atomic_package(go_pt p) { return atomic_load(go_atomic(go_u4, p)); }
void StoreUint32_go_sync_atomic_package(go_pt p, go_u4 v) { atomic_store(go_atomic(go_u4, p), v); }
go_up SwapUintptr_go_sync_atomic_package(go_pt p, go_up v) { return atomic_exchange(go_atomic(go_up, p), v); }
go_tf CompareAndSwapUintptr_go_sync_atomic_package(go_pt p, go_up old, go_up v) { return atomic_compare_exchange_strong(go_atomic(go_up, p), &old, v); }
go_up AddUintptr_go_sync_atomic_package(go_pt p, go_up d) { return atomic_fetch_add(go_atomic(go_up, p), d) + d; }
go_up AndUintptr_go_sync_atomic_package(go_pt p, go_up m) { return atomic_fetch_and(go_atomic(go_up, p), m); }
go_up OrUintptr_go_sync_atomic_package(go_pt p, go_up m) { return atomic_fetch_or(go_atomic(go_up, p), m); }
go_up LoadUintptr_go_sync_atomic_package(go_pt p) { return atomic_load(go_atomic(go_up, p)); }
void StoreUintptr_go_sync_atomic_package(go_pt p, go_up v) { atomic_store(go_atomic(go_up, p), v); }
go_i8 SwapInt64_go_sync_atomic_package(go_pt p, go_i8 v) { return atomic_exchange(go_atomic(go_i8, p), v); }
go_tf CompareAndSwapInt64_go_sync_atomic_package(go_pt p, go_i8 old, go_i8 v) { return atomic_compare_exchange_strong(go_atomic(go_i8, p), &old, v); }
go_i8 AddInt64_go_sync_atomic_package(go_pt p, go_i8 d) { return atomic_fetch_add(go_atomic(go_i8, p), d) + d; }
go_i8 AndInt64_go_sync_atomic_package(go_pt p, go_i8 m) { return atomic_fetch_and(go_atomic(go_i8, p), m); }
go_i8 OrInt64_go_sync_atomic_package(go_pt p, go_i8 m) { return atomic_fetch_or(go_atomic(go_i8, p), m); }
go_i8 LoadInt64_go_sync_atomic_package(go_pt p) { return atomic_load(go_atomic(go_i8, p)); }
void StoreInt64_go_sync_atomic_package(go_pt p, go_i8 v) { atomic_store(go_atomic(go_i8, p), v); }
go_u8 SwapUint64_go_sync_atomic_package(go_pt p, go_u8 v) { return atomic_exchange(go_atomic(go_u8, p), v); }
go_tf CompareAndSwapUint64_go_sync_atomic_package(go_pt p, go_u8 old, go_u8 v) { return atomic_compare_exchange_strong(go_atomic(go_u8, p), &old, v); }
go_u8 AddUint64_go_sync_atomic_package(go_pt p, go_u8 d) { return atomic_fetch_add(go_atomic(go_u8, p), d) + d; }
go_u8 AndUint64_go_sync_atomic_package(go_pt p, go_u8 m) { return atomic_fetch_and(go_atomic(go_u8, p), m); }
go_u8 OrUint64_go_sync_atomic_package(go_pt p, go_u8 m) { return atomic_fetch_or(go_atomic(go_u8, p), m); }
go_u8 LoadUint64_go_sync_atomic_package(go_pt p) { return atomic_load(go_atomic(go_u8, p)); }
void StoreUint64_go_sync_atomic_package(go_pt p, go_u8 v) { atomic_store(go_atomic(go_u8, p), v); }
go_pt SwapPointer_go_sync_atomic_package(go_pt p, go_pt v) { return (go_pt){ atomic_exchange(go_atomic(void*, p), v.ptr) }; }
go_tf CompareAndSwapPointer_go_sync_atomic_package(go_pt p, go_pt old, go_pt v) { void* o = old.ptr; return atomic_compare_exchange_strong(go_atomic(void*, p), &o, v.ptr); }
go_pt LoadPointer_go_sync_atomic_package(go_pt p) { return (go_pt){ atomic_load(go_atomic(void*, p)) }; }
void StorePointer_go_sync_atomic_package(go_pt p, go_pt v) { atomic_store(go_atomic(void*, p), v.ptr); }

go_ii runtime_procPin_go_sync_atomic_package(void) { return 0; }
void runtime_procUnpin_go_sync_atomic_package(void) {}
