// Package conformance runs the upstream Go language test suite (a copy of $GOROOT/test,
// see testdata/UPSTREAM) through gd and tracks which tests pass in status.txt.
package conformance

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Dirs are the directories of testdata that contain test cases, matching upstream's
// cmd/internal/testdir.
var Dirs = []string{".", "ken", "chan", "interface", "internal/runtime/sys", "syntax", "dwarf", "fixedbugs",
	"codegen", "abi", "typeparam", "typeparam/mdempsky", "arenas", "simd"}

// Recipe is the parsed execution recipe of a test case (the first comment line of the file).
type Recipe struct {
	Action  string   // run, compile, errorcheck, ...
	Args    []string // arguments passed to the test program.
	Flags   []string // unsupported go command/compiler flags.
	Lang    string   // Go language version for go.mod, from -lang= or -gomodversion.
	Timeout int      // seconds, from -t.
	Skip    string   // non-empty when the test is out of scope, see [Classify].
}

// Skip reasons are prefixed with a category:
//
//	gc:       depends on gc-specific behaviour (compiler flags, diagnostics, runtime internals).
//	upstream: not run upstream on this platform either.
//	todo:     in scope, but the harness does not support the action yet.
const (
	skipGC       = "gc:"
	skipUpstream = "upstream:"
	skipTodo     = "todo:"
)

// ParseRecipe parses the execution recipe from the header of a test file, following
// upstream's rules.
func ParseRecipe(src string) (Recipe, error) {
	var line string
	for rest := src; line == "" && rest != ""; {
		line, rest, _ = strings.Cut(rest, "\n")
		if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) {
			line = ""
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Recipe{}, fmt.Errorf("execution recipe not found")
	}
	r := Recipe{Action: fields[0]}
	args := fields[1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		flag := args[0]
		args = args[1:]
		value := func() string {
			if len(args) == 0 {
				return ""
			}
			v := args[0]
			args = args[1:]
			return v
		}
		switch {
		case flag == "-0", flag == "-1", flag == "-s":
			// expected error / single file packages: only meaningful for errorcheck and *dir.
		case flag == "-t":
			r.Timeout, _ = strconv.Atoi(value())
		case flag == "-gomodversion":
			r.Lang = value()
		case strings.HasPrefix(flag, "-lang=go"):
			r.Lang = strings.TrimPrefix(flag, "-lang=go")
		case flag == "-goexperiment", flag == "-godebug":
			r.Flags = append(r.Flags, flag+"="+value())
		default:
			r.Flags = append(r.Flags, flag)
		}
	}
	r.Args = args
	return r, nil
}

// Classify decides whether the test in file (relative to dir) is in scope for gd, setting
// r.Skip to the reason if it is not.
func (r *Recipe) Classify(dir, file string, src []byte) {
	r.Skip = r.classify(dir, file, src)
}

func (r *Recipe) classify(dir, file string, src []byte) string {
	// Like upstream, build constraints are checked first, as helper files excluded by them
	// don't have an execution recipe.
	if match, expr := matchConstraints(gd, dir, file, src); !match {
		if ok, _ := matchConstraints(gc, dir, file, src); ok || strings.Contains(expr, "goexperiment.") || strings.Contains(expr, "cgo") {
			return skipGC + "build " + expr
		}
		return skipUpstream + "build " + expr
	}
	switch r.Action {
	case "run", "compile", "build":
	case "skip":
		return skipUpstream + "skip"
	case "asmcheck":
		return skipGC + "asmcheck"
	case "errorcheck", "errorcheckdir", "errorcheckoutput", "errorcheckwithauto", "errorcheckandrundir":
		var flags []string
		for _, flag := range r.Flags {
			if flag == "-m" || strings.HasPrefix(flag, "-m=") || (strings.HasPrefix(flag, "-d=") && flag != "-d=panic") || flag == "-live" {
				return skipGC + "diagnostics " + flag // escape analysis, liveness, ssa checks.
			}
			if flag != "-d=panic" { // (crashing on errors, which gd reports all of)
				flags = append(flags, flag)
			}
		}
		r.Flags = flags
		if r.Action != "errorcheck" {
			return skipTodo + "errorcheck"
		}
	case "compiledir", "builddir", "rundir", "runindir", "buildrundir", "buildrun", "runoutput":
		return skipTodo + r.Action
	default:
		return skipTodo + "unknown action " + r.Action
	}
	for _, flag := range r.Flags {
		switch {
		case strings.HasPrefix(flag, "-goexperiment="):
			return skipGC + "goexperiment " + strings.TrimPrefix(flag, "-goexperiment=")
		case strings.HasPrefix(flag, "-godebug="):
			return skipGC + "godebug " + strings.TrimPrefix(flag, "-godebug=")
		default:
			return skipGC + "flags " + strings.Join(r.Flags, " ")
		}
	}
	if why := r.classifySource(file, src); why != "" {
		return why
	}
	return ""
}

