package c99

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func (c99 Target) DefinedVariable(name source.DefinedVariable) error {
	return c99.definedVariable(false, name)
}

func (c99 Target) definedVariable(decl bool, name source.DefinedVariable) error {
	if name.String == "_" {
		_, err := c99.Write([]byte("_"))
		return err
	}
	if !decl && !c99.StackAllocated(name) {
		fmt.Fprintf(c99, "(*%s)", name.String) // boxed.
		return nil
	}
	_, err := c99.Write([]byte(name.String))
	return err
}

// DefinedFunction writes a package-level function used as a value (calls use the
// function's name directly, see [Target.FunctionName]).
func (c99 Target) DefinedFunction(name source.DefinedFunction) error {
	sig, ok := name.TypeAndValue().Type.(*types.Signature)
	if fn, isFunc := name.Unique.(*types.Func); !ok && isFunc { // pkg.F
		sig, ok = fn.Type().(*types.Signature)
	}
	if id, isIdent := name.Location.Node.(*ast.Ident); isIdent && c99.Closures.info != nil {
		if inst, isInstance := c99.Closures.info.Instances[id]; isInstance {
			sig, ok = subst(inst.Type).(*types.Signature)
		}
	}
	if name.Method || !ok {
		fmt.Fprint(c99, c99.FunctionName(name))
		return nil
	}
	cname, err := c99.FunctionInstance(name)
	if err != nil {
		return err
	}
	value, err := c99.FunctionValue(cname, sig)
	if err != nil {
		return name.Errorf("%w", err)
	}
	fmt.Fprint(c99, value)
	return nil
}

// FunctionName returns the C name of a package-level function (or the method name part of
// a method), which is always qualified by its package, as functions have external linkage,
// so that they can be called from any file of the package.
func (c99 Target) FunctionName(name source.DefinedFunction) string {
	return fmt.Sprintf("%s_go_%s_package", name.String, name.Package)
}

// DefinedConstant writes the value of a constant (so that constants of other files and
// packages need not be declared).
func (c99 Target) DefinedConstant(name source.DefinedConstant) error {
	if tv := name.TypeAndValue(); tv.Value != nil {
		return c99.Constant(tv)
	}
	_, err := c99.Write([]byte(name.String))
	return err
}

func (c99 Target) SpecificationImport(spec source.Import) error {
	path, _ := strconv.Unquote(spec.Path.Value)
	fmt.Fprintf(c99.Prelude, `#include <go/%s.h>`, path)
	fmt.Fprintln(c99.Prelude)
	return nil
}

func (c99 Target) TypeDefinition(spec source.TypeDefinition) error {
	if _, generic := spec.TypeParameters.Get(); generic {
		return nil // instances are defined where they are used, see [Target.TypeOf].
	}

	// Local types (defined in functions) are named after their position, as they may have
	// the same name, and defined like package-level types, so that helpers (at file scope)
	// can use them, with type descriptors that are static in the file.
	obj, _ := spec.Name.Unique.(*types.TypeName)
	name := spec.Name.String + "_go_" + c99.CurrentPackage + "_package"
	out, static := io.Writer(c99), ""
	if !spec.Global {
		if obj == nil {
			return spec.Location.Errorf("unsupported local type")
		}
		if obj.IsAlias() {
			return nil // C uses the aliased type.
		}
		name = c99.typeCName(obj.Type().(*types.Named)) + "_go_" + c99.CurrentPackage + "_package"
		out, static = c99.Generic, "static "
	}
	ctype := c99.TypeOf(spec.Type.TypeAndValue().Type)
	if iface, ok := spec.Type.TypeAndValue().Type.Underlying().(*types.Interface); ok && iface.NumMethods() > 0 {
		ctype = c99.InterfaceTypeOf(spec.Type.TypeAndValue().Type) // the table of methods.
	}
	c99.defineType(name, []types.Type{spec.Type.TypeAndValue().Type}, func(w io.Writer) {
		fmt.Fprintf(w, "\ntypedef %s %s;", ctype, name)
	})
	if spec.Global {
		fmt.Fprintf(c99.Declarations, "\nextern const go_type go_type_%s;", name)
	}
	var fields string
	if rtype, ok := spec.Type.TypeAndValue().Type.(*types.Struct); ok {
		var list []string // (first, as the descriptors of the fields' types may be written to out)
		for i := range rtype.NumFields() {
			field := rtype.Field(i)
			list = append(list, fmt.Sprintf("{.name=%q,.type=%s,.offset=offsetof(%s, %s),.exported=%v,.embedded=%v}",
				field.Name(), c99.ReflectTypeOf(field.Type()), name, fieldName(field, i), field.Exported(), field.Anonymous()))
		}
		if rtype.NumFields() == 0 {
			list = append(list, "{0}") // C has no empty arrays.
		}
		fmt.Fprintf(out, "\n%sconst go_field go_fields_%s[] = {%s};", static, name, strings.Join(list, ", "))
		fields = fmt.Sprintf(", .data={.fields={&go_fields_%s[0], %d}}", name, rtype.NumFields())
	}
	var methods string
	if obj != nil {
		methods = c99.methodTable(obj.Type()) + c99.equalField(obj.Type())
	}
	fmt.Fprintf(out, "\n%sconst go_type go_type_%s = {.name=%q, .kind=go_kind_%s%s%s};\n", static, name,
		c99.CurrentName+"."+spec.Name.String, kindOf(spec.Type.TypeAndValue().Type), fields, methods)
	return nil
}

