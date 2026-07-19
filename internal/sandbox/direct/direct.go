// Package direct runs submissions as local child processes for development.
package direct

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gedex/batasd/internal/sandbox"
)

// Runner executes commands directly on the host.
type Runner struct{}

// NewRunner creates a direct development runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run executes command as a local child process with best-effort limits.
func (r *Runner) Run(ctx context.Context, command sandbox.Command) sandbox.Result {
	if len(command.Args) == 0 {
		return sandbox.Result{Err: errors.New("command args cannot be empty")}
	}

	timeout := time.Duration(command.Limits.WallTimeMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(runCtx, command.Args[0], command.Args[1:]...)
	cmd.Dir = command.Dir
	cmd.Stdin = strings.NewReader(command.Input)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return sandbox.Result{Err: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return sandbox.Result{Err: err}
	}

	stdout := sandbox.NewOutputBuffer(sandbox.MaxOutputBytes(command.Limits), cancel)
	stderr := sandbox.NewOutputBuffer(sandbox.MaxOutputBytes(command.Limits), cancel)

	if err := cmd.Start(); err != nil {
		return sandbox.Result{Err: err}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stdout, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stderr, stderrPipe)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	wallTimeMS := time.Since(start).Milliseconds()
	result := sandbox.Result{
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		TimeMS:      wallTimeMS,
		WallTimeMS:  wallTimeMS,
		OutputLimit: stdout.Exceeded() || stderr.Exceeded(),
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Message = "wall time limit exceeded"
		return result
	}

	if waitErr == nil {
		exitCode := 0
		result.ExitCode = &exitCode
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		exitCode := exitErr.ExitCode()
		result.ExitCode = &exitCode
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			signal := status.Signal().String()
			result.ExitSignal = &signal
		}
		return result
	}

	result.Err = waitErr
	return result
}
