package c99

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/quaadgras/gd-compiler/internal/escape"
	"github.com/quaadgras/gd-compiler/internal/parser"
)

var (
	//go:embed library library/.clangd
	library embed.FS
)

func Build(dir string, test bool) error {
	packages, err := parser.Load(dir, test)
	if err != nil {
		return err
	}
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

	for _, pkg := range packages {
		pkg = escape.Analysis(pkg)

		if err := os.MkdirAll("./.c/go/"+pkg.Name, 0755); err != nil {
			return err
		}

		public, err := os.Create("./.c/go/" + pkg.Name + ".h")
		if err != nil {
			return err
		}
		fmt.Fprintf(public, "#ifndef GO_%s_H\n", pkg.Name)
		fmt.Fprintf(public, "#define GO_%s_H\n", pkg.Name)
		fmt.Fprintln(public, "#include <go.h>")
		fmt.Fprintln(public, "#include <go/"+pkg.Name+"/private.h>")
		fmt.Fprintf(public, "void init_go_%s_package();\n", pkg.Name)

		private, err := os.Create("./.c/go/" + pkg.Name + "/private.h")
		if err != nil {
			return err
		}
		fmt.Fprintf(private, "#ifndef GO_%s_PRIVATE_H\n", pkg.Name)
		fmt.Fprintf(private, "#define GO_%s_PRIVATE_H\n", pkg.Name)
		fmt.Fprintln(private, "#include <go.h>")
		// The private header has all of the types of the package, then the declarations
		// of its variables and functions, which may use any of the types.
		var typeDefs, declarations bytes.Buffer

		init, err := os.Create("./.c/go/" + pkg.Name + "/init.c")
		if err != nil {
			return err
		}
		fmt.Fprintln(init, "#include <go.h>")
		fmt.Fprintln(init, "#include <go/"+pkg.Name+".h>")
		fmt.Fprintln(init, "#include <go/"+pkg.Name+"/private.h>")

		inits := &Initializers{Symbols: make(map[string]struct{})}
		var syntax []*ast.File
		for _, file := range pkg.Files {
			syntax = append(syntax, file.Location.Node.(*ast.File))
		}
		closures := NewClosures(&pkg.Info, syntax)
		closures.generics = NewGenerics(pkg.Files)
		for _, file := range pkg.Files {
			out, err := os.Create("./.c/go/" + pkg.Name + "/" + filepath.Base(file.FileSet.File(file.Open).Name()) + ".c")
			if err != nil {
				return err
			}
			var cc Target
			cc.CurrentPackage = pkg.Name
			cc.Prelude = out
			cc.Writer = new(bytes.Buffer)
			cc.Private = &typeDefs
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
		fmt.Fprintf(init, "\nvoid init_go_%s_package() {", pkg.Name)
		if err := inits.WriteTo(init, pkg.InitOrder); err != nil {
			return err
		}
		fmt.Fprintln(init, "\n}")
		if err := init.Close(); err != nil {
			return err
		}
		fmt.Fprintf(public, "\n#endif // GO_%s_H\n", pkg.Name)
		if err := public.Close(); err != nil {
			return err
		}
		if _, err := private.Write(typeDefs.Bytes()); err != nil {
			return err
		}
		fmt.Fprintln(private)
		if _, err := private.Write(declarations.Bytes()); err != nil {
			return err
		}
		fmt.Fprintf(private, "\n#endif // GO_%s_PRIVATE_H\n", pkg.Name)
		if err := private.Close(); err != nil {
			return err
		}
	}
	return nil
}
