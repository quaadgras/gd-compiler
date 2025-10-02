package c

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quaadgras/go-compiler/internal/escape"
	"github.com/quaadgras/go-compiler/internal/parser"
)

var (
	//go:embed go.c
	runtime string
	//go:embed go.h
	headers string

	//go:embed map.c
	mapimpl string
	//go:embed map.h
	maphead string

	//go:embed .clangd
	clangd string
)

func Build(dir string, test bool) error {
	packages, err := parser.Load(dir, test)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(".", ".c"), 0755); err != nil {
		return err
	}
	for _, pkg := range packages {
		pkg = escape.Analysis(pkg)
		for _, file := range pkg.Files {
			out, err := os.Create("./.c/" + filepath.Base(file.FileSet.File(file.Open).Name()) + ".c")
			if err != nil {
				return err
			}
			var cc Target
			cc.CurrentPackage = pkg.Name
			cc.Prelude = out
			cc.Writer = new(bytes.Buffer)
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
	}
	if err := os.WriteFile("./.c/go.h", []byte(headers), 0644); err != nil {
		return err
	}
	if err := os.WriteFile("./.c/map.h", []byte(maphead), 0644); err != nil {
		return err
	}
	if err := os.WriteFile("./.c/map.c", []byte(mapimpl), 0644); err != nil {
		return err
	}
	if err := os.WriteFile("./.c/.clangd", []byte(clangd), 0644); err != nil {
		return err
	}
	return os.WriteFile("./.c/go.c", []byte(runtime), 0644)
}
