package callback

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

func TestDeliverPostsCompletedSubmissionResponse(t *testing.T) {
	repo := &fakeAttemptRepository{}
	stdout := "hello\n"
	finishedAt := time.Date(2026, 7, 17, 1, 2, 3, 0, time.UTC)
	var received submission.Response

	deliverer := NewDeliverer(repo, time.Second)
	deliverer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		return testResponse(http.StatusNoContent), nil
	})}

	sub := testSubmission("http://callback.test/submissions")
	err := deliverer.Deliver(context.Background(), sub, submission.Result{
		StatusCode: status.Accepted,
		Stdout:     &stdout,
		FinishedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	if received.Token != "sub_test" {
		t.Fatalf("token = %q, want sub_test", received.Token)
	}
	if received.Status.Code != status.Accepted {
		t.Fatalf("status = %q, want %q", received.Status.Code, status.Accepted)
	}
	if received.Stdout == nil || *received.Stdout != stdout {
		t.Fatalf("stdout = %v, want %q", received.Stdout, stdout)
	}
	if len(repo.attempts) != 1 {
		t.Fatalf("len(attempts) = %d, want 1", len(repo.attempts))
	}
	if repo.attempts[0].StatusCode == nil || *repo.attempts[0].StatusCode != http.StatusNoContent {
		t.Fatalf("status code = %v, want %d", repo.attempts[0].StatusCode, http.StatusNoContent)
	}
	if repo.attempts[0].Error != nil {
		t.Fatalf("error = %q, want nil", *repo.attempts[0].Error)
	}
}

func TestDeliverRecordsNonSuccessStatus(t *testing.T) {
	repo := &fakeAttemptRepository{}
	deliverer := NewDeliverer(repo, time.Second)
	deliverer.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusInternalServerError), nil
	})}

	err := deliverer.Deliver(context.Background(), testSubmission("http://callback.test/submissions"), submission.Result{
		StatusCode: status.RuntimeError,
		FinishedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("Deliver returned nil error, want callback status error")
	}

	if len(repo.attempts) != 1 {
		t.Fatalf("len(attempts) = %d, want 1", len(repo.attempts))
	}
	if repo.attempts[0].StatusCode == nil || *repo.attempts[0].StatusCode != http.StatusInternalServerError {
		t.Fatalf("status code = %v, want %d", repo.attempts[0].StatusCode, http.StatusInternalServerError)
	}
	if repo.attempts[0].Error == nil {
		t.Fatal("error = nil, want callback error")
	}
}

func TestDeliverSkipsSubmissionWithoutCallbackURL(t *testing.T) {
	repo := &fakeAttemptRepository{}
	deliverer := NewDeliverer(repo, time.Second)
	err := deliverer.Deliver(context.Background(), &submission.Submission{Token: "sub_test"}, submission.Result{
		StatusCode: status.Accepted,
		FinishedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.attempts) != 0 {
		t.Fatalf("len(attempts) = %d, want 0", len(repo.attempts))
	}
}

func testSubmission(callbackURL string) *submission.Submission {
	return &submission.Submission{
		Token:       "sub_test",
		Language:    "python-3.12",
		CallbackURL: &callbackURL,
		StatusCode:  status.Processing,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

type fakeAttemptRepository struct {
	attempts []submission.CallbackAttempt
}

func (r *fakeAttemptRepository) CreateCallbackAttempt(_ context.Context, attempt submission.CallbackAttempt) error {
	r.attempts = append(r.attempts, attempt)
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     http.Header{},
	}
}
