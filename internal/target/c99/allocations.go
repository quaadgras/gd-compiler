package c99

import "github.com/quaadgras/gd-compiler/internal/source"

// StackAllocated reports whether ident is an ordinary C variable, rather than a pointer to
// a heap allocated box (for variables captured by closures, see [Closures]).
func (c99 Target) StackAllocated(ident source.DefinedVariable) bool {
	return !c99.Closures.Captured(ident.Unique)
}
