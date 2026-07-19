package main

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRecoverUnfinishedSubmissionsEnqueuesRecoveredTokens(t *testing.T) {
	repo := &fakeRecoverer{tokens: []string{"sub_old", "sub_new"}}
	queue := &fakeEnqueuer{}

	err := recoverUnfinishedSubmissions(context.Background(), slog.Default(), repo, queue)
	if err != nil {
		t.Fatal(err)
	}

	if repo.recoveredAt.IsZero() {
		t.Fatal("recoveredAt was not set")
	}
	if len(queue.tokens) != 2 {
		t.Fatalf("len(tokens) = %d, want 2", len(queue.tokens))
	}
	if queue.tokens[0] != "sub_old" || queue.tokens[1] != "sub_new" {
		t.Fatalf("tokens = %v, want [sub_old sub_new]", queue.tokens)
	}
}

type fakeRecoverer struct {
	tokens      []string
	recoveredAt time.Time
}

func (r *fakeRecoverer) RecoverUnfinished(_ context.Context, recoveredAt time.Time) ([]string, error) {
	r.recoveredAt = recoveredAt
	return r.tokens, nil
}

type fakeEnqueuer struct {
	tokens []string
}

func (q *fakeEnqueuer) Enqueue(_ context.Context, token string) error {
	q.tokens = append(q.tokens, token)
	return nil
}
