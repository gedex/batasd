package execution

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

func TestEngineRunMapsAcceptedAndWrongAnswer(t *testing.T) {
	exitCode := 0
	registry := fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}
	runner := fakeRunner{
		result: sandbox.Result{
			Stdout:   "hello\n",
			ExitCode: &exitCode,
		},
	}

	engine := NewEngine(registry, runner, t.TempDir())

	expected := "hello"
	accepted := engine.Run(context.Background(), &submission.Submission{
		Token:          "sub_test",
		Language:       "python-3.12",
		Source:         "print('hello')",
		ExpectedOutput: &expected,
		Limits:         submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})
	if accepted.StatusCode != status.Accepted {
		t.Fatalf("expected accepted, got %s", accepted.StatusCode)
	}

	wrongExpected := "bye"
	wrongAnswer := engine.Run(context.Background(), &submission.Submission{
		Token:          "sub_test",
		Language:       "python-3.12",
		Source:         "print('hello')",
		ExpectedOutput: &wrongExpected,
		Limits:         submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})
	if wrongAnswer.StatusCode != status.WrongAnswer {
		t.Fatalf("expected wrong answer, got %s", wrongAnswer.StatusCode)
	}
}

func TestEngineRunMapsRuntimeError(t *testing.T) {
	exitCode := 1
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		result: sandbox.Result{
			Stderr:   "boom\n",
			ExitCode: &exitCode,
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "raise Exception('boom')",
		Limits:   submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})
	if result.StatusCode != status.RuntimeError {
		t.Fatalf("expected runtime error, got %s", result.StatusCode)
	}
}

func TestEngineRunMapsOutputLimitTruncation(t *testing.T) {
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		result: sandbox.Result{
			Stdout:      "xxxx",
			OutputLimit: true,
			StdoutLimit: true,
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "print('too much')",
		Limits:   submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1},
	})
	if result.StatusCode != status.OutputLimitExceeded {
		t.Fatalf("expected output limit exceeded, got %s", result.StatusCode)
	}
	if !result.StdoutTruncated {
		t.Fatal("StdoutTruncated = false, want true")
	}
	if result.StderrTruncated {
		t.Fatal("StderrTruncated = true, want false")
	}
}

func TestEngineCompileOutputLimitSetsCompileOutputTruncated(t *testing.T) {
	engine := NewEngine(fakeRegistry{
		"c-test": {
			Slug:       "c-test",
			SourceFile: "main.c",
			Compile:    []string{"compiler", "main.c"},
			Run:        []string{"./main"},
			Enabled:    true,
		},
	}, fakeRunner{
		run: func(command sandbox.Command) sandbox.Result {
			if command.Args[0] == "compiler" {
				return sandbox.Result{
					Stderr:      "compile output",
					OutputLimit: true,
					StderrLimit: true,
				}
			}
			t.Fatalf("unexpected run command: %v", command.Args)
			return sandbox.Result{}
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "c-test",
		Source:   "int main() { return 0; }",
		Limits:   submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1},
	})
	if result.StatusCode != status.OutputLimitExceeded {
		t.Fatalf("expected output limit exceeded, got %s", result.StatusCode)
	}
	if !result.StderrTruncated {
		t.Fatal("StderrTruncated = false, want true")
	}
	if !result.CompileOutputTruncated {
		t.Fatal("CompileOutputTruncated = false, want true")
	}
}

func TestEngineRunMapsMemoryLimitExceeded(t *testing.T) {
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		result: sandbox.Result{
			MemoryLimit: true,
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "bytearray(1024 * 1024 * 512)",
		Limits:   submission.Limits{WallTimeMS: 1000, MemoryKB: 64000, MaxOutputKB: 1024},
	})
	if result.StatusCode != status.MemoryLimitExceeded {
		t.Fatalf("expected memory limit exceeded, got %s", result.StatusCode)
	}
	if result.Message == nil || *result.Message != "memory limit exceeded" {
		t.Fatalf("message = %v, want memory limit exceeded", result.Message)
	}
}

func TestEngineRunDoesNotTreatExit137AsMemoryLimit(t *testing.T) {
	exitCode := 137
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		result: sandbox.Result{
			ExitCode: &exitCode,
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "raise SystemExit(137)",
		Limits:   submission.Limits{WallTimeMS: 1000, MemoryKB: 64000, MaxOutputKB: 1024},
	})
	if result.StatusCode != status.RuntimeError {
		t.Fatalf("expected runtime error, got %s", result.StatusCode)
	}
}

