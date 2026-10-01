//go:build ignore

// Goroutines and channels, see runtime/chan.go and runtime/select.go.
//
// Goroutines are threads. Channel operations are serialized by a single lock, which keeps
// select simple: a goroutine that has to wait (for a send, a receive, or any of the cases of
// a select) queues a sudog for each of its operations on their channels, and sleeps on its
// own condition variable, until another goroutine completes one of them (marking it done,
// so that its other sudogs are skipped), or closes the channel.

#include <go.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <threads.h>

typedef struct go_waiter {
    cnd_t cond;
    go_tf done;
    int index; // of the select case that completed.
    go_tf ok;  // false when woken because the channel was closed.
} go_waiter;

typedef struct go_sudog {
    struct go_sudog *prev, *next;
    go_waiter* waiter;
    void* elem; // the value to send, or where to receive it.
    int index;
} go_sudog;

typedef struct { go_sudog *first, *last; } go_waitq;

typedef struct go_channel {
    size_t elemsize, cap, count, recvx, sendx;
    char* buf;
    go_waitq recvq, sendq;
    go_tf closed;
} go_channel;

static mtx_t go_chan_lock;
static once_flag go_chan_once = ONCE_FLAG_INIT;
static void go_chan_init(void) { mtx_init(&go_chan_lock, mtx_plain); }
static void go_lock(void) { call_once(&go_chan_once, go_chan_init); mtx_lock(&go_chan_lock); }
static void go_unlock(void) { mtx_unlock(&go_chan_lock); }

static void go_enqueue(go_waitq* q, go_sudog* s) {
    s->next = NULL;
    s->prev = q->last;
    if (q->last) q->last->next = s; else q->first = s;
    q->last = s;
}

static void go_remove(go_waitq* q, go_sudog* s) {
    if (s->prev) s->prev->next = s->next; else if (q->first == s) q->first = s->next; else return;
    if (s->next) s->next->prev = s->prev; else q->last = s->prev;
    s->prev = s->next = NULL;
}

// go_dequeue removes the first sudog whose waiter is still waiting.
static go_sudog* go_dequeue(go_waitq* q) {
    while (q->first) {
        go_sudog* s = q->first;
        go_remove(q, s);
        if (!s->waiter->done) return s;
    }
    return NULL;
}

static void go_complete(go_sudog* s, go_tf ok) {
    s->waiter->done = true;
    s->waiter->index = s->index;
    s->waiter->ok = ok;
    cnd_signal(&s->waiter->cond);
}

go_ch go_chan(go_ii elem_size, go_ii cap) {
    if (cap < 0 || (elem_size > 0 && cap > ((go_ii)1 << 47) / elem_size)) go_panic_error("makechan: size out of range");
    go_channel* ch = go_new(sizeof(go_channel), NULL).ptr;
    ch->elemsize = (size_t)elem_size;
    ch->cap = (size_t)cap;
    if (cap > 0) ch->buf = go_new(cap * (elem_size > 0 ? elem_size : 1), NULL).ptr;
    return ch;
}

// go_try_send sends v, if it can without waiting (with the lock held).
static go_tf go_try_send(go_channel* ch, const void* v) {
    if (ch->closed) {
        go_unlock();
        go_panic_error("send on closed channel");
    }
    go_sudog* r = go_dequeue(&ch->recvq);
    if (r) {
        if (r->elem) memcpy(r->elem, v, ch->elemsize);
        go_complete(r, true);
        return true;
    }
    if (ch->count < ch->cap) {
        memcpy(ch->buf + ch->sendx * ch->elemsize, v, ch->elemsize);
        ch->sendx = (ch->sendx + 1) % ch->cap;
        ch->count++;
        return true;
    }
    return false;
}

