package parser

import (
	"fmt"
	"go/version"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/syntax"
	"golang.org/x/tools/go/packages"
)

// pragmas are the directives (//go:...) before a declaration, as gc's parser collects them
// (see cmd/compile/internal/noder): gd checks them as gc does, for //go:embed.
type pragmas struct {
	embeds []syntax.Pos
}

// directives returns the handler of directives for gc's parser.
func directives(report func(error)) func(syntax.Pos, bool, string, syntax.Pragma) syntax.Pragma {
	return func(pos syntax.Pos, blankLine bool, text string, old syntax.Pragma) syntax.Pragma {
		pragma, _ := old.(*pragmas)
		if pragma == nil {
			pragma = new(pragmas)
		}
		if text == "" { // (unused)
			for _, pos := range pragma.embeds {
				report(syntax.Error{Pos: pos, Msg: "misplaced go:embed directive"})
			}
			return nil
		}
		if !blankLine {
			report(syntax.Error{Pos: pos, Msg: "misplaced compiler directive"})
			return pragma
		}
		if text == "go:embed" || strings.HasPrefix(text, "go:embed ") {
			args, err := parseGoEmbed(text[len("go:embed"):])
			if err != nil {
				report(syntax.Error{Pos: pos, Msg: err.Error()})
			}
			if len(args) == 0 {
				report(syntax.Error{Pos: pos, Msg: "usage: //go:embed pattern..."})
				return pragma
			}
			pragma.embeds = append(pragma.embeds, pos)
		}
		return pragma
	}
}

// checkEmbeds reports the //go:embed directives of file that gc would reject.
func checkEmbeds(file *syntax.File, module *packages.Module, report func(error)) {
	importsEmbed := false
	for _, decl := range file.DeclList {
		if imp, ok := decl.(*syntax.ImportDecl); ok && imp.Path != nil && imp.Path.Value == `"embed"` {
			importsEmbed = true
		}
	}
	lang := ""
	if module != nil && module.GoVersion != "" {
		lang = "go" + module.GoVersion
	}
	check := func(decl syntax.Decl, withinFunc bool) {
		var pragma *pragmas
		switch d := decl.(type) {
		case *syntax.VarDecl:
			pragma, _ = d.Pragma.(*pragmas)
			if pragma == nil || len(pragma.embeds) == 0 {
				return
			}
			var msg string
			switch {
			case !importsEmbed:
				msg = `go:embed requires import "embed" (or import _ "embed", if package is not used)`
			case len(d.NameList) > 1:
				msg = "go:embed cannot apply to multiple vars"
			case d.Values != nil:
				msg = "go:embed cannot apply to var with initializer"
			case d.Type == nil:
				msg = "go:embed cannot apply to var without type"
			case withinFunc:
				msg = "go:embed cannot apply to var inside func"
			case lang != "" && version.Compare(lang, "go1.16") < 0:
				msg = fmt.Sprintf("go:embed requires go1.16 or later (-lang was set to %s; check go.mod)", lang)
			}
			if msg != "" {
				report(syntax.Error{Pos: pragma.embeds[0], Msg: msg})
			}
			return
		case *syntax.ConstDecl:
			pragma, _ = d.Pragma.(*pragmas)
		case *syntax.TypeDecl:
			pragma, _ = d.Pragma.(*pragmas)
		case *syntax.FuncDecl:
			pragma, _ = d.Pragma.(*pragmas)
		}
		if pragma != nil {
			for _, pos := range pragma.embeds {
				report(syntax.Error{Pos: pos, Msg: "misplaced go:embed directive"})
			}
		}
	}
	for _, decl := range file.DeclList {
		check(decl, false)
		if fn, ok := decl.(*syntax.FuncDecl); ok && fn.Body != nil {
			syntax.Inspect(fn.Body, func(n syntax.Node) bool {
				if stmt, ok := n.(*syntax.DeclStmt); ok {
					for _, d := range stmt.DeclList {
						check(d, true)
					}
				}
				return true
			})
		}
	}
}
