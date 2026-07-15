package memory

import (
	"context"
	"sync"
)

type Queue struct {
	mu    sync.Mutex
	items []string
}

func NewQueue() *Queue {
	return &Queue{items: []string{}}
}

func (q *Queue) Enqueue(_ context.Context, token string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, token)
	return nil
}

func (q *Queue) Stats() Stats {
	q.mu.Lock()
	defer q.mu.Unlock()

	return Stats{Pending: len(q.items)}
}

type Stats struct {
	Pending int `json:"pending"`
}
