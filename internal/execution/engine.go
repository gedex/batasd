// Package execution translates submissions into sandbox commands and statuses.
package execution

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

const (
	maxAdditionalFilesArchiveBytes   = 10 * 1024 * 1024
	maxAdditionalFilesExtractedBytes = 20 * 1024 * 1024
	maxAdditionalFilesEntries        = 256
)

// LanguageRegistry resolves enabled languages by slug.
type LanguageRegistry interface {
	Get(slug string) (language.Language, bool)
}

// Engine executes submissions using a language catalog and sandbox runner.
type Engine struct {
	languages LanguageRegistry
	runner    sandbox.Runner
	workDir   string
}

// NewEngine creates an execution engine rooted at workDir.
func NewEngine(languages LanguageRegistry, runner sandbox.Runner, workDir string) *Engine {
	return &Engine{
		languages: languages,
		runner:    runner,
		workDir:   workDir,
	}
}

// Run executes sub and returns the result to persist.
func (e *Engine) Run(ctx context.Context, sub *submission.Submission) submission.Result {
	finishedAt := time.Now().UTC()

	lang, ok := e.languages.Get(sub.Language)
	if !ok {
		return result(status.InternalError, finishedAt, withMessage("unsupported language"))
	}

	if err := validateTokens("arguments", sub.Arguments); err != nil {
		return result(status.InternalError, finishedAt, withMessage(err.Error()))
	}
	if err := validateTokens("compiler_options", sub.CompilerOptions); err != nil {
		return result(status.InternalError, finishedAt, withMessage(err.Error()))
	}

	if err := os.MkdirAll(e.workDir, 0o700); err != nil {
		return result(status.SandboxError, finishedAt, withMessage(err.Error()))
	}

	dir, err := os.MkdirTemp(e.workDir, "sub-"+sub.Token+"-")
	if err != nil {
		return result(status.SandboxError, finishedAt, withMessage(err.Error()))
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o777); err != nil {
		return result(status.SandboxError, finishedAt, withMessage(err.Error()))
	}

	sourcePath := filepath.Join(dir, lang.SourceFile)
	if err := os.WriteFile(sourcePath, []byte(sub.Source), 0o644); err != nil {
		return result(status.SandboxError, finishedAt, withMessage(err.Error()))
	}
	if sub.AdditionalFiles != nil {
		if err := extractAdditionalFiles(dir, sub.AdditionalFiles); err != nil {
			return result(status.SandboxError, finishedAt, withMessage(err.Error()))
		}
	}

	if len(lang.Compile) > 0 {
		compile := e.runner.Run(ctx, sandbox.Command{
			Args:   appendArgs(lang.Compile, sub.CompilerOptions),
			Dir:    dir,
			Limits: sub.Limits,
		})
		if compile.OutputLimit {
			return fromSandbox(status.OutputLimitExceeded, compile, compileOutput(compile), "compile output limit exceeded")
		}
		if compile.TimedOut {
			return fromSandbox(status.TimeLimitExceeded, compile, compileOutput(compile), "compile time limit exceeded")
		}
		if compile.MemoryLimit {
			return fromSandbox(status.MemoryLimitExceeded, compile, compileOutput(compile), "memory limit exceeded")
		}
		if compile.Err != nil {
			return fromSandbox(status.SandboxError, compile, compileOutput(compile), compile.Err.Error())
		}
		if compile.ExitCode != nil && *compile.ExitCode != 0 {
			return fromSandbox(status.CompilationError, compile, compileOutput(compile), "compilation failed")
		}
	}

	run, failed, ok := e.runSubmission(ctx, lang, sub, dir)
	if !ok {
		return failed
	}

	return fromSandbox(status.Accepted, run, nil, "")
}

