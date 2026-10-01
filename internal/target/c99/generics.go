package c99

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// Generics are compiled by monomorphization: each instance of a generic function, method or
// type (with its type arguments) is compiled separately, with its type parameters
// substituted, see [source.Substitute]. Instances are static to each file that uses them.
type Generics struct {
	funcs   map[*types.Func]source.FunctionDefinition                // generic functions.
	methods map[*types.TypeName]map[string]source.FunctionDefinition // of generic types.
	ctxt    *types.Context
}

// NewGenerics finds the generic functions and methods of a package.
func NewGenerics(files []source.File) *Generics {
	g := &Generics{
		funcs:   make(map[*types.Func]source.FunctionDefinition),
		methods: make(map[*types.TypeName]map[string]source.FunctionDefinition),
		ctxt:    types.NewContext(),
	}
	for _, file := range files {
		for _, def := range file.Definitions {
			if node, _ := def.Get(); node != nil {
				if decl, ok := node.(source.FunctionDefinition); ok {
					fn, ok := decl.Name.Unique.(*types.Func)
					if !ok {
						continue
					}
					sig := fn.Type().(*types.Signature)
					if sig.TypeParams().Len() > 0 {
						g.funcs[fn] = decl
					}
					if sig.RecvTypeParams().Len() > 0 {
						named, ok := types.Unalias(derefType(sig.Recv().Type())).(*types.Named)
						if !ok {
							continue
						}
						obj := named.Origin().Obj()
						if g.methods[obj] == nil {
							g.methods[obj] = make(map[string]source.FunctionDefinition)
						}
						g.methods[obj][fn.Name()] = decl
					}
				}
			}
		}
	}
	return g
}

// IsGeneric reports whether decl is a generic function or method (which are compiled as
// instances, rather than where they are declared).
func IsGeneric(decl source.FunctionDefinition) bool {
	fn, ok := decl.Name.Unique.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.TypeParams().Len() > 0 || sig.RecvTypeParams().Len() > 0
}

// subst substitutes the type parameters of the instance being compiled in t.
func subst(t types.Type) types.Type {
	if source.Substitute != nil && t != nil {
		return source.Substitute(t)
	}
	return t
}

// substituter returns a function that substitutes the type parameters params with args.
func (g *Generics) substituter(params []*types.TypeParam, args []types.Type) func(types.Type) types.Type {
	m := make(map[*types.TypeParam]types.Type, len(args))
	for i, param := range params {
		m[param] = args[i]
	}
	var apply func(t types.Type) types.Type
	tuple := func(t *types.Tuple) *types.Tuple {
		if t == nil {
			return nil
		}
		var vars []*types.Var
		for v := range t.Variables() {
			vars = append(vars, types.NewParam(v.Pos(), v.Pkg(), v.Name(), apply(v.Type())))
		}
		return types.NewTuple(vars...)
	}
	apply = func(t types.Type) types.Type {
		if named, ok := t.(*types.Named); ok && isLocal(named.Obj()) && len(m) > 0 && (declaredWithin(named.Origin().Obj(), m) || mentionsParams(named, m, nil)) {
			return g.localInstance(named, params, apply)
		}
		switch typ := t.(type) {
		case *types.TypeParam:
			if r, ok := m[typ]; ok {
				return r
			}
		case *types.Alias:
			return apply(types.Unalias(typ))
		case *types.Pointer:
			return types.NewPointer(apply(typ.Elem()))
		case *types.Slice:
			return types.NewSlice(apply(typ.Elem()))
		case *types.Array:
			return types.NewArray(apply(typ.Elem()), typ.Len())
		case *types.Map:
			return types.NewMap(apply(typ.Key()), apply(typ.Elem()))
		case *types.Chan:
			return types.NewChan(typ.Dir(), apply(typ.Elem()))
		case *types.Tuple:
			return tuple(typ)
		case *types.Signature:
			return types.NewSignatureType(nil, nil, nil, tuple(typ.Params()), tuple(typ.Results()), typ.Variadic())
		case *types.Interface: // (with its methods, embedded interfaces are flattened)
			if typ.NumMethods() == 0 {
				return typ
			}
			var methods []*types.Func
			for m := range typ.Methods() {
				methods = append(methods, types.NewFunc(m.Pos(), m.Pkg(), m.Name(), apply(m.Type()).(*types.Signature)))
			}
			return types.NewInterfaceType(methods, nil).Complete()
		case *types.Struct:
			var fields []*types.Var
			var tags []string
			for i := range typ.NumFields() {
				f := typ.Field(i)
				fields = append(fields, types.NewField(f.Pos(), f.Pkg(), f.Name(), apply(f.Type()), f.Embedded()))
				tags = append(tags, typ.Tag(i))
			}
			return types.NewStruct(fields, tags)
		case *types.Named:
			if typ.TypeArgs().Len() == 0 {
				return typ
			}
			var targs []types.Type
			for a := range typ.TypeArgs().Types() {
				targs = append(targs, apply(a))
			}
			inst, err := types.Instantiate(g.ctxt, typ.Origin(), targs, false)
			if err != nil {
				panic(err)
			}
			return inst
		}
		return t
	}
	return apply
}

