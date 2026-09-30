//go:build ignore

// Semaphores and notification lists (for package sync), see runtime/sema.go, and the time
// and random numbers of the runtime.

#include <go.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <threads.h>
#include <time.h>

// Waiters sleep on a single condition variable, and are all woken (to check whether it's
// their turn) by each release, which is simple, if not efficient.
static mtx_t go_sema_lock;
static cnd_t go_sema_cond;
static once_flag go_sema_once = ONCE_FLAG_INIT;
static void go_sema_init(void) { mtx_init(&go_sema_lock, mtx_plain); cnd_init(&go_sema_cond); }

void go_semacquire(go_pt addr) {
    _Atomic go_u4* s = go_nil_check(addr.ptr);
    call_once(&go_sema_once, go_sema_init);
    mtx_lock(&go_sema_lock);
    while (atomic_load(s) == 0) cnd_wait(&go_sema_cond, &go_sema_lock);
    atomic_fetch_sub(s, 1);
    mtx_unlock(&go_sema_lock);
}

void go_semrelease(go_pt addr) {
    _Atomic go_u4* s = go_nil_check(addr.ptr);
    call_once(&go_sema_once, go_sema_init);
    mtx_lock(&go_sema_lock);
    atomic_fetch_add(s, 1);
    cnd_broadcast(&go_sema_cond);
    mtx_unlock(&go_sema_lock);
}

// A notification list (sync.Cond) is a ticket (wait) of the next waiter, and of the next
// to notify (notify), see runtime.notifyList.
go_u4 go_notify_add(go_pt wait) { return atomic_fetch_add((_Atomic go_u4*)go_nil_check(wait.ptr), 1); }

void go_notify_wait(go_pt notify, go_u4 ticket) {
    _Atomic go_u4* n = go_nil_check(notify.ptr);
    call_once(&go_sema_once, go_sema_init);
    mtx_lock(&go_sema_lock);
    while ((go_i4)(ticket - atomic_load(n)) >= 0) cnd_wait(&go_sema_cond, &go_sema_lock);
    mtx_unlock(&go_sema_lock);
}

void go_notify(go_pt wait, go_pt notify, go_tf all) {
    _Atomic go_u4* w = go_nil_check(wait.ptr);
    _Atomic go_u4* n = go_nil_check(notify.ptr);
    call_once(&go_sema_once, go_sema_init);
    mtx_lock(&go_sema_lock);
    if (atomic_load(w) != atomic_load(n)) {
        if (all) atomic_store(n, atomic_load(w)); else atomic_fetch_add(n, 1);
        cnd_broadcast(&go_sema_cond);
    }
    mtx_unlock(&go_sema_lock);
}

go_i8 go_nanotime(void) {
    struct timespec ts;
    timespec_get(&ts, TIME_UTC);
    return (go_i8)ts.tv_sec * 1000000000 + ts.tv_nsec;
}

go_u8 go_rand(void) { // splitmix64, seeded by the time.
    static _Atomic go_u8 state;
    if (atomic_load(&state) == 0) atomic_store(&state, (go_u8)go_nanotime() | 1);
    go_u8 z = atomic_fetch_add(&state, 0x9e3779b97f4a7c15ULL) + 0x9e3779b97f4a7c15ULL;
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ULL;
    z = (z ^ (z >> 27)) * 0x94d049bb133111ebULL;
    return z ^ (z >> 31);
}

_Noreturn void go_fatal(go_ss msg) {
    fprintf(stderr, "fatal error: %.*s\n", (int)msg.len, msg.ptr);
    exit(2);
}
