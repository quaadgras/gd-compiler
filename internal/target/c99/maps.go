package c99

import (
	"fmt"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// MapFuncsOf returns the names of the hash and equality functions for map keys of type
// key, generated from its structure: fields and elements are hashed (and compared) in
// turn, floats are normalized so that +0 and -0 are the same key, and strings are hashed
// by their contents.
func (c99 Target) MapFuncsOf(key types.Type) (hash, same string, err error) {
	if isString(key) {
		return "go_hash_ss", "go_same_ss", nil
	}
	ctype := c99.TypeOf(key)
	id := identifier.ReplaceAllString(ctype, "_")
	hash, same = "go_hash_"+id, "go_same_"+id
	var hashes, equals []string
	if err := c99.keyParts(key, "(*k)", "(*x)", "(*y)", &hashes, &equals); err != nil {
		return "", "", err
	}
	c99.Requires(hash, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static go_u8 %s(const void* item, go_u8 seed0, go_u8 seed1) { const %s* k = item; go_u8 h = seed0; %s return h; }\n",
			hash, ctype, strings.Join(hashes, " "))
		return nil
	})
	c99.Requires(same, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static go_tf %s(const void* a, const void* b) { const %s* x = a; const %s* y = b; return %s; }\n",
			same, ctype, ctype, strings.Join(equals, " && "))
		return nil
	})
	return hash, same, nil
}

// keyParts appends the C statements that hash the key k (an expression of type t), and the
// C expressions that compare x and y (of type t).
func (c99 Target) keyParts(t types.Type, k, x, y string, hashes, equals *[]string) error {
	mix := func(value, ctype string) {
		*hashes = append(*hashes, fmt.Sprintf("{ %s v = %s; h = go_hash_bytes(&v, sizeof(v), h, seed1); }", ctype, value))
	}
	switch typ := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case typ.Info()&types.IsString != 0:
			*hashes = append(*hashes, fmt.Sprintf("h = go_hash_ss(&%s, h, seed1);", k))
			*equals = append(*equals, fmt.Sprintf("go_string_eq(%s, %s)", x, y))
		case typ.Info()&types.IsFloat != 0:
			mix(fmt.Sprintf("%[1]s == 0 ? 0 : %[1]s", k), c99.TypeOf(t)) // -0 is 0.
			*equals = append(*equals, fmt.Sprintf("%s == %s", x, y))
		case typ.Info()&types.IsComplex != 0:
			for _, part := range []string{"f1", "f2"} {
				mix(fmt.Sprintf("%[1]s.%[2]s == 0 ? 0 : %[1]s.%[2]s", k, part), "go_f8")
				*equals = append(*equals, fmt.Sprintf("%s.%s == %s.%[2]s", x, part, y))
			}
		case typ.Kind() == types.UnsafePointer:
			mix(k+".ptr", "void*")
			*equals = append(*equals, fmt.Sprintf("%s.ptr == %s.ptr", x, y))
		default: // integers and booleans.
			mix(k, c99.TypeOf(t))
			*equals = append(*equals, fmt.Sprintf("%s == %s", x, y))
		}
	case *types.Pointer:
		mix(k+".ptr", "void*")
		*equals = append(*equals, fmt.Sprintf("%s.ptr == %s.ptr", x, y))
	case *types.Chan:
		mix(k, "go_ch")
		*equals = append(*equals, fmt.Sprintf("%s == %s", x, y))
	case *types.Array:
		if typ.Len() == 0 || zeroSize(t) {
			break
		}
		i := fmt.Sprintf("go_i%d", strings.Count(k, "go_i")) // for nested arrays.
		sub := ".a[" + i + "]"
		var elems, unused []string
		if err := c99.keyParts(typ.Elem(), k+sub, x+sub, y+sub, &elems, &unused); err != nil {
			return err
		}
		*hashes = append(*hashes, fmt.Sprintf("for (go_ii %[1]s = 0; %[1]s < %[2]d; %[1]s++) { %[3]s }", i, typ.Len(), strings.Join(elems, " ")))
		*equals = append(*equals, fmt.Sprintf("%s(&%s, &%s)", c99.equalPtrFunc(t), x, y))
	case *types.Struct:
		for field := range typ.Fields() {
			if field.Name() == "_" {
				continue // blank fields are ignored by ==.
			}
			sub := "." + source.CIdent(field.Name())
			if err := c99.keyParts(field.Type(), k+sub, x+sub, y+sub, hashes, equals); err != nil {
				return err
			}
		}
	case *types.Interface: // by the dynamic type of the value.
		*hashes = append(*hashes, fmt.Sprintf("h = go_vv_hash(%s, h, seed1);", asEmptyInterface(k, t)))
		*equals = append(*equals, fmt.Sprintf("go_vv_eq(%s, %s)", asEmptyInterface(x, t), asEmptyInterface(y, t)))
	default:
		return fmt.Errorf("unsupported map key type %s", t)
	}
	if len(*equals) == 0 {
		*equals = append(*equals, "true") // struct{} and [0]T.
	}
	return nil
}

// MakeMap writes a new map of type typ, with room for hint entries, and the entries of a
// map literal (a C array of entries, of the struct type entry), when count > 0.
func (c99 Target) MakeMap(typ *types.Map, hint, entries, entry string, count int) error {
	hash, same, err := c99.MapFuncsOf(typ.Key())
	if err != nil {
		return err
	}
	init, stride, offset := "NULL", "0", "0"
	if count > 0 {
		init, stride, offset = entries, "sizeof("+entry+")", "offsetof("+entry+", val)"
	}
	fmt.Fprintf(c99, "go_make(sizeof(%s), sizeof(%s), %s, %s, %s, %d, %s, %s, %s)",
		c99.TypeOf(typ.Key()), c99.TypeOf(typ.Elem()), hash, same, hint, count, init, stride, offset)
	return nil
}
