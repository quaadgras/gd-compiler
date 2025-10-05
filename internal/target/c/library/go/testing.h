#include <go.h>
#include <stdlib.h>

static inline void T_FailNow_go_testing_package(go_pointer t) {
    go_print("Test failed\n");
    exit(1);
}
