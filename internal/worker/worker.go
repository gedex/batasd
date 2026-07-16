// Package worker consumes queued submissions and stores execution results.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

// Queue supplies submission tokens to workers.
type Queue interface {
	Dequeue(ctx context.Context) (string, error)
	Done()
}

// Repository loads submissions and stores worker status transitions.
type Repository interface {
	FindByToken(ctx context.Context, token string) (*submission.Submission, error)
	MarkProcessing(ctx context.Context, token string, startedAt time.Time) error
	StoreResult(ctx context.Context, token string, result submission.Result) error
}

// Engine executes a submission.
type Engine interface {
	Run(ctx context.Context, submission *submission.Submission) submission.Result
}

// Worker processes queued submissions.
type Worker struct {
	logger *slog.Logger
	queue  Queue
	repo   Repository
	engine Engine
}

// New creates a worker using queue, repo, and engine.
func New(logger *slog.Logger, queue Queue, repo Repository, engine Engine) *Worker {
	return &Worker{
		logger: logger,
		queue:  queue,
		repo:   repo,
		engine: engine,
	}
}

// Run processes submissions until ctx is done.
func (w *Worker) Run(ctx context.Context, id int) {
	logger := w.logger.With("worker_id", id)
	logger.Info("worker started")
	defer logger.Info("worker stopped")

	for {
		token, err := w.queue.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.Error("dequeue submission", "error", err)
			continue
		}

		w.process(ctx, logger, token)
		w.queue.Done()
	}
}

func (w *Worker) process(ctx context.Context, logger *slog.Logger, token string) {
	sub, err := w.repo.FindByToken(ctx, token)
	if err != nil {
		if !errors.Is(err, submission.ErrNotFound) {
			logger.Error("fetch submission", "token", token, "error", err)
		}
		return
	}
	if sub.StatusCode != status.Queued {
		logger.Info("skip submission with non-queued status", "token", token, "status", sub.StatusCode)
		return
	}

	startedAt := time.Now().UTC()
	if err := w.repo.MarkProcessing(ctx, token, startedAt); err != nil {
		logger.Error("mark submission processing", "token", token, "error", err)
		return
	}

	result := w.engine.Run(ctx, sub)
	if err := w.repo.StoreResult(ctx, token, result); err != nil {
		logger.Error("store submission result", "token", token, "error", err)
		return
	}

	logger.Info("submission completed", "token", token, "status", result.StatusCode)
}
