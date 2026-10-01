package c99

import (
	"fmt"
	"go/types"
	"io"
	"strings"
)

// equality returns a C expression that reports whether the values x and y (C expressions)
// of the comparable type t are equal. Values of composite types are compared by functions,
// so that x and y are evaluated once.
func (c99 Target) equality(x, y string, t types.Type) string {
	switch typ := t.Underlying().(type) {
	case *types.Interface:
		return fmt.Sprintf("go_vv_eq(%s, %s)", asEmptyInterface(x, t), asEmptyInterface(y, t))
	case *types.Pointer:
		return fmt.Sprintf("((%s).ptr == (%s).ptr)", x, y)
	case *types.Basic:
		switch {
		case typ.Info()&types.IsString != 0:
			return fmt.Sprintf("go_string_eq(%s, %s)", x, y)
		case typ.Kind() == types.UnsafePointer:
			return fmt.Sprintf("((%s).ptr == (%s).ptr)", x, y)
		case typ.Info()&types.IsComplex != 0:
			return fmt.Sprintf("%s(%s, %s)", c99.equalFunc(t), x, y)
		}
	case *types.Struct, *types.Array:
		return fmt.Sprintf("%s(%s, %s)", c99.equalFunc(t), x, y)
	}
	return fmt.Sprintf("((%s) == (%s))", x, y)
}

// equalityOf is [Target.equality] for addressable values x and y, which are compared by
// pointer when they are structs or arrays (as values of large arrays are best not copied).
func (c99 Target) equalityOf(x, y string, t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Struct, *types.Array:
		return fmt.Sprintf("%s(&(%s), &(%s))", c99.equalPtrFunc(t), x, y)
	}
	return c99.equality(x, y, t)
}

// equalFunc returns the name of a function (static in each file) that reports whether two
// values of the struct, array or complex type t are equal.
func (c99 Target) equalFunc(t types.Type) string {
	ctype := c99.TypeOf(t)
	symbol := "go_equal_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		fmt.Fprintf(w, "static inline go_tf %s(%s a, %[2]s b) { return %s(&a, &b); }\n", symbol, ctype, c99.equalPtrFunc(t))
		return nil
	})
	return symbol
}

// equalPtrFunc is [Target.equalFunc] for pointers to the values (as values of large arrays
// are best not copied), as void pointers, for the type descriptors of t.
func (c99 Target) equalPtrFunc(t types.Type) string {
	ctype := c99.TypeOf(t)
	symbol := "go_equal_ptr_" + identifier.ReplaceAllString(ctype, "_")
	c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
		var terms []string
		switch typ := t.Underlying().(type) {
		case *types.Struct: // (as gc orders them: comparisons that may panic in order, and between them cheap ones before calls)
			var calls []string
			flush := func() { terms, calls = append(terms, calls...), nil }
			for i := range typ.NumFields() {
				field := typ.Field(i)
				if field.Name() == "_" {
					continue // blank fields are not compared.
				}
				name := fieldName(field, i)
				x, y := "x->"+name, "y->"+name
				switch {
				case canPanicEqual(field.Type()):
					flush()
					terms = append(terms, c99.equalityOf(x, y, field.Type()))
				case isString(field.Type()):
					terms = append(terms, fmt.Sprintf("go_string_len(%s) == go_string_len(%s)", x, y))
					calls = append(calls, c99.equalityOf(x, y, field.Type()))
				default:
					if _, isBasic := field.Type().Underlying().(*types.Basic); isBasic {
						terms = append(terms, c99.equalityOf(x, y, field.Type()))
					} else {
						calls = append(calls, c99.equalityOf(x, y, field.Type()))
					}
				}
			}
			flush()
		case *types.Array:
			if typ.Len() > 0 && !zeroSize(t) { // (values of no size are all equal)
				fmt.Fprintf(w, "static go_tf %s(const void* a, const void* b) { const %s *x = a, *y = b; for (go_ii i = 0; i < %d; i++) if (!%s) return false; return true; }\n",
					symbol, ctype, typ.Len(), c99.equalityOf("x->a[i]", "y->a[i]", typ.Elem()))
				return nil
			}
		case *types.Basic: // complex
			terms = append(terms, "x->f1 == y->f1", "x->f2 == y->f2")
		}
		if len(terms) == 0 {
			terms = append(terms, "true")
		}
		fmt.Fprintf(w, "static go_tf %s(const void* a, const void* b) { const %s *x = a, *y = b; (void)x; (void)y; return %s; }\n",
			symbol, ctype, strings.Join(terms, " && "))
		return nil
	})
	return symbol
}

// equalField returns the fields of a type descriptor for the equality of values of t, when
// they are compared as the dynamic values of interfaces: a function for structs and arrays
// (the runtime compares values of other types by their kind).
func (c99 Target) equalField(t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Struct, *types.Array:
	default:
		return ""
	}
	if !types.Comparable(t) {
		return ""
	}
	fields := ", .equal=" + c99.equalPtrFunc(t)
	if hash, _, err := c99.MapFuncsOf(t); err == nil {
		fields += ", .hash=" + hash
	}
	return fields
}

// canPanicEqual reports whether comparing values of t may panic (when they hold interfaces).
func canPanicEqual(t types.Type) bool {
	switch typ := t.Underlying().(type) {
	case *types.Interface:
		return true
	case *types.Array:
		return canPanicEqual(typ.Elem())
	case *types.Struct:
		for field := range typ.Fields() {
			if canPanicEqual(field.Type()) {
				return true
			}
		}
	}
	return false
}
