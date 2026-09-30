package conformance

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Result of a test case.
type Result string

const (
	Pass Result = "pass"
	Fail Result = "fail"
	Skip Result = "skip"
)

// Status of a test case, as recorded in status.txt.
type Status struct {
	Result Result
	Reason string // why the test was skipped, or the stage (and first error) it failed at.
}

const statusHeader = `# Conformance status of gd against testdata (upstream $GOROOT/test).
# Recorded with Fil-C, regenerate with:
#
#	GD_CC=filcc GD_LDFLAGS=-static go test ./internal/conformance -update
#
# path	pass|fail|skip	reason
`

// ReadStatus reads a status file, a missing file is treated as empty.
func ReadStatus(name string) (map[string]Status, error) {
	statuses := make(map[string]Status)
	f, err := os.Open(name)
	if os.IsNotExist(err) {
		return statuses, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: malformed line %q", name, line)
		}
		status := Status{Result: Result(fields[1])}
		if len(fields) == 3 {
			status.Reason = fields[2]
		}
		statuses[fields[0]] = status
	}
	return statuses, scanner.Err()
}

// WriteStatus writes statuses to a status file, sorted by path.
func WriteStatus(name string, statuses map[string]Status) error {
	paths := make([]string, 0, len(statuses))
	for path := range statuses {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString(statusHeader)
	for _, path := range paths {
		status := statuses[path]
		fmt.Fprintf(&b, "%s\t%s\t%s\n", path, status.Result, status.Reason)
	}
	return os.WriteFile(name, []byte(b.String()), 0644)
}
