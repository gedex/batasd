// Package callback delivers completion webhooks for submissions.
package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gedex/batasd/internal/submission"
)

const defaultTimeout = 5 * time.Second

// Attempt describes one callback delivery attempt.
type Attempt struct {
	SubmissionToken string
	Attempt         int
	StatusCode      *int
	Error           *string
	CreatedAt       time.Time
}

// AttemptRepository records callback delivery attempts.
type AttemptRepository interface {
	CreateCallbackAttempt(ctx context.Context, attempt Attempt) error
}

// Deliverer sends completion callbacks and records their outcomes.
type Deliverer struct {
	repo   AttemptRepository
	client *http.Client
}

// NewDeliverer creates a callback deliverer with an HTTP client timeout.
func NewDeliverer(repo AttemptRepository, timeout time.Duration) *Deliverer {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Deliverer{
		repo: repo,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Deliver posts the completed submission response to its callback URL.
func (d *Deliverer) Deliver(ctx context.Context, sub *submission.Submission, result submission.Result) error {
	if sub.CallbackURL == nil {
		return nil
	}

	callbackURL := strings.TrimSpace(*sub.CallbackURL)
	if callbackURL == "" {
		return nil
	}

	attempt := Attempt{
		SubmissionToken: sub.Token,
		Attempt:         1,
		CreatedAt:       time.Now().UTC(),
	}

	err := d.post(ctx, callbackURL, sub, result, &attempt)
	if recordErr := d.record(ctx, attempt); recordErr != nil {
		if err != nil {
			return fmt.Errorf("%w; record callback attempt: %v", err, recordErr)
		}
		return fmt.Errorf("record callback attempt: %w", recordErr)
	}
	return err
}

func (d *Deliverer) post(ctx context.Context, callbackURL string, sub *submission.Submission, result submission.Result, attempt *Attempt) error {
	body, err := json.Marshal(submission.ToResultResponse(sub, result))
	if err != nil {
		return d.markError(attempt, fmt.Errorf("encode callback payload: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return d.markError(attempt, fmt.Errorf("create callback request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "batasd-callback/1")

	resp, err := d.client.Do(req)
	if err != nil {
		return d.markError(attempt, fmt.Errorf("send callback: %w", err))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	attempt.StatusCode = &resp.StatusCode
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return d.markError(attempt, fmt.Errorf("callback returned status %d", resp.StatusCode))
	}
	return nil
}

func (d *Deliverer) markError(attempt *Attempt, err error) error {
	message := err.Error()
	attempt.Error = &message
	return err
}

func (d *Deliverer) record(ctx context.Context, attempt Attempt) error {
	if d.repo == nil {
		return nil
	}
	return d.repo.CreateCallbackAttempt(ctx, attempt)
}
