package c99

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/quaadgras/go-compiler/internal/escape"
	"github.com/quaadgras/go-compiler/internal/parser"
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

		init, err := os.Create("./.c/go/" + pkg.Name + "/init.c")
		if err != nil {
			return err
		}
		fmt.Fprintln(init, "#include <go.h>")
		fmt.Fprintln(init, "#include <go/"+pkg.Name+".h>")
		fmt.Fprintln(init, "#include <go/"+pkg.Name+"/private.h>")
		fmt.Fprintf(init, "\nvoid init_go_%s_package() {", pkg.Name)

		for _, file := range pkg.Files {
			out, err := os.Create("./.c/go/" + pkg.Name + "/" + filepath.Base(file.FileSet.File(file.Open).Name()) + ".c")
			if err != nil {
				return err
			}
			var cc Target
			cc.CurrentPackage = pkg.Name
			cc.Prelude = out
			cc.Writer = new(bytes.Buffer)
			cc.Private = private
			cc.Exports = public
			cc.Generic = cc.Prelude
			cc.Init = init
			cc.Symbols = make(map[string]struct{})
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

		fmt.Fprintln(init, "\n}")
		if err := init.Close(); err != nil {
			return err
		}
		fmt.Fprintf(public, "\n#endif // GO_%s_H\n", pkg.Name)
		if err := public.Close(); err != nil {
			return err
		}
		fmt.Fprintf(private, "\n#endif // GO_%s_PRIVATE_H\n", pkg.Name)
		if err := private.Close(); err != nil {
			return err
		}
	}
	return nil
}
