package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gedex/batasd/internal/submission"
)

func TestParseSubmissionListQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/submissions?limit=3&before=sub_1&status=accepted&language=python-3.12", nil)

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
	if query.Language != "python-3.12" {
		t.Fatalf("language = %q, want python-3.12", query.Language)
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
