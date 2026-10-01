// Hooks of crypto/subtle, which the Go runtime implements: data independent timing is
// not supported (by C).
#include <go/crypto/subtle.h>
#include <go/crypto/subtle/private.h>

go_tf setDITEnabled_go_crypto_subtle_package(void) { return false; }
void setDITDisabled_go_crypto_subtle_package(void) {}