func kindOf(t types.Type) string {
	switch t := t.Underlying().(type) {
	case *types.Basic:
		if t.Kind() == types.UnsafePointer {
			return "unsafe_pointer"
		}
		return t.Name()
	case *types.Array:
		return "array"
	case *types.Chan:
		return "chan"
	case *types.Slice:
		return "slice"
	case *types.Signature:
		return "func"
	case *types.Interface:
		return "interface"
	case *types.Map:
		return "map"
	case *types.Pointer:
		return "pointer"
	case *types.Struct:
		return "struct"
	}
	panic("unexpected kindOf: " + t.String())
}

func (c99 Target) VariableDefinition(spec source.VariableDefinition) error {
	var name = spec.Name
	var value func() error
	var rtype types.Type
	var ztype string
	vtype, ok := spec.Type.Get()
	assignValue, hasValue := spec.Value.Get()
	if !ok && !hasValue {
		return fmt.Errorf("missing type for value %s", name.String)
	}
	// var a, b = f(): the first variable evaluates the tuple into a temporary (written
	// with the variable's value, as for package-level variables, it's in init).
	var tupleValue source.Expression
	var declareTuple bool
	if result, isTuple := spec.Result.Get(); isTuple && hasValue {
		node := source.LocationOf(assignValue).Node
		temp, ok := c99.Closures.tuples[node]
		if result == 0 || !ok {
			ts := tupleTypes(assignValue)
			if result >= len(ts) {
				return spec.Location.Errorf("unsupported declaration of multiple variables")
			}
			temp = tupleVar{name: fmt.Sprintf("go_var_%d", c99.Closures.count), types: ts}
			c99.Closures.count++
			c99.Closures.tuples[node] = temp
			tupleValue, declareTuple = assignValue, true
		}
		assignValue = c99.element(temp.types[result], fmt.Sprintf("%s.r%d", temp.name, result))
		spec.Value = xyz.New(assignValue)
	}
	if ok {
		rtype = vtype.TypeAndValue().Type
		ztype = c99.TypeOf(vtype.TypeAndValue().Type)
	} else {
		rtype = assignValue.TypeAndValue().Type
		ztype = c99.TypeOf(assignValue.TypeAndValue().Type)
	}
	if !hasValue {
		value = func() error {
			if ztype[0] == '*' {
				fmt.Fprintf(c99, "null")
				return nil
			}
			fmt.Fprintf(c99, "{0}")
			return nil
		}
	} else {
		value = func() error {
			return c99.ExpressionAs(assignValue, rtype)
		}
		_, isInterface := rtype.Underlying().(*types.Interface)
		_, fromInterface := assignValue.TypeAndValue().Type.Underlying().(*types.Interface)
		if isInterface && !fromInterface && !isNil(assignValue) {
			value = func() error {
				return c99.FunctionCall(source.FunctionCall{
					Location:  spec.Location,
					Function:  source.Expressions.Type.As(vtype),
					Arguments: []source.Expression{assignValue},
				})
			}
		}
	}
	if spec.Global {
		// Constant values (and composites of them) are static initializers, others are
		// assigned by the package's init function.
		static, isStatic := "", false
		if hasValue {
			static, isStatic = c99.StaticInitializer(assignValue, rtype)
		}
		if name.String != "_" {
			fmt.Fprintf(c99, "%s ", c99.TypeOf(rtype))
			if err := c99.definedVariable(true, name); err != nil {
				return err
			}
			if isStatic {
				fmt.Fprintf(c99, " = %s", static)
			}
			fmt.Fprintf(c99, ";")
			fmt.Fprintf(c99.Declarations, "extern %s %s;\n", c99.TypeOf(rtype), name.String)
		}
		if !hasValue || isStatic {
			return nil // C zero initializes globals.
		}
		// Initializers are part of the package's init function, in another file.
		c99.Writer = c99.Initializers.For(name.Unique)
		c99.Prelude = &c99.Initializers.Prelude
		c99.Generic = c99.Prelude
		c99.Symbols = c99.Initializers.Symbols
		c99.Tabs = 1
	}
	// the definition, in order (see [Order]), for package-level variables (their init
	// statements are not otherwise statements). The value callbacks above write with c99.
	emit := func(cc Target) error {
		c99 = cc
		if c99.Tabs > 0 {
			fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
		}
		if declareTuple {
			value, ts, err := c99.tupleValue(tupleValue, -1)
			if err != nil {
				return spec.Location.Errorf("%w", err)
			}
			temp := c99.Closures.tuples[source.LocationOf(tupleValue).Node]
			fmt.Fprintf(c99, "%s %s = %s; ", c99.TupleOf(ts), temp.name, value)
		}
		if name.String == "_" {
			fmt.Fprintf(c99, "go_ignore(")
			if err := value(); err != nil {
				return err
			}
			fmt.Fprintf(c99, ")")
		} else {
			if spec.Global {
				if err := c99.definedVariable(true, name); err != nil {
					return err
				}
			} else if !c99.StackAllocated(name) { // boxed, see [Closures].
				fmt.Fprintf(c99, "%s* %s = ", c99.TypeOf(rtype), name.String)
				zero := !hasValue || (xyz.ValueOf(assignValue) == source.Expressions.Composite &&
					len(source.Expressions.Composite.Get(assignValue).Elements) == 0)
				switch {
				case zero:
					fmt.Fprintf(c99, "go_new(sizeof(%s), NULL).ptr", c99.TypeOf(rtype))
				case c99.Header: // a single declaration.
					fmt.Fprintf(c99, "%s(", c99.BoxOf(rtype))
					if err := value(); err != nil {
						return err
					}
					fmt.Fprintf(c99, ")")
				default: // without a helper, so that the type can be local to the function.
					fmt.Fprintf(c99, "go_new(sizeof(%s), NULL).ptr; *%s = ", c99.TypeOf(rtype), name.String)
					if err := value(); err != nil {
						return err
					}
				}
				if c99.Tabs > 0 {
					fmt.Fprintf(c99, ";")
				}
				return nil
			} else {
				fmt.Fprintf(c99, "%s ", c99.TypeOf(rtype))
				if err := c99.definedVariable(true, name); err != nil {
					return err
				}
			}
			fmt.Fprint(c99, " = ")
			if err := value(); err != nil {
				return err
			}
		}
		if c99.Tabs > 0 || spec.Global {
			fmt.Fprintf(c99, ";")
		}
		return nil
	}
	if spec.Global && hasValue {
		return c99.ordered([]source.Expression{assignValue}, nil, false, emit)
	}
	return emit(c99)
}

