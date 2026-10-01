// Package os, implemented with C11's standard library, see library/go/os.h, and POSIX's
// where there is one (whose declarations strict C11 otherwise hides).
#if (defined(__unix__) || defined(__APPLE__)) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif
#include <go/os.h>
#include <go/io.h>
#include <ctype.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

#define S(name) name##_go_os_package

go_ll S(Args);
go_pt S(Stdin), S(Stdout), S(Stderr);
go_if S(ErrNotExist), S(ErrExist), S(ErrPermission), S(ErrClosed), S(ErrInvalid);

const go_method go_os_file_methods[] = {
    {"Close", "func() error", (void(*)(void))I_File_Close_go_os_package},
    {"Name", "func() string", (void(*)(void))I_File_Name_go_os_package},
    {"Read", "func([]uint8) (int, error)", (void(*)(void))I_File_Read_go_os_package},
    {"Readdir", "func(int) ([]fs.FileInfo, error)", (void(*)(void))I_File_Readdir_go_os_package},
    {"Readdirnames", "func(int) ([]string, error)", (void(*)(void))I_File_Readdirnames_go_os_package},
    {"Sync", "func() error", (void(*)(void))I_File_Sync_go_os_package},
    {"Write", "func([]uint8) (int, error)", (void(*)(void))I_File_Write_go_os_package},
    {"WriteString", "func(string) (int, error)", (void(*)(void))I_File_WriteString_go_os_package},
};
const go_type go_type_File_go_os_package = {.name="os.File", .kind=go_kind_struct, .size=sizeof(File_go_os_package)};

static go_pt go_os_file(FILE* f, const char* name) {
    File_go_os_package file = { f, go_string_new(name), false, false };
    return go_new(sizeof file, &file);
}

void init_go_os_package(void) {
    static go_tf done = false;
    if (done) return;
    done = true;
    init_go_errors_package();
    init_go_io_package();
    go_ll args = go_slice_make(go_ss, go_argc, go_argc);
    for (int i = 0; i < go_argc; i++) ((go_ss*)args.ptr.ptr)[i] = go_string_new(go_argv[i]);
    S(Args) = args;
    S(Stdin) = go_os_file(stdin, "/dev/stdin");
    S(Stdout) = go_os_file(stdout, "/dev/stdout");
    S(Stderr) = go_os_file(stderr, "/dev/stderr");
    S(ErrNotExist) = New_go_errors_package(go_string_new("file does not exist"));
    S(ErrExist) = New_go_errors_package(go_string_new("file already exists"));
    S(ErrPermission) = New_go_errors_package(go_string_new("permission denied"));
    S(ErrClosed) = New_go_errors_package(go_string_new("file already closed"));
    S(ErrInvalid) = New_go_errors_package(go_string_new("invalid argument"));
}

// go_os_cstring returns a copy of s, terminated by NUL, for C's functions.
static char* go_os_cstring(go_ss s) {
    go_ii n = go_string_len(s);
    char* c = go_new(n + 1, NULL).ptr;
    if (n > 0) memcpy(c, s.ptr, (size_t)n);
    return c;
}

// go_os_error returns an error like Go's *PathError: "op name: reason", from errno.
static go_if go_os_error(const char* op, go_ss name, int err) {
    const char* reason = strerror(err);
    go_ii n = (go_ii)strlen(op) + go_string_len(name) + (go_ii)strlen(reason) + 3;
    char* msg = go_new(n + 1, NULL).ptr;
    snprintf(msg, (size_t)n + 1, "%s %.*s: %s", op, (int)go_string_len(name), name.ptr, reason);
    char* r = msg + strlen(op) + 1 + go_string_len(name) + 2; // Go's reasons are lowercase.
    if (*r) *r = (char)tolower((unsigned char)*r);
    return New_go_errors_package(go_string_new(msg));
}

_Noreturn void S(Exit)(go_ii code) {
    fflush(stdout);
    fflush(stderr);
    exit((int)code);
}

