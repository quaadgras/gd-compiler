package c99

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/escape"
	"github.com/quaadgras/gd-compiler/internal/parser"
	"github.com/quaadgras/gd-compiler/internal/source"
)

var (
	//go:embed library library/.clangd library/hooks
	library embed.FS
)

func Build(dir string, test bool) error {
	packages, err := parser.Load(dir, test)
	if err != nil {
		return err
	}
	clear(compiledPackages)
	if err := os.RemoveAll("./.c"); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(".", ".c"), 0755); err != nil {
		return err
	}
	stdlib, err := fs.Sub(library, "library")
	if err != nil {
		return err
	}
	if err := os.CopyFS("./.c", stdlib); err != nil {
		return err
	}
	if err := os.RemoveAll("./.c/hooks"); err != nil { // copied with their packages.
		return err
	}

	// The packages to compile: those that the package in dir imports (transitively), other
	// than native packages (implemented in C11, see library/go), and what only they import.
	byPath := make(map[string]source.Package)
	for _, pkg := range packages {
		byPath[pkg.Path] = pkg
	}
	compile := make(map[string]bool)
	var visit func(path string)
	native := make(map[string]bool)
	visit = func(path string) {
		if compile[path] || native[path] {
			return
		}
		if IsNative(path) { // with the packages that its implementation uses.
			native[path] = true
			for _, imported := range nativeImports(path) {
				visit(imported)
			}
			return
		}
		compile[path] = true
		for _, imported := range byPath[path].Imports {
			visit(imported)
		}
	}
	if len(packages) > 0 {
		visit(packages[len(packages)-1].Path)
	}
	for path := range native { // the implementation of native packages (their header is in library/go).
		if hooks, err := fs.ReadFile(library, "library/hooks/"+path+".c"); err == nil {
			if err := os.MkdirAll("./.c/go/"+path, 0755); err != nil {
				return err
			}
			if err := os.WriteFile("./.c/go/"+path+"/native.c", hooks, 0644); err != nil {
				return err
			}
		}
	}
	for _, pkg := range packages {
		if !compile[pkg.Path] {
			continue
		}
		if err := compilePackage(escape.Analysis(pkg), compile, byPath); err != nil {
			return err
		}
	}
	return nil
}

// hasNativeInit reports whether the package with the path is native, and implemented by a
// file (library/hooks), which defines its init function.
func hasNativeInit(path string) bool {
	_, err := fs.Stat(library, "library/hooks/"+path+".c")
	return err == nil && IsNative(path)
}

// nativeImports returns the import paths of the packages that the implementation of the
// native package with the path uses, from the "// gd:import path" lines of its header.
func nativeImports(path string) []string {
	header, err := fs.ReadFile(library, "library/go/"+path+".h")
	if err != nil {
		return nil
	}
	var imports []string
	for _, line := range strings.Split(string(header), "\n") {
		if imported, ok := strings.CutPrefix(line, "// gd:import "); ok {
			imports = append(imports, strings.TrimSpace(imported))
		}
	}
	return imports
}

// IsNative reports whether the package with the import path is implemented in C11, by a
// header in library/go.
func IsNative(path string) bool {
	_, err := fs.Stat(library, "library/go/"+path+".h")
	return err == nil
}

