package conformance

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Expected errors are written in the test's comments, as upstream's $GOROOT/test/run.go
// reads them: each quoted regular expression of a // ERROR "re" comment must match one
// of the errors reported on its line (LINE+n refers to the n'th line after it), and every
// error reported must be expected.
var (
	errorComment = regexp.MustCompile(`// (?:GC_)?ERROR (.*)`)
	errorQuoted  = regexp.MustCompile(`"([^"]*)"`)
	errorLine    = regexp.MustCompile(`LINE(([+-])(\d+))?`)
)

// implementationErrors match the expected errors of gc's implementation restrictions and
// directives, which are not in the spec, and that gd may report or not (as it compiles
// such programs).
var implementationErrors = regexp.MustCompile(`stack frame too large|(channel|map) element type too large|` +
	`larger than address space|misplaced compiler directive|go:embed|^embed$|//go:nowritebarrier|//go:cgo_`)

type wantedError struct {
	re      *regexp.Regexp
	reStr   string
	prefix  string // file:line
	lineNum int
}

// wantedErrors returns the errors that the comments of the test file (named short) expect.
func wantedErrors(short string, src []byte) ([]wantedError, error) {
	var wants []wantedError
	for i, line := range strings.Split(string(src), "\n") {
		lineNum := i + 1
		if strings.Contains(line, "////") {
			continue // (a double comment disables an ERROR comment)
		}
		m := errorComment.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		quoted := errorQuoted.FindAllStringSubmatch(m[1], -1)
		if quoted == nil {
			return nil, fmt.Errorf("%s:%d: invalid errorcheck line: %s", short, lineNum, line)
		}
		for _, q := range quoted {
			rx := errorLine.ReplaceAllStringFunc(q[1], func(m string) string {
				n := lineNum
				if strings.HasPrefix(m, "LINE+") {
					d, _ := strconv.Atoi(m[5:])
					n += d
				} else if strings.HasPrefix(m, "LINE-") {
					d, _ := strconv.Atoi(m[5:])
					n -= d
				}
				return fmt.Sprintf("%s:%d", short, n)
			})
			re, err := regexp.Compile(rx)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: invalid regexp %q: %v", short, lineNum, rx, err)
			}
			wants = append(wants, wantedError{re: re, reStr: rx, prefix: fmt.Sprintf("%s:%d", short, lineNum), lineNum: lineNum})
		}
	}
	return wants, nil
}

// errorCheck reports whether the errors that gd reported (its output, for the test file
// at path) are those the test expects, see [wantedErrors].
func errorCheck(path string, src []byte, output string) error {
	short := filepath.Base(path)
	wants, err := wantedErrors(short, src)
	if err != nil {
		return err
	}
	var out []string // the errors, with paths shortened, and continuation lines joined.
	for _, line := range strings.Split(output, "\n") {
		line = strings.ReplaceAll(line, path, short)
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "":
		case strings.HasPrefix(line, "\t") && len(out) > 0:
			out[len(out)-1] += "\n" + line
		default:
			out = append(out, line)
		}
	}
	var errs []string
	for _, want := range wants {
		var msgs []string
		msgs, out = partition(want.prefix+":", out)
		if len(msgs) == 0 {
			if !implementationErrors.MatchString(want.reStr) {
				errs = append(errs, fmt.Sprintf("%s: missing error %q", want.prefix, want.reStr))
			}
			continue
		}
		matched := false
		for _, msg := range msgs {
			text := msg // (without the leading file:line: so as not to match the file name)
			if _, suffix, ok := strings.Cut(text, " "); ok {
				text = suffix
			}
			if want.re.MatchString(text) {
				matched = true
			} else {
				out = append(out, msg)
			}
		}
		if !matched {
			errs = append(errs, fmt.Sprintf("%s: no match for %q in: %s", want.prefix, want.reStr, strings.Join(msgs, "; ")))
		}
	}
	for _, msg := range out {
		errs = append(errs, "unmatched error: "+msg)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// partition returns the strings with the prefix, and the others.
func partition(prefix string, strs []string) (matched, unmatched []string) {
	for _, s := range strs {
		if strings.HasPrefix(s, prefix) {
			matched = append(matched, s)
		} else {
			unmatched = append(unmatched, s)
		}
	}
	return matched, unmatched
}