// Local types of generic functions (whose types may be made of the function's type
// parameters) are types of their own in each instance of the function: localInstance
// returns a new named type for the local type t (or its instance) in the instance whose
// type arguments substitute params (with apply), the same for the same instance.
func (g *Generics) localInstance(t *types.Named, params []*types.TypeParam, apply func(types.Type) types.Type) *types.Named {
	var outer, inner, ids []string // (ids tell local types of the same name apart)
	for _, param := range params {
		arg := apply(param)
		outer = append(outer, argName(arg))
		ids = append(ids, typeName(arg)+fmt.Sprint(localPositions(arg, nil)))
	}
	var targs []types.Type
	for a := range t.TypeArgs().Types() {
		arg := apply(a)
		targs = append(targs, arg)
		inner = append(inner, argName(arg))
		ids = append(ids, typeName(arg)+fmt.Sprint(localPositions(arg, nil)))
	}
	display := "[" + strings.Join(outer, ",") // (as gc names them: main.T[outer;inner])
	if len(inner) > 0 {
		display += ";" + strings.Join(inner, ",")
	}
	display += "]"
	obj := t.Origin().Obj()
	key := fmt.Sprintf("%p %s", obj, strings.Join(ids, ";"))
	if fresh, ok := localInstances[key]; ok {
		return fresh
	}
	fresh := types.NewNamed(types.NewTypeName(obj.Pos(), obj.Pkg(), obj.Name(), nil), nil, nil)
	localInstances[key] = fresh
	localNames[fresh] = display
	localGen[fresh.Obj()] = localGen[obj]
	localKeys[fresh] = mangle(strings.Join(ids, ";"))
	for _, param := range params {
		localArgs[fresh] = append(localArgs[fresh], apply(param))
	}
	localArgs[fresh] = append(localArgs[fresh], targs...)
	underlying := t.Underlying()
	if len(targs) > 0 {
		inst, err := types.Instantiate(g.ctxt, t.Origin(), targs, false)
		if err != nil {
			panic(err)
		}
		underlying = inst.Underlying()
	}
	fresh.SetUnderlying(apply(underlying))
	return fresh
}

var (
	localInstances = make(map[string]*types.Named) // by local type and type arguments.
	localNames     = make(map[*types.Named]string) // their type arguments, as gc names them.
	localKeys      = make(map[*types.Named]string) // their type arguments, for C names.
	localArgs      = make(map[*types.Named][]types.Type)

	// localGen numbers the types defined in functions of each package, in the order of
	// their declarations, as gc does: it names them Name·N in the type arguments of others.
	localGen = make(map[*types.TypeName]int)
)

// argName returns the name of t as a type argument in the name of a local type, where gc
// names local types Name·N (see localGen).
func argName(t types.Type) string {
	name := typeName(t)
	if named, ok := types.Unalias(t).(*types.Named); ok {
		if gen := localGen[named.Origin().Obj()]; gen > 0 {
			name += fmt.Sprintf("·%d", gen)
		}
	}
	return name
}

// isLocalType reports whether named is a local type, or one of its instances.
func isLocalType(named *types.Named) bool {
	_, ok := localNames[named]
	return ok || isLocal(named.Obj())
}

// declaredWithin reports whether obj is declared in the function (or method) whose type
// parameters m has (as its scope is in theirs).
func declaredWithin(obj types.Object, m map[*types.TypeParam]types.Type) bool {
	for param := range m {
		scope := param.Obj().Parent()
		if scope == nil {
			continue
		}
		for s := obj.Parent(); s != nil; s = s.Parent() {
			if s == scope {
				return true
			}
		}
	}
	return false
}

