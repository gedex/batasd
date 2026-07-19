package sandbox

import (
	"testing"

	"github.com/gedex/batasd/internal/submission"
)

func TestOutputBufferCapturesWithinLimit(t *testing.T) {
	var canceled bool
	buffer := NewOutputBuffer(5, func() { canceled = true })

	n, err := buffer.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("n = %d, want 5", n)
	}
	if buffer.String() != "hello" {
		t.Fatalf("String = %q, want hello", buffer.String())
	}
	if buffer.Exceeded() {
		t.Fatal("Exceeded = true, want false")
	}
	if canceled {
		t.Fatal("cancel callback ran before limit was exceeded")
	}
}

func TestOutputBufferCancelsOnceWhenLimitExceeded(t *testing.T) {
	calls := 0
	buffer := NewOutputBuffer(5, func() { calls++ })

	n, err := buffer.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello world") {
		t.Fatalf("n = %d, want %d", n, len("hello world"))
	}
	if buffer.String() != "hello" {
		t.Fatalf("String = %q, want hello", buffer.String())
	}
	if !buffer.Exceeded() {
		t.Fatal("Exceeded = false, want true")
	}
	if calls != 1 {
		t.Fatalf("cancel calls = %d, want 1", calls)
	}

	_, _ = buffer.Write([]byte("again"))
	if calls != 1 {
		t.Fatalf("cancel calls = %d, want 1 after repeated overflow", calls)
	}
}

func TestMaxOutputBytesUsesDefaultForMissingLimit(t *testing.T) {
	if got := MaxOutputBytes(submission.Limits{}); got != defaultMaxOutputBytes {
		t.Fatalf("MaxOutputBytes = %d, want %d", got, defaultMaxOutputBytes)
	}
	if got := MaxOutputBytes(submission.Limits{MaxOutputKB: 2}); got != 2048 {
		t.Fatalf("MaxOutputBytes = %d, want 2048", got)
	}
}
