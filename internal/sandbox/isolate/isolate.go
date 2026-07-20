// Package isolate runs submissions inside isolate sandboxes.
package isolate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gedex/batasd/internal/sandbox"
)

const (
	boxDir              = "/box"
	defaultWallTime     = 10 * time.Second
	managerGraceTimeout = 5 * time.Second
)

// Runner executes commands with isolate.
type Runner struct {
	binary string
	useCG  bool
	boxes  chan int
}

// NewRunner creates an isolate runner using a pool of box IDs.
func NewRunner(binary string, boxIDStart, boxIDCount int, useCG bool) *Runner {
	boxes := make(chan int, boxIDCount)
	for id := boxIDStart; id < boxIDStart+boxIDCount; id++ {
		boxes <- id
	}
	return &Runner{
		binary: binary,
		useCG:  useCG,
		boxes:  boxes,
	}
}

// Run executes command in a freshly initialized isolate box.
func (r *Runner) Run(ctx context.Context, command sandbox.Command) sandbox.Result {
	if len(command.Args) == 0 {
		return sandbox.Result{Err: errors.New("command args cannot be empty")}
	}
	if strings.TrimSpace(r.binary) == "" {
		return sandbox.Result{Err: errors.New("isolate binary cannot be empty")}
	}
	if r.boxes == nil {
		return sandbox.Result{Err: errors.New("isolate box pool is not configured")}
	}

	hostDir, err := filepath.Abs(command.Dir)
	if err != nil {
		return sandbox.Result{Err: err}
	}
	resolvedArgs, err := resolveExecutable(command.Args)
	if err != nil {
		return sandbox.Result{Err: err}
	}
	command.Args = resolvedArgs

	boxID, err := r.acquireBox(ctx)
	if err != nil {
		return sandbox.Result{Err: err}
	}
	defer r.releaseBox(boxID)

	if err := r.initBox(ctx, boxID); err != nil {
		return sandbox.Result{Err: err}
	}
	defer r.cleanupBox(boxID)

	metaFile, err := os.CreateTemp("", "batasd-isolate-meta-*")
	if err != nil {
		return sandbox.Result{Err: err}
	}
	metaPath := metaFile.Name()
	_ = metaFile.Close()
	defer os.Remove(metaPath)

	timeout := wallTimeout(command.Limits.WallTimeMS)
	runCtx, cancel := context.WithTimeout(ctx, timeout+managerGraceTimeout)
	defer cancel()

	args := r.runArgs(boxID, hostDir, metaPath, command)
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
		StdoutLimit: stdout.Exceeded(),
		StderrLimit: stderr.Exceeded(),
		TimeMS:      wallTimeMS,
		WallTimeMS:  wallTimeMS,
		OutputLimit: stdout.Exceeded() || stderr.Exceeded(),
	}

	meta, metaErr := readMetaFile(metaPath)
	if metaErr == nil {
		applyMeta(&result, meta)
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		if result.Message == "" {
			result.Message = "wall time limit exceeded"
		}
		return result
	}

	if metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
		result.Err = metaErr
		return result
	}

	if waitErr == nil {
		if result.ExitCode == nil && result.ExitSignal == nil && !result.TimedOut {
			exitCode := 0
			result.ExitCode = &exitCode
		}
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		exitCode := exitErr.ExitCode()
		if exitCode == 1 {
			if result.ExitCode == nil && result.ExitSignal == nil && !result.TimedOut {
				result.ExitCode = &exitCode
			}
			return result
		}
		result.Err = fmt.Errorf("isolate run failed with exit code %d: %s", exitCode, strings.TrimSpace(result.Stderr))
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			signal := status.Signal().String()
			result.ExitSignal = &signal
		}
		return result
	}

	result.Err = waitErr
	return result
}

