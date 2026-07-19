package submission

import (
	"testing"
	"time"

	"github.com/gedex/batasd/internal/status"
)

func TestToResponseIncludesTruncationFlags(t *testing.T) {
	sub := &Submission{
		Token:                  "sub_test",
		StatusCode:             status.OutputLimitExceeded,
		StdoutTruncated:        true,
		CompileOutputTruncated: true,
		CreatedAt:              time.Now().UTC(),
	}

	response := ToResponse(sub)
	if !response.StdoutTruncated {
		t.Fatal("StdoutTruncated = false, want true")
	}
	if response.StderrTruncated {
		t.Fatal("StderrTruncated = true, want false")
	}
	if !response.CompileOutputTruncated {
		t.Fatal("CompileOutputTruncated = false, want true")
	}
}

func TestToResultResponseIncludesTruncationFlags(t *testing.T) {
	sub := &Submission{
		Token:      "sub_test",
		StatusCode: status.Processing,
		CreatedAt:  time.Now().UTC(),
	}

	response := ToResultResponse(sub, Result{
		StatusCode:             status.OutputLimitExceeded,
		StderrTruncated:        true,
		CompileOutputTruncated: true,
		FinishedAt:             time.Now().UTC(),
	})
	if response.StdoutTruncated {
		t.Fatal("StdoutTruncated = true, want false")
	}
	if !response.StderrTruncated {
		t.Fatal("StderrTruncated = false, want true")
	}
	if !response.CompileOutputTruncated {
		t.Fatal("CompileOutputTruncated = false, want true")
	}
}

func TestValidResponseField(t *testing.T) {
	if !ValidResponseField("token") {
		t.Fatal("ValidResponseField(token) = false, want true")
	}
	if ValidResponseField("source") {
		t.Fatal("ValidResponseField(source) = true, want false")
	}
}

func TestSelectResponseFields(t *testing.T) {
	response := Response{
		Token:      "sub_test",
		Status:     status.Describe(status.Accepted),
		Language:   "python-3.12",
		Stdout:     stringPtr("hello\n"),
		CreatedAt:  "2026-07-19T00:00:00Z",
		Limits:     Limits{Runs: 1},
		Arguments:  []string{"one"},
		MemoryKB:   int64Ptr(42),
		ExitSignal: stringPtr("KILL"),
	}

	selected := SelectResponseFields(response, []string{"token", "status", "stdout", "memory_kb"})
	if len(selected) != 4 {
		t.Fatalf("len(selected) = %d, want 4", len(selected))
	}
	if selected["token"] != "sub_test" {
		t.Fatalf("token = %v, want sub_test", selected["token"])
	}
	if selected["stdout"] == nil {
		t.Fatal("stdout = nil, want selected stdout pointer")
	}
	if _, ok := selected["language"]; ok {
		t.Fatal("language was selected unexpectedly")
	}
}

func TestSelectListResponseFieldsKeepsPagination(t *testing.T) {
	nextBefore := "sub_next"
	response := ListResponse{
		Submissions: []Response{
			{
				Token:    "sub_test",
				Language: "python-3.12",
				Status:   status.Describe(status.Accepted),
			},
		},
		Pagination: PaginationInfo{
			Limit:      1,
			NextBefore: &nextBefore,
		},
	}

	selected := SelectListResponseFields(response, []string{"token", "status"})
	submissions, ok := selected["submissions"].([]map[string]any)
	if !ok {
		t.Fatalf("submissions = %T, want []map[string]any", selected["submissions"])
	}
	if len(submissions) != 1 {
		t.Fatalf("len(submissions) = %d, want 1", len(submissions))
	}
	if _, ok := submissions[0]["language"]; ok {
		t.Fatal("language was selected unexpectedly")
	}
	if selected["pagination"] != response.Pagination {
		t.Fatal("pagination was not preserved")
	}
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
