// Hooks of crypto/rand, which the Go runtime implements.
#include <go/crypto/rand.h>
#include <go/crypto/rand/private.h>

void fatal_go_crypto_rand_package(go_ss msg) { go_fatal(msg); }
