package execution

import (
	"context"
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

type fakeRegistry map[string]language.Language

func (r fakeRegistry) Get(slug string) (language.Language, bool) {
	lang, ok := r[slug]
	return lang, ok
}

type fakeRunner struct {
	result sandbox.Result
}

func (r fakeRunner) Run(context.Context, sandbox.Command) sandbox.Result {
	return r.result
}
