package c99

import (
	"bytes"
	"fmt"
	"go/types"
	"io"
)

// TypeDefs are the C type definitions of a package (for its private header), which are
// written in dependency order: a type is defined after the types it contains by value
// (Go allows types to be declared in any order).
type TypeDefs struct {
	items map[string]*typeDef
	order []string // of definition, for determinism.
}

type typeDef struct {
	text string
	deps []string
}

// defineType defines the C type symbol (if it hasn't been), written by define, that
// contains the (Go) types deps by value.
func (c99 Target) defineType(symbol string, deps []types.Type, define func(w io.Writer)) {
	if _, ok := c99.TypeDefs.items[symbol]; ok {
		return
	}
	item := &typeDef{}
	c99.TypeDefs.items[symbol] = item
	c99.TypeDefs.order = append(c99.TypeDefs.order, symbol)
	for _, dep := range deps {
		item.deps = append(item.deps, c99.valueDeps(dep)...)
	}
	var buf bytes.Buffer
	define(&buf)
	item.text = buf.String()
}

// valueDeps returns the C types that a value of type t contains (and so, that must be
// defined before a type that contains it), other than those of fixed C types.
func (c99 Target) valueDeps(t types.Type) []string {
	switch typ := types.Unalias(t).(type) {
	case *types.Named:
		if _, ok := typ.Underlying().(*types.Interface); ok {
			return nil
		}
		return []string{c99.TypeOf(typ)}
	case *types.Array, *types.Struct:
		return []string{c99.TypeOf(typ)}
	}
	return nil // basic types, and pointers, slices, maps, channels, funcs and interfaces.
}

func NewTypeDefs() *TypeDefs {
	return &TypeDefs{items: make(map[string]*typeDef)}
}

// Emit writes the type definitions in dependency order.
func (defs *TypeDefs) Emit(w io.Writer) error {
	done := make(map[string]bool)
	var write func(symbol string) error
	write = func(symbol string) error {
		item, ok := defs.items[symbol]
		if !ok || done[symbol] {
			return nil // defined elsewhere (another package), or already written.
		}
		done[symbol] = true // before its dependencies, as recursive types can't be by value.
		for _, dep := range item.deps {
			if err := write(dep); err != nil {
				return err
			}
		}
		_, err := fmt.Fprint(w, item.text)
		return err
	}
	for _, symbol := range defs.order {
		if err := write(symbol); err != nil {
			return err
		}
	}
	return nil
}