go_ss S(Getenv)(go_ss key) {
    const char* v = getenv(go_os_cstring(key));
    return v ? go_string_new(v) : go_string_new("");
}
go_tuple_go_ss_go_tf S(LookupEnv)(go_ss key) {
    const char* v = getenv(go_os_cstring(key));
    return (go_tuple_go_ss_go_tf){ v ? go_string_new(v) : go_string_new(""), v != NULL };
}
// C11 can't change the environment: these are errors.
go_if S(Setenv)(go_ss key, go_ss value) { return New_go_errors_package(go_string_new("setenv: not supported")); }
go_if S(Unsetenv)(go_ss key) { return New_go_errors_package(go_string_new("unsetenv: not supported")); }
go_ll S(Environ)(void) { return (go_ll){0}; }
go_ii S(Getpid)(void) { return 0; }

static go_tuple_go_pt_go_if go_os_open(go_ss name, const char* mode) {
    FILE* f = fopen(go_os_cstring(name), mode);
    if (!f) return (go_tuple_go_pt_go_if){ {0}, go_os_error("open", name, errno) };
    File_go_os_package file = { f, name, false, false };
    return (go_tuple_go_pt_go_if){ go_new(sizeof file, &file), {0} };
}
go_tuple_go_pt_go_if S(Open)(go_ss name) { return go_os_open(name, "rb"); }
go_tuple_go_pt_go_if S(Create)(go_ss name) { return go_os_open(name, "w+b"); }

go_tuple_go_ll_go_if S(ReadFile)(go_ss name) {
    FILE* f = fopen(go_os_cstring(name), "rb");
    if (!f) return (go_tuple_go_ll_go_if){ {0}, go_os_error("open", name, errno) };
    go_ll data = go_slice_make(go_u1, 0, 512);
    for (;;) {
        if (data.len == data.cap) {
            go_ll bigger = go_slice_make(go_u1, data.len, data.cap * 2);
            memcpy(bigger.ptr.ptr, data.ptr.ptr, (size_t)data.len);
            data = bigger;
        }
        size_t n = fread((char*)data.ptr.ptr + data.len, 1, (size_t)(data.cap - data.len), f);
        data.len += (go_ii)n;
        if (n == 0) break;
    }
    int err = ferror(f) ? errno : 0;
    fclose(f);
    if (err) return (go_tuple_go_ll_go_if){ data, go_os_error("read", name, err) };
    return (go_tuple_go_ll_go_if){ data, {0} };
}

go_if S(WriteFile)(go_ss name, go_ll data, go_u4 perm) {
    (void)perm; // C11 has no permissions.
    FILE* f = fopen(go_os_cstring(name), "wb");
    if (!f) return go_os_error("open", name, errno);
    size_t n = data.len > 0 ? fwrite(data.ptr.ptr, 1, (size_t)data.len, f) : 0;
    int err = n < (size_t)data.len ? errno : 0;
    if (fclose(f) != 0 && !err) err = errno;
    return err ? go_os_error("write", name, err) : (go_if){0};
}

static File_go_os_package* go_os_check(go_pt f);

// A fileStat is the information about a file, of a *os.fileStat (an fs.FileInfo).
typedef struct { go_ss name; go_i8 size; go_u4 mode; go_i8 sec, nsec; } go_os_stat;
static go_os_stat* go_os_stat_of(void* recv) { return go_nil_check(((go_pt*)recv)->ptr); }
static go_tf go_os_stat_IsDir(void* r) { return (go_os_stat_of(r)->mode & (1u << 31)) != 0; }
static Time_go_time_package go_os_stat_ModTime(void* r) { return Unix_go_time_package(go_os_stat_of(r)->sec, go_os_stat_of(r)->nsec); }
static FileMode_go_io_fs_package go_os_stat_Mode(void* r) { return go_os_stat_of(r)->mode; }
static go_ss go_os_stat_Name(void* r) { return go_os_stat_of(r)->name; }
static go_i8 go_os_stat_Size(void* r) { return go_os_stat_of(r)->size; }
static go_vv go_os_stat_Sys(void* r) { (void)r; return (go_vv){0}; }
static const go_method go_os_stat_methods[] = {
    {"IsDir", "func() bool", (void(*)(void))go_os_stat_IsDir},
    {"ModTime", "func() time.Time", (void(*)(void))go_os_stat_ModTime},
    {"Mode", "func() fs.FileMode", (void(*)(void))go_os_stat_Mode},
    {"Name", "func() string", (void(*)(void))go_os_stat_Name},
    {"Size", "func() int64", (void(*)(void))go_os_stat_Size},
    {"Sys", "func() interface {}", (void(*)(void))go_os_stat_Sys},
};
static const go_type go_os_stat_type = {.name="os.fileStat", .kind=go_kind_struct, .size=sizeof(go_os_stat), .pkg="os"};
static const go_type go_os_stat_ptr_type = {.name="*os.fileStat", .kind=go_kind_pointer, .size=sizeof(go_pt),
    .data={.pointer={.elem=&go_os_stat_type}}, .methods=go_os_stat_methods, .nmethods=6};
