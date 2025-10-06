#ifndef go_runtime_errors_imported
#define go_runtime_errors_imported
#include <go.h>

static const go_type type_errorString = {
    .name = "errors.errorString",
    .kind = go_kind_string,
};

static go_ss I_errorString_Error(void* e) { return *(go_ss*)e; }

static go_if New_go_errors_package(go_ss text) {
    return go_interface_new(sizeof(go_ss), &text, &type_errorString, &(go_error){.Error = I_errorString_Error});
}

#endif // go_runtime_errors_imported
