// Hooks of math/rand, which the Go runtime implements.
#include <go/math/rand.h>

go_u8 runtime_rand_go_math_rand_package(void) { return go_rand(); }
