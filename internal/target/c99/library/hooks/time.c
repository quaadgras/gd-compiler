// Hooks of package time, which the Go runtime implements: clocks, Sleep, and timers (run
// by a thread that calls their functions when they are due).
#include <go/time.h>
#include <threads.h>
#include <time.h>

#define S(name) name##_go_time_package

static go_tuple_go_i8_go_i4_go_i8 go_time_now(void) {
    struct timespec ts;
    timespec_get(&ts, TIME_UTC);
    return (go_tuple_go_i8_go_i4_go_i8){ (go_i8)ts.tv_sec, (go_i4)ts.tv_nsec, go_nanotime() };
}
go_tuple_go_i8_go_i4_go_i8 S(now)(void) { return go_time_now(); }
go_tuple_go_i8_go_i4_go_i8 S(runtimeNow)(void) { return go_time_now(); }
go_i8 S(runtimeNano)(void) { return go_nanotime(); }
go_tf S(runtimeIsBubbled)(void) { return false; }

void S(Sleep)(Duration_go_time_package d) {
    if (d <= 0) { thrd_yield(); return; }
    struct timespec ts = { (time_t)(d / 1000000000), (long)(d % 1000000000) };
    while (thrd_sleep(&ts, &ts) == -1) {} // (resumed when interrupted)
}

// A timer is a time.Timer (which is what time's code sees), and what the runtime needs.
typedef struct go_timer {
    Timer_go_time_package timer;
    go_i8 when, period;
    go_fn f;
    go_vv arg;
    go_up seq;
    go_tf active;
    struct go_timer* next;
} go_timer;

static go_timer* go_timers; // all of them (active or not), until they're stopped.
static mtx_t go_timers_lock;
static cnd_t go_timers_cond;
static once_flag go_timers_once = ONCE_FLAG_INIT;

static int go_timer_thread(void* arg) {
    (void)arg;
    mtx_lock(&go_timers_lock);
    for (;;) {
        go_timer* next = NULL;
        for (go_timer* t = go_timers; t; t = t->next) {
            if (t->active && (!next || t->when < next->when)) next = t;
        }
        if (!next) {
            cnd_wait(&go_timers_cond, &go_timers_lock);
            continue;
        }
        go_i8 now = go_nanotime();
        if (next->when > now) {
            struct timespec until = { (time_t)(next->when / 1000000000), (long)(next->when % 1000000000) };
            cnd_timedwait(&go_timers_cond, &go_timers_lock, &until);
            continue;
        }
        go_i8 delta = now - next->when;
        if (next->period > 0) {
            next->when += next->period * (1 + delta / next->period);
        } else {
            next->active = false;
        }
        go_fn f = next->f;
        go_vv arg = next->arg;
        go_up seq = next->seq;
        mtx_unlock(&go_timers_lock);
        ((void(*)(void*, go_vv, go_up, go_i8))f.ptr)(f.env, arg, seq, delta);
        mtx_lock(&go_timers_lock);
    }
    return 0;
}

static void go_timers_init(void) {
    mtx_init(&go_timers_lock, mtx_plain);
    cnd_init(&go_timers_cond);
    thrd_t thread;
    if (thrd_create(&thread, go_timer_thread, NULL) != thrd_success) go_panic_error("time: failed to start the timer thread");
    thrd_detach(thread);
}

go_pt S(newTimer)(go_i8 when, go_i8 period, go_fn f, go_vv arg, go_pt cp) {
    (void)cp;
    call_once(&go_timers_once, go_timers_init);
    go_timer* t = go_new(sizeof(go_timer), NULL).ptr;
    t->timer.initTimer = true;
    t->when = when, t->period = period, t->f = f, t->arg = arg, t->active = f.ptr != NULL;
    mtx_lock(&go_timers_lock);
    t->next = go_timers;
    go_timers = t;
    cnd_signal(&go_timers_cond);
    mtx_unlock(&go_timers_lock);
    return (go_pt){ t };
}

go_tf S(stopTimer)(go_pt p) {
    go_timer* t = go_nil_check(p.ptr);
    call_once(&go_timers_once, go_timers_init);
    mtx_lock(&go_timers_lock);
    go_tf was = t->active;
    t->active = false;
    t->seq++;
    mtx_unlock(&go_timers_lock);
    return was;
}

go_tf S(resetTimer)(go_pt p, go_i8 when, go_i8 period) {
    go_timer* t = go_nil_check(p.ptr);
    call_once(&go_timers_once, go_timers_init);
    mtx_lock(&go_timers_lock);
    go_tf was = t->active;
    t->when = when, t->period = period, t->active = t->f.ptr != NULL;
    t->seq++;
    cnd_signal(&go_timers_cond);
    mtx_unlock(&go_timers_lock);
    return was;
}