// go_try_recv receives into v (if not NULL), if it can without waiting (with the lock held).
static go_tf go_try_recv(go_channel* ch, void* v, go_tf* ok) {
    if (ch->count > 0) {
        char* slot = ch->buf + ch->recvx * ch->elemsize;
        if (v) memcpy(v, slot, ch->elemsize);
        ch->recvx = (ch->recvx + 1) % ch->cap;
        ch->count--;
        go_sudog* s = go_dequeue(&ch->sendq); // it can now send into the buffer.
        if (s) {
            memcpy(ch->buf + ch->sendx * ch->elemsize, s->elem, ch->elemsize);
            ch->sendx = (ch->sendx + 1) % ch->cap;
            ch->count++;
            go_complete(s, true);
        }
        *ok = true;
        return true;
    }
    go_sudog* s = go_dequeue(&ch->sendq);
    if (s) {
        if (v) memcpy(v, s->elem, ch->elemsize);
        go_complete(s, true);
        *ok = true;
        return true;
    }
    if (ch->closed) {
        if (v) memset(v, 0, ch->elemsize);
        *ok = false;
        return true;
    }
    return false;
}

static _Noreturn void go_block_forever(void) {
    cnd_t cond;
    cnd_init(&cond);
    for (;;) cnd_wait(&cond, &go_chan_lock);
}

void go_send(go_ch c, go_ii size, const void* v) {
    (void)size;
    go_channel* ch = c;
    go_lock();
    if (!ch) go_block_forever();
    if (go_try_send(ch, v)) { go_unlock(); return; }
    go_waiter w = {0};
    cnd_init(&w.cond);
    go_sudog s = { .waiter = &w, .elem = (void*)v };
    go_enqueue(&ch->sendq, &s);
    while (!w.done) cnd_wait(&w.cond, &go_chan_lock);
    cnd_destroy(&w.cond);
    go_unlock();
    if (!w.ok) go_panic_error("send on closed channel");
}

go_tf go_recv(go_ch c, go_ii size, void* v) {
    (void)size;
    go_channel* ch = c;
    go_tf ok;
    go_lock();
    if (!ch) go_block_forever();
    if (go_try_recv(ch, v, &ok)) { go_unlock(); return ok; }
    go_waiter w = {0};
    cnd_init(&w.cond);
    go_sudog s = { .waiter = &w, .elem = v };
    go_enqueue(&ch->recvq, &s);
    while (!w.done) cnd_wait(&w.cond, &go_chan_lock);
    cnd_destroy(&w.cond);
    go_unlock();
    if (!w.ok && v) memset(v, 0, ch->elemsize);
    return w.ok;
}

void go_close(go_ch c) {
    go_channel* ch = c;
    go_lock();
    if (!ch) { go_unlock(); go_panic_error("close of nil channel"); }
    if (ch->closed) { go_unlock(); go_panic_error("close of closed channel"); }
    ch->closed = true;
    go_sudog* s;
    while ((s = go_dequeue(&ch->recvq))) {
        if (s->elem) memset(s->elem, 0, ch->elemsize);
        go_complete(s, false);
    }
    while ((s = go_dequeue(&ch->sendq))) go_complete(s, false);
    go_unlock();
}

go_ii go_chan_len(go_ch c) {
    go_channel* ch = c;
    if (!ch) return 0;
    go_lock();
    go_ii n = (go_ii)ch->count;
    go_unlock();
    return n;
}

go_ii go_chan_cap(go_ch c) { return c ? (go_ii)((go_channel*)c)->cap : 0; }

