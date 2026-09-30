package conformance

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	update = flag.Bool("update", false, "rewrite status.txt with the results of this run")
	keep   = flag.Bool("keep", false, "keep the working directory of each test case (logged)")
)

// The C compiler used to build gd's output, overridable to use Fil-C, for example:
//
//	GD_CC=filcc GD_LDFLAGS=-static go test ./internal/conformance
var (
	cc      = env("GD_CC", "cc")
	cflags  = strings.Fields(env("GD_CFLAGS", "-std=c11 -w"))
	ldflags = strings.Fields(env("GD_LDFLAGS", "-lm -pthread"))
)

const (
	statusFile = "status.txt"
	testdata   = "testdata"
)

var (
	gdBinary  string // built by TestMain.
	goVersion string // language version of the go toolchain, for go.mod.
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "gd-conformance")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	gdBinary = filepath.Join(dir, "gd")
	if out, err := exec.Command("go", "build", "-o", gdBinary, "github.com/quaadgras/gd-compiler").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building gd: %v\n%s", err, out)
		os.Exit(1)
	}
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go env GOVERSION:", err)
		os.Exit(1)
	}
	goVersion = languageVersion(strings.TrimSpace(string(out)))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// languageVersion converts a toolchain version such as go1.27.1 into 1.27.
func languageVersion(v string) string {
	v = strings.TrimPrefix(v, "go")
	v, _, _ = strings.Cut(v, " ")
	if major, rest, ok := strings.Cut(v, "."); ok {
		minor, _, _ := strings.Cut(rest, ".")
		minor = strings.TrimRightFunc(minor, func(r rune) bool { return r < '0' || r > '9' })
		return major + "." + minor
	}
	return v
}

// TestGoRepo runs each test case in testdata through gd. A test case only fails the Go
// test when it regresses: status.txt says it passes, but it no longer does. Test cases
// that are expected to fail are reported as skipped, along with the reason.
func TestGoRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("conformance suite is slow")
	}
	root, err := filepath.Abs(testdata)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := Cases(root)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ReadStatus(statusFile)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mutex   sync.Mutex
		results = make(map[string]Status)
	)
	t.Cleanup(func() {
		report(t, expected, results)
		if !*update {
			return
		}
		updated := make(map[string]Status)
		for _, name := range cases {
			if status, ok := results[name]; ok {
				updated[name] = status
			} else if status, ok := expected[name]; ok {
				updated[name] = status // not selected by -run.
			}
		}
		if err := WriteStatus(statusFile, updated); err != nil {
			t.Error(err)
		}
	})
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			status := runCase(t, root, name)
			mutex.Lock()
			results[name] = status
			mutex.Unlock()

			want, known := expected[name]
			switch {
			case status.Result == Pass && want.Result != Pass:
				t.Logf("now passing (was %s %s), run with -update", orNew(want.Result), want.Reason)
			case status.Result == Pass:
			case want.Result == Pass:
				t.Errorf("regression: %s %s", status.Result, status.Reason)
			case status.Result == Skip:
				t.Skip(status.Reason)
			default:
				if !known {
					t.Skipf("new failure: %s", status.Reason)
				}
				t.Skipf("expected failure: %s", status.Reason)
			}
		})
	}
}

func orNew(r Result) Result {
	if r == "" {
		return "new"
	}
	return r
}

