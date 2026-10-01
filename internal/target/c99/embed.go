package c99

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

// embeds returns the statements (for the package's init function) that initialize its
// //go:embed variables (before any other, as gc's are static data), with the files that
// they embed: strings and byte slices of one file, and embed.FS of the files and their
// directories (sorted as package embed requires), whose contents go into static arrays of
// the file (C limits the length of string literals).
func (c99 Target) embeds(pkg source.Package) (string, error) {
	var vars []*types.Var
	for v := range pkg.Embeds {
		vars = append(vars, v)
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Pos() < vars[j].Pos() })
	var b strings.Builder
	n := 0
	data := func(name string) (string, error) {
		bytes, err := os.ReadFile(filepath.Join(pkg.Dir, filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		if len(bytes) == 0 {
			return "((go_ss){0})", nil
		}
		symbol := fmt.Sprintf("go_embed_%d", n)
		n++
		fmt.Fprintf(c99.Prelude, "static const char %s[%d] = {", symbol, len(bytes))
		for i, c := range bytes {
			if i%32 == 0 {
				fmt.Fprintf(c99.Prelude, "\n\t")
			}
			fmt.Fprintf(c99.Prelude, "%d,", int8(c))
		}
		fmt.Fprintf(c99.Prelude, "\n};\n")
		return fmt.Sprintf("((go_ss){ .ptr = %s, .len = %d })", symbol, len(bytes)), nil
	}
	for _, v := range vars {
		files := pkg.Embeds[v]
		name := source.CIdent(v.Name()) + "_go_" + pkg.Ident + "_package"
		switch typ := v.Type().Underlying().(type) {
		case *types.Basic, *types.Slice: // string, []byte
			if len(files) != 1 {
				return "", fmt.Errorf("%s: invalid go:embed: multiple files for type %s", v.Name(), v.Type())
			}
			value, err := data(files[0])
			if err != nil {
				return "", err
			}
			if _, isSlice := typ.(*types.Slice); isSlice {
				value = "go_bytes_from_string(" + value + ")"
			}
			fmt.Fprintf(&b, "\n\t%s = %s;", name, value)
		case *types.Struct: // embed.FS { files *[]file }
			named, ok := v.Type().(*types.Named)
			if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "embed" || named.Obj().Name() != "FS" {
				return "", fmt.Errorf("%s: go:embed cannot apply to var of type %s", v.Name(), v.Type())
			}
			field := typ.Field(0)
			file := field.Type().(*types.Pointer).Elem().(*types.Slice).Elem()
			fileStruct := file.Underlying().(*types.Struct)
			entries := embedEntries(files)
			fmt.Fprintf(&b, "\n\t{ go_ll go_files = go_slice_make(%s, %d, %d);", c99.TypeOf(file), len(entries), len(entries))
			for i, entry := range entries {
				contents := "((go_ss){0})"
				if !strings.HasSuffix(entry, "/") {
					var err error
					if contents, err = data(entry); err != nil {
						return "", err
					}
				}
				name := fmt.Sprintf("((go_ss){ .ptr = %s, .len = %d })", cString(entry), len(entry))
				fmt.Fprintf(&b, "\n\t\t((%[1]s*)go_files.ptr.ptr)[%[2]d].%[3]s = %[4]s; ((%[1]s*)go_files.ptr.ptr)[%[2]d].%[5]s = %[6]s;",
					c99.TypeOf(file), i, fieldName(fileStruct.Field(0), 0), name, fieldName(fileStruct.Field(1), 1), contents)
			}
			fmt.Fprintf(&b, "\n\t\t%s.%s = go_new(sizeof(go_ll), &go_files); }", name, fieldName(field, 0))
		default:
			return "", fmt.Errorf("%s: go:embed cannot apply to var of type %s", v.Name(), v.Type())
		}
	}
	return b.String(), nil
}

// embedEntries returns the files, with their directories (ending in /), in the order of
// package embed: by directory, then name.
func embedEntries(files []string) []string {
	seen := make(map[string]bool)
	var entries []string
	for _, file := range files {
		for dir := file; ; {
			i := strings.LastIndexByte(strings.TrimSuffix(dir, "/"), '/')
			if i < 0 {
				break
			}
			dir = dir[:i+1]
			if seen[dir] {
				break
			}
			seen[dir] = true
			entries = append(entries, dir)
		}
		if !seen[file] {
			seen[file] = true
			entries = append(entries, file)
		}
	}
	split := func(name string) (string, string) {
		name = strings.TrimSuffix(name, "/")
		i := strings.LastIndexByte(name, '/')
		if i < 0 {
			return ".", name
		}
		return name[:i], name[i+1:]
	}
	sort.Slice(entries, func(i, j int) bool {
		xdir, xelem := split(entries[i])
		ydir, yelem := split(entries[j])
		return xdir < ydir || xdir == ydir && xelem < yelem
	})
	return entries
}
