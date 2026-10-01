package c99

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"go/build"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	overlay, err := standardOverlay()
	if err != nil {
		return err
	}
	packages, err := parser.Load(dir, test, overlay)
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
	for _, dir := range []string{"./.c/hooks", "./.c/overlay"} { // (hooks are copied with their packages)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
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
	cache := cacheDir()
	if cache != "" {
		trimCache(cache)
	}
	keys := make(map[string]string) // of the packages' C code, see packageKey.
	for _, pkg := range packages {
		if !compile[pkg.Path] {
			continue
		}
		pkg = escape.Analysis(pkg)
		key := ""
		if cache != "" && pkg.Path != packages[len(packages)-1].Path { // (not the package being built)
			key = packageKey(pkg, keys)
			keys[pkg.Path] = key
		}
		if key != "" && restorePackage(cache, key, pkg.Path) {
			registerPackage(pkg) // (for the instances of its generics, in the packages that import it)
			continue
		}
		if err := compilePackage(pkg, compile, byPath); err != nil {
			return err
		}
		if key != "" {
			if err := storePackage(cache, key, pkg.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

// standardOverlay returns the files that replace those of standard library packages (by
// path), as their Go source is coupled to the Go runtime: library/overlay has the Go
// source of the packages that replace them, whose files replace those of the originals,
// which are otherwise ignored (unless the folder has a file named partial, then only the
// files of the same names are replaced).
func standardOverlay() (map[string][]byte, error) {
	overlay := make(map[string][]byte)
	err := fs.WalkDir(library, "library/overlay", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == "library/overlay" {
			return err
		}
		if matches, _ := fs.Glob(library, path+"/*.go"); len(matches) == 0 {
			return nil // (a folder of packages)
		}
		importPath := strings.TrimPrefix(path, "library/overlay/")
		pkg, err := build.Default.Import(importPath, "", 0)
		if err != nil {
			return err
		}
		_, err = fs.Stat(library, path+"/partial")
		partial := err == nil
		if !partial {
			for _, name := range pkg.GoFiles {
				overlay[filepath.Join(pkg.Dir, name)] = []byte("//go:build ignore\n\npackage " + pkg.Name + "\n")
			}
		}
		entries, err := fs.ReadDir(library, path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			src, err := fs.ReadFile(library, path+"/"+entry.Name())
			if err != nil {
				return err
			}
			// (the files are ignored by the Go toolchain when building gd)
			src = []byte(strings.Replace(string(src), "//go:build ignore\n", "", 1))
			name := "gd_" + entry.Name()
			if partial {
				name = entry.Name() // (replacing the original)
			}
			overlay[filepath.Join(pkg.Dir, name)] = src
		}
		return nil
	})
	return overlay, err
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

// registerPackage returns the closures (and generics) of pkg, as it's compiled, see
// compiledPackages.
func registerPackage(pkg source.Package) *Closures {
	var syntax []*ast.File
	for _, file := range pkg.Files {
		syntax = append(syntax, file.Location.Node.(*ast.File))
	}
	closures := NewClosures(&pkg.Info, syntax)
	closures.generics = NewGenerics(pkg.Files)
	compiledPackages[pkg.Path] = closures
	var locals []*types.TypeName // (numbered as gc numbers them, see localGen)
	for _, obj := range pkg.Info.Defs {
		if name, ok := obj.(*types.TypeName); ok && isLocal(name) && !name.IsAlias() && !isTypeParam(name.Type()) {
			locals = append(locals, name)
		}
	}
	sort.Slice(locals, func(i, j int) bool { return locals[i].Pos() < locals[j].Pos() })
	for i, name := range locals {
		localGen[name] = i + 1
	}
	return closures
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
	closures := registerPackage(pkg)
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

	var embedded string
	if len(pkg.Embeds) > 0 {
		var cc Target
		cc.CurrentPackage, cc.CurrentPath, cc.CurrentName = pkg.Ident, pkg.Path, pkg.Name
		cc.Prelude, cc.Generic = &inits.Prelude, &inits.Prelude
		cc.Writer = new(bytes.Buffer)
		cc.Private, cc.TypeDefs, cc.Declarations = &typeDefs, typeDefinitions, &declarations
		cc.Symbols, cc.Initializers, cc.Closures = inits.Symbols, inits, closures
		if embedded, err = cc.embeds(pkg); err != nil {
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
	fmt.Fprint(init, embedded) // (before the other variables, which may use them)
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

func isTypeParam(t types.Type) bool {
	_, ok := t.(*types.TypeParam)
	return ok
}
