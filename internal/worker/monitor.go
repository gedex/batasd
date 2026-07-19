package worker

import (
	"sort"
	"sync"
	"time"
)

const (
	workerStateIdle       = "idle"
	workerStateProcessing = "processing"
	workerStateStarting   = "starting"
	workerStateStopped    = "stopped"
)

// Monitor tracks in-process worker state for runtime visibility endpoints.
type Monitor struct {
	mu      sync.Mutex
	workers map[int]workerSnapshot
}

// State is a public snapshot of one worker.
type State struct {
	ID           int        `json:"id"`
	State        string     `json:"state"`
	CurrentToken *string    `json:"current_token,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	RunningMS    *int64     `json:"running_ms,omitempty"`
}

type workerSnapshot struct {
	id           int
	state        string
	currentToken string
	startedAt    time.Time
}

// NewMonitor creates a monitor pre-populated with worker IDs from 1 to count.
func NewMonitor(count int) *Monitor {
	monitor := &Monitor{workers: make(map[int]workerSnapshot, count)}
	for id := 1; id <= count; id++ {
		monitor.workers[id] = workerSnapshot{id: id, state: workerStateStarting}
	}
	return monitor
}

// MarkIdle records that worker id is ready to receive work.
func (m *Monitor) MarkIdle(id int) {
	m.set(workerSnapshot{id: id, state: workerStateIdle})
}

// MarkProcessing records that worker id is processing token.
func (m *Monitor) MarkProcessing(id int, token string, startedAt time.Time) {
	m.set(workerSnapshot{
		id:           id,
		state:        workerStateProcessing,
		currentToken: token,
		startedAt:    startedAt.UTC(),
	})
}

// MarkStopped records that worker id has stopped.
func (m *Monitor) MarkStopped(id int) {
	m.set(workerSnapshot{id: id, state: workerStateStopped})
}

// Snapshot returns worker states sorted by worker ID.
func (m *Monitor) Snapshot(now time.Time) []State {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]int, 0, len(m.workers))
	for id := range m.workers {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	out := make([]State, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.workers[id].State(now))
	}
	return out
}

func (m *Monitor) set(snapshot workerSnapshot) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.workers[snapshot.id] = snapshot
}

func (s workerSnapshot) State(now time.Time) State {
	state := State{
		ID:    s.id,
		State: s.state,
	}
	if s.currentToken != "" {
		token := s.currentToken
		state.CurrentToken = &token
	}
	if !s.startedAt.IsZero() {
		startedAt := s.startedAt
		runningMS := now.Sub(startedAt).Milliseconds()
		if runningMS < 0 {
			runningMS = 0
		}
		state.StartedAt = &startedAt
		state.RunningMS = &runningMS
	}
	return state
}
