// Hooks of internal/runtime/atomic, which the Go runtime implements in assembly, with C11
// atomics.
#include <go/internal/runtime/atomic.h>
#include <stdatomic.h>

#define go_atomic(T, p) ((_Atomic(T)*)go_nil_check((p).ptr))
void And_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { atomic_fetch_and(go_atomic(go_u4, ptr), v); }
go_u4 And32_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { return atomic_fetch_and(go_atomic(go_u4, ptr), v); }
go_u8 And64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 v) { return atomic_fetch_and(go_atomic(go_u8, ptr), v); }
void And8_go_internal_runtime_atomic_package(go_pt ptr, go_u1 v) { atomic_fetch_and(go_atomic(go_u1, ptr), v); }
go_up Anduintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { return atomic_fetch_and(go_atomic(go_up, ptr), v); }
go_tf Cas_go_internal_runtime_atomic_package(go_pt ptr, go_u4 old, go_u4 v) { return atomic_compare_exchange_strong(go_atomic(go_u4, ptr), &old, v); }
go_tf Cas64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 old, go_u8 v) { return atomic_compare_exchange_strong(go_atomic(go_u8, ptr), &old, v); }
go_tf CasRel_go_internal_runtime_atomic_package(go_pt ptr, go_u4 old, go_u4 v) { return atomic_compare_exchange_strong(go_atomic(go_u4, ptr), &old, v); }
go_tf Casint32_go_internal_runtime_atomic_package(go_pt ptr, go_i4 old, go_i4 v) { return atomic_compare_exchange_strong(go_atomic(go_i4, ptr), &old, v); }
go_tf Casint64_go_internal_runtime_atomic_package(go_pt ptr, go_i8 old, go_i8 v) { return atomic_compare_exchange_strong(go_atomic(go_i8, ptr), &old, v); }
go_tf Casuintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up old, go_up v) { return atomic_compare_exchange_strong(go_atomic(go_up, ptr), &old, v); }
go_i4 Loadint32_go_internal_runtime_atomic_package(go_pt ptr) { return atomic_load(go_atomic(go_i4, ptr)); }
go_i8 Loadint64_go_internal_runtime_atomic_package(go_pt ptr) { return atomic_load(go_atomic(go_i8, ptr)); }
go_uu Loaduint_go_internal_runtime_atomic_package(go_pt ptr) { return atomic_load(go_atomic(go_uu, ptr)); }
go_up Loaduintptr_go_internal_runtime_atomic_package(go_pt ptr) { return atomic_load(go_atomic(go_up, ptr)); }
void Or_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { atomic_fetch_or(go_atomic(go_u4, ptr), v); }
go_u4 Or32_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { return atomic_fetch_or(go_atomic(go_u4, ptr), v); }
go_u8 Or64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 v) { return atomic_fetch_or(go_atomic(go_u8, ptr), v); }
void Or8_go_internal_runtime_atomic_package(go_pt ptr, go_u1 v) { atomic_fetch_or(go_atomic(go_u1, ptr), v); }
go_up Oruintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { return atomic_fetch_or(go_atomic(go_up, ptr), v); }
void Store_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { atomic_store(go_atomic(go_u4, ptr), v); }
void Store64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 v) { atomic_store(go_atomic(go_u8, ptr), v); }
void Store8_go_internal_runtime_atomic_package(go_pt ptr, go_u1 v) { atomic_store(go_atomic(go_u1, ptr), v); }
void StoreRel_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { atomic_store(go_atomic(go_u4, ptr), v); }
void StoreRel64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 v) { atomic_store(go_atomic(go_u8, ptr), v); }
void StoreReluintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { atomic_store(go_atomic(go_up, ptr), v); }
void Storeint32_go_internal_runtime_atomic_package(go_pt ptr, go_i4 v) { atomic_store(go_atomic(go_i4, ptr), v); }
void Storeint64_go_internal_runtime_atomic_package(go_pt ptr, go_i8 v) { atomic_store(go_atomic(go_i8, ptr), v); }
void Storeuintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { atomic_store(go_atomic(go_up, ptr), v); }
go_u4 Xadd_go_internal_runtime_atomic_package(go_pt ptr, go_i4 v) { return atomic_fetch_add(go_atomic(go_u4, ptr), (go_u4)v) + (go_u4)v; }
go_u8 Xadd64_go_internal_runtime_atomic_package(go_pt ptr, go_i8 v) { return atomic_fetch_add(go_atomic(go_u8, ptr), (go_u8)v) + (go_u8)v; }
go_i4 Xaddint32_go_internal_runtime_atomic_package(go_pt ptr, go_i4 v) { return atomic_fetch_add(go_atomic(go_i4, ptr), (go_i4)v) + (go_i4)v; }
go_i8 Xaddint64_go_internal_runtime_atomic_package(go_pt ptr, go_i8 v) { return atomic_fetch_add(go_atomic(go_i8, ptr), (go_i8)v) + (go_i8)v; }
go_up Xadduintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { return atomic_fetch_add(go_atomic(go_up, ptr), (go_up)v) + (go_up)v; }
go_u4 Xchg_go_internal_runtime_atomic_package(go_pt ptr, go_u4 v) { return atomic_exchange(go_atomic(go_u4, ptr), v); }
go_u8 Xchg64_go_internal_runtime_atomic_package(go_pt ptr, go_u8 v) { return atomic_exchange(go_atomic(go_u8, ptr), v); }
go_u1 Xchg8_go_internal_runtime_atomic_package(go_pt ptr, go_u1 v) { return atomic_exchange(go_atomic(go_u1, ptr), v); }
go_i4 Xchgint32_go_internal_runtime_atomic_package(go_pt ptr, go_i4 v) { return atomic_exchange(go_atomic(go_i4, ptr), v); }
go_i8 Xchgint64_go_internal_runtime_atomic_package(go_pt ptr, go_i8 v) { return atomic_exchange(go_atomic(go_i8, ptr), v); }
go_up Xchguintptr_go_internal_runtime_atomic_package(go_pt ptr, go_up v) { return atomic_exchange(go_atomic(go_up, ptr), v); }
go_tf Casp1_go_internal_runtime_atomic_package(go_pt ptr, go_pt old, go_pt v) { void* o = old.ptr; return atomic_compare_exchange_strong(go_atomic(void*, ptr), &o, v.ptr); }
go_tf casPointer_go_internal_runtime_atomic_package(go_pt ptr, go_pt old, go_pt v) { return Casp1_go_internal_runtime_atomic_package(ptr, old, v); }
void StorepNoWB_go_internal_runtime_atomic_package(go_pt ptr, go_pt v) { atomic_store(go_atomic(void*, ptr), v.ptr); }
void storePointer_go_internal_runtime_atomic_package(go_pt ptr, go_pt v) { atomic_store(go_atomic(void*, ptr), v.ptr); }
