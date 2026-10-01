// Hooks of crypto/internal/fips140, which the Go runtime implements (the service indicator
// is per goroutine, so per thread).
#include <go/crypto/internal/fips140.h>
#include <go/crypto/internal/fips140/private.h>

static go_thread_local go_u1 go_fips140_indicator;

go_u1 getIndicator_go_crypto_internal_fips140_package(void) { return go_fips140_indicator; }
void setIndicator_go_crypto_internal_fips140_package(go_u1 indicator) { go_fips140_indicator = indicator; }
void fatal_go_crypto_internal_fips140_package(go_ss msg) { go_fatal(msg); }
