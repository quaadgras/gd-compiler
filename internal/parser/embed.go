package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

// embeds returns the files of the //go:embed variables of pkg (in dir), as gc and the go
// command find them (their errors are reported by go list, and by [check]).
func embeds(pkg *packages.Package, dir string) map[*types.Var][]string {
	var result map[*types.Var][]string
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				doc := vs.Doc
				if doc == nil && !gen.Lparen.IsValid() {
					doc = gen.Doc
				}
				patterns := embedPatterns(doc)
				if len(patterns) == 0 || len(vs.Names) != 1 {
					continue
				}
				v, ok := pkg.TypesInfo.Defs[vs.Names[0]].(*types.Var)
				if !ok {
					continue
				}
				files, err := resolveEmbed(dir, patterns)
				if err != nil {
					continue
				}
				if result == nil {
					result = make(map[*types.Var][]string)
				}
				result[v] = files
			}
		}
	}
	return result
}

// embedPatterns returns the patterns of the //go:embed directives of doc.
func embedPatterns(doc *ast.CommentGroup) []string {
	if doc == nil {
		return nil
	}
	var patterns []string
	for _, c := range doc.List {
		text, ok := strings.CutPrefix(c.Text, "//go:embed")
		if !ok || (text != "" && !unicode.IsSpace(rune(text[0]))) {
			continue
		}
		args, err := parseGoEmbed(text)
		if err != nil {
			continue
		}
		patterns = append(patterns, args...)
	}
	return patterns
}

// parseGoEmbed parses the patterns of a //go:embed directive (as cmd/compile does).
func parseGoEmbed(args string) ([]string, error) {
	var list []string
	for args = strings.TrimSpace(args); args != ""; args = strings.TrimSpace(args) {
		var path string
	Switch:
		switch args[0] {
		default:
			i := len(args)
			for j, c := range args {
				if unicode.IsSpace(c) {
					i = j
					break
				}
			}
			path = args[:i]
			args = args[i:]
		case '`':
			i := strings.Index(args[1:], "`")
			if i < 0 {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
			path = args[1 : 1+i]
			args = args[1+i+1:]
		case '"':
			i := 1
			for ; i < len(args); i++ {
				if args[i] == '\\' {
					i++
					continue
				}
				if args[i] == '"' {
					q, err := strconv.Unquote(args[:i+1])
					if err != nil {
						return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args[:i+1])
					}
					path = q
					args = args[i+1:]
					break Switch
				}
			}
			if i >= len(args) {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
		}
		if args != "" {
			r, _ := utf8.DecodeRuneInString(args)
			if !unicode.IsSpace(r) {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
		}
		list = append(list, path)
	}
	return list, nil
}

// resolveEmbed returns the files that the patterns match in dir (directories match the
// files in them, but those starting with . or _, unless the pattern starts with all:),
// as cmd/go does, sorted as package embed sorts them.
func resolveEmbed(dir string, patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	for _, pattern := range patterns {
		glob, all := strings.CutPrefix(pattern, "all:")
		matches, err := filepath.Glob(filepath.Join(dir, filepath.FromSlash(glob)))
		if err != nil {
			return nil, err
		}
		count := 0
		for _, match := range matches {
			info, err := os.Lstat(match)
			if err != nil {
				return nil, err
			}
			rel, _ := filepath.Rel(dir, match)
			switch {
			case info.Mode().IsRegular():
				if !seen[rel] {
					seen[rel] = true
					files = append(files, filepath.ToSlash(rel))
				}
				count++
			case info.IsDir():
				err := filepath.WalkDir(match, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					name := d.Name()
					if p != match && !all && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if d.IsDir() {
						if p != match {
							if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
								return filepath.SkipDir // (another module)
							}
						}
						return nil
					}
					if !d.Type().IsRegular() {
						return nil
					}
					rel, _ := filepath.Rel(dir, p)
					if !seen[rel] {
						seen[rel] = true
						files = append(files, filepath.ToSlash(rel))
					}
					count++
					return nil
				})
				if err != nil {
					return nil, err
				}
			}
		}
		if count == 0 {
			return nil, fmt.Errorf("pattern %s: no matching files found", pattern)
		}
	}
	sort.Slice(files, func(i, j int) bool { return embedLess(files[i], files[j]) })
	return files, nil
}

// embedLess orders names as package embed does: by directory, then name.
func embedLess(x, y string) bool {
	xdir, xelem, _ := embedSplit(x)
	ydir, yelem, _ := embedSplit(y)
	return xdir < ydir || xdir == ydir && xelem < yelem
}

// embedSplit splits name into its directory and element (as package embed's split).
func embedSplit(name string) (dir, elem string, isDir bool) {
	name, isDir = strings.CutSuffix(name, "/")
	i := strings.LastIndexByte(name, '/')
	if i < 0 {
		return ".", name, isDir
	}
	return name[:i], name[i+1:], isDir
}