static FileInfo_go_io_fs_package go_os_stat_vtable = {
    go_os_stat_IsDir, go_os_stat_ModTime, go_os_stat_Mode, go_os_stat_Name, go_os_stat_Size, go_os_stat_Sys,
};

#if defined(__unix__) || defined(__APPLE__)
#include <sys/stat.h>

static go_tuple_go_if_go_if go_os_stat_file(const char* op, go_ss name, go_tf follow) {
    struct stat st;
    const char* path = go_os_cstring(name);
    if ((follow ? stat(path, &st) : lstat(path, &st)) != 0) return (go_tuple_go_if_go_if){ {0}, go_os_error(op, name, errno) };
    go_os_stat info = { .size = (go_i8)st.st_size, .sec = (go_i8)st.st_mtime };
    go_ii n = go_string_len(name), i = n;
    while (i > 0 && name.ptr[i - 1] != '/') i--;
    while (n > 1 && i == n && name.ptr[n - 1] == '/') { n--; i = n; while (i > 0 && name.ptr[i - 1] != '/') i--; } // (of "dir/")
    info.name = go_string_slice(name, i, n);
    go_u4 mode = (go_u4)(st.st_mode & 0777);
    if (S_ISDIR(st.st_mode)) mode |= 1u << 31;
    if (S_ISLNK(st.st_mode)) mode |= 1u << 27;
    if (S_ISFIFO(st.st_mode)) mode |= 1u << 25;
    if (S_ISSOCK(st.st_mode)) mode |= 1u << 24;
    if (S_ISCHR(st.st_mode)) mode |= (1u << 26) | (1u << 21);
    if (S_ISBLK(st.st_mode)) mode |= 1u << 26;
    if (st.st_mode & S_ISUID) mode |= 1u << 23;
    if (st.st_mode & S_ISGID) mode |= 1u << 22;
    if (st.st_mode & S_ISVTX) mode |= 1u << 20;
    info.mode = mode;
    go_pt* box = go_new(sizeof(go_pt), NULL).ptr;
    *box = go_new(sizeof info, &info);
    return (go_tuple_go_if_go_if){ { (go_pt){ box }, &go_os_stat_ptr_type, &go_os_stat_vtable }, {0} };
}
#include <dirent.h>
#include <unistd.h>

static int go_os_compare_names(const void* a, const void* b) { return go_string_cmp(*(const go_ss*)a, *(const go_ss*)b); }

// go_os_list returns the names of the files in the directory name, sorted.
static go_tuple_go_ll_go_if go_os_list(const char* op, go_ss name) {
    DIR* dir = opendir(go_os_cstring(name));
    if (!dir) return (go_tuple_go_ll_go_if){ {0}, go_os_error(op, name, errno) };
    go_ll names = go_slice_make(go_ss, 0, 0);
    struct dirent* entry;
    while ((entry = readdir(dir))) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) continue;
        size_t n = strlen(entry->d_name);
        char* copy = go_new((go_ii)n + 1, entry->d_name).ptr;
        go_ss s = { copy, (go_ii)n };
        names = go_append(names, sizeof(go_ss), &s);
    }
    closedir(dir);
    if (names.len > 1) qsort(names.ptr.ptr, (size_t)names.len, sizeof(go_ss), go_os_compare_names);
    return (go_tuple_go_ll_go_if){ names, {0} };
}

