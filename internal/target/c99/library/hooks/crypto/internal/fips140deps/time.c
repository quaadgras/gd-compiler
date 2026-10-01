// Hooks of crypto/internal/fips140deps/time, which the Go runtime implements.
#include <go/crypto/internal/fips140deps/time.h>
#include <go/crypto/internal/fips140deps/time/private.h>

go_i8 monoTime_go_crypto_internal_fips140deps_time_package(void) { return go_nanotime(); }
