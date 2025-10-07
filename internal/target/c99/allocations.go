package c99

import "github.com/quaadgras/gd-compiler/internal/source"

func (c99 Target) StackAllocated(ident source.DefinedVariable) bool {
	return true
	if ident.Escapes.Block == nil {
		return true
	}
	return ident.IsGlobal || (!ident.Escapes.Function().Possible && !ident.Escapes.Block().Possible && !ident.Escapes.Goroutine().Possible && !ident.Escapes.Containment().Possible)
}
