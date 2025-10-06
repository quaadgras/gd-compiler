#ifndef go_runtime_package_imported
#define go_runtime_package_imported
#include <go.h>
#include <threads.h>
#include <go/errors.h>

typedef struct { go_ii f1; go_if f2; } go_aaiitfzz;
typedef struct { go_up f1; go_ss f2; go_ii f3; go_tf f4; } go_aaupssiitfzz;

typedef struct { go_u4 f1; go_u8 f2; go_u8 f3; } go_aau4u8u8zz;
typedef struct {
    go_u8 Alloc;
    go_u8 TotalAlloc;
    go_u8 Sys;
    go_u8 Lookups;
    go_u8 Mallocs;
    go_u8 Frees;
    go_u8 HeapAlloc;
    go_u8 HeapSys;
    go_u8 HeapIdle;
    go_u8 HeapInuse;
    go_u8 HeapReleased;
    go_u8 HeapObjects;
    go_u8 StackInuse;
    go_u8 StackSys;
    go_u8 MSpanInuse;
    go_u8 MSpanSys;
    go_u8 MCacheInuse;
    go_u8 MCacheSys;
    go_u8 BuckHashSys;
    go_u8 GCSys;
    go_u8 OtherSys;
    go_u8 NextGC;
    go_u8 LastGC;
    go_u8 PauseTotalNs;
    go_u8 PauseNs[256];
    go_u8 PauseEnd[256];
    go_u8 NumGC;
    go_u4 NumForcedGC;
    go_u4 GCCPUFraction;
    go_tf EnableGC;
    go_tf DebugGC;
    go_aau4u8u8zz BySize[61]; // size, mallocs, frees
} go_tt_MemStats_go_runtime_package;

typedef struct { go_up Stack0[32]; } StackRecord_go_runtime_package;
typedef struct { go_i8 Count; go_i8 Cycles; StackRecord_go_runtime_package StackRecord; } BlockProfileRecord_go_runtime_package;
typedef struct {} Cleanup_go_runtime_package;

static inline go_ll StackRecord_Stack_go_runtime_package(StackRecord_go_runtime_package* sr) {
    return (go_ll){};
}

static const go_ss Compiler_go_runtime_package = go_string_new("gd");
static const go_ss GOARCH_go_runtime_package = go_string_new(go_ARCH);
static const go_ss GOOS_go_runtime_package = go_string_new(go_OS);
extern go_ii MemProfileRate_go_runtime_package;

static inline go_aaiitfzz BlockProfile_go_runtime_package(go_ll p) { return (go_aaiitfzz){}; }
static inline void Breakpoint_go_runtime_package(void) {
    #if defined(_MSC_VER)
        __debugbreak();
    #elif defined(clang) && __has_builtin(__builtin_debugtrap)
        __builtin_debugtrap();
    #elif (defined(GNUC) || defined(clang)) && (defined(i386) || defined(x86_64))
        asm volatile("int $3");
    #elif (defined(GNUC) || defined(clang)) && defined(thumb)
        asm volatile(".inst 0xde01");
    #elif (defined(GNUC) || defined(clang)) && defined(aarch64)
        asm volatile(".inst 0xd4200000");
    #elif (defined(GNUC) || defined(clang)) && defined(arm) && !defined(thumb)
        asm volatile(".inst 0xe7f001f0");
    #elif defined(GNUC) || defined(clang)
        __builtin_trap();
    #elif defined(_POSIX_VERSION) && defined(SIGTRAP)
        #include <signal.h>
        raise(SIGTRAP);
    #else
        // Do nothing on unsupported platforms
    #endif
}
static inline go_aaupssiitfzz Caller_go_runtime_package(go_ii skip) { return (go_aaupssiitfzz){}; }
static inline go_ii Callers_go_runtime_package(go_ii skip, go_ll pc) { return 0; }
static inline void GC_go_runtime_package(void) {}
static inline go_ii GOMAXPROCS_go_runtime_package(go_ii n) { return -1; }
static inline void Goexit_go_runtime_package(void) { thrd_exit(0); }
static inline go_aaiitfzz GoroutineProfile_go_runtime_package(go_ll p) { return (go_aaiitfzz){}; }
static inline void Gosched_go_runtime_package(void) { thrd_yield(); }
static inline void KeepAlive_go_runtime_package(go_vv x) {};
static inline void LockOSThread_go_runtime_package(void) {}
static inline go_aaiitfzz MemProfile_go_runtime_package(go_ll p, go_tf inuseZero) { return (go_aaiitfzz){}; }
static inline go_aaiitfzz MutexProfile_go_runtime_package(go_ll p) { return (go_aaiitfzz){}; }
static inline go_ii NumCPU_go_runtime_package(void) { return 1; }
static inline go_ii NumCgoCall_go_runtime_package(void) { return 0; }
static inline go_ii NumGoroutine_go_runtime_package(void) { return 1; }
static inline void ReadMemStats_go_runtime_package(go_pt m) {}
static inline go_ll ReadTrace_go_runtime_package(void) { return (go_ll){}; }
static inline void SetBlockProfileRate_go_runtime_package(go_ii rate) {}
static inline void SetCPUProfileRate_go_runtime_package(go_ii hz) {}
static inline void SetCgoTraceback_go_runtime_package(go_ii version, go_pt traceback, go_pt context, go_pt symbolizer) {}
static inline void SetDefaultGOMAXPROCS_go_runtime_package(void) {}
static inline void SetFinalizer_go_runtime_package(go_vv obj, go_vv finalizer) {}
static inline go_ii SetMutexProfileFraction_go_runtime_package(go_ii rate) { return -1; }
static inline go_ii Stack_go_runtime_package(go_ll buf, go_tf all) { return 0; }
static inline go_if StartTrace_go_runtime_package(void) { return New_go_errors_package(go_string_new("tracing not supported")); }
static inline void StopTrace_go_runtime_package(void) {}
static inline go_aaiitfzz ThreadCreateProfile_go_runtime_package(go_ll p) { return (go_aaiitfzz){}; }
static inline void UnlockOSThread_go_runtime_package(void) {}
static inline go_ss Version_go_runtime_package(void) { return go_string_new("go1.25.1"); }