go_tuple_go_ll_go_if S(ReadDir)(go_ss name) {
    go_tuple_go_ll_go_if names = go_os_list("open", name);
    if (names.r1.go_type) return names;
    go_ll entries = go_slice_make(go_if, 0, names.r0.len);
    go_ss dir = go_string_concat(name, go_string_new("/"));
    for (go_ii i = 0; i < names.r0.len; i++) {
        go_tuple_go_if_go_if info = S(Lstat)(go_string_concat(dir, ((go_ss*)names.r0.ptr.ptr)[i]));
        if (info.r1.go_type) continue; // (removed since)
        go_if entry = FileInfoToDirEntry_go_io_fs_package(info.r0);
        entries = go_append(entries, sizeof(go_if), &entry);
    }
    return (go_tuple_go_ll_go_if){ entries, {0} };
}

go_tuple_go_ll_go_if S(File_Readdirnames)(go_pt f, go_ii n) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return (go_tuple_go_ll_go_if){ {0}, S(ErrInvalid) };
    if (file->listed) return (go_tuple_go_ll_go_if){ {0}, n > 0 ? EOF__go_io_package : (go_if){0} };
    file->listed = true; // (all of the names, at once)
    go_tuple_go_ll_go_if names = go_os_list("readdirent", file->name);
    if (!names.r1.go_type && names.r0.len == 0 && n > 0) names.r1 = EOF__go_io_package;
    return names;
}

go_tuple_go_ll_go_if S(File_Readdir)(go_pt f, go_ii n) {
    go_tuple_go_ll_go_if names = S(File_Readdirnames)(f, n);
    if (names.r1.go_type) return names;
    File_go_os_package* file = f.ptr;
    go_ll infos = go_slice_make(go_if, 0, names.r0.len);
    go_ss dir = go_string_concat(file->name, go_string_new("/"));
    for (go_ii i = 0; i < names.r0.len; i++) {
        go_tuple_go_if_go_if info = S(Lstat)(go_string_concat(dir, ((go_ss*)names.r0.ptr.ptr)[i]));
        if (info.r1.go_type) continue; // (removed since)
        infos = go_append(infos, sizeof(go_if), &info.r0);
    }
    return (go_tuple_go_ll_go_if){ infos, {0} };
}

go_tuple_go_ss_go_if S(Readlink)(go_ss name) {
    for (size_t size = 128;; size *= 2) {
        char* buf = go_new((go_ii)size, NULL).ptr;
        ssize_t n = readlink(go_os_cstring(name), buf, size);
        if (n < 0) return (go_tuple_go_ss_go_if){ {0}, go_os_error("readlink", name, errno) };
        if ((size_t)n < size) return (go_tuple_go_ss_go_if){ { buf, (go_ii)n }, {0} };
    }
}

go_tuple_go_ss_go_if S(Getwd)(void) {
    for (size_t size = 256;; size *= 2) {
        char* buf = go_new((go_ii)size, NULL).ptr;
        if (getcwd(buf, size)) return (go_tuple_go_ss_go_if){ go_string_new(buf), {0} };
        if (errno != ERANGE) return (go_tuple_go_ss_go_if){ {0}, go_os_error("getwd", go_string_new(""), errno) };
    }
}
#include <fcntl.h>

