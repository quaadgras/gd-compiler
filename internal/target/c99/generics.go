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
func (g *Generics) substituter(params *types.TypeParamList, args []types.Type) func(types.Type) types.Type {
	m := make(map[*types.TypeParam]types.Type, len(args))
	for i := range params.Len() {
		m[params.At(i)] = args[i]
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

// instanceSuffix names the type arguments of an instance.
func (c99 Target) instanceSuffix(args []types.Type) string {
	var names []string
	for _, arg := range args {
		names = append(names, identifier.ReplaceAllString(c99.TypeOf(arg), "_"))
	}
	return "__" + strings.Join(names, "__")
}

// typeCName returns the C name of a named type (without its package suffix), including the
// type arguments of instances of generic types.
func (c99 Target) typeCName(named *types.Named) string {
	if named.TypeArgs().Len() == 0 {
		return named.Obj().Name()
	}
	return named.Obj().Name() + c99.instanceSuffix(slicesOf(named.TypeArgs()))
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
	if !ok {
		return "", name.Errorf("unsupported instance of %s (from another package)", fn.Name())
	}
	var args []types.Type
	for a := range inst.TypeArgs.Types() {
		args = append(args, subst(a)) // within an instance, arguments may be type parameters.
	}
	cname += c99.instanceSuffix(args)
	sig := fn.Origin().Type().(*types.Signature)
	return cname, c99.emitInstance(decl, c99.Closures.generics.substituter(sig.TypeParams(), args), cname)
}

// MethodInstance emits (if it hasn't been) the method of an instance of a generic type.
func (c99 Target) MethodInstance(named *types.Named, method string) error {
	if named.TypeArgs().Len() == 0 || c99.Closures.generics == nil {
		return nil
	}
	decl, ok := c99.Closures.generics.methods[named.Origin().Obj()][method]
	if !ok {
		return nil // promoted, or from another package.
	}
	fn := decl.Name.Unique.(*types.Func)
	sig := fn.Type().(*types.Signature)
	cname := c99.typeCName(named) + "_" + c99.FunctionName(decl.Name)
	return c99.emitInstance(decl, c99.Closures.generics.substituter(sig.RecvTypeParams(), slicesOf(named.TypeArgs())), cname)
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
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		name := types.TypeString(named, func(pkg *types.Package) string { return pkg.Name() })
		fmt.Fprintf(w, "static const go_type %s = {.name=%s, .kind=go_kind_%s};\n", symbol, cString(name), kindOf(named.Underlying()))
		return nil
	})
	return "&" + symbol
}
