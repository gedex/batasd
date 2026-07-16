// Package memory provides an in-process FIFO submission queue.
package memory

import (
	"context"
	"sync"
)

// Queue is a blocking in-memory queue for local development.
type Queue struct {
	ch      chan string
	mu      sync.Mutex
	pending int
	active  int
}

// NewQueue creates an empty in-memory queue.
func NewQueue() *Queue {
	return &Queue{ch: make(chan string, 1024)}
}

// Enqueue adds token to the queue or returns ctx.Err if the context is done.
func (q *Queue) Enqueue(ctx context.Context, token string) error {
	q.mu.Lock()
	q.pending++
	q.mu.Unlock()

	select {
	case q.ch <- token:
		return nil
	case <-ctx.Done():
		q.mu.Lock()
		q.pending--
		q.mu.Unlock()
		return ctx.Err()
	}
}

// Dequeue blocks until a submission token is available or ctx is done.
func (q *Queue) Dequeue(ctx context.Context) (string, error) {
	select {
	case token := <-q.ch:
		q.mu.Lock()
		q.pending--
		q.active++
		q.mu.Unlock()
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Done marks the currently processed token as complete.
func (q *Queue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active > 0 {
		q.active--
	}
}

// Stats returns a snapshot of queue activity.
func (q *Queue) Stats() Stats {
	q.mu.Lock()
	defer q.mu.Unlock()

	return Stats{Pending: q.pending, Active: q.active}
}

// Stats describes the current queue depth and activity.
type Stats struct {
	Pending int `json:"pending"`
	Active  int `json:"active"`
}