go_tuple_go_pt_go_if S(OpenFile)(go_ss name, go_ii flag, go_u4 perm) {
    int flags = 0; // (from Go's, as numbered on Linux)
    switch (flag & 3) { case 1: flags = O_WRONLY; break; case 2: flags = O_RDWR; break; default: flags = O_RDONLY; }
    if (flag & 0x40) flags |= O_CREAT;
    if (flag & 0x80) flags |= O_EXCL;
    if (flag & 0x200) flags |= O_TRUNC;
    if (flag & 0x400) flags |= O_APPEND;
    if (flag & 0x101000) flags |= O_SYNC;
    int fd = open(go_os_cstring(name), flags, (mode_t)(perm & 0777));
    if (fd < 0) return (go_tuple_go_pt_go_if){ {0}, go_os_error("open", name, errno) };
    const char* mode = (flag & 3) == 0 ? "rb" : (flag & 0x400) ? ((flag & 3) == 2 ? "a+b" : "ab") : (flag & 3) == 2 ? "r+b" : "wb";
    FILE* f = fdopen(fd, mode);
    if (!f) { int err = errno; close(fd); return (go_tuple_go_pt_go_if){ {0}, go_os_error("open", name, err) }; }
    File_go_os_package file = { f, name, false, false };
    return (go_tuple_go_pt_go_if){ go_new(sizeof file, &file), {0} };
}

go_if S(Mkdir)(go_ss name, go_u4 perm) {
    return mkdir(go_os_cstring(name), (mode_t)(perm & 0777)) == 0 ? (go_if){0} : go_os_error("mkdir", name, errno);
}

go_if S(MkdirAll)(go_ss path, go_u4 perm) {
    struct stat st;
    if (stat(go_os_cstring(path), &st) == 0) return S_ISDIR(st.st_mode) ? (go_if){0} : go_os_error("mkdir", path, ENOTDIR);
    go_ii n = go_string_len(path), i = n;
    while (i > 0 && path.ptr[i - 1] == '/') i--; // (the parent)
    while (i > 0 && path.ptr[i - 1] != '/') i--;
    if (i > 1) {
        go_if err = S(MkdirAll)(go_string_slice(path, 0, i - 1), perm);
        if (err.go_type) return err;
    }
    if (mkdir(go_os_cstring(path), (mode_t)(perm & 0777)) != 0) {
        int err = errno;
        if (stat(go_os_cstring(path), &st) == 0 && S_ISDIR(st.st_mode)) return (go_if){0};
        return go_os_error("mkdir", path, err);
    }
    return (go_if){0};
}

go_tuple_go_ss_go_if S(MkdirTemp)(go_ss dir, go_ss pattern) {
    if (go_string_len(dir) == 0) {
        const char* tmp = getenv("TMPDIR");
        dir = go_string_new(tmp && *tmp ? tmp : "/tmp");
    }
    go_ii n = go_string_len(pattern), star = -1;
    for (go_ii i = 0; i < n; i++) if (pattern.ptr[i] == '*') star = i;
    go_ss prefix = star >= 0 ? go_string_slice(pattern, 0, star) : pattern;
    go_ss suffix = star >= 0 ? go_string_slice(pattern, star + 1, n) : (go_ss){0};
    for (int try = 0; try < 10000; try++) {
        char random[24];
        snprintf(random, sizeof random, "%llu", (unsigned long long)(go_rand() % 10000000000ull));
        go_ss name = go_string_concat(go_string_concat(go_string_concat(dir, go_string_new("/")), prefix),
            go_string_concat(go_string_new(go_new((go_ii)strlen(random) + 1, random).ptr), suffix));
        if (mkdir(go_os_cstring(name), 0700) == 0) return (go_tuple_go_ss_go_if){ name, {0} };
        if (errno != EEXIST) return (go_tuple_go_ss_go_if){ {0}, go_os_error("mkdirtemp", name, errno) };
    }
    return (go_tuple_go_ss_go_if){ {0}, go_os_error("mkdirtemp", dir, EEXIST) };
}

go_if S(Chdir)(go_ss dir) { return chdir(go_os_cstring(dir)) == 0 ? (go_if){0} : go_os_error("chdir", dir, errno); }
go_if S(File_Chdir)(go_pt f) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return S(ErrInvalid);
    return fchdir(fileno(file->f)) == 0 ? (go_if){0} : go_os_error("chdir", file->name, errno);
}

