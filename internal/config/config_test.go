package config

import (
	"strings"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/submission"
)

func TestValidateRejectsInvalidHTTPMaxBodyBytes(t *testing.T) {
	cfg := validTestConfig()
	cfg.HTTP.MaxBodyBytes = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "HTTP_MAX_BODY_BYTES") {
		t.Fatalf("error = %q, want HTTP_MAX_BODY_BYTES", err.Error())
	}
}

func TestValidateRejectsInvalidSubmissionWaitTimeout(t *testing.T) {
	cfg := validTestConfig()
	cfg.Submissions.WaitTimeout = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "SUBMISSION_WAIT_TIMEOUT_MS") {
		t.Fatalf("error = %q, want SUBMISSION_WAIT_TIMEOUT_MS", err.Error())
	}
}

func TestValidateRejectsInvalidSubmissionWaitPollInterval(t *testing.T) {
	cfg := validTestConfig()
	cfg.Submissions.WaitPollInterval = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "SUBMISSION_WAIT_POLL_INTERVAL_MS") {
		t.Fatalf("error = %q, want SUBMISSION_WAIT_POLL_INTERVAL_MS", err.Error())
	}
}

func validTestConfig() Config {
	return Config{
		AppEnv: "local",
		HTTP: HTTPConfig{
			Addr:         ":18080",
			MaxBodyBytes: 1024,
		},
		Sandbox: SandboxConfig{
			Driver:            "direct",
			IsolateBoxIDCount: 1,
		},
		Queue: QueueConfig{
			Workers: 1,
		},
		Callback: CallbackConfig{
			Timeout: time.Second,
		},
		Submissions: SubmissionConfig{
			DefaultLimits:    submission.Limits{Runs: 1},
			MaxLimits:        submission.Limits{Runs: 1},
			WaitTimeout:      time.Second,
			WaitPollInterval: time.Millisecond,
		},
	}
}
