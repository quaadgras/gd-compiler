package c99

import (
	"fmt"
	"go/constant"
	"go/types"
	"io"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// DataComposite writes a composite literal. The type of a literal may be elided (within
// another composite literal), and when that type is a pointer, it's &T{...}.
func (c99 Target) DataComposite(data source.DataComposite) error {
	ctype := data.TypeAndValue().Type
	if ctype == nil {
		return data.Errorf("composite literal missing type")
	}
	if pointer, ok := ctype.Underlying().(*types.Pointer); ok {
		fmt.Fprintf(c99, "go_new(sizeof(%s), &", c99.TypeOf(pointer.Elem()))
		defer fmt.Fprintf(c99, ")")
		ctype = pointer.Elem()
	}
	switch typ := ctype.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(c99, "(%s){{", c99.TypeOf(ctype))
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "0")
		}
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				pair := source.Expressions.KeyValue.Get(elem)
				fmt.Fprintf(c99, "[%s]=", pair.Key.TypeAndValue().Value.ExactString())
				elem = pair.Value
			}
			if err := c99.ExpressionAs(elem, typ.Elem()); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, "}}")
		return nil
	case *types.Slice:
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "go_slice_make(%s, 0, 0)", c99.TypeOf(typ.Elem()))
			return nil
		}
		length, index := 0, 0 // elements may have (constant) indexes, as keys.
		for _, elem := range data.Elements {
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				if key := source.Expressions.KeyValue.Get(elem).Key.TypeAndValue().Value; key != nil {
					v, _ := constant.Int64Val(constant.ToInt(key))
					index = int(v)
				}
			}
			index++
			length = max(length, index)
		}
		if length > 2*len(data.Elements)+64 { // sparse: []T{1 << 30: x}, without a C literal of every element.
			var indexes, values []string
			index := 0
			for _, elem := range data.Elements {
				if xyz.ValueOf(elem) == source.Expressions.KeyValue {
					pair := source.Expressions.KeyValue.Get(elem)
					v, _ := constant.Int64Val(constant.ToInt(pair.Key.TypeAndValue().Value))
					index = int(v)
					elem = pair.Value
				}
				var buf strings.Builder
				cc := c99
				cc.Writer = &buf
				if err := cc.ExpressionAs(elem, typ.Elem()); err != nil {
					return err
				}
				indexes = append(indexes, strconv.Itoa(index))
				values = append(values, buf.String())
				index++
			}
			fmt.Fprintf(c99, "go_slice_sparse(%[1]d, sizeof(%[2]s), %[3]d, (go_ii[]){%[4]s}, (%[2]s[]){%[5]s})", length, c99.TypeOf(typ.Elem()),
				len(indexes), strings.Join(indexes, ", "), strings.Join(values, ", "))
			return nil
		}
		fmt.Fprintf(c99, "go_slice_literal(%d, %s, ", length, c99.TypeOf(typ.Elem()))
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if xyz.ValueOf(elem) == source.Expressions.KeyValue {
				pair := source.Expressions.KeyValue.Get(elem)
				fmt.Fprintf(c99, "[%s]=", pair.Key.TypeAndValue().Value.ExactString())
				elem = pair.Value
			}
			if err := c99.ExpressionAs(elem, typ.Elem()); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, ")")
		return nil
	case *types.Map:
		entry := "go_map_entry_" + identifier.ReplaceAllString(c99.TypeOf(typ.Key())+"_"+c99.TypeOf(typ.Elem()), "_")
		c99.Requires(entry, c99.Generic, func(w io.Writer) error {
			fmt.Fprintf(w, "typedef struct { %s key; %s val; } %s;\n", c99.TypeOf(typ.Key()), c99.TypeOf(typ.Elem()), entry)
			return nil
		})
		var entries strings.Builder
		cc := c99
		cc.Writer = &entries
		fmt.Fprintf(&entries, "(%s[]){", entry)
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(&entries, ", ")
			}
			pair := source.Expressions.KeyValue.Get(elem)
			fmt.Fprintf(&entries, "{ ")
			if err := cc.ExpressionAs(pair.Key, typ.Key()); err != nil {
				return err
			}
			fmt.Fprintf(&entries, ", ")
			if err := cc.ExpressionAs(pair.Value, typ.Elem()); err != nil {
				return err
			}
			fmt.Fprintf(&entries, " }")
		}
		fmt.Fprintf(&entries, "}")
		if err := c99.MakeMap(typ, fmt.Sprint(len(data.Elements)), entries.String(), entry, len(data.Elements)); err != nil {
			return data.Errorf("%w", err)
		}
		return nil
	case *types.Struct:
		fmt.Fprintf(c99, "(%s){", c99.TypeOf(ctype))
		if len(data.Elements) == 0 {
			fmt.Fprintf(c99, "0") // C has no empty initializers before C23.
		}
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			switch xyz.ValueOf(elem) {
			case source.Expressions.KeyValue:
				pair := source.Expressions.KeyValue.Get(elem)
				name := c99.toString(pair.Key)
				path, ftype := promotedField(typ, name) // (or a field promoted from an embedded struct)
				if ftype == nil {
					return data.Errorf("unsupported field %s", name)
				}
				fmt.Fprintf(c99, ".%s=", strings.Join(path, "."))
				if err := c99.ExpressionAs(pair.Value, ftype); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				if field.Name() == "_" { // (blank fields are not set)
					fmt.Fprintf(c99, ".%s = (%s){0}", fieldName(field, i), c99.TypeOf(field.Type()))
					continue
				}
				fmt.Fprintf(c99, ".%s = ", fieldName(field, i))
				if err := c99.ExpressionAs(elem, field.Type()); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(c99, "}")
		return nil
	default:
		return data.Errorf("unexpected composite type: " + typ.String())
	}
}

// promotedField returns the path (of C names) to the field of the struct type typ with the
// C name name, and its type, the shallowest (as Go finds them) through embedded structs.
func promotedField(typ *types.Struct, name string) ([]string, types.Type) {
	type candidate struct {
		st   *types.Struct
		path []string
	}
	level := []candidate{{typ, nil}}
	for len(level) > 0 {
		var next []candidate
		for _, c := range level {
			for i := range c.st.NumFields() {
				f := c.st.Field(i)
				path := append(append([]string{}, c.path...), fieldName(f, i))
				if fieldName(f, i) == name {
					return path, f.Type()
				}
				if st, ok := f.Type().Underlying().(*types.Struct); ok && f.Anonymous() {
					next = append(next, candidate{st, path})
				}
			}
		}
		level = next
	}
	return nil, nil
}