// mentionsParams reports whether t is made of the type parameters of m.
func mentionsParams(t types.Type, m map[*types.TypeParam]types.Type, seen map[types.Type]bool) bool {
	if seen == nil {
		seen = make(map[types.Type]bool)
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	switch typ := t.(type) {
	case *types.TypeParam:
		_, ok := m[typ]
		return ok
	case *types.Alias:
		return mentionsParams(types.Unalias(typ), m, seen)
	case *types.Named:
		for a := range typ.TypeArgs().Types() {
			if mentionsParams(a, m, seen) {
				return true
			}
		}
		return isLocal(typ.Obj()) && mentionsParams(typ.Origin().Underlying(), m, seen)
	case *types.Pointer:
		return mentionsParams(typ.Elem(), m, seen)
	case *types.Slice:
		return mentionsParams(typ.Elem(), m, seen)
	case *types.Array:
		return mentionsParams(typ.Elem(), m, seen)
	case *types.Chan:
		return mentionsParams(typ.Elem(), m, seen)
	case *types.Map:
		return mentionsParams(typ.Key(), m, seen) || mentionsParams(typ.Elem(), m, seen)
	case *types.Struct:
		for field := range typ.Fields() {
			if mentionsParams(field.Type(), m, seen) {
				return true
			}
		}
	case *types.Tuple:
		for v := range typ.Variables() {
			if mentionsParams(v.Type(), m, seen) {
				return true
			}
		}
	case *types.Signature:
		return mentionsParams(typ.Params(), m, seen) || mentionsParams(typ.Results(), m, seen)
	case *types.Interface:
		for method := range typ.Methods() {
			if mentionsParams(method.Type(), m, seen) {
				return true
			}
		}
	}
	return false
}

// instanceSuffix names the type arguments of an instance.
func (c99 Target) instanceSuffix(args []types.Type) string {
	var names []string
	for _, arg := range args {
		name := typeName(arg) // (by Go type: C types may be the same, and local types by position)
		for _, pos := range localPositions(arg, nil) {
			name += fmt.Sprintf(" %d", pos)
		}
		names = append(names, mangle(name))
	}
	return "__" + strings.Join(names, "__")
}

// typeCName returns the C name of a named type (without its package suffix), including the
// type arguments of instances of generic types.
func (c99 Target) typeCName(named *types.Named) string {
	if key, ok := localKeys[named]; ok { // (a local type of an instance of a generic function)
		return fmt.Sprintf("%s_%d_%s", source.CIdent(named.Obj().Name()), named.Obj().Pos(), key)
	}
	if obj := named.Obj(); obj.Pkg() != nil && obj.Parent() != nil && obj.Parent() != obj.Pkg().Scope() {
		name := fmt.Sprintf("%s_%d", source.CIdent(obj.Name()), obj.Pos()) // a local type, defined in a function.
		if named.TypeArgs().Len() > 0 {
			name += c99.instanceSuffix(slicesOf(named.TypeArgs()))
		}
		return name
	}
	if named.TypeArgs().Len() == 0 {
		return source.CIdent(named.Obj().Name())
	}
	return source.CIdent(named.Obj().Name()) + c99.instanceSuffix(slicesOf(named.TypeArgs()))
}

func slicesOf(list *types.TypeList) []types.Type {
	var ts []types.Type
	for t := range list.Types() {
		ts = append(ts, t)
	}
	return ts
}

// FunctionInstance returns the C name of the function name refers to, which, for a generic
// function, is the instance with the type arguments at that use of name, which is emitted
// if it hasn't been.
func (c99 Target) FunctionInstance(name source.DefinedFunction) (string, error) {
	cname := c99.FunctionName(name)
	id, ok := name.Location.Node.(*ast.Ident)
	if !ok || c99.Closures.generics == nil {
		return cname, nil
	}
	inst, ok := c99.Closures.info.Instances[id]
	if !ok {
		return cname, nil
	}
	fn, ok := name.Unique.(*types.Func)
	if !ok {
		return cname, nil
	}
	decl, ok := c99.Closures.generics.funcs[fn.Origin()]
	if !ok { // of another package.
		var done func()
		c99, done = c99.inPackage(fn.Origin().Pkg())
		defer done()
		if decl, ok = c99.Closures.generics.funcs[fn.Origin()]; !ok {
			return "", name.Errorf("unsupported instance of %s (from another package)", fn.Name())
		}
	}
	var args []types.Type
	for a := range inst.TypeArgs.Types() {
		args = append(args, subst(a)) // within an instance, arguments may be type parameters.
	}
	cname += c99.instanceSuffix(args)
	sig := fn.Origin().Type().(*types.Signature)
	return cname, c99.emitInstance(decl, c99.Closures.generics.substituter(typeParams(sig.TypeParams()), args), cname)
}

// MethodInstance emits (if it hasn't been) the method of an instance of a generic type.
func (c99 Target) MethodInstance(named *types.Named, method string) error {
	if named.TypeArgs().Len() == 0 || c99.Closures.generics == nil {
		return nil
	}
	decl, ok := c99.Closures.generics.methods[named.Origin().Obj()][method]
	if !ok { // promoted, or of another package.
		var done func()
		c99, done = c99.inPackage(named.Origin().Obj().Pkg())
		defer done()
		if decl, ok = c99.Closures.generics.methods[named.Origin().Obj()][method]; !ok {
			return nil
		}
	}
	fn := decl.Name.Unique.(*types.Func)
	sig := fn.Type().(*types.Signature)
	if sig.TypeParams().Len() > 0 {
		return nil // a generic method, instantiated where it's called, see [Target.GenericMethod].
	}
	cname := c99.typeCName(named) + "_" + c99.FunctionName(decl.Name)
	return c99.emitInstance(decl, c99.Closures.generics.substituter(typeParams(sig.RecvTypeParams()), slicesOf(named.TypeArgs())), cname)
}

// GenericMethod returns the C name of the instance of the generic method fn (a method with
// type parameters of its own) of recv (a named type), with the type arguments of its call
// at id, emitting it if it hasn't been.
func (c99 Target) GenericMethod(recv *types.Named, fn *types.Func, id *ast.Ident) (string, error) {
	inst, ok := c99.Closures.info.Instances[id]
	if !ok {
		return "", fmt.Errorf("unsupported generic method %s", fn.Name())
	}
	var margs []types.Type
	for a := range inst.TypeArgs.Types() {
		margs = append(margs, subst(a))
	}
	cname := c99.methodCName(recv, fn.Name()) + c99.instanceSuffix(margs)
	c99, done := c99.inPackage(fn.Origin().Pkg())
	defer done()
	decl, ok := c99.Closures.generics.funcs[fn.Origin()]
	if !ok {
		if decl, ok = c99.Closures.generics.methods[recv.Origin().Obj()][fn.Name()]; !ok {
			return "", fmt.Errorf("unsupported generic method %s", fn.Name())
		}
	}
	sig := decl.Name.Unique.(*types.Func).Type().(*types.Signature)
	params := append(typeParams(sig.RecvTypeParams()), typeParams(sig.TypeParams())...)
	args := append(slicesOf(recv.TypeArgs()), margs...)
	return cname, c99.emitInstance(decl, c99.Closures.generics.substituter(params, args), cname)
}

func typeParams(list *types.TypeParamList) []*types.TypeParam {
	var params []*types.TypeParam
	for p := range list.TypeParams() {
		params = append(params, p)
	}
	return params
}

// compiledPackages are the closures (and generics) of the packages compiled so far, by import path,
// for the instances of their generic functions and methods in the packages that import them.
var compiledPackages = make(map[string]*Closures)

// inPackage returns c99, switched to compiling the code of pkg (the instance of one of its
// generic functions or methods, in a file of another package, which includes its header),
// and a function to call after.
func (c99 Target) inPackage(pkg *types.Package) (Target, func()) {
	other, ok := compiledPackages[pkg.Path()]
	if pkg == nil || pkg.Path() == c99.CurrentPath || !ok {
		return c99, func() {}
	}
	c99.Requires("#include "+pkg.Path(), c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "\n#include <go/%s.h>\n", pkg.Path())
		return nil
	})
	closures := *other
	closures.count = c99.Closures.count // for names that are unique in the file.
	outer := c99.Closures
	c99.Closures = &closures
	c99.CurrentPackage, c99.CurrentPath, c99.CurrentName = source.PackageIdent(pkg), pkg.Path(), pkg.Name()
	return c99, func() { outer.count = closures.count }
}

