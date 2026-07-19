// Package sandbox defines the interface implemented by execution sandboxes.
package sandbox

import (
	"context"

	"github.com/gedex/batasd/internal/submission"
)

// Command describes a process execution request inside a sandbox.
type Command struct {
	Args   []string
	Dir    string
	Input  string
	Limits submission.Limits
}

// Result describes the outcome of a sandbox command.
type Result struct {
	Stdout      string
	Stderr      string
	Message     string
	ExitCode    *int
	ExitSignal  *string
	TimeMS      int64
	WallTimeMS  int64
	MemoryKB    *int64
	TimedOut    bool
	MemoryLimit bool
	OutputLimit bool
	Err         error
}

// Runner executes commands inside a sandbox.
type Runner interface {
	Run(ctx context.Context, command Command) Result
}
