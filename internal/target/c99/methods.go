package c99

import (
	"fmt"
	"go/types"
	"hash/fnv"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// methodTable returns the fields of a type descriptor for the method set of t (a named
// type, or a pointer to one): a static table of its methods, which the interface values
// that are made at run time (type assertions, conversions between interfaces) look up by
// name. Promoted methods (of embedded fields) are not included yet.
func (c99 Target) methodTable(t types.Type) string {
	named, ok := types.Unalias(derefType(t)).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	if _, ok := named.Underlying().(*types.Interface); ok {
		return ""
	}
	_, isPointer := t.Underlying().(*types.Pointer)
	var entries []string
	set := types.NewMethodSet(t)
	for i := range set.Len() {
		sel := set.At(i)
		if len(sel.Index()) > 1 {
			continue // promoted.
		}
		fn := sel.Obj().(*types.Func)
		prefix := "I_"
		if _, ptrRecv := fn.Type().(*types.Signature).Recv().Type().Underlying().(*types.Pointer); isPointer && !ptrRecv {
			prefix = "IP_" // the receiver is the value pointed to.
		}
		if err := c99.MethodInstance(named, fn.Name()); err != nil {
			panic(err)
		}
		entries = append(entries, fmt.Sprintf("{ %s, %s, (void(*)(void))%s%s_%s_go_%s_package }",
			cString(fn.Name()), cString(typeName(fn.Type())), prefix, c99.typeCName(named),
			source.CIdent(fn.Name()), source.PackageIdent(named.Obj().Pkg())))
	}
	if len(entries) == 0 {
		return ""
	}
	symbol := "go_methods_" + identifier.ReplaceAllString(typeName(t), "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static const go_method %s[] = { %s };\n", symbol, strings.Join(entries, ", "))
		return nil
	})
	return fmt.Sprintf(", .methods=%s, .nmethods=%d", symbol, len(entries))
}

// interfaceMethods returns a C expression for the (static) methods that the interface
// iface requires, in the order of its table of methods, and how many.
func (c99 Target) interfaceMethods(iface types.Type) (string, int) {
	typ := iface.Underlying().(*types.Interface)
	if typ.NumMethods() == 0 {
		return "NULL", 0
	}
	var entries []string
	for i := range typ.NumMethods() {
		m := typ.Method(i)
		entries = append(entries, fmt.Sprintf("{ %s, %s }", cString(m.Name()), cString(typeName(m.Type()))))
	}
	hash := fnv.New64a()
	hash.Write([]byte(strings.Join(entries, ",")))
	symbol := fmt.Sprintf("go_imethods_%x", hash.Sum64())
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static const go_imethod %s[] = { %s };\n", symbol, strings.Join(entries, ", "))
		return nil
	})
	return symbol, typ.NumMethods()
}

// toInterface returns a C expression that converts vv (a C expression for an empty
// interface value) to the interface type iface: a type assertion (which panics) when
// assert, otherwise ok (a C expression for a go_tf*, or NULL) reports whether it could.
func (c99 Target) toInterface(vv string, iface types.Type, assert bool, ok string) string {
	if iface.Underlying().(*types.Interface).Empty() {
		if assert {
			return fmt.Sprintf("go_if_to_vv(go_to_iface(%s, %s, NULL, 0, true, NULL))", vv, cString(typeName(iface)))
		}
		return vv
	}
	methods, n := c99.interfaceMethods(iface)
	return fmt.Sprintf("go_to_iface(%s, %s, %s, %d, %t, %s)", vv, cString(typeName(iface)), methods, n, assert, ok)
}

// asEmptyInterface returns a C expression for the interface value value (of the interface
// type t) as an empty interface value.
func asEmptyInterface(value string, t types.Type) string {
	if t.Underlying().(*types.Interface).Empty() {
		return value
	}
	return "go_if_to_vv(" + value + ")"
}
