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
// name.
func (c99 Target) methodTable(t types.Type) string {
	named, ok := types.Unalias(derefType(t)).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	if IsNative(named.Obj().Pkg().Path()) { // which implement some of the methods of their types.
		symbol := "go_methods_" + c99.typeCName(named) + "_go_" + source.PackageIdent(named.Obj().Pkg()) + "_package"
		if _, isPointer := t.Underlying().(*types.Pointer); isPointer {
			symbol = "go_methods_ptr_" + strings.TrimPrefix(symbol, "go_methods_")
		}
		return fmt.Sprintf("\n#ifdef %[1]s\n, .methods=%[1]s, .nmethods=go_n%[2]s\n#endif\n", symbol, strings.TrimPrefix(symbol, "go_"))
	}
	if _, ok := named.Underlying().(*types.Interface); ok {
		return ""
	}
	_, isPointer := t.Underlying().(*types.Pointer)
	var entries []string
	set := types.NewMethodSet(t)
	for i := range set.Len() {
		fn := set.At(i).Obj().(*types.Func)
		prefix := "I_"
		if isPointer && !pointerReceiver(named, fn) {
			prefix = "IP_" // the receiver is the value pointed to.
		}
		if err := c99.MethodInstance(named, fn.Name()); err != nil {
			panic(err)
		}
		if err := c99.promotedMethod(named, fn.Pkg(), fn.Name()); err != nil {
			panic(err)
		}
		entries = append(entries, fmt.Sprintf("{ %s, %s, (void(*)(void))%s%s }",
			cString(fn.Name()), cString(typeName(fn.Type())), prefix, c99.methodCName(named, fn.Name())))
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

// methodCName returns the C name of the method of the named type (or struct type, for the
// methods promoted from its embedded fields).
func (c99 Target) methodCName(t types.Type, method string) string {
	named, ok := t.(*types.Named)
	if !ok {
		return identifier.ReplaceAllString(c99.TypeOf(t), "_") + "_" + source.CIdent(method)
	}
	return fmt.Sprintf("%s_%s_go_%s_package", c99.typeCName(named), source.CIdent(method), source.PackageIdent(named.Obj().Pkg()))
}

// pointerReceiver reports whether the method fn of t (a named type, or a pointer to one)
// takes a pointer receiver. The wrappers of promoted methods take one when they are not in
// the method set of the named type itself.
func pointerReceiver(t types.Type, fn *types.Func) bool {
	named := derefType(t)
	if _, index, _ := types.LookupFieldOrMethod(named, true, fn.Pkg(), fn.Name()); len(index) > 1 {
		return types.NewMethodSet(named).Lookup(fn.Pkg(), fn.Name()) == nil
	}
	_, ok := fn.Type().(*types.Signature).Recv().Type().Underlying().(*types.Pointer)
	return ok
}

// promotedMethod emits (if it hasn't been, and if the method is promoted from an embedded
// field of the named type) a function, static in each file, with the C name of the method
// of the named type, that calls the method of the embedded field, and its interface wrappers.
func (c99 Target) promotedMethod(named types.Type, pkg *types.Package, method string) error {
	obj, index, _ := types.LookupFieldOrMethod(named, true, pkg, method)
	fn, ok := obj.(*types.Func)
	if !ok || len(index) < 2 {
		return nil
	}
	name := c99.methodCName(named, method)
	return c99.Requires(name, c99.Generic, func(w io.Writer) error {
		sig := fn.Type().(*types.Signature)
		pointer := pointerReceiver(named, fn)
		recvType := c99.TypeOf(named)
		expr := "go_recv"
		var typ types.Type = named
		if pointer {
			recvType = "go_pt"
			expr = fmt.Sprintf("go_pointer_get(go_recv, %s)", c99.TypeOf(named))
		}
		for _, i := range index[:len(index)-1] { // the embedded field.
			if ptr, ok := typ.Underlying().(*types.Pointer); ok {
				expr = fmt.Sprintf("go_pointer_get(%s, %s)", expr, c99.TypeOf(ptr.Elem()))
				typ = ptr.Elem()
			}
			field := typ.Underlying().(*types.Struct).Field(i)
			expr += "." + fieldName(field, i)
			typ = field.Type()
		}
		var params, args []string
		for i, v := range slicesOfVars(sig.Params()) {
			args = append(args, fmt.Sprintf("p%d", i))
			params = append(params, c99.TypeOf(v.Type())+" "+args[i])
		}
		var call string
		if _, iface := typ.Underlying().(*types.Interface); iface {
			call = fmt.Sprintf("go_interface_methods(%s, %s)->%s(%s)", c99.InterfaceTypeOf(typ), expr, source.CIdent(method),
				strings.Join(append([]string{expr + ".ptr.ptr"}, args...), ", "))
		} else {
			recvNamed, ok := types.Unalias(derefType(sig.Recv().Type())).(*types.Named)
			if !ok {
				return fmt.Errorf("unsupported receiver type %s", sig.Recv().Type())
			}
			if err := c99.MethodInstance(recvNamed, method); err != nil {
				return err
			}
			if err := c99.promotedMethod(recvNamed, pkg, method); err != nil {
				return err
			}
			_, wantPointer := sig.Recv().Type().Underlying().(*types.Pointer)
			_, isPointer := typ.Underlying().(*types.Pointer)
			recv := expr
			switch {
			case wantPointer && !isPointer:
				recv = fmt.Sprintf("((go_pt){ .ptr = &(%s) })", expr)
			case !wantPointer && isPointer:
				recv = fmt.Sprintf("go_pointer_get(%s, %s)", expr, c99.TypeOf(derefType(typ)))
			}
			call = fmt.Sprintf("%s(%s)", c99.methodCName(recvNamed, method), strings.Join(append([]string{recv}, args...), ", "))
		}
		result, ret := c99.TupleOfResults(sig), ""
		if sig.Results().Len() > 0 {
			ret = "return "
		}
		wrapper := func(prefix, recv string) {
			fmt.Fprintf(w, "static %s %s%s(%s) { %s%s(%s); }\n", result, prefix, name,
				strings.Join(append([]string{"void* go_recv"}, params...), ", "), ret, name, strings.Join(append([]string{recv}, args...), ", "))
		}
		fmt.Fprintf(w, "static %s %s(%s) { %s%s; }\n", result, name, strings.Join(append([]string{recvType + " go_recv"}, params...), ", "), ret, call)
		wrapper("I_", fmt.Sprintf("*(%s*)go_recv", recvType))
		if !pointer {
			wrapper("IP_", fmt.Sprintf("go_pointer_get(*(go_pt*)go_recv, %s)", recvType))
		}
		return nil
	})
}

func slicesOfVars(tuple *types.Tuple) []*types.Var {
	var vars []*types.Var
	for v := range tuple.Variables() {
		vars = append(vars, v)
	}
	return vars
}

// methodValue writes a method value x.M (a func value, bound to a copy of the receiver x), or
// a method expression T.M (a func value, that takes the receiver as its first argument).
func (c99 Target) methodValue(sel source.Selection, fn source.DefinedFunction) error {
	xtype := sel.X.TypeAndValue().Type
	obj, ok := fn.Unique.(*types.Func)
	if !ok {
		return sel.Errorf("unsupported method value")
	}
	if concrete, _, _ := types.LookupFieldOrMethod(xtype, true, obj.Pkg(), obj.Name()); concrete != nil {
		if m, ok := concrete.(*types.Func); ok {
			obj = m // the method of a type parameter's constraint, in an instance.
		}
	}
	name := source.CIdent(obj.Name())
	iface, isInterface := xtype.Underlying().(*types.Interface)
	var named types.Type
	if !isInterface {
		var err error
		if named, err = c99.receiverType(xtype, obj.Pkg(), obj.Name()); err != nil {
			return sel.Errorf("%w", err)
		}
	}
	if sel.X.TypeAndValue().IsType() { // a method expression.
		sig := obj.Type().(*types.Signature)
		ctype := c99.TypeOf(xtype)
		var call string
		if isInterface {
			call = fmt.Sprintf("go_interface_methods(%s, go_recv)->%s(go_recv.ptr.ptr", c99.InterfaceTypeOf(iface), name)
		} else {
			recv := "go_recv"
			if _, isPointer := xtype.Underlying().(*types.Pointer); isPointer && !pointerReceiver(xtype, obj) {
				recv = fmt.Sprintf("go_pointer_get(go_recv, %s)", c99.TypeOf(named))
			}
			call = fmt.Sprintf("%s(%s", c99.methodCName(named, obj.Name()), recv)
		}
		params := []string{"void* go_env", ctype + " go_recv"}
		for i, v := range slicesOfVars(sig.Params()) {
			params = append(params, fmt.Sprintf("%s p%d", c99.TypeOf(v.Type()), i))
			call += fmt.Sprintf(", p%d", i)
		}
		ret := ""
		if sig.Results().Len() > 0 {
			ret = "return "
		}
		symbol := "go_method_expr_" + identifier.ReplaceAllString(typeName(xtype)+"_"+obj.Name(), "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static %s %s(%s) { %s%s); }\n", c99.TupleOfResults(sig), symbol, strings.Join(params, ", "), ret, call)
			return nil
		})
		fmt.Fprintf(c99, "go_make_func(%s)", symbol)
		return nil
	}
	if isInterface {
		ctype := c99.InterfaceTypeOf(iface)
		symbol := "go_method_value_" + identifier.ReplaceAllString(typeName(xtype)+"_"+name, "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "static inline go_fn %s(go_if v) { go_nil_check(v.vtable); return go_make_closure(go_interface_methods(%s, v)->%s, v.ptr.ptr); }\n",
				symbol, ctype, name)
			return nil
		})
		fmt.Fprintf(c99, "%s(%s)", symbol, c99.toString(sel.X))
		return nil
	}
	recv, err := c99.methodReceiver(fn, sel.X)
	if err != nil {
		return err
	}
	ctype := c99.TypeOf(named)
	if pointerReceiver(xtype, obj) {
		ctype = "go_pt"
	}
	box := "go_box_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(box, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline void* %s(%s v) { return go_new(sizeof v, &v).ptr; }\n", box, ctype)
		return nil
	})
	fmt.Fprintf(c99, "go_make_closure(I_%s, %s(%s))", c99.methodCName(named, obj.Name()), box, recv)
	return nil
}

// receiverType returns the named type (or struct type) of the receiver of the method of t
// (that type, or a pointer to it), emitting the functions the method needs: instances of
// methods of generic types, and wrappers of promoted methods.
func (c99 Target) receiverType(t types.Type, pkg *types.Package, method string) (types.Type, error) {
	recv := types.Unalias(derefType(t))
	switch typ := recv.(type) {
	case *types.Named:
		if err := c99.MethodInstance(typ, method); err != nil {
			return nil, err
		}
	case *types.Struct:
	default:
		return nil, fmt.Errorf("unsupported receiver type %s", t)
	}
	return recv, c99.promotedMethod(recv, pkg, method)
}