// report logs a summary of the run.
func report(t *testing.T, expected, results map[string]Status) {
	counts := make(map[Result]int)
	reasons := make(map[string]int)
	var promoted []string
	for name, status := range results {
		counts[status.Result]++
		if status.Result == Fail {
			reasons[status.Reason]++
		}
		if status.Result == Pass && expected[name].Result != Pass {
			promoted = append(promoted, name)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d pass, %d fail, %d skip", counts[Pass], counts[Fail], counts[Skip])
	if runnable := counts[Pass] + counts[Fail]; runnable > 0 {
		fmt.Fprintf(&b, " (%.1f%% of runnable)", 100*float64(counts[Pass])/float64(runnable))
	}
	top := make([]string, 0, len(reasons))
	for reason := range reasons {
		top = append(top, reason)
	}
	sort.Slice(top, func(i, j int) bool {
		if reasons[top[i]] != reasons[top[j]] {
			return reasons[top[i]] > reasons[top[j]]
		}
		return top[i] < top[j]
	})
	if len(top) > 15 {
		top = top[:15]
	}
	if len(top) > 0 {
		b.WriteString("\ntop failure reasons:")
	}
	for _, reason := range top {
		fmt.Fprintf(&b, "\n%6d  %s", reasons[reason], reason)
	}
	if len(promoted) > 0 && !*update {
		sort.Strings(promoted)
		fmt.Fprintf(&b, "\n%d newly passing (run with -update to record): %s", len(promoted), strings.Join(promoted, " "))
	}
	t.Log(b.String())
}

// runCase runs a single test case, returning its status.
func runCase(t *testing.T, root, name string) Status {
	src, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := ParseRecipe(string(src))
	if err != nil {
		return Status{Skip, skipUpstream + err.Error()}
	}
	recipe.Classify(root, name, src)
	if recipe.Skip != "" {
		return Status{Skip, recipe.Skip}
	}
	dir := t.TempDir()
	if *keep {
		dir, err = os.MkdirTemp("", "gd-conformance-case")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("working directory: %s", dir)
	}
	lang := goVersion
	if recipe.Lang != "" {
		lang = recipe.Lang
	}
	base := filepath.Base(name)
	if strings.HasSuffix(base, "_test.go") {
		base = strings.TrimSuffix(base, "_test.go") + ".go"
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo "+lang+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, base), src, 0644); err != nil {
		t.Fatal(err)
	}

	// Go -> C
	if out, err := command(t, dir, time.Minute, gdBinary, "build"); err != nil {
		return failure("gd", out, err, dir)
	}

	// C -> executable
	sources, err := cSources(filepath.Join(dir, ".c"))
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{}, cflags...)
	args = append(args, "-I", filepath.Join(dir, ".c"))
	link := recipe.Action == "run" || (recipe.Action == "build" && isMain(src))
	exe := filepath.Join(dir, "test.exe")
	if link {
		args = append(args, "-o", exe)
		args = append(args, sources...)
		args = append(args, ldflags...)
	} else {
		args = append(args, "-c")
		args = append(args, sources...)
	}
	objects := filepath.Join(dir, "obj")
	if err := os.Mkdir(objects, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := command(t, objects, time.Minute, cc, args...); err != nil {
		return failure("cc", out, err, dir)
	}
	if recipe.Action != "run" {
		return Status{Result: Pass}
	}

	// Run it, in the test directory like upstream, as some tests read files.
	timeout := 10 * time.Second
	if recipe.Timeout > 0 {
		timeout = time.Duration(recipe.Timeout) * time.Second
	}
	out, err := command(t, filepath.Join(root, filepath.Dir(name)), timeout, exe, recipe.Args...)
	if err != nil {
		return failure("exit", out, err, dir)
	}
	want, err := os.ReadFile(filepath.Join(root, strings.TrimSuffix(name, ".go")+".out"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if got := bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n")); !bytes.Equal(got, want) {
		t.Logf("output mismatch\n--- want\n%s\n--- got\n%s", want, got)
		return Status{Fail, "output: mismatch"}
	}
	return Status{Result: Pass}
}

func isMain(src []byte) bool {
	return regexp.MustCompile(`(?m)^package main\b`).Match(src)
}

func cSources(dir string) ([]string, error) {
	var sources []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".c") {
			sources = append(sources, path)
		}
		return err
	})
	return sources, err
}

// command runs name with args in dir, returning its combined output.
func command(t *testing.T, dir string, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local", "PWD="+dir)
	cmd.WaitDelay = time.Second
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		err = context.DeadlineExceeded
	}
	return buf.Bytes(), err
}

var (
	positions  = regexp.MustCompile(`(\S*/)?[\w.-]+\.(go|c|h):\d+(:\d+)?:?\s*`)
	hexes      = regexp.MustCompile(`0x[0-9a-f]+`)
	timestamps = regexp.MustCompile(`^\d{4}/\d\d/\d\d \d\d:\d\d:\d\d `)
)

// failure records the failing stage along with a normalized first line of the error, so
// that failures with the same cause can be grouped together in status.txt.
func failure(stage string, out []byte, err error, dir string) Status {
	if errors.Is(err, context.DeadlineExceeded) {
		return Status{Fail, stage + ": timeout"}
	}
	lines := strings.Split(strings.ReplaceAll(string(out), dir, ""), "\n")
	first := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if first == "" {
			first = line
		}
		// Prefer the most specific line for the stage.
		if (stage == "cc" && strings.Contains(line, "error:")) || strings.HasPrefix(line, "panic:") {
			first = line
			break
		}
	}
	if stage == "exit" {
		if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
			first = strings.TrimSpace(exit.String() + " " + truncate(first, 60))
		}
	}
	first = timestamps.ReplaceAllString(first, "")
	first = positions.ReplaceAllString(first, "")
	if stage == "gd" {
		// Drop %v dumps of compiler data structures, they vary by test case.
		if i := strings.IndexAny(first, "{[%"); i > 0 {
			first = first[:i]
		}
	}
	first = hexes.ReplaceAllString(first, "0x?")
	first = strings.Join(strings.Fields(first), " ")
	if first == "" {
		first = err.Error()
	}
	return Status{Fail, stage + ": " + truncate(first, 120)}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