static inline Cleanup_go_runtime_package AddCleanup_go_runtime_package(const go_type T, const go_type S, go_pt ptr, go_fn cleanup, void* arg) {
    return (Cleanup_go_runtime_package){};
}
static inline void Cleanup_Stop_go_runtime_package(Cleanup_go_runtime_package c) {}

typedef struct {
    go_error error;

    void(*RuntimeError)(void*);
} Error_go_runtime_package;

typedef struct {
    go_up PC;
    go_pt Func;
    go_ss Function;
    go_ss File;
    go_ii Line;
    go_up Entry;
} Frame_go_runtime_package;

typedef struct {

} Frames_go_runtime_package;

typedef struct {
    Frame_go_runtime_package f1;
    go_tf f2;
} go_aa__runtime_Frame__tfzz;

static inline go_pt CallersFrames(go_ll callers) { return (go_pt){}; }
static inline go_aa__runtime_Frame__tfzz Next_go_runtime_package(void) {
    return (go_aa__runtime_Frame__tfzz){};
}

typedef struct {

} Func_go_runtime_package;

typedef struct {
    go_ss f1;
    go_ii f2;
} go_aassiizz;

static inline go_pt FuncForPC_go_runtime_package(go_up pc) { return (go_pt){}; }
static inline go_up Func_Entry_go_runtime_package(go_pt f) { return (go_up){}; }
static inline go_aassiizz Func_FileLine_go_runtime_package(go_pt f) { return (go_aassiizz){}; }
static inline go_ss Func_Name_go_runtime_package(go_pt f) { return (go_ss){}; }

typedef struct {
    go_i8 AllocBytes;
    go_i8 FreeBytes;
    go_i8 AllocObjects;
    go_i8 FreeObjects;
    go_up Stack0[32];
} MemProfileRecord_go_runtime_package;

static inline go_ii MemProfileRecord_InUseBytes_go_runtime_package(go_pt r) {
    MemProfileRecord_go_runtime_package record = go_pointer_get(r, MemProfileRecord_go_runtime_package);
    return record.AllocBytes - record.FreeBytes;
}
static inline go_ii MemProfileRecord_InUseObjects_go_runtime_package(go_pt r) {
    MemProfileRecord_go_runtime_package record = go_pointer_get(r, MemProfileRecord_go_runtime_package);
    return record.AllocObjects - record.FreeObjects;
}
static inline go_ll MemProfileRecord_Stack0_go_runtime_package(void) {
    return (go_ll){};
}

typedef struct {
} PanicNilError_go_runtime_package;

static inline go_ss PanicNilError_Error_go_runtime_package(PanicNilError_go_runtime_package p) {
    return go_string_new("runtime error: invalid memory address or nil pointer dereference");
}
static inline void PanicNilError_RuntimeError_go_runtime_package(PanicNilError_go_runtime_package p) {}

typedef struct {
} Pinner_go_runtime_package;

static inline void Pinner_Pin_go_runtime_package(Pinner_go_runtime_package* p, go_vv pointer) {}
static inline void Pinner_Unpin_go_runtime_package(Pinner_go_runtime_package* p) {}

typedef struct {
} TypeAssertionError_go_runtime_package;

static inline go_ss TypeAssertionError_Error_go_runtime_package(TypeAssertionError_go_runtime_package e) {
    return go_string_new("interface conversion: interface is nil, not ");
}
static inline void TypeAssertionError_RuntimeError_go_runtime_package(TypeAssertionError_go_runtime_package e) {}

#endif // go_runtime_package_imported
