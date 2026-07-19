// Package docker runs submissions inside short-lived Docker containers.
package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gedex/batasd/internal/sandbox"
)

const workspaceDir = "/workspace"

// Runner executes commands in a Docker container.
type Runner struct {
	binary string
	image  string
}

// NewRunner creates a Docker runner using binary and image.
func NewRunner(binary, image string) *Runner {
	return &Runner{binary: binary, image: image}
}

// Run executes command in a short-lived Docker container.
func (r *Runner) Run(ctx context.Context, command sandbox.Command) sandbox.Result {
	if len(command.Args) == 0 {
		return sandbox.Result{Err: errors.New("command args cannot be empty")}
	}
	if strings.TrimSpace(r.binary) == "" {
		return sandbox.Result{Err: errors.New("docker binary cannot be empty")}
	}
	if strings.TrimSpace(r.image) == "" {
		return sandbox.Result{Err: errors.New("docker image cannot be empty")}
	}

	hostDir, err := filepath.Abs(command.Dir)
	if err != nil {
		return sandbox.Result{Err: err}
	}

	timeout := time.Duration(command.Limits.WallTimeMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	containerName, err := randomContainerName()
	if err != nil {
		return sandbox.Result{Err: err}
	}
	defer r.forceRemove(containerName)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := r.dockerArgs(containerName, hostDir, command)
	start := time.Now()
	cmd := exec.CommandContext(runCtx, r.binary, args...)
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
	result.MemoryLimit = r.inspectOOM(containerName)

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
		if exitCode == 125 {
			result.Err = fmt.Errorf("docker run failed: %s", strings.TrimSpace(result.Stderr))
			return result
		}
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

func (r *Runner) dockerArgs(containerName, hostDir string, command sandbox.Command) []string {
	args := []string{
		"run",
		"--name", containerName,
		"--workdir", workspaceDir,
		"--volume", hostDir + ":" + workspaceDir + ":rw",
		"--tmpfs", "/tmp:rw,nosuid,nodev,noexec,size=64m",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--env", "PYTHONDONTWRITEBYTECODE=1",
	}

	if !command.Limits.Network {
		args = append(args, "--network", "none")
	}
	if command.Limits.MaxProcesses > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(command.Limits.MaxProcesses))
	}
	if command.Limits.MemoryKB > 0 {
		memory := strconv.FormatInt(command.Limits.MemoryKB, 10) + "k"
		args = append(args, "--memory", memory, "--memory-swap", memory)
	}

	args = append(args, r.image)
	args = append(args, command.Args...)
	return args
}

func (r *Runner) inspectOOM(containerName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, r.binary, "inspect", "--format", "{{.State.OOMKilled}}", containerName).Output()
	if err != nil {
		return false
	}
	return dockerOOMKilled(output)
}

func dockerOOMKilled(output []byte) bool {
	return strings.EqualFold(strings.TrimSpace(string(output)), "true")
}

func (r *Runner) forceRemove(containerName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, r.binary, "rm", "-f", containerName).Run()
}

func randomContainerName() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "batasd-" + hex.EncodeToString(bytes[:]), nil
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