go_tuple_go_pt_go_pt_go_if S(Pipe)(void) {
    int fds[2];
    if (pipe(fds) != 0) return (go_tuple_go_pt_go_pt_go_if){ {0}, {0}, go_os_error("pipe", go_string_new(""), errno) };
    File_go_os_package r = { fdopen(fds[0], "rb"), go_string_new("|0"), false, false };
    File_go_os_package w = { fdopen(fds[1], "wb"), go_string_new("|1"), false, false };
    return (go_tuple_go_pt_go_pt_go_if){ go_new(sizeof r, &r), go_new(sizeof w, &w), {0} };
}
#else
go_tuple_go_pt_go_if S(OpenFile)(go_ss name, go_ii flag, go_u4 perm) {
    (void)perm;
    return go_os_open(name, (flag & 3) == 0 ? "rb" : (flag & 0x400) ? "ab" : (flag & 3) == 2 ? "r+b" : "wb");
}
go_if S(Mkdir)(go_ss name, go_u4 perm) { return go_os_error("mkdir", name, ENOSYS); }
go_if S(MkdirAll)(go_ss path, go_u4 perm) { return go_os_error("mkdir", path, ENOSYS); }
go_tuple_go_ss_go_if S(MkdirTemp)(go_ss dir, go_ss pattern) { return (go_tuple_go_ss_go_if){ {0}, go_os_error("mkdirtemp", dir, ENOSYS) }; }
go_if S(Chdir)(go_ss dir) { return go_os_error("chdir", dir, ENOSYS); }
go_if S(File_Chdir)(go_pt f) { return go_os_error("chdir", go_string_new(""), ENOSYS); }
go_tuple_go_pt_go_pt_go_if S(Pipe)(void) { return (go_tuple_go_pt_go_pt_go_if){ {0}, {0}, go_os_error("pipe", go_string_new(""), ENOSYS) }; }
go_tuple_go_ll_go_if S(ReadDir)(go_ss name) { return (go_tuple_go_ll_go_if){ {0}, go_os_error("open", name, ENOSYS) }; }
go_tuple_go_ll_go_if S(File_Readdirnames)(go_pt f, go_ii n) { return (go_tuple_go_ll_go_if){ {0}, go_os_error("readdirent", go_string_new(""), ENOSYS) }; }
go_tuple_go_ll_go_if S(File_Readdir)(go_pt f, go_ii n) { return S(File_Readdirnames)(f, n); }
go_tuple_go_ss_go_if S(Readlink)(go_ss name) { return (go_tuple_go_ss_go_if){ {0}, go_os_error("readlink", name, ENOSYS) }; }
go_tuple_go_ss_go_if S(Getwd)(void) { return (go_tuple_go_ss_go_if){ {0}, go_os_error("getwd", go_string_new(""), ENOSYS) }; }

static go_tuple_go_if_go_if go_os_stat_file(const char* op, go_ss name, go_tf follow) {
    (void)follow;
    return (go_tuple_go_if_go_if){ {0}, go_os_error(op, name, ENOSYS) };
}
#endif

go_tuple_go_if_go_if S(Stat)(go_ss name) { return go_os_stat_file("stat", name, true); }
go_tuple_go_if_go_if S(Lstat)(go_ss name) { return go_os_stat_file("lstat", name, false); }

go_tuple_go_i8_go_if S(File_Seek)(go_pt f, go_i8 offset, go_ii whence) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return (go_tuple_go_i8_go_if){ 0, S(ErrInvalid) };
    if (fseek(file->f, (long)offset, whence == 1 ? SEEK_CUR : whence == 2 ? SEEK_END : SEEK_SET) != 0) {
        return (go_tuple_go_i8_go_if){ 0, go_os_error("seek", file->name, errno) };
    }
    return (go_tuple_go_i8_go_if){ (go_i8)ftell(file->f), {0} };
}

// IsNotExist and IsExist recognize the errors of os (which are strings, see go_os_error).
static go_tf go_os_error_is(go_if err, go_if sentinel, int errnum) {
    if (!err.go_type) return false;
    if (err.ptr.ptr == sentinel.ptr.ptr) return true;
    go_ss msg = ((go_error*)err.vtable)->Error(err.ptr.ptr);
    const char* reason = strerror(errnum);
    go_ii n = go_string_len(msg), m = (go_ii)strlen(reason);
    if (n < m) return false;
    for (go_ii i = 1; i < m; i++) if (msg.ptr[n - m + i] != reason[i]) return false; // (the first letter is lowercase)
    return true;
}
go_tf S(IsNotExist)(go_if err) { return go_os_error_is(err, S(ErrNotExist), ENOENT); }
go_tf S(IsExist)(go_if err) { return go_os_error_is(err, S(ErrExist), EEXIST); }

