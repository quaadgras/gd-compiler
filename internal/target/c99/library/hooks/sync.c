// Hooks of sync, which the Go runtime implements.
#include <go/sync.h>

#define S(name) name##_go_sync_package

go_u4 S(runtime_randn)(go_u4 n) { return (go_u4)(go_rand() % n); }
void S(runtime_registerPoolCleanup)(go_fn cleanup) {}
go_ii S(runtime_procPin)(void) { return 0; }
void S(runtime_procUnpin)(void) {}
void S(runtime_Semacquire)(go_pt s) { go_semacquire(s); }
void S(runtime_SemacquireWaitGroup)(go_pt s, go_tf synctestDurable) { go_semacquire(s); }
void S(runtime_SemacquireRWMutexR)(go_pt s, go_tf lifo, go_ii skipframes) { go_semacquire(s); }
void S(runtime_SemacquireRWMutex)(go_pt s, go_tf lifo, go_ii skipframes) { go_semacquire(s); }
void S(runtime_Semrelease)(go_pt s, go_tf handoff, go_ii skipframes) { go_semrelease(s); }

static go_pt S(wait)(go_pt l) { return (go_pt){ &((notifyList_go_sync_package*)go_nil_check(l.ptr))->wait }; }
static go_pt S(notify)(go_pt l) { return (go_pt){ &((notifyList_go_sync_package*)go_nil_check(l.ptr))->notify }; }
go_u4 S(runtime_notifyListAdd)(go_pt l) { return go_notify_add(S(wait)(l)); }
void S(runtime_notifyListWait)(go_pt l, go_u4 t) { go_notify_wait(S(notify)(l), t); }
void S(runtime_notifyListNotifyAll)(go_pt l) { go_notify(S(wait)(l), S(notify)(l), true); }
void S(runtime_notifyListNotifyOne)(go_pt l) { go_notify(S(wait)(l), S(notify)(l), false); }
void S(runtime_notifyListCheck)(go_up size) {}
void S(throw)(go_ss msg) { go_fatal(msg); }
void S(fatal)(go_ss msg) { go_fatal(msg); }