int go_select(go_select_case* cases, int n, go_tf block) {
    static go_thread_local unsigned seed = 2463534242u;
    go_lock();
    int start = 0;
    if (n > 0) { // start at a random case, as Go chooses randomly between ready cases.
        seed ^= seed << 13; seed ^= seed >> 17; seed ^= seed << 5;
        start = (int)(seed % (unsigned)n);
    }
    for (int k = 0; k < n; k++) {
        int i = (start + k) % n;
        go_channel* ch = cases[i].c;
        if (!ch) continue; // never ready.
        if (cases[i].send ? go_try_send(ch, cases[i].elem) : go_try_recv(ch, cases[i].elem, &cases[i].ok)) {
            go_unlock();
            return i;
        }
    }
    if (!block) { go_unlock(); return -1; }
    go_waiter w = {0};
    cnd_init(&w.cond);
    go_sudog* sudogs = go_new(n * (go_ii)sizeof(go_sudog), NULL).ptr;
    int waiting = 0;
    for (int i = 0; i < n; i++) {
        go_channel* ch = cases[i].c;
        if (!ch) continue;
        sudogs[i] = (go_sudog){ .waiter = &w, .elem = cases[i].elem, .index = i };
        go_enqueue(cases[i].send ? &ch->sendq : &ch->recvq, &sudogs[i]);
        waiting++;
    }
    if (waiting == 0) go_block_forever();
    while (!w.done) cnd_wait(&w.cond, &go_chan_lock);
    for (int i = 0; i < n; i++) { // the sudogs of the other cases.
        go_channel* ch = cases[i].c;
        if (ch) go_remove(cases[i].send ? &ch->sendq : &ch->recvq, &sudogs[i]);
    }
    cnd_destroy(&w.cond);
    go_unlock();
    cases[w.index].ok = w.ok;
    if (cases[w.index].send && !w.ok) go_panic_error("send on closed channel");
    if (!cases[w.index].send && !w.ok && cases[w.index].elem) memset(cases[w.index].elem, 0, ((go_channel*)cases[w.index].c)->elemsize);
    return w.index;
}

static _Atomic go_ii go_goroutines = 1; // (the main goroutine)
go_ii go_num_goroutines(void) { return atomic_load(&go_goroutines); }

static void go_run_goroutine(void* arg) {
    go_fn* fn = arg;
    ((void(*)(void*))fn->ptr)(fn->env);
    atomic_fetch_sub(&go_goroutines, 1);
}

#if defined(__unix__) || defined(__APPLE__)
// POSIX threads, as goroutines need larger stacks than the default (Go's stacks grow), which
// C11 threads can't set. The stack is only committed as it is used.
#include <pthread.h>

static void* go_start_goroutine(void* arg) { go_run_goroutine(arg); return NULL; }

void go_start(go_fn fn) {
    go_fn* arg = go_new(sizeof(go_fn), &fn).ptr;
    atomic_fetch_add(&go_goroutines, 1);
    pthread_attr_t attr;
    pthread_t thread;
    pthread_attr_init(&attr);
    pthread_attr_setstacksize(&attr, (size_t)64 << 20);
    pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);
    int err = pthread_create(&thread, &attr, go_start_goroutine, arg);
    pthread_attr_destroy(&attr);
    if (err != 0) go_panic_error("runtime error: failed to start a goroutine");
}
typedef struct { int (*main)(void); int result; } go_main_goroutine;

static void* go_start_main(void* arg) {
    go_main_goroutine* m = arg;
    m->result = m->main();
    return NULL;
}

int go_run_main(int (*main)(void)) {
    go_main_goroutine m = { .main = main };
    pthread_attr_t attr;
    pthread_t thread;
    pthread_attr_init(&attr);
    pthread_attr_setstacksize(&attr, (size_t)512 << 20);
    int err = pthread_create(&thread, &attr, go_start_main, &m);
    pthread_attr_destroy(&attr);
    if (err != 0) return main(); // run it on this thread instead.
    pthread_join(thread, NULL);
    exit(m.result); // other goroutines stop when main returns.
}
#else
int go_run_main(int (*main)(void)) { exit(main()); }

static int go_start_goroutine(void* arg) { go_run_goroutine(arg); return 0; }

void go_start(go_fn fn) {
    go_fn* arg = go_new(sizeof(go_fn), &fn).ptr;
    atomic_fetch_add(&go_goroutines, 1);
    thrd_t thread;
    if (thrd_create(&thread, go_start_goroutine, arg) != thrd_success) {
        go_panic_error("runtime error: failed to start a goroutine");
    }
    thrd_detach(thread);
}
#endif
