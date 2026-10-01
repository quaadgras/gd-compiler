// Hooks of crypto/fips140, which the Go runtime implements (per goroutine, so per thread).
#include <go/crypto/fips140.h>
#include <go/crypto/fips140/private.h>

static go_thread_local go_tf go_fips140_bypassed;

void setBypass_go_crypto_fips140_package(void) { go_fips140_bypassed = true; }
go_tf isBypassed_go_crypto_fips140_package(void) { return go_fips140_bypassed; }
void unsetBypass_go_crypto_fips140_package(void) { go_fips140_bypassed = false; }