func (c99 Target) emitInstance(decl source.FunctionDefinition, substitute func(types.Type) types.Type, cname string) error {
	return c99.Requires(cname, c99.Prelude, func(w io.Writer) error {
		cc := c99
		cc.Writer = w
		cc.Tabs = 0
		cc.Instance = cname
		cc.Order, cc.Deferred, cc.BreakLabel, cc.Header, cc.Environment = nil, nil, "", false, nil
		previous := source.Substitute
		source.Substitute = substitute
		defer func() { source.Substitute = previous }()
		return cc.FunctionDefinition(decl)
	})
}

// instanceType defines (if it hasn't been) the C type of an instance of a generic type.
func (c99 Target) instanceType(named *types.Named, cname string) {
	c99.defineType(cname, []types.Type{named.Underlying()}, func(w io.Writer) {
		fmt.Fprintf(w, "\n#ifndef %[1]s_defined\n#define %[1]s_defined\ntypedef %[2]s %[1]s;\n#endif\n", cname, c99.TypeOf(named.Underlying()))
	})
}

// instanceDescriptor defines (if it hasn't been) the type descriptor of an instance of a
// generic type, static in each file.
func (c99 Target) instanceDescriptor(named *types.Named) string {
	symbol := "go_rtype_" + identifier.ReplaceAllString(c99.typeCName(named), "_")
	c99.staticDescriptor(symbol, typeName(named), named)
	return "&" + symbol
}