// StaticInitializer returns a C static initializer for expr (of type t) when it is a
// constant, or a composite literal of constants.
func (c99 Target) StaticInitializer(expr source.Expression, t types.Type) (string, bool) {
	if tv := expr.TypeAndValue(); tv.Value != nil {
		if tv.Value.Kind() == constant.Complex || !isBasic(t) {
			return "", false
		}
		var buf strings.Builder
		cc := c99
		cc.Writer = &buf
		if err := cc.ConstantValue(tv.Value, true); err != nil {
			return "", false
		}
		return buf.String(), true
	}
	if xyz.ValueOf(expr) != source.Expressions.Composite {
		return "", false
	}
	data := source.Expressions.Composite.Get(expr)
	var elems []string
	switch typ := data.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		for _, elem := range data.Elements {
			prefix := ""
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				pair := source.Expressions.KeyValue.Get(elem)
				if pair.Key.TypeAndValue().Value == nil {
					return "", false
				}
				prefix = fmt.Sprintf("[%s]=", pair.Key.TypeAndValue().Value.ExactString())
				elem = pair.Value
			}
			value, ok := c99.StaticInitializer(elem, typ.Elem())
			if !ok {
				return "", false
			}
			elems = append(elems, prefix+value)
		}
		if len(elems) == 0 {
			elems = append(elems, "0")
		}
		return "{{" + strings.Join(elems, ", ") + "}}", true
	case *types.Struct:
		for i, elem := range data.Elements {
			field := typ.Field(i)
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				pair := source.Expressions.KeyValue.Get(elem)
				name := c99.toString(pair.Key)
				for f := range typ.Fields() {
					if f.Name() == name {
						field = f
					}
				}
				elem = pair.Value
			}
			value, ok := c99.StaticInitializer(elem, field.Type())
			if !ok {
				return "", false
			}
			elems = append(elems, "."+fieldName(field, fieldIndex(typ, field))+" = "+value)
		}
		if len(elems) == 0 {
			elems = append(elems, "0")
		}
		return "{" + strings.Join(elems, ", ") + "}", true
	}
	return "", false
}

