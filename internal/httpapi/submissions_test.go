package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

func TestParseSubmissionListQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions?limit=3&before=sub_1&status=accepted&language=python", nil)

	query, err := parseSubmissionListQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	if query.Limit != 3 {
		t.Fatalf("limit = %d, want 3", query.Limit)
	}
	if query.BeforeToken != "sub_1" {
		t.Fatalf("before = %q, want sub_1", query.BeforeToken)
	}
	if query.StatusCode != "accepted" {
		t.Fatalf("status = %q, want accepted", query.StatusCode)
	}
	if query.Language != "python" {
		t.Fatalf("language = %q, want python", query.Language)
	}
}

func TestParseSubmissionListQueryRejectsInvalidLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions?limit=many", nil)

	_, err := parseSubmissionListQuery(req)
	if err == nil {
		t.Fatal("parseSubmissionListQuery returned nil error, want validation error")
	}

	var validation submission.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "limit" {
		t.Fatalf("field = %q, want limit", validation.Field)
	}
}

func TestCreateWithWaitTimeoutReturnsAcceptedAndCurrentSubmission(t *testing.T) {
	repo := &waitTestRepository{}
	service := submission.NewService(submission.ServiceConfig{
		Repository: repo,
		Queue:      &waitTestQueue{},
		Languages: waitTestLanguages{
			"python": {
				Slug:    "python",
				Version: "3.12",
			},
		},
	})
	handler := SubmissionHandler{
		Service:          service,
		WaitTimeout:      time.Nanosecond,
		WaitPollInterval: time.Hour,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/submissions?wait=true", strings.NewReader(`{
		"language":"python",
		"source":"print(\"hello\")"
	}`))
	recorder := httptest.NewRecorder()

	handler.Create(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var response submission.Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status.Code != status.Queued {
		t.Fatalf("submission status = %q, want %q", response.Status.Code, status.Queued)
	}
}

func TestParseSubmissionWait(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/submissions?wait=true", nil)

	wait, err := parseSubmissionWait(req)
	if err != nil {
		t.Fatal(err)
	}
	if !wait {
		t.Fatal("wait = false, want true")
	}
}

func TestParseSubmissionWaitDefaultsFalse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/submissions", nil)

	wait, err := parseSubmissionWait(req)
	if err != nil {
		t.Fatal(err)
	}
	if wait {
		t.Fatal("wait = true, want false")
	}
}

func TestParseSubmissionWaitRejectsInvalidValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/submissions?wait=eventually", nil)

	_, err := parseSubmissionWait(req)
	if err == nil {
		t.Fatal("parseSubmissionWait returned nil error, want validation error")
	}

	var validation submission.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "wait" {
		t.Fatalf("field = %q, want wait", validation.Field)
	}
}

func TestParseSubmissionFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions/sub_test?fields=token,status,stdout,token", nil)

	fields, err := parseSubmissionFields(req)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"token", "status", "stdout"}
	if len(fields) != len(want) {
		t.Fatalf("len(fields) = %d, want %d", len(fields), len(want))
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("fields[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}

func TestParseSubmissionFieldsAllDefaultsToNil(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions/sub_test?fields=*", nil)

	fields, err := parseSubmissionFields(req)
	if err != nil {
		t.Fatal(err)
	}
	if fields != nil {
		t.Fatalf("fields = %#v, want nil", fields)
	}
}

func TestParseSubmissionFieldsRejectsInvalidValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions/sub_test?fields=token,source", nil)

	_, err := parseSubmissionFields(req)
	if err == nil {
		t.Fatal("parseSubmissionFields returned nil error, want validation error")
	}

	var validation submission.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "fields" {
		t.Fatalf("field = %q, want fields", validation.Field)
	}
}

func TestParseSubmissionFieldsRejectsMixedWildcard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions/sub_test?fields=token,*", nil)

	_, err := parseSubmissionFields(req)
	if err == nil {
		t.Fatal("parseSubmissionFields returned nil error, want validation error")
	}

	var validation submission.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "fields" {
		t.Fatalf("field = %q, want fields", validation.Field)
	}
}

type waitTestLanguages map[string]language.Runtime

func (l waitTestLanguages) Get(slug string) (language.Language, bool) {
	runtime, ok := l.Resolve(slug, "")
	if !ok {
		return language.Language{}, false
	}
	return language.Language{
		Slug:           runtime.Slug,
		Name:           runtime.Name,
		DefaultVersion: runtime.Version,
		Versions: []language.Version{
			{
				Version:    runtime.Version,
				SourceFile: runtime.SourceFile,
				Compile:    runtime.Compile,
				Run:        runtime.Run,
				Enabled:    true,
			},
		},
		Enabled: true,
	}, true
}

func (l waitTestLanguages) Resolve(slug, version string) (language.Runtime, bool) {
	runtime, ok := l[slug]
	if !ok {
		return language.Runtime{}, false
	}
	if version != "" && version != runtime.Version {
		return language.Runtime{}, false
	}
	return runtime, true
}

type waitTestQueue struct{}

func (q *waitTestQueue) Enqueue(_ context.Context, _ string) error {
	return nil
}

type waitTestRepository struct {
	sub *submission.Submission
}

func (r *waitTestRepository) Create(_ context.Context, sub *submission.Submission) error {
	r.sub = sub
	return nil
}

func (r *waitTestRepository) FindByToken(_ context.Context, token string) (*submission.Submission, error) {
	if r.sub == nil || r.sub.Token != token {
		return nil, submission.ErrNotFound
	}
	return r.sub, nil
}

func (r *waitTestRepository) List(_ context.Context, query submission.ListQuery) (submission.ListResult, error) {
	return submission.ListResult{Limit: query.Limit}, nil
}

func (r *waitTestRepository) MarkProcessing(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (r *waitTestRepository) StoreResult(_ context.Context, _ string, _ submission.Result) error {
	return nil
}

func (r *waitTestRepository) RecoverUnfinished(_ context.Context, _ time.Time) ([]string, error) {
	return nil, nil
}

func (r *waitTestRepository) CreateCallbackAttempt(_ context.Context, _ submission.CallbackAttempt) error {
	return nil
}

func (r *waitTestRepository) ListCallbackAttempts(_ context.Context, _ string) ([]submission.CallbackAttempt, error) {
	return nil, nil
}
