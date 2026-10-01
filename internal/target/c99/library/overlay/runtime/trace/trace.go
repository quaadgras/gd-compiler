//go:build ignore

// Package trace, for gd: execution tracing is not supported, so tracing is never enabled,
// and its annotations do nothing.
package trace

import (
	"context"
	"errors"
	"io"
	"time"
)

var errUnsupported = errors.New("runtime/trace: tracing is not supported by gd")

// Start enables tracing for the current program: gd can't.
func Start(w io.Writer) error { return errUnsupported }

// Stop stops the current tracing, if any.
func Stop() {}

// IsEnabled reports whether tracing is enabled.
func IsEnabled() bool { return false }

// Task is a data type for tracing a user-defined, logical operation.
type Task struct{}

// NewTask creates a task instance with the type taskType.
func NewTask(pctx context.Context, taskType string) (ctx context.Context, task *Task) {
	return pctx, &Task{}
}

// End marks the end of the operation represented by the Task.
func (t *Task) End() {}

// Log emits a one-off event with the given category and message.
func Log(ctx context.Context, category, message string) {}

// Logf is like Log, but the value is formatted using the specified format spec.
func Logf(ctx context.Context, category, format string, args ...any) {}

// WithRegion starts a region associated with its calling goroutine, runs fn, and then
// ends the region.
func WithRegion(ctx context.Context, regionType string, fn func()) { fn() }

// Region is a region of code whose execution time interval is traced.
type Region struct{}

// StartRegion starts a region and returns it.
func StartRegion(ctx context.Context, regionType string) *Region { return &Region{} }

// End marks the end of the traced code region.
func (r *Region) End() {}

// FlightRecorderConfig is used to configure a FlightRecorder.
type FlightRecorderConfig struct {
	MinAge   time.Duration
	MaxBytes uint64
}

// FlightRecorder represents a single consumer of a Go execution trace.
type FlightRecorder struct{}

// NewFlightRecorder creates a new flight recorder from the provided configuration.
func NewFlightRecorder(cfg FlightRecorderConfig) *FlightRecorder { return &FlightRecorder{} }

// Start activates the flight recorder: gd can't.
func (fr *FlightRecorder) Start() error { return errUnsupported }

// Stop ends recording of trace data.
func (fr *FlightRecorder) Stop() {}

// Enabled returns true if the flight recorder is active.
func (fr *FlightRecorder) Enabled() bool { return false }

// WriteTo snapshots the moving window tracked by the flight recorder.
func (fr *FlightRecorder) WriteTo(w io.Writer) (n int64, err error) { return 0, errUnsupported }
