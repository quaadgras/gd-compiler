package parser

import (
	"go/ast"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

func loadImport(pkg *source.Package, in *ast.ImportSpec) source.Import {
	var out source.Import
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	if in.Name != nil {
		out.Rename = xyz.New(source.ImportedPackage(loadIdentifier(pkg, in.Name)))
	}
	out.Path = loadConstant(pkg, in.Path)
	if in.Comment != nil {
		out.Comment = xyz.New(loadCommentGroup(pkg, in.Comment))
	}
	out.End = locationIn(pkg, in, in.End())
	return out
}