func (r *Runner) acquireBox(ctx context.Context) (int, error) {
	select {
	case boxID := <-r.boxes:
		return boxID, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (r *Runner) releaseBox(boxID int) {
	r.boxes <- boxID
}

func (r *Runner) initBox(ctx context.Context, boxID int) error {
	initCtx, cancel := context.WithTimeout(ctx, managerGraceTimeout)
	defer cancel()

	args := append(r.baseArgs(boxID), "--init")
	output, err := exec.CommandContext(initCtx, r.binary, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("isolate init failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *Runner) cleanupBox(boxID int) {
	ctx, cancel := context.WithTimeout(context.Background(), managerGraceTimeout)
	defer cancel()

	args := append(r.baseArgs(boxID), "--cleanup")
	_ = exec.CommandContext(ctx, r.binary, args...).Run()
}

func (r *Runner) baseArgs(boxID int) []string {
	args := []string{"--box-id", strconv.Itoa(boxID)}
	if r.useCG {
		args = append(args, "--cg")
	}
	return args
}

func (r *Runner) runArgs(boxID int, hostDir, metaPath string, command sandbox.Command) []string {
	args := r.baseArgs(boxID)
	args = append(args,
		"--silent",
		"--meta", metaPath,
		"--chdir", boxDir,
		"--dir", boxDir+"="+hostDir+":rw",
		"--env", "PATH=/usr/local/bin:/usr/bin:/bin",
		"--env", "HOME="+boxDir,
		"--env", "CGO_ENABLED=0",
		"--env", "CARGO_HOME=/usr/local/cargo",
		"--env", "GOCACHE="+boxDir+"/.cache/go-build",
		"--env", "GOMODCACHE="+boxDir+"/.cache/go-mod",
		"--env", "PYTHONDONTWRITEBYTECODE=1",
		"--env", "RUSTUP_HOME=/usr/local/rustup",
	)

	if command.Limits.CPUTimeMS > 0 {
		args = append(args, "--time", millisecondsAsSeconds(command.Limits.CPUTimeMS))
	}
	if command.Limits.CPUExtraMS > 0 {
		args = append(args, "--extra-time", millisecondsAsSeconds(command.Limits.CPUExtraMS))
	}
	if command.Limits.WallTimeMS > 0 {
		args = append(args, "--wall-time", millisecondsAsSeconds(command.Limits.WallTimeMS))
	}
	if command.Limits.MemoryKB > 0 {
		args = append(args, "--mem", strconv.FormatInt(command.Limits.MemoryKB, 10))
		if r.useCG {
			args = append(args, "--cg-mem", strconv.FormatInt(command.Limits.MemoryKB, 10))
		}
	}
	if command.Limits.StackKB > 0 {
		args = append(args, "--stack", strconv.FormatInt(command.Limits.StackKB, 10))
	}
	if command.Limits.MaxFileKB > 0 {
		args = append(args, "--fsize", strconv.FormatInt(command.Limits.MaxFileKB, 10))
	}
	if command.Limits.MaxProcesses > 0 {
		args = append(args, "--processes="+strconv.Itoa(command.Limits.MaxProcesses))
	}
	if command.Limits.Network {
		args = append(args, "--share-net")
	}

	args = append(args, "--run", "--")
	args = append(args, command.Args...)
	return args
}

func resolveExecutable(args []string) ([]string, error) {
	resolved := append([]string(nil), args...)
	if strings.ContainsRune(resolved[0], '/') {
		return resolved, nil
	}

	path, err := exec.LookPath(resolved[0])
	if err != nil {
		return nil, err
	}
	resolved[0] = path
	return resolved, nil
}

func wallTimeout(wallTimeMS int64) time.Duration {
	if wallTimeMS <= 0 {
		return defaultWallTime
	}
	return time.Duration(wallTimeMS) * time.Millisecond
}

func millisecondsAsSeconds(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64)
}

type meta map[string]string

func readMetaFile(path string) (meta, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(meta)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func applyMeta(result *sandbox.Result, values meta) {
	if value, ok := parseSecondsMS(values["time"]); ok {
		result.TimeMS = value
	}
	if value, ok := parseSecondsMS(values["time-wall"]); ok {
		result.WallTimeMS = value
	}
	if value, ok := parseInt64(values["max-rss"]); ok {
		result.MemoryKB = &value
	}
	if value, ok := parseInt64(values["cg-mem"]); ok && result.MemoryKB == nil {
		result.MemoryKB = &value
	}
	if value, ok := parseInt(values["exitcode"]); ok {
		result.ExitCode = &value
	}
	if signal := strings.TrimSpace(values["exitsig"]); signal != "" {
		result.ExitSignal = &signal
		if result.ExitCode == nil {
			exitCode := -1
			result.ExitCode = &exitCode
		}
	}
	if message := strings.TrimSpace(values["message"]); message != "" {
		result.Message = message
	}
	if metaFlag(values, "cg-oom-killed") {
		result.MemoryLimit = true
		if result.Message == "" {
			result.Message = "memory limit exceeded"
		}
	}

	switch values["status"] {
	case "TO":
		result.TimedOut = true
		if result.Message == "" {
			result.Message = "time limit exceeded"
		}
	case "XX":
		if result.Message == "" {
			result.Message = "isolate internal error"
		}
		result.Err = errors.New(result.Message)
	case "SG":
		if result.ExitCode == nil {
			exitCode := -1
			result.ExitCode = &exitCode
		}
	}
}

func metaFlag(values meta, key string) bool {
	value, ok := values[key]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "no":
		return false
	default:
		return true
	}
}

func parseSecondsMS(value string) (int64, bool) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return int64(seconds * 1000), true
}

func parseInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed, true
}
