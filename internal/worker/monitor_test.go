package worker

import (
	"testing"
	"time"
)

func TestMonitorSnapshotSortsWorkers(t *testing.T) {
	monitor := NewMonitor(2)
	monitor.MarkIdle(2)
	monitor.MarkIdle(1)

	states := monitor.Snapshot(time.Now())
	if len(states) != 2 {
		t.Fatalf("len(states) = %d, want 2", len(states))
	}
	if states[0].ID != 1 || states[1].ID != 2 {
		t.Fatalf("states = %#v, want sorted by id", states)
	}
	if states[0].State != workerStateIdle || states[1].State != workerStateIdle {
		t.Fatalf("states = %#v, want idle workers", states)
	}
}

func TestMonitorSnapshotReportsProcessingWorker(t *testing.T) {
	monitor := NewMonitor(1)
	startedAt := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	monitor.MarkProcessing(1, "sub_test", startedAt)

	states := monitor.Snapshot(startedAt.Add(1500 * time.Millisecond))
	if len(states) != 1 {
		t.Fatalf("len(states) = %d, want 1", len(states))
	}

	state := states[0]
	if state.State != workerStateProcessing {
		t.Fatalf("State = %q, want processing", state.State)
	}
	if state.CurrentToken == nil || *state.CurrentToken != "sub_test" {
		t.Fatalf("CurrentToken = %v, want sub_test", state.CurrentToken)
	}
	if state.StartedAt == nil || !state.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", state.StartedAt, startedAt)
	}
	if state.RunningMS == nil || *state.RunningMS != 1500 {
		t.Fatalf("RunningMS = %v, want 1500", state.RunningMS)
	}
}

func TestMonitorStoppedClearsProcessingFields(t *testing.T) {
	monitor := NewMonitor(1)
	monitor.MarkProcessing(1, "sub_test", time.Now())
	monitor.MarkStopped(1)

	state := monitor.Snapshot(time.Now())[0]
	if state.State != workerStateStopped {
		t.Fatalf("State = %q, want stopped", state.State)
	}
	if state.CurrentToken != nil {
		t.Fatalf("CurrentToken = %v, want nil", state.CurrentToken)
	}
	if state.StartedAt != nil {
		t.Fatalf("StartedAt = %v, want nil", state.StartedAt)
	}
	if state.RunningMS != nil {
		t.Fatalf("RunningMS = %v, want nil", state.RunningMS)
	}
}
