//go:build ignore

package debug

import (
	"errors"
	"os"
	"runtime"
)

// PrintStack prints to standard error the stack trace returned by runtime.Stack.
func PrintStack() {
	os.Stderr.Write(Stack())
}

// Stack returns a formatted stack trace of the goroutine that calls it.
func Stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

// CrashOptions provides options that control the formatting of the fatal crash message.
type CrashOptions struct {
	/* for future expansion */
}

// SetCrashOutput configures a single additional file where unhandled panics and other
// fatal errors are printed: gd doesn't support it.
func SetCrashOutput(f *os.File, opts CrashOptions) error {
	if f == nil {
		return nil
	}
	return errors.New("runtime/debug: SetCrashOutput is not supported by gd")
}