// Build contexts to evaluate build constraints with. gd has no cgo, and no goexperiments.
var gd, gc = func() (gd, gc build.Context) {
	gc = build.Default
	gc.CgoEnabled = false
	gc.BuildTags = []string{"test_run"}
	gd = gc
	gd.Compiler = "gd"
	gd.ToolTags = nil
	return
}()

func matchConstraints(ctx build.Context, dir, file string, src []byte) (bool, string) {
	var expr string
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "package ") {
			break
		}
		if constraint.IsGoBuild(line) {
			expr = strings.TrimSpace(strings.TrimPrefix(line, "//go:build"))
		}
	}
	if expr == "" {
		expr = path.Base(file)
	}
	ok, err := ctx.MatchFile(filepath.Join(dir, filepath.Dir(file)), filepath.Base(file))
	return ok && err == nil, expr
}

// runtimeAPIs that depend on the runtime and garbage collector of gc, rather than the
// language spec.
var runtimeAPIs = map[string]string{
	"SetFinalizer":      "finalizers",
	"AddCleanup":        "finalizers",
	"ReadMemStats":      "memstats",
	"MemProfile":        "memstats",
	"MemProfileRate":    "memstats",
	"Caller":            "stack-introspection",
	"Callers":           "stack-introspection",
	"CallersFrames":     "stack-introspection",
	"FuncForPC":         "stack-introspection",
	"Stack":             "stack-introspection",
	"SetCPUProfileRate": "profiling",
}

// runtimePackages that are gc implementation details.
var runtimePackages = []string{"internal", "runtime/debug", "runtime/pprof", "runtime/metrics",
	"runtime/trace", "runtime/race", "runtime/cgo", "weak"}

func (r *Recipe) classifySource(file string, src []byte) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "" // leave it to the compiler.
	}
	runtimeName := ""
	for _, spec := range f.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		switch path {
		case "C":
			return skipGC + "cgo"
		case "os/exec":
			return skipGC + "toolchain" // these tests run go tool compile/link.
		}
		for _, pkg := range runtimePackages {
			if path == pkg || strings.HasPrefix(path, pkg+"/") {
				return skipGC + "import " + path
			}
		}
		if path == "runtime" {
			runtimeName = "runtime"
			if spec.Name != nil {
				runtimeName = spec.Name.Name
			}
		}
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:linkname ") {
				return skipGC + "linkname"
			}
		}
	}
	var why string
	if runtimeName != "" {
		ast.Inspect(f, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok || why != "" {
				return why == ""
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == runtimeName {
				if kind, ok := runtimeAPIs[sel.Sel.Name]; ok {
					why = skipGC + kind + " runtime." + sel.Sel.Name
				}
			}
			return true
		})
	}
	return why
}

// Cases returns the test files under root, as slash-separated paths relative to root.
func Cases(root string) ([]string, error) {
	var files []string
	for _, dir := range Dirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".go") {
				files = append(files, path.Join(dir, entry.Name()))
			}
		}
	}
	return files, nil
}
