// Hooks of maps, which the Go runtime implements.
#include <go/maps.h>
#include <go/maps/private.h>

go_vv clone_go_maps_package(go_vv m) {
    go_kv clone = go_map_clone(*(go_kv*)m.ptr.ptr);
    return go_any_new(sizeof(go_kv), &clone, m.go_type);
}
