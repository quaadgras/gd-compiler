//go:build ignore

// Semaphores and notification lists (for package sync), see runtime/sema.go, and the time
// and random numbers of the runtime.

#include <go.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <threads.h>
#include <time.h>

// Semaphores are kept in a table of buckets, by address (as runtime.semtable): a waiter
// queues itself on the bucket of its semaphore and sleeps on its own condition variable,
// until a release wakes it (only the first waiter of the semaphore), to try again.
typedef struct go_sema_waiter {
    void* addr;
    cnd_t cond;
    go_tf woken;
    struct go_sema_waiter* next;
} go_sema_waiter;

typedef struct {
    mtx_t lock;
    cnd_t cond; // (of notification lists, which are woken all at once)
    go_sema_waiter *first, *last;
} go_sema_bucket;

#define go_sema_buckets 251
static go_sema_bucket go_sema_table[go_sema_buckets];
static once_flag go_sema_once = ONCE_FLAG_INIT;
static void go_sema_init(void) {
    for (int i = 0; i < go_sema_buckets; i++) {
        mtx_init(&go_sema_table[i].lock, mtx_plain);
        cnd_init(&go_sema_table[i].cond);
    }
}
static go_sema_bucket* go_sema_bucket_of(void* addr) {
    call_once(&go_sema_once, go_sema_init);
    return &go_sema_table[((uintptr_t)addr >> 3) % go_sema_buckets];
}

static go_tf go_sema_try(_Atomic go_u4* s) {
    go_u4 v = atomic_load(s);
    while (v > 0) {
        if (atomic_compare_exchange_weak(s, &v, v - 1)) return true;
    }
    return false;
}

void go_semacquire(go_pt addr) {
    _Atomic go_u4* s = go_nil_check(addr.ptr);
    if (go_sema_try(s)) return;
    go_sema_bucket* b = go_sema_bucket_of(addr.ptr);
    go_sema_waiter w = { .addr = addr.ptr };
    cnd_init(&w.cond);
    mtx_lock(&b->lock);
    while (!go_sema_try(s)) {
        w.woken = false;
        w.next = NULL;
        if (b->last) b->last->next = &w; else b->first = &w;
        b->last = &w;
        while (!w.woken) cnd_wait(&w.cond, &b->lock);
    }
    mtx_unlock(&b->lock);
    cnd_destroy(&w.cond);
}

void go_semrelease(go_pt addr) {
    _Atomic go_u4* s = go_nil_check(addr.ptr);
    atomic_fetch_add(s, 1);
    go_sema_bucket* b = go_sema_bucket_of(addr.ptr);
    mtx_lock(&b->lock);
    go_sema_waiter* prev = NULL;
    for (go_sema_waiter* w = b->first; w; prev = w, w = w->next) {
        if (w->addr != addr.ptr) continue;
        if (prev) prev->next = w->next; else b->first = w->next;
        if (b->last == w) b->last = prev;
        w->woken = true;
        cnd_signal(&w->cond);
        break;
    }
    mtx_unlock(&b->lock);
}

// A notification list (sync.Cond) is a ticket (wait) of the next waiter, and of the next
// to notify (notify), see runtime.notifyList. Its waiters sleep on the condition variable
// of its bucket.
go_u4 go_notify_add(go_pt wait) { return atomic_fetch_add((_Atomic go_u4*)go_nil_check(wait.ptr), 1); }

void go_notify_wait(go_pt notify, go_u4 ticket) {
    _Atomic go_u4* n = go_nil_check(notify.ptr);
    go_sema_bucket* b = go_sema_bucket_of(notify.ptr);
    mtx_lock(&b->lock);
    while ((go_i4)(ticket - atomic_load(n)) >= 0) cnd_wait(&b->cond, &b->lock);
    mtx_unlock(&b->lock);
}

void go_notify(go_pt wait, go_pt notify, go_tf all) {
    _Atomic go_u4* w = go_nil_check(wait.ptr);
    _Atomic go_u4* n = go_nil_check(notify.ptr);
    go_sema_bucket* b = go_sema_bucket_of(notify.ptr);
    mtx_lock(&b->lock);
    if (atomic_load(w) != atomic_load(n)) {
        if (all) atomic_store(n, atomic_load(w)); else atomic_fetch_add(n, 1);
        cnd_broadcast(&b->cond);
    }
    mtx_unlock(&b->lock);
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

// runtime.MemProfileRate (gd has no memory profiles), defined here as runtime has no C file.
go_ii MemProfileRate_go_runtime_package = 512 * 1024;
