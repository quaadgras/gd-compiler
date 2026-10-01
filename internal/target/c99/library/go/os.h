#ifndef go_os_package_imported
#define go_os_package_imported
// Package os, implemented natively (see library/hooks/os.c) with C11's standard library:
// the subset of it that doesn't need an operating system interface beyond C's (files,
// standard streams, the environment, and exiting).
// gd:import io
// gd:import errors
// gd:import io/fs
#include <go.h>
#include <stdio.h>
#include <go/errors.h>
#include <go/io/fs.h> // (os.FileMode and others are io/fs's)

typedef struct { FILE* f; go_ss name; go_tf closed; } File_go_os_package;
extern const go_type go_type_File_go_os_package;
extern const go_method go_os_file_methods[]; // of *os.File
#define go_methods_ptr_File_go_os_package go_os_file_methods
#define go_nmethods_ptr_File_go_os_package 6

#ifndef go_tuple_go_ii_go_if_defined
#define go_tuple_go_ii_go_if_defined
typedef struct { go_ii r0; go_if r1; } go_tuple_go_ii_go_if;
#endif
#ifndef go_tuple_go_ll_go_if_defined
#define go_tuple_go_ll_go_if_defined
typedef struct { go_ll r0; go_if r1; } go_tuple_go_ll_go_if;
#endif
#ifndef go_tuple_go_pt_go_if_defined
#define go_tuple_go_pt_go_if_defined
typedef struct { go_pt r0; go_if r1; } go_tuple_go_pt_go_if;
#endif
#ifndef go_tuple_go_ss_go_tf_defined
#define go_tuple_go_ss_go_tf_defined
typedef struct { go_ss r0; go_tf r1; } go_tuple_go_ss_go_tf;
#endif

extern go_ll Args_go_os_package;
extern go_pt Stdin_go_os_package, Stdout_go_os_package, Stderr_go_os_package;
extern go_if ErrNotExist_go_os_package, ErrExist_go_os_package, ErrPermission_go_os_package, ErrClosed_go_os_package, ErrInvalid_go_os_package;

void init_go_os_package(void);
_Noreturn void Exit_go_os_package(go_ii code);
go_ss Getenv_go_os_package(go_ss key);
go_tuple_go_ss_go_tf LookupEnv_go_os_package(go_ss key);
go_if Setenv_go_os_package(go_ss key, go_ss value);
go_if Unsetenv_go_os_package(go_ss key);
go_ll Environ_go_os_package(void);
go_ii Getpid_go_os_package(void);

go_tuple_go_pt_go_if Open_go_os_package(go_ss name);
go_tuple_go_pt_go_if Create_go_os_package(go_ss name);
go_tuple_go_ll_go_if ReadFile_go_os_package(go_ss name);
go_if WriteFile_go_os_package(go_ss name, go_ll data, go_u4 perm);
go_if Remove_go_os_package(go_ss name);
go_if RemoveAll_go_os_package(go_ss path);

go_tuple_go_ii_go_if File_Write_go_os_package(go_pt f, go_ll b);
go_tuple_go_ii_go_if File_WriteString_go_os_package(go_pt f, go_ss s);
go_tuple_go_ii_go_if File_Read_go_os_package(go_pt f, go_ll b);
go_if File_Close_go_os_package(go_pt f);
go_ss File_Name_go_os_package(go_pt f);
go_if File_Sync_go_os_package(go_pt f);

// for interface values of *os.File (see I_ wrappers)
go_tuple_go_ii_go_if I_File_Write_go_os_package(void* f, go_ll b);
go_tuple_go_ii_go_if I_File_WriteString_go_os_package(void* f, go_ss s);
go_tuple_go_ii_go_if I_File_Read_go_os_package(void* f, go_ll b);
go_if I_File_Close_go_os_package(void* f);
go_ss I_File_Name_go_os_package(void* f);
go_if I_File_Sync_go_os_package(void* f);

#endif // go_os_package_imported
