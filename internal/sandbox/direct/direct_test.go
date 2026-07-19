package direct

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/submission"
)

func TestRunKillsProcessGroupOnTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group cancellation is Unix-specific")
	}

	start := time.Now()
	result := NewRunner().Run(context.Background(), sandbox.Command{
		Args: []string{"/bin/sh", "-c", "sleep 5 & wait"},
		Limits: submission.Limits{
			WallTimeMS:  100,
			MaxOutputKB: 1,
		},
	})

	if !result.TimedOut {
		t.Fatalf("TimedOut = false, want true; result = %#v", result)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Run took %s, want process group cleanup under 3s", elapsed)
	}
}
