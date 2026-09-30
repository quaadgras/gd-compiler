package c99

import (
	"fmt"
	"go/constant"
	"go/types"
	"math"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

func (c99 Target) DefinedVariable(name source.DefinedVariable) error {
	return c99.definedVariable(false, name)
}

func (c99 Target) definedVariable(decl bool, name source.DefinedVariable) error {
	var prefix string
	if !decl && !c99.StackAllocated(name) {
		prefix = "*"
	}
	if name.String == "_" {
		_, err := c99.Write([]byte("_"))
		return err
	}
	if !decl && prefix != "" {
		fmt.Fprintf(c99, "%s", prefix)
	}
	_, err := c99.Write([]byte(name.String))
	if err != nil {
		return err
	}
	return err
}

// DefinedFunction writes the name of a package-level function (or method name), which is
// always qualified by its package, as functions have external linkage, so that they can be
// called from any file of the package.
func (c99 Target) DefinedFunction(name source.DefinedFunction) error {
	fmt.Fprintf(c99, "%s_go_%s_package", name.String, name.Package)
	return nil
}

func (c99 Target) DefinedConstant(name source.DefinedConstant) error {
	if name.Shadow > 0 {
		fmt.Fprintf(c99, `@"%s.%d"`, name.String, name.Shadow)
		return nil
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

	header := c99.Private
	suffix := ""
	if spec.Exported && spec.Global {
		header = c99.Exports
	}
	if spec.Exported {
		suffix = "_go_" + c99.CurrentPackage + "_package"
	}
	if !spec.Global {
		header = c99
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	}
	ctype := c99.TypeOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintln(header)
	fmt.Fprintf(header, "typedef %s %s%s;", ctype, spec.Name.String, suffix)
	if spec.Global {
		fmt.Fprintln(header)
		fmt.Fprintf(header, "extern const go_type go_type_%s%s;", spec.Name.String, suffix)
	}

	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(c99, "\nconst go_field go_fields_%s%s[] = {", spec.Name.String, suffix)
		for i := range rtype.NumFields() {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			field := rtype.Field(i)
			fmt.Fprintf(c99, "{.name=%q,.type=%s,.offset=offsetof(%s%s, %s),.exported=%v,.embedded=%v}",
				field.Name(), c99.ReflectTypeOf(field.Type()),
				spec.Name.String, suffix, field.Name(), field.Exported(), field.Anonymous())
		}
		if rtype.NumFields() == 0 {
			fmt.Fprintf(c99, "{0}") // C has no empty arrays.
		}
		fmt.Fprintf(c99, "};")
	default:
	}

	fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
	fmt.Fprintf(c99, "const go_type go_type_%s%s = {", spec.Name.String, suffix)
	fmt.Fprintf(c99, ".name=%q,", spec.Name.String)
	kind := kindOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(c99, ".kind=go_kind_%s", kind)
	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(c99, ", .data={.fields={&go_fields_%s%s[0], %d}}", spec.Name.String, suffix, rtype.NumFields())
	}
	fmt.Fprintf(c99, "}")
	fmt.Fprintf(c99, ";\n")
	return nil
}

func kindOf(t types.Type) string {
	switch t := t.(type) {
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
			return c99.Expression(assignValue)
		}
		_, isInterface := rtype.Underlying().(*types.Interface)
		if isInterface {
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
		fmt.Fprintf(c99, "%s ", c99.TypeOf(rtype))
		if err := c99.definedVariable(true, name); err != nil {
			return err
		}
		fmt.Fprintf(c99, ";")
		if spec.Global {
			c99.Writer = c99.Private
			defer fmt.Fprintln(c99.Private)
		}
		fmt.Fprintf(c99, "extern %s ", c99.TypeOf(rtype))
		if err := c99.definedVariable(true, name); err != nil {
			return err
		}
		fmt.Fprintf(c99, ";")
		c99.Writer = c99.Init
		c99.Tabs = 1
	}
	if c99.Tabs > 0 {
		fmt.Fprintf(c99, "\n%s", strings.Repeat("\t", c99.Tabs))
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
		} else {
			fmt.Fprintf(c99, "%s ", c99.TypeOf(rtype))
			if err := c99.definedVariable(true, name); err != nil {
				return err
			}
		}
		stackAllocated := c99.StackAllocated(name)
		stackAllocated = true
		fmt.Fprint(c99, " = ")
		if !stackAllocated {
			fmt.Fprintf(c99, "new(%s); *", c99.TypeOf(assignValue.TypeAndValue().Type))
			if err := c99.definedVariable(true, name); err != nil {
				return err
			}
			fmt.Fprint(c99, " = ")
		}
		if err := value(); err != nil {
			return err
		}
	}
	if c99.Tabs > 0 || spec.Global {
		fmt.Fprintf(c99, ";")
	}
	return nil
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
