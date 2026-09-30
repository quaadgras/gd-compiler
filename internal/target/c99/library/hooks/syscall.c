// Hooks of package syscall (see library/overlay/syscall), with C11's streams.
#include <go/syscall.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <threads.h>

#define S(name) name##_go_syscall_package

static FILE* go_fds[256];
static mtx_t go_fds_lock;
static once_flag go_fds_once = ONCE_FLAG_INIT;
static void go_fds_init(void) {
    mtx_init(&go_fds_lock, mtx_plain);
    go_fds[0] = stdin, go_fds[1] = stdout, go_fds[2] = stderr;
}

static FILE* go_fd(go_ii fd) {
    call_once(&go_fds_once, go_fds_init);
    return fd >= 0 && fd < 256 ? go_fds[fd] : NULL;
}

static char* go_cstring(go_ss s) {
    go_ii n = go_string_len(s);
    char* c = go_new(n + 1, NULL).ptr;
    if (n > 0) memcpy(c, s.ptr, (size_t)n);
    return c;
}

static go_ii go_errno(void) { return errno ? -(go_ii)errno : -5; } // (EIO)

go_ii S(open)(go_ss path, go_ii mode) {
    const char* m = "rb";
    if (mode & 0x400) m = (mode & 0x2) ? "a+b" : "ab"; // O_APPEND
    else if (mode & 0x40) m = (mode & 0x2) ? "w+b" : "wb"; // O_CREAT
    else if (mode & 0x3) m = "r+b";
    errno = 0;
    FILE* f = fopen(go_cstring(path), m);
    if (!f) return go_errno();
    call_once(&go_fds_once, go_fds_init);
    mtx_lock(&go_fds_lock);
    for (go_ii fd = 3; fd < 256; fd++) {
        if (!go_fds[fd]) {
            go_fds[fd] = f;
            mtx_unlock(&go_fds_lock);
            return fd;
        }
    }
    mtx_unlock(&go_fds_lock);
    fclose(f);
    return -24; // EMFILE
}

go_ii S(read)(go_ii fd, go_ll p) {
    FILE* f = go_fd(fd);
    if (!f) return -9; // EBADF
    if (p.len == 0) return 0;
    errno = 0;
    size_t n = fread(p.ptr.ptr, 1, (size_t)p.len, f);
    if (n == 0 && ferror(f)) return go_errno();
    return (go_ii)n;
}

go_ii S(write)(go_ii fd, go_ll p) {
    FILE* f = go_fd(fd);
    if (!f) return -9;
    errno = 0;
    size_t n = p.len > 0 ? fwrite(p.ptr.ptr, 1, (size_t)p.len, f) : 0;
    fflush(f);
    if (n < (size_t)p.len) return go_errno();
    return (go_ii)n;
}

go_ii S(closeFD)(go_ii fd) {
    FILE* f = go_fd(fd);
    if (!f) return -9;
    mtx_lock(&go_fds_lock);
    go_fds[fd] = NULL;
    mtx_unlock(&go_fds_lock);
    return fclose(f) == 0 ? 0 : go_errno();
}

go_i8 S(seek)(go_ii fd, go_i8 offset, go_ii whence) {
    FILE* f = go_fd(fd);
    if (!f) return -9;
    errno = 0;
    if (fseek(f, (long)offset, whence == 0 ? SEEK_SET : whence == 1 ? SEEK_CUR : SEEK_END) != 0) return go_errno();
    return (go_i8)ftell(f);
}

go_ss S(getenv)(go_ss key) {
    const char* v = getenv(go_cstring(key));
    return v ? go_string_new(v) : go_string_new("");
}
go_tf S(hasenv)(go_ss key) { return getenv(go_cstring(key)) != NULL; }