func compilePackage(pkg source.Package, compiled map[string]bool, byPath map[string]source.Package) error {
	dir := "./.c/go/" + pkg.Path
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Hooks implement the functions of the package that have no body (as they are
	// implemented by the Go runtime, or in assembly) in C11.
	if hooks, err := fs.ReadFile(library, "library/hooks/"+pkg.Path+".c"); err == nil {
		if err := os.WriteFile(dir+"/hooks.c", hooks, 0644); err != nil {
			return err
		}
	}
	public, err := os.Create(dir + ".h")
	if err != nil {
		return err
	}
	fmt.Fprintf(public, "#ifndef GO_%s_H\n", pkg.Ident)
	fmt.Fprintf(public, "#define GO_%s_H\n", pkg.Ident)
	fmt.Fprintln(public, "#include <go.h>")
	fmt.Fprintln(public, "#include <go/"+pkg.Path+"/private.h>")
	fmt.Fprintf(public, "void init_go_%s_package(void);\n", pkg.Ident)

	private, err := os.Create(dir + "/private.h")
	if err != nil {
		return err
	}
	fmt.Fprintf(private, "#ifndef GO_%s_PRIVATE_H\n", pkg.Ident)
	fmt.Fprintf(private, "#define GO_%s_PRIVATE_H\n", pkg.Ident)
	fmt.Fprintln(private, "#include <go.h>")
	for _, imported := range pkg.Imports { // for the types and functions of imports.
		fmt.Fprintf(private, "#include <go/%s.h>\n", imported)
	}
	// The private header has all of the types of the package, then the declarations
	// of its variables and functions, which may use any of the types.
	var typeDefs, declarations bytes.Buffer

	init, err := os.Create(dir + "/init.c")
	if err != nil {
		return err
	}
	fmt.Fprintln(init, "#include <go.h>")
	fmt.Fprintln(init, "#include <go/"+pkg.Path+".h>")
	fmt.Fprintln(init, "#include <go/"+pkg.Path+"/private.h>")

	inits := &Initializers{Symbols: make(map[string]struct{})}
	var syntax []*ast.File
	for _, file := range pkg.Files {
		syntax = append(syntax, file.Location.Node.(*ast.File))
	}
	typeDefinitions := NewTypeDefs()
	closures := NewClosures(&pkg.Info, syntax)
	closures.generics = NewGenerics(pkg.Files)
	compiledPackages[pkg.Path] = closures
	for _, file := range pkg.Files {
		out, err := os.Create(dir + "/" + filepath.Base(file.FileSet.File(file.Open).Name()) + ".c")
		if err != nil {
			return err
		}
		var cc Target
		cc.CurrentPackage = pkg.Ident
		cc.CurrentPath = pkg.Path
		cc.CurrentName = pkg.Name
		cc.Prelude = out
		cc.Writer = new(bytes.Buffer)
		cc.Private = &typeDefs
		cc.TypeDefs = typeDefinitions
		cc.Declarations = &declarations
		cc.Generic = cc.Prelude
		cc.Symbols = make(map[string]struct{})
		cc.Initializers = inits
		cc.Closures = closures
		if err := cc.File(file); err != nil {
			return err
		}
		fmt.Fprintln(out)
		if _, err := out.Write(cc.Writer.(*bytes.Buffer).Bytes()); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}

	if _, err := init.Write(inits.Prelude.Bytes()); err != nil {
		return err
	}
	// A package is initialized once, after the packages it imports.
	fmt.Fprintf(init, "\nvoid init_go_%s_package(void) {\n\tstatic go_tf done = false;\n\tif (done) return;\n\tdone = true;", pkg.Ident)
	for _, imported := range pkg.Imports {
		if compiled[imported] || hasNativeInit(imported) {
			fmt.Fprintf(init, "\n\tinit_go_%s_package();", byPath[imported].Ident)
		}
	}
	if err := inits.WriteTo(init, pkg.InitOrder); err != nil {
		return err
	}
	fmt.Fprintln(init, "\n}")
	if err := init.Close(); err != nil {
		return err
	}
	fmt.Fprintf(public, "\n#endif // GO_%s_H\n", pkg.Ident)
	if err := public.Close(); err != nil {
		return err
	}
	if err := typeDefinitions.Emit(&typeDefs); err != nil {
		return err
	}
	if _, err := private.Write(typeDefs.Bytes()); err != nil {
		return err
	}
	fmt.Fprintln(private)
	if _, err := private.Write(declarations.Bytes()); err != nil {
		return err
	}
	fmt.Fprintf(private, "\n#endif // GO_%s_PRIVATE_H\n", pkg.Ident)
	return private.Close()
}