func (e *Engine) runSubmission(ctx context.Context, lang language.Language, sub *submission.Submission, dir string) (sandbox.Result, submission.Result, bool) {
	aggregate := runAggregate{}
	for range runCount(sub.Limits) {
		run := e.runner.Run(ctx, sandbox.Command{
			Args:   appendArgs(lang.Run, sub.Arguments),
			Dir:    dir,
			Input:  sub.Input,
			Limits: sub.Limits,
		})

		switch {
		case run.OutputLimit:
			return run, fromSandbox(status.OutputLimitExceeded, run, nil, "output limit exceeded"), false
		case run.TimedOut:
			return run, fromSandbox(status.TimeLimitExceeded, run, nil, "time limit exceeded"), false
		case run.MemoryLimit:
			return run, fromSandbox(status.MemoryLimitExceeded, run, nil, "memory limit exceeded"), false
		case run.Err != nil:
			return run, fromSandbox(status.SandboxError, run, nil, run.Err.Error()), false
		case run.ExitCode != nil && *run.ExitCode != 0:
			return run, fromSandbox(status.RuntimeError, run, nil, "program exited with non-zero status"), false
		}

		if sub.ExpectedOutput != nil && trimTrailingWhitespace(run.Stdout) != trimTrailingWhitespace(*sub.ExpectedOutput) {
			return run, fromSandbox(status.WrongAnswer, run, nil, "output did not match expected output"), false
		}

		aggregate.Add(run)
	}

	return aggregate.Result(), submission.Result{}, true
}

func runCount(limits submission.Limits) int {
	if limits.Runs <= 0 {
		return 1
	}
	return limits.Runs
}

type runAggregate struct {
	result     sandbox.Result
	count      int
	timeMS     int64
	wallTimeMS int64
	memoryKB   *int64
}

func (a *runAggregate) Add(run sandbox.Result) {
	a.result = run
	a.count++
	a.timeMS += run.TimeMS
	a.wallTimeMS += run.WallTimeMS
	if run.MemoryKB != nil && (a.memoryKB == nil || *run.MemoryKB > *a.memoryKB) {
		value := *run.MemoryKB
		a.memoryKB = &value
	}
}

func (a runAggregate) Result() sandbox.Result {
	result := a.result
	if a.count == 0 {
		return result
	}
	result.TimeMS = a.timeMS / int64(a.count)
	result.WallTimeMS = a.wallTimeMS / int64(a.count)
	result.MemoryKB = a.memoryKB
	return result
}

func extractAdditionalFiles(dir string, files *submission.AdditionalFiles) error {
	if files == nil {
		return nil
	}
	if files.Encoding != "zip_base64" {
		return fmt.Errorf("unsupported additional_files encoding %q", files.Encoding)
	}

	archive, err := decodeAdditionalFilesArchive(files.Content)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("read additional_files zip: %w", err)
	}
	if len(reader.File) > maxAdditionalFilesEntries {
		return fmt.Errorf("additional_files cannot contain more than %d entries", maxAdditionalFilesEntries)
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	seen := map[string]struct{}{}
	var extractedBytes int64
	for _, file := range reader.File {
		target, cleanName, err := safeZipTarget(root, file.Name)
		if err != nil {
			return err
		}
		if _, ok := seen[cleanName]; ok {
			return fmt.Errorf("additional_files contains duplicate path %q", cleanName)
		}
		seen[cleanName] = struct{}{}

		mode := file.FileInfo().Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("additional_files path %q cannot be a symlink", cleanName)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if !mode.IsRegular() {
			return fmt.Errorf("additional_files path %q must be a regular file", cleanName)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		remaining := maxAdditionalFilesExtractedBytes - extractedBytes
		if remaining <= 0 {
			return fmt.Errorf("additional_files extracted content exceeds %d bytes", maxAdditionalFilesExtractedBytes)
		}
		n, err := extractZipFile(file, target, remaining)
		if err != nil {
			return err
		}
		extractedBytes += n
	}

	return nil
}

func decodeAdditionalFilesArchive(content string) ([]byte, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("additional_files content is required")
	}
	if len(content) > base64.StdEncoding.EncodedLen(maxAdditionalFilesArchiveBytes) {
		return nil, fmt.Errorf("additional_files archive exceeds %d bytes", maxAdditionalFilesArchiveBytes)
	}

	archive, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("decode additional_files content: %w", err)
	}
	if len(archive) > maxAdditionalFilesArchiveBytes {
		return nil, fmt.Errorf("additional_files archive exceeds %d bytes", maxAdditionalFilesArchiveBytes)
	}
	return archive, nil
}

