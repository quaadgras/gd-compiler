// Hooks of crypto/internal/sysrand: random bytes from the operating system's device.
#include <go/crypto/internal/sysrand.h>
#include <stdio.h>

go_tf read_go_crypto_internal_sysrand_package(go_ll b) {
    if (b.len == 0) return true;
    FILE* f = fopen("/dev/urandom", "rb");
    if (!f) return false;
    size_t n = fread(b.ptr.ptr, 1, (size_t)b.len, f);
    fclose(f);
    return n == (size_t)b.len;
}
