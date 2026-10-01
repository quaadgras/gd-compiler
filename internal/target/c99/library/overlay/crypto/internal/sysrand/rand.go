//go:build ignore

// Package sysrand, for gd: the operating system's random bytes, read by C11 (see
// library/hooks/crypto/internal/sysrand.c), without the packages that the standard
// library's uses for its system calls.
package sysrand

// read fills b with random bytes, reporting whether it could.
func read(b []byte) bool

// Read fills b with cryptographically secure random bytes from the operating system. It
// always fills b entirely and crashes the program irrecoverably if an error is
// encountered.
func Read(b []byte) {
	if !read(b) {
		panic("crypto/rand: failed to read random data")
	}
}
