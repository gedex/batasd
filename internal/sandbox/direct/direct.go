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

	maxOutputBytes := command.Limits.MaxOutputKB * 1024
	if maxOutputBytes <= 0 {
		maxOutputBytes = 1024 * 1024
	}
	stdout := newLimitBuffer(maxOutputBytes)
	stderr := newLimitBuffer(maxOutputBytes)

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

type limitBuffer struct {
	mu       sync.Mutex
	buf      []byte
	limit    int64
	exceeded bool
}

func newLimitBuffer(limit int64) *limitBuffer {
	return &limitBuffer{limit: limit}
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	available := int(b.limit) - len(b.buf)
	if available > 0 {
		if len(p) <= available {
			b.buf = append(b.buf, p...)
		} else {
			b.buf = append(b.buf, p[:available]...)
			b.exceeded = true
		}
	} else if len(p) > 0 {
		b.exceeded = true
	}

	return len(p), nil
}

func (b *limitBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return string(b.buf)
}

func (b *limitBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.exceeded
}
