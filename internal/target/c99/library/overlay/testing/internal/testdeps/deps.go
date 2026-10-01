//go:build ignore

// Package testdeps, for gd: the dependencies of the test binaries that go test generates,
// without those that are coupled to the Go runtime (profiles, fuzzing and coverage, which
// gd doesn't support).
package testdeps

import (
	"errors"
	"io"
	"reflect"
	"regexp"
	"time"
)

// Cover indicates whether coverage is enabled.
var Cover bool

// TestDeps is an implementation of the testing.testDeps interface, suitable for passing
// to [testing.MainStart].
type TestDeps struct{}

var matchPat string
var matchRe *regexp.Regexp

func (TestDeps) MatchString(pat, str string) (result bool, err error) {
	if matchRe == nil || matchPat != pat {
		matchPat = pat
		matchRe, err = regexp.Compile(matchPat)
		if err != nil {
			return
		}
	}
	return matchRe.MatchString(str), nil
}

var errUnsupported = errors.New("not supported by gd")

func (TestDeps) StartCPUProfile(w io.Writer) error { return errUnsupported }
func (TestDeps) StopCPUProfile()                   {}
func (TestDeps) WriteProfileTo(name string, w io.Writer, debug int) error {
	return errUnsupported
}

// ImportPath is the import path of the testing binary, set by the generated main function.
var ImportPath string

func (TestDeps) ImportPath() string { return ImportPath }

var ModulePath string

func (TestDeps) ModulePath() string { return ModulePath }

func (TestDeps) StartTestLog(w io.Writer) {}
func (TestDeps) StopTestLog() error       { return nil }
func (TestDeps) SetPanicOnExit0(v bool)   {}

// corpusEntry is internal/fuzz.CorpusEntry (an alias, as in package testing).
type corpusEntry = struct {
	Parent     string
	Path       string
	Data       []byte
	Values     []any
	Generation int
	IsSeed     bool
}

func (TestDeps) CoordinateFuzzing(timeout time.Duration, limit int64, minimizeTimeout time.Duration, minimizeLimit int64,
	parallel int, seed []corpusEntry, types []reflect.Type, corpusDir, cacheDir string) error {
	return errors.New("fuzzing is not supported by gd")
}

func (TestDeps) RunFuzzWorker(fn func(corpusEntry) error) error {
	return errors.New("fuzzing is not supported by gd")
}

func (TestDeps) ReadCorpus(dir string, types []reflect.Type) ([]corpusEntry, error) {
	return nil, nil // (only the seed corpus is run)
}

func (TestDeps) CheckCorpus(vals []any, types []reflect.Type) error {
	if len(vals) != len(types) {
		return errors.New("wrong number of values in corpus entry")
	}
	for i := range types {
		if reflect.TypeOf(vals[i]) != types[i] {
			return errors.New("mismatched types in corpus entry")
		}
	}
	return nil
}

func (TestDeps) ResetCoverage()    {}
func (TestDeps) SnapshotCoverage() {}

var CoverMode string
var Covered string
var CoverSelectedPackages []string

var (
	CoverSnapshotFunc           func() float64
	CoverProcessTestDirFunc     func(dir string, cfile string, cm string, cpkg string, w io.Writer, selpkgs []string) error
	CoverMarkProfileEmittedFunc func(val bool)
)

func (TestDeps) InitRuntimeCoverage() (mode string, tearDown func(string, string) (string, error), snapcov func() float64) {
	return
}
