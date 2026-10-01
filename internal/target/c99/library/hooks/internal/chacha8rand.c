// Hooks of internal/chacha8rand, which Go implements in assembly (with a generic version).
#include <go/internal/chacha8rand.h>
#include <go/internal/chacha8rand/private.h>

void block_go_internal_chacha8rand_package(go_pt seed, go_pt blocks, go_u4 counter) {
    block_generic_go_internal_chacha8rand_package(seed, blocks, counter);
}
