// Package os, implemented with C11's standard library, see library/go/os.h.
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
    {"Read", "func([]byte) (int, error)", (void(*)(void))I_File_Read_go_os_package},
    {"Sync", "func() error", (void(*)(void))I_File_Sync_go_os_package},
    {"Write", "func([]byte) (int, error)", (void(*)(void))I_File_Write_go_os_package},
    {"WriteString", "func(string) (int, error)", (void(*)(void))I_File_WriteString_go_os_package},
};
const go_type go_type_File_go_os_package = {.name="os.File", .kind=go_kind_struct, .size=sizeof(File_go_os_package)};

static go_pt go_os_file(FILE* f, const char* name) {
    File_go_os_package file = { f, go_string_new(name), false };
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
    File_go_os_package file = { f, name, false };
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
