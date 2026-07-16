// Package execution translates submissions into sandbox commands and statuses.
package execution

import (
	"context"
	"fmt"
	"os"
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
	if sub.AdditionalFiles != nil {
		return result(status.InternalError, finishedAt, withMessage("additional_files are not supported yet"))
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
		if compile.Err != nil {
			return fromSandbox(status.SandboxError, compile, compileOutput(compile), compile.Err.Error())
		}
		if compile.ExitCode != nil && *compile.ExitCode != 0 {
			return fromSandbox(status.CompilationError, compile, compileOutput(compile), "compilation failed")
		}
	}

	run := e.runner.Run(ctx, sandbox.Command{
		Args:   appendArgs(lang.Run, sub.Arguments),
		Dir:    dir,
		Input:  sub.Input,
		Limits: sub.Limits,
	})

	switch {
	case run.OutputLimit:
		return fromSandbox(status.OutputLimitExceeded, run, nil, "output limit exceeded")
	case run.TimedOut:
		return fromSandbox(status.TimeLimitExceeded, run, nil, "time limit exceeded")
	case run.Err != nil:
		return fromSandbox(status.SandboxError, run, nil, run.Err.Error())
	case run.ExitCode != nil && *run.ExitCode != 0:
		return fromSandbox(status.RuntimeError, run, nil, "program exited with non-zero status")
	}

	if sub.ExpectedOutput != nil && trimTrailingWhitespace(run.Stdout) != trimTrailingWhitespace(*sub.ExpectedOutput) {
		return fromSandbox(status.WrongAnswer, run, nil, "output did not match expected output")
	}

	return fromSandbox(status.Accepted, run, nil, "")
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