func isBasic(t types.Type) bool {
	_, ok := t.Underlying().(*types.Basic)
	return ok
}

// Constant writes a constant (of type tv.Type), in an expression. Constants of complex
// types are complex values, whatever the kind of the constant (1 and 1.5 may be complex).
func (c99 Target) Constant(tv types.TypeAndValue) error {
	if basic, ok := tv.Type.Underlying().(*types.Basic); ok && basic.Info()&types.IsComplex != 0 {
		value := constant.ToComplex(tv.Value)
		ctor := "go_complex128"
		if basic.Kind() == types.Complex64 {
			ctor = "go_complex64"
		}
		fmt.Fprintf(c99, "%s(", ctor)
		if err := c99.ConstantValue(constant.ToFloat(constant.Real(value)), false); err != nil {
			return err
		}
		fmt.Fprintf(c99, ", ")
		if err := c99.ConstantValue(constant.ToFloat(constant.Imag(value)), false); err != nil {
			return err
		}
		fmt.Fprintf(c99, ")")
		return nil
	}
	return c99.ConstantValue(tv.Value, false)
}

// ConstantValue writes a constant value computed by the type checker, as a C constant
// expression (so that it is valid in static initializers when global is true).
func (c99 Target) ConstantValue(value constant.Value, global bool) error {
	switch value.Kind() {
	case constant.Bool:
		fmt.Fprintf(c99, "%t", constant.BoolVal(value))
	case constant.String:
		str := constant.StringVal(value)
		if !global {
			fmt.Fprintf(c99, "(go_ss)")
		}
		fmt.Fprintf(c99, "{ .ptr = %s, .len = %d }", cString(str), len(str))
	case constant.Int:
		// C integer literals have the type of the smallest of int, long, long long that fits,
		// so give them an explicit suffix.
		if v, exact := constant.Int64Val(value); exact {
			if v == math.MinInt64 {
				fmt.Fprintf(c99, "(-9223372036854775807LL-1)")
			} else {
				fmt.Fprintf(c99, "%dLL", v)
			}
		} else if v, exact := constant.Uint64Val(value); exact {
			fmt.Fprintf(c99, "%dULL", v)
		} else {
			return fmt.Errorf("constant %v overflows 64 bits", value)
		}
	case constant.Float:
		f, _ := constant.Float64Val(value)
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(s, ".e") {
			s += ".0"
		}
		fmt.Fprintf(c99, "%s", s)
	default:
		return fmt.Errorf("unsupported constant value %v", value)
	}
	return nil
}

// cString quotes s as a C string literal. Unlike %q, hex escapes are ended by splitting
// the literal, as C hex escapes consume every following hex digit.
func cString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	hex := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if hex && strings.IndexByte("0123456789abcdefABCDEF", c) >= 0 {
			b.WriteString(`""`)
		}
		hex = false
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c < 0x20 || c >= 0x7f || c == '?':
			fmt.Fprintf(&b, `\x%02x`, c)
			hex = true
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (c99 Target) ConstantDefinition(def source.ConstantDefinition) error {
	if c99.Tabs > 0 {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	if def.Name.String != "_" {
		fmt.Fprintf(c99, "const %s ", c99.TypeOf(def.TypeAndValue().Type))
		if err := c99.DefinedConstant(def.Name); err != nil {
			return err
		}
		fmt.Fprintf(c99, " = ")
	} else {
		fmt.Fprintf(c99, "go_ignore(")
	}
	// Use the exact value computed by the type checker, as Go constant expressions (such
	// as 1<<63, or iota) don't mean the same thing in C, or may be implied by a previous spec.
	if tv := def.TypeAndValue(); tv.Value != nil && tv.Value.Kind() != constant.Complex {
		if err := c99.ConstantValue(tv.Value, def.Global); err != nil {
			return def.Location.Errorf("%w", err)
		}
	} else if value, ok := def.Value.Get(); ok {
		if err := c99.Expression(value); err != nil {
			return err
		}
	} else {
		return def.Location.Errorf("constant %s has no value", def.Name.String)
	}
	if def.Name.String == "_" {
		fmt.Fprintf(c99, ")")
	}
	if c99.Tabs > 0 || def.Global {
		fmt.Fprintf(c99, ";")
	}
	return nil
}