func TestEngineRunExecutesMultipleRuns(t *testing.T) {
	exitCode := 0
	calls := 0
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		run: func(command sandbox.Command) sandbox.Result {
			calls++
			memoryKB := int64(calls * 100)
			return sandbox.Result{
				Stdout:     "hello\n",
				ExitCode:   &exitCode,
				TimeMS:     int64(calls * 10),
				WallTimeMS: int64(calls * 15),
				MemoryKB:   &memoryKB,
			}
		},
	}, t.TempDir())

	expected := "hello"
	result := engine.Run(context.Background(), &submission.Submission{
		Token:          "sub_test",
		Language:       "python-3.12",
		Source:         "print('hello')",
		ExpectedOutput: &expected,
		Limits:         submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024, Runs: 3},
	})
	if result.StatusCode != status.Accepted {
		t.Fatalf("expected accepted, got %s", result.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if result.TimeMS == nil || *result.TimeMS != 20 {
		t.Fatalf("TimeMS = %v, want 20", result.TimeMS)
	}
	if result.WallTimeMS == nil || *result.WallTimeMS != 30 {
		t.Fatalf("WallTimeMS = %v, want 30", result.WallTimeMS)
	}
	if result.MemoryKB == nil || *result.MemoryKB != 300 {
		t.Fatalf("MemoryKB = %v, want 300", result.MemoryKB)
	}
}

func TestEngineRunStopsOnFailedRepeatedRun(t *testing.T) {
	successCode := 0
	failedCode := 1
	calls := 0
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		run: func(command sandbox.Command) sandbox.Result {
			calls++
			if calls == 2 {
				return sandbox.Result{Stderr: "boom\n", ExitCode: &failedCode}
			}
			return sandbox.Result{Stdout: "hello\n", ExitCode: &successCode}
		},
	}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "print('hello')",
		Limits:   submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024, Runs: 3},
	})
	if result.StatusCode != status.RuntimeError {
		t.Fatalf("expected runtime error, got %s", result.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestEngineRunExtractsAdditionalFiles(t *testing.T) {
	exitCode := 0
	var inspected bool
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{
		run: func(command sandbox.Command) sandbox.Result {
			got, err := os.ReadFile(filepath.Join(command.Dir, "lib", "helper.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "hello from helper" {
				t.Fatalf("helper.txt = %q, want helper content", got)
			}
			inspected = true
			return sandbox.Result{
				Stdout:   "hello\n",
				ExitCode: &exitCode,
			}
		},
	}, t.TempDir())

	expected := "hello"
	result := engine.Run(context.Background(), &submission.Submission{
		Token:          "sub_test",
		Language:       "python-3.12",
		Source:         "print('hello')",
		ExpectedOutput: &expected,
		AdditionalFiles: &submission.AdditionalFiles{
			Encoding: "zip_base64",
			Content:  zipBase64(t, map[string]string{"lib/helper.txt": "hello from helper"}),
		},
		Limits: submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})

	if result.StatusCode != status.Accepted {
		t.Fatalf("expected accepted, got %s: %v", result.StatusCode, result.Message)
	}
	if !inspected {
		t.Fatal("runner did not inspect extracted files")
	}
}

func TestEngineRunRejectsAdditionalFilesPathTraversal(t *testing.T) {
	workDir := t.TempDir()
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{}, workDir)

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "print('hello')",
		AdditionalFiles: &submission.AdditionalFiles{
			Encoding: "zip_base64",
			Content:  zipBase64(t, map[string]string{"../escape.txt": "bad"}),
		},
		Limits: submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})

	if result.StatusCode != status.SandboxError {
		t.Fatalf("expected sandbox error, got %s", result.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(workDir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("escape file stat error = %v, want not exist", err)
	}
}

func TestEngineRunRejectsAdditionalFilesSourceOverwrite(t *testing.T) {
	engine := NewEngine(fakeRegistry{
		"python-3.12": {
			Slug:       "python-3.12",
			SourceFile: "main.py",
			Run:        []string{"python3", "main.py"},
			Enabled:    true,
		},
	}, fakeRunner{}, t.TempDir())

	result := engine.Run(context.Background(), &submission.Submission{
		Token:    "sub_test",
		Language: "python-3.12",
		Source:   "print('hello')",
		AdditionalFiles: &submission.AdditionalFiles{
			Encoding: "zip_base64",
			Content:  zipBase64(t, map[string]string{"main.py": "print('overwrite')"}),
		},
		Limits: submission.Limits{WallTimeMS: 1000, MaxOutputKB: 1024},
	})

	if result.StatusCode != status.SandboxError {
		t.Fatalf("expected sandbox error, got %s", result.StatusCode)
	}
}

func zipBase64(t *testing.T, files map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

type fakeRegistry map[string]language.Language

func (r fakeRegistry) Get(slug string) (language.Language, bool) {
	lang, ok := r[slug]
	return lang, ok
}

type fakeRunner struct {
	result sandbox.Result
	run    func(sandbox.Command) sandbox.Result
}

func (r fakeRunner) Run(_ context.Context, command sandbox.Command) sandbox.Result {
	if r.run != nil {
		return r.run(command)
	}
	return r.result
}