go_if S(Remove)(go_ss name) {
    return remove(go_os_cstring(name)) == 0 ? (go_if){0} : go_os_error("remove", name, errno);
}
go_if S(RemoveAll)(go_ss path) { // (of files: C11 can't list directories)
    if (remove(go_os_cstring(path)) != 0 && errno != ENOENT) return go_os_error("unlinkat", path, errno);
    return (go_if){0};
}

static File_go_os_package* go_os_check(go_pt f) {
    File_go_os_package* file = f.ptr;
    return file && !file->closed ? file : NULL;
}

go_tuple_go_ii_go_if S(File_Write)(go_pt f, go_ll b) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return (go_tuple_go_ii_go_if){ 0, S(ErrInvalid) };
    size_t n = b.len > 0 ? fwrite(b.ptr.ptr, 1, (size_t)b.len, file->f) : 0;
    fflush(file->f); // os.Files are unbuffered.
    if (n < (size_t)b.len) return (go_tuple_go_ii_go_if){ (go_ii)n, go_os_error("write", file->name, errno) };
    return (go_tuple_go_ii_go_if){ (go_ii)n, {0} };
}
go_tuple_go_ii_go_if S(File_WriteString)(go_pt f, go_ss s) {
    go_ii n = go_string_len(s);
    return S(File_Write)(f, (go_ll){ { (void*)s.ptr }, n, n });
}
go_tuple_go_ii_go_if S(File_Read)(go_pt f, go_ll b) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return (go_tuple_go_ii_go_if){ 0, S(ErrInvalid) };
    if (b.len == 0) return (go_tuple_go_ii_go_if){0};
    size_t n = fread(b.ptr.ptr, 1, (size_t)b.len, file->f);
    if (n == 0) {
        if (ferror(file->f)) return (go_tuple_go_ii_go_if){ 0, go_os_error("read", file->name, errno) };
        return (go_tuple_go_ii_go_if){ 0, EOF__go_io_package };
    }
    return (go_tuple_go_ii_go_if){ (go_ii)n, {0} };
}
go_if S(File_Close)(go_pt f) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return S(ErrClosed);
    file->closed = true;
    return fclose(file->f) == 0 ? (go_if){0} : go_os_error("close", file->name, errno);
}
go_ss S(File_Name)(go_pt f) { return ((File_go_os_package*)go_nil_check(f.ptr))->name; }
go_if S(File_Sync)(go_pt f) {
    File_go_os_package* file = go_os_check(f);
    if (!file) return S(ErrClosed);
    fflush(file->f);
    return (go_if){0};
}

go_tuple_go_ii_go_if I_File_Write_go_os_package(void* f, go_ll b) { return S(File_Write)(*(go_pt*)f, b); }
go_tuple_go_ii_go_if I_File_WriteString_go_os_package(void* f, go_ss s) { return S(File_WriteString)(*(go_pt*)f, s); }
go_tuple_go_ii_go_if I_File_Read_go_os_package(void* f, go_ll b) { return S(File_Read)(*(go_pt*)f, b); }
go_if I_File_Close_go_os_package(void* f) { return S(File_Close)(*(go_pt*)f); }
go_ss I_File_Name_go_os_package(void* f) { return S(File_Name)(*(go_pt*)f); }
go_if I_File_Sync_go_os_package(void* f) { return S(File_Sync)(*(go_pt*)f); }
go_tuple_go_ll_go_if I_File_Readdirnames_go_os_package(void* f, go_ii n) { return S(File_Readdirnames)(*(go_pt*)f, n); }
go_tuple_go_ll_go_if I_File_Readdir_go_os_package(void* f, go_ii n) { return S(File_Readdir)(*(go_pt*)f, n); }
