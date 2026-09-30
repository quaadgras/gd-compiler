// Hooks of internal/bytealg, which the Go runtime implements in assembly.
#include <go/internal/bytealg.h>
#include <string.h>

static const char* go_bytealg_bytes(go_ll b) { return b.ptr.ptr; }

static go_ii go_bytealg_compare(const char* a, go_ii na, const char* b, go_ii nb) {
    go_ii n = na < nb ? na : nb;
    int c = n > 0 ? memcmp(a, b, (size_t)n) : 0;
    if (c != 0) return c < 0 ? -1 : 1;
    return na < nb ? -1 : na > nb ? 1 : 0;
}

static go_ii go_bytealg_count(const char* s, go_ii n, go_u1 c) {
    go_ii count = 0;
    for (go_ii i = 0; i < n; i++) count += (go_u1)s[i] == c;
    return count;
}

static go_ii go_bytealg_index_byte(const char* s, go_ii n, go_u1 c) {
    if (n <= 0) return -1;
    const char* p = memchr(s, c, (size_t)n);
    return p ? (go_ii)(p - s) : -1;
}

static go_ii go_bytealg_index(const char* a, go_ii na, const char* b, go_ii nb) {
    if (nb == 0) return 0;
    for (go_ii i = 0; i + nb <= na; i++) {
        if (a[i] == b[0] && memcmp(a + i, b, (size_t)nb) == 0) return i;
    }
    return -1;
}

go_ll MakeNoZero_go_internal_bytealg_package(go_ii n) { return go_slice_make(go_u1, n, n); }
go_ii Compare_go_internal_bytealg_package(go_ll a, go_ll b) { return go_bytealg_compare(go_bytealg_bytes(a), a.len, go_bytealg_bytes(b), b.len); }
go_ii abigen_runtime_cmpstring_go_internal_bytealg_package(go_ss a, go_ss b) { return go_bytealg_compare(a.ptr, a.len, b.ptr, b.len); }
go_ii Count_go_internal_bytealg_package(go_ll b, go_u1 c) { return go_bytealg_count(go_bytealg_bytes(b), b.len, c); }
go_ii CountString_go_internal_bytealg_package(go_ss s, go_u1 c) { return go_bytealg_count(s.ptr, s.len, c); }
go_tf abigen_runtime_memequal_go_internal_bytealg_package(go_pt a, go_pt b, go_up size) { return size == 0 || memcmp(a.ptr, b.ptr, size) == 0; }
go_ii IndexByte_go_internal_bytealg_package(go_ll b, go_u1 c) { return go_bytealg_index_byte(go_bytealg_bytes(b), b.len, c); }
go_ii IndexByteString_go_internal_bytealg_package(go_ss s, go_u1 c) { return go_bytealg_index_byte(s.ptr, s.len, c); }
go_ii Index_go_internal_bytealg_package(go_ll a, go_ll b) { return go_bytealg_index(go_bytealg_bytes(a), a.len, go_bytealg_bytes(b), b.len); }
go_ii IndexString_go_internal_bytealg_package(go_ss a, go_ss b) { return go_bytealg_index(a.ptr, a.len, b.ptr, b.len); }
