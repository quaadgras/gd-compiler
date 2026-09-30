// Hooks of internal/synctest: goroutines are never in a bubble (testing/synctest is not
// supported).
#include <go/internal/synctest.h>

#define S(name) name##_go_internal_synctest_package

void S(Run)(go_fn f) { go_panic_error("synctest.Run is not supported"); }
void S(Wait)(void) { go_panic_error("synctest.Wait is not supported"); }
go_tf S(IsInBubble)(void) { return false; }
go_ii S(associate)(go_pt p) { return 0; } // not in a bubble.
void S(disassociate)(go_pt b) {}
go_tf S(isAssociated)(go_pt p) { return false; }
go_vv S(acquire)(void) { return (go_vv){0}; }
void S(release)(go_vv b) {}
void S(inBubble)(go_vv b, go_fn f) { ((void(*)(void*))f.ptr)(f.env); }