func safeZipTarget(root, name string) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("additional_files contains an empty path")
	}
	if strings.Contains(name, "\\") {
		return "", "", fmt.Errorf("additional_files path %q cannot contain backslashes", name)
	}

	clean := path.Clean(name)
	if clean == "." || clean == ".." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return "", "", fmt.Errorf("additional_files path %q is not allowed", name)
	}

	target := filepath.Join(root, filepath.FromSlash(clean))
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	if absTarget != root && !strings.HasPrefix(absTarget, root+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("additional_files path %q escapes the execution directory", name)
	}
	return absTarget, clean, nil
}

func extractZipFile(file *zip.File, target string, remaining int64) (int64, error) {
	source, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	defer destination.Close()

	limited := &io.LimitedReader{R: source, N: remaining + 1}
	written, err := io.Copy(destination, limited)
	if err != nil {
		return written, err
	}
	if written > remaining {
		return written, fmt.Errorf("additional_files extracted content exceeds %d bytes", maxAdditionalFilesExtractedBytes)
	}
	return written, nil
}

func appendArgs(base []string, extra []string) []string {
	args := make([]string, 0, len(base)+len(extra))
	args = append(args, base...)
	args = append(args, extra...)
	return args
}

func validateTokens(field string, values []string) error {
	if len(values) > 64 {
		return fmt.Errorf("%s cannot contain more than 64 values", field)
	}
	for i, value := range values {
		if value == "" {
			return fmt.Errorf("%s[%d] cannot be empty", field, i)
		}
		if len(value) > 256 {
			return fmt.Errorf("%s[%d] cannot be longer than 256 bytes", field, i)
		}
		for _, r := range value {
			if r == utf8.RuneError {
				return fmt.Errorf("%s[%d] must be valid UTF-8", field, i)
			}
			if unicode.IsControl(r) {
				return fmt.Errorf("%s[%d] cannot contain control characters", field, i)
			}
		}
	}
	return nil
}

func trimTrailingWhitespace(value string) string {
	return strings.TrimRightFunc(value, unicode.IsSpace)
}

func compileOutput(run sandbox.Result) *string {
	output := run.Stdout
	if run.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += run.Stderr
	}
	if output == "" {
		return nil
	}
	return &output
}

func fromSandbox(statusCode string, run sandbox.Result, compileOutput *string, message string) submission.Result {
	finishedAt := time.Now().UTC()
	msg := ptrIfNotEmpty(message)
	if msg == nil {
		msg = ptrIfNotEmpty(run.Message)
	}

	return submission.Result{
		StatusCode:    statusCode,
		Stdout:        ptrIfNotEmpty(run.Stdout),
		Stderr:        ptrIfNotEmpty(run.Stderr),
		CompileOutput: compileOutput,
		Message:       msg,
		ExitCode:      run.ExitCode,
		ExitSignal:    run.ExitSignal,
		TimeMS:        int64Ptr(run.TimeMS),
		WallTimeMS:    int64Ptr(run.WallTimeMS),
		MemoryKB:      run.MemoryKB,
		FinishedAt:    finishedAt,
	}
}

type resultOption func(*submission.Result)

func result(statusCode string, finishedAt time.Time, opts ...resultOption) submission.Result {
	out := submission.Result{StatusCode: statusCode, FinishedAt: finishedAt}
	for _, opt := range opts {
		opt(&out)
	}
	return out
}

func withMessage(message string) resultOption {
	return func(result *submission.Result) {
		result.Message = ptrIfNotEmpty(message)
	}
}

func ptrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
