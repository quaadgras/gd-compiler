package parser

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/syntax"
	"golang.org/x/tools/go/packages"
)

// branchError matches the errors of go/types about labels, and break, continue, goto and
// fallthrough statements, which gc reports with its parser instead (see [check]).
var branchError = regexp.MustCompile(`^(label \S+ (declared and not used|not declared|already declared)|` +
	`goto \S+ jumps (into block|over variable declaration at line \d+)|invalid (break|continue) label \S+|` +
	`break not in for, switch, or select statement|continue not in for statement|` +
	`fallthrough statement out of place|cannot fallthrough .*)$`)

// versionError matches the errors about language versions, see [check].
var versionError = regexp.MustCompile(`requires go[0-9]+\.[0-9]+ or later$`)

// callError matches the errors of go/parser that gc's type checker reports.
var callError = regexp.MustCompile(`^expression in (go|defer) must be function call$`)

// tokenError matches the errors of go/types about tokens, that gc's parser reports.
var tokenError = regexp.MustCompile(`^(malformed constant: .*|invalid import path.*)$`)

// A diagnostic is an error at a position of a file, with the lines that continue it.
type diagnostic struct {
	file      string
	line, col int
	msg       string
	more      []string
	parse     bool
}

func (d diagnostic) String() string {
	s := d.msg
	if d.file != "" && d.col == 0 { // (of //line directives without columns)
		s = fmt.Sprintf("%s:%d: %s", d.file, d.line, d.msg)
	} else if d.file != "" {
		s = fmt.Sprintf("%s:%d:%d: %s", d.file, d.line, d.col, d.msg)
	}
	for _, more := range d.more {
		s += "\n\t" + more
	}
	return s
}

var position = regexp.MustCompile(`^(.*):(\d+):(\d+)$`)

// check returns the errors of the packages (the ones gd compiles, not their dependencies)
// as gc reports them: it parses them with gc's parser (see package syntax), and reports
// its errors only, if there are syntax errors, else those of its checks of branches with
// those of go/types (but its own about branches), sorted by position.
func check(pkgs []*packages.Package) error {
	var parsed, typed []diagnostic
	seen := make(map[string]bool)
	fileVersions := make(map[string]string) // (from //go:build constraints)
	for _, pkg := range pkgs {
		for _, name := range pkg.CompiledGoFiles {
			if seen[name] {
				continue
			}
			seen[name] = true
			f, err := os.Open(name)
			if err != nil {
				continue // (reported by go/packages)
			}
			file, _ := syntax.Parse(syntax.NewFileBase(name), f, func(err error) {
				if e, ok := err.(syntax.Error); ok {
					parsed = append(parsed, diagnostic{e.Pos.RelFilename(), int(e.Pos.RelLine()), int(e.Pos.RelCol()), e.Msg, nil, false})
				}
			}, nil, syntax.CheckBranches)
			f.Close()
			if file != nil {
				fileVersions[name] = file.GoVersion
			}
		}
		lang := ""
		if pkg.Module != nil && pkg.Module.GoVersion != "" {
			lang = "go" + pkg.Module.GoVersion
		}
		dropping := false
		for _, err := range pkg.Errors {
			if msg, ok := strings.CutPrefix(err.Msg, "\t"); ok { // the continuation of the previous error.
				if !dropping && len(typed) > 0 {
					typed[len(typed)-1].more = append(typed[len(typed)-1].more, err.Pos+": "+msg)
				}
				continue
			}
			d := diagnostic{msg: err.Msg}
			if m := position.FindStringSubmatch(err.Pos); m != nil {
				d.file = m[1]
				d.line, _ = strconv.Atoi(m[2])
				d.col, _ = strconv.Atoi(m[3])
			} else if err.Pos != "" && err.Pos != "-" {
				d.msg = err.Pos + ": " + err.Msg
			}
			if versionError.MatchString(d.msg) { // (as gc reports them)
				if v := fileVersions[d.file]; v != "" {
					d.msg += " (file declares //go:build " + v + ")"
				} else if lang != "" {
					d.msg += " (-lang was set to " + lang + "; check go.mod)"
				}
			}
			dropping = err.Kind == packages.TypeError && branchError.MatchString(err.Msg)
			if !dropping {
				if err.Kind == packages.ParseError {
					d.parse = true // (go/parser's, reported when gc's parser has no syntax errors)
				}
				typed = append(typed, d)
			}
		}
	}
	syntaxErrors := false
	for _, d := range parsed {
		if strings.HasPrefix(d.msg, "syntax error") {
			syntaxErrors = true
		}
	}
	all := parsed
	if !syntaxErrors {
		lines := make(map[string]bool) // (with errors of gc's parser, which go/types would not report)
		for _, d := range parsed {
			lines[fmt.Sprintf("%s:%d", d.file, d.line)] = true
		}
		for _, d := range typed {
			if d.parse && len(parsed) > 0 && !callError.MatchString(d.msg) {
				continue // (gc's parser reported the errors of the syntax)
			}
			if lines[fmt.Sprintf("%s:%d", d.file, d.line)] && tokenError.MatchString(d.msg) {
				continue // (of the tokens that gc's parser reported)
			}
			all = append(all, d)
		}
	}
	if len(all) == 0 {
		return nil
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.file != b.file {
			return a.file < b.file
		}
		if a.line != b.line {
			return a.line < b.line
		}
		return a.col < b.col
	})
	var errs []error
	lastSyntax, lastOther := "", ""
	for _, d := range all { // (one syntax error per line, and one of the same other errors)
		line := fmt.Sprintf("%s:%d", d.file, d.line)
		if strings.HasPrefix(d.msg, "syntax error") {
			if d.file != "" && line == lastSyntax {
				continue
			}
			lastSyntax = line
		} else {
			if d.file != "" && line+d.msg == lastOther {
				continue
			}
			lastOther = line + d.msg
		}
		errs = append(errs, errors.New(d.String()))
	}
	return errors.Join(errs...)
}
