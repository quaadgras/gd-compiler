//go:build ignore

// Package syscall, for gd: a small subset of the standard library's, over C11 (see
// library/hooks/syscall.c), for the packages that use it (such as time). File descriptors
// index a table of C's streams (0, 1 and 2 are the standard ones).
package syscall

// Hooks, implemented in C. Negative results are -errno.

func open(path string, mode int) int
func read(fd int, p []byte) int
func write(fd int, p []byte) int
func closeFD(fd int) int
func seek(fd int, offset int64, whence int) int64
func getenv(key string) string
func hasenv(key string) bool

// An Errno is an unsigned number describing an error condition.
type Errno uintptr

const (
	EPERM   Errno = 1
	ENOENT  Errno = 2
	EINTR   Errno = 4
	EIO     Errno = 5
	EBADF   Errno = 9
	EAGAIN  Errno = 11
	ENOMEM  Errno = 12
	EACCES  Errno = 13
	EEXIST  Errno = 17
	ENOTDIR Errno = 20
	EISDIR  Errno = 21
	EINVAL  Errno = 22
	EMFILE  Errno = 24
	ENOSPC  Errno = 28
	ESPIPE  Errno = 29
	ERANGE  Errno = 34
	ENOSYS  Errno = 38
)

var errors = map[Errno]string{
	EPERM: "operation not permitted", ENOENT: "no such file or directory", EINTR: "interrupted system call",
	EIO: "input/output error", EBADF: "bad file descriptor", EAGAIN: "resource temporarily unavailable",
	ENOMEM: "cannot allocate memory", EACCES: "permission denied", EEXIST: "file exists",
	ENOTDIR: "not a directory", EISDIR: "is a directory", EINVAL: "invalid argument",
	EMFILE: "too many open files", ENOSPC: "no space left on device", ESPIPE: "illegal seek",
	ERANGE: "numerical result out of range", ENOSYS: "function not implemented",
}

func (e Errno) Error() string {
	if s, ok := errors[e]; ok {
		return s
	}
	return "errno " + itoa(int(e))
}

func (e Errno) Temporary() bool { return e == EINTR || e == EMFILE || e.Timeout() }
func (e Errno) Timeout() bool   { return e == EAGAIN }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// A Signal is a number describing a process signal.
type Signal int

const (
	SIGHUP  Signal = 1
	SIGINT  Signal = 2
	SIGQUIT Signal = 3
	SIGKILL Signal = 9
	SIGSEGV Signal = 11
	SIGPIPE Signal = 13
	SIGTERM Signal = 15
	SIGCHLD Signal = 17
)

func (s Signal) Signal()        {}
func (s Signal) String() string { return "signal " + itoa(int(s)) }

const (
	O_RDONLY = 0x0
	O_WRONLY = 0x1
	O_RDWR   = 0x2
	O_CREAT  = 0x40
	O_TRUNC  = 0x200
	O_APPEND = 0x400

	PROT_NONE   = 0x0
	PROT_READ   = 0x1
	PROT_WRITE  = 0x2
	PROT_EXEC   = 0x4
	MAP_PRIVATE = 0x2
	MAP_ANON    = 0x20
	MAP_SHARED  = 0x1

	Stdin  = 0
	Stdout = 1
	Stderr = 2
)

func errnoOf(n int) error {
	if n < 0 {
		return Errno(-n)
	}
	return nil
}

func Open(path string, mode int, perm uint32) (fd int, err error) {
	fd = open(path, mode)
	if fd < 0 {
		return -1, Errno(-fd)
	}
	return fd, nil
}

func Read(fd int, p []byte) (n int, err error) {
	n = read(fd, p)
	if n < 0 {
		return -1, Errno(-n)
	}
	return n, nil
}

func Write(fd int, p []byte) (n int, err error) {
	n = write(fd, p)
	if n < 0 {
		return -1, Errno(-n)
	}
	return n, nil
}

func Close(fd int) error { return errnoOf(closeFD(fd)) }

func Seek(fd int, offset int64, whence int) (off int64, err error) {
	off = seek(fd, offset, whence)
	if off < 0 {
		return -1, Errno(-off)
	}
	return off, nil
}

func Getenv(key string) (value string, found bool) {
	if !hasenv(key) {
		return "", false
	}
	return getenv(key), true
}

func Setenv(key, value string) error { return ENOSYS }
func Unsetenv(key string) error      { return ENOSYS }
func Environ() []string              { return nil }
func Getpid() int                    { return 1 }
func Getppid() int                   { return 0 }
func Getuid() int                    { return 0 }
func Getgid() int                    { return 0 }
func Getpagesize() int               { return getpagesize() }
func Kill(pid int, sig Signal) error { return ENOSYS }
func Exit(code int)                  { panic("syscall.Exit is not supported by gd") }

func getpagesize() int
func mmap(fd int, offset int64, length int, prot int, flags int) (data []byte, errno int)
func munmap(b []byte) int
func mprotect(b []byte, prot int) int

// Mmap maps memory (on POSIX systems, see library/hooks/syscall.c).
func Mmap(fd int, offset int64, length int, prot int, flags int) (data []byte, err error) {
	if length <= 0 {
		return nil, EINVAL
	}
	data, errno := mmap(fd, offset, length, prot, flags)
	if errno != 0 {
		return nil, Errno(errno)
	}
	return data, nil
}

func Munmap(b []byte) error {
	if errno := munmap(b); errno != 0 {
		return Errno(errno)
	}
	return nil
}

func Mprotect(b []byte, prot int) error {
	if errno := mprotect(b, prot); errno != 0 {
		return Errno(errno)
	}
	return nil
}
func Syscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err Errno) { return 0, 0, ENOSYS }
func Syscall6(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err Errno) {
	return 0, 0, ENOSYS
}
func RawSyscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err Errno) { return 0, 0, ENOSYS }
