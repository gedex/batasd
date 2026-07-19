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
