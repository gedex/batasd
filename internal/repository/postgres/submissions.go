// Package postgres stores submissions in PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

// SubmissionRepository persists submissions and execution results in PostgreSQL.
type SubmissionRepository struct {
	db *pgxpool.Pool
}

// NewSubmissionRepository creates a submission repository backed by db.
func NewSubmissionRepository(db *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

// Create inserts sub as a new submission row.
func (r *SubmissionRepository) Create(ctx context.Context, sub *submission.Submission) error {
	arguments, err := json.Marshal(sub.Arguments)
	if err != nil {
		return err
	}
	compilerOptions, err := json.Marshal(sub.CompilerOptions)
	if err != nil {
		return err
	}
	limits, err := json.Marshal(sub.Limits)
	if err != nil {
		return err
	}
	additionalFiles, err := json.Marshal(sub.AdditionalFiles)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `INSERT INTO submissions (
		token,
		language_slug,
		source,
		input,
		expected_output,
		arguments,
		compiler_options,
		limits,
		additional_files,
		callback_url,
		status_code,
		created_at,
		queued_at,
		updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, NULLIF($9::jsonb, 'null'::jsonb), $10, $11, $12, $13, $14
	)`,
		sub.Token,
		sub.Language,
		sub.Source,
		sub.Input,
		sub.ExpectedOutput,
		arguments,
		compilerOptions,
		limits,
		additionalFiles,
		sub.CallbackURL,
		sub.StatusCode,
		sub.CreatedAt,
		sub.QueuedAt,
		sub.UpdatedAt,
	)
	return err
}

// FindByToken returns the submission identified by token.
func (r *SubmissionRepository) FindByToken(ctx context.Context, token string) (*submission.Submission, error) {
	row := r.db.QueryRow(ctx, `SELECT
		token,
		language_slug,
		source,
		input,
		expected_output,
		arguments,
		compiler_options,
		limits,
		additional_files,
		callback_url,
		status_code,
		stdout,
		stderr,
		stdout_truncated,
		stderr_truncated,
		compile_output,
		message,
		exit_code,
		exit_signal,
		time_ms,
		wall_time_ms,
		memory_kb,
		created_at,
		queued_at,
		started_at,
		finished_at,
		updated_at
	FROM submissions WHERE token = $1`, token)

	sub := &submission.Submission{}
	var expectedOutput pgtype.Text
	var callbackURL pgtype.Text
	var stdout pgtype.Text
	var stderr pgtype.Text
	var stdoutTruncated pgtype.Bool
	var stderrTruncated pgtype.Bool
	var compileOutput pgtype.Text
	var message pgtype.Text
	var exitSignal pgtype.Text
	var exitCode pgtype.Int4
	var timeMS pgtype.Int8
	var wallTimeMS pgtype.Int8
	var memoryKB pgtype.Int8
	var queuedAt pgtype.Timestamptz
	var startedAt pgtype.Timestamptz
	var finishedAt pgtype.Timestamptz
	var arguments []byte
	var compilerOptions []byte
	var limits []byte
	var additionalFiles []byte

	err := row.Scan(
		&sub.Token,
		&sub.Language,
		&sub.Source,
		&sub.Input,
		&expectedOutput,
		&arguments,
		&compilerOptions,
		&limits,
		&additionalFiles,
		&callbackURL,
		&sub.StatusCode,
		&stdout,
		&stderr,
		&stdoutTruncated,
		&stderrTruncated,
		&compileOutput,
		&message,
		&exitCode,
		&exitSignal,
		&timeMS,
		&wallTimeMS,
		&memoryKB,
		&sub.CreatedAt,
		&queuedAt,
		&startedAt,
		&finishedAt,
		&sub.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, submission.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	sub.ExpectedOutput = textPtr(expectedOutput)
	sub.CallbackURL = textPtr(callbackURL)
	sub.Stdout = textPtr(stdout)
	sub.Stderr = textPtr(stderr)
	sub.StdoutTruncated = boolValue(stdoutTruncated)
	sub.StderrTruncated = boolValue(stderrTruncated)
	sub.CompileOutput = textPtr(compileOutput)
	sub.Message = textPtr(message)
	sub.ExitCode = intPtr(exitCode)
	sub.ExitSignal = textPtr(exitSignal)
	sub.TimeMS = int64Ptr(timeMS)
	sub.WallTimeMS = int64Ptr(wallTimeMS)
	sub.MemoryKB = int64Ptr(memoryKB)
	sub.QueuedAt = timePtr(queuedAt)
	sub.StartedAt = timePtr(startedAt)
	sub.FinishedAt = timePtr(finishedAt)

	if err := json.Unmarshal(arguments, &sub.Arguments); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(compilerOptions, &sub.CompilerOptions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(limits, &sub.Limits); err != nil {
		return nil, err
	}
	if len(additionalFiles) > 0 {
		var files submission.AdditionalFiles
		if err := json.Unmarshal(additionalFiles, &files); err != nil {
			return nil, err
		}
		sub.AdditionalFiles = &files
	}

	return sub, nil
}

// MarkProcessing records that token has started execution.
func (r *SubmissionRepository) MarkProcessing(ctx context.Context, token string, startedAt time.Time) error {
	tag, err := r.db.Exec(ctx, `UPDATE submissions
		SET status_code = $2,
			started_at = $3,
			updated_at = $3
		WHERE token = $1`, token, "processing", startedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return submission.ErrNotFound
	}
	return nil
}

// StoreResult records the final result for token.
func (r *SubmissionRepository) StoreResult(ctx context.Context, token string, result submission.Result) error {
	tag, err := r.db.Exec(ctx, `UPDATE submissions
		SET status_code = $2,
			stdout = $3,
			stderr = $4,
			stdout_truncated = $5,
			stderr_truncated = $6,
			compile_output = $7,
			message = $8,
			exit_code = $9,
			exit_signal = $10,
			time_ms = $11,
			wall_time_ms = $12,
			memory_kb = $13,
			finished_at = $14,
			updated_at = $14
		WHERE token = $1`,
		token,
		result.StatusCode,
		result.Stdout,
		result.Stderr,
		result.StdoutTruncated,
		result.StderrTruncated,
		result.CompileOutput,
		result.Message,
		result.ExitCode,
		result.ExitSignal,
		result.TimeMS,
		result.WallTimeMS,
		result.MemoryKB,
		result.FinishedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return submission.ErrNotFound
	}
	return nil
}

// RecoverUnfinished resets unfinished submissions to queued and returns their tokens.
func (r *SubmissionRepository) RecoverUnfinished(ctx context.Context, recoveredAt time.Time) ([]string, error) {
	rows, err := r.db.Query(ctx, `WITH unfinished AS (
		SELECT token
		FROM submissions
		WHERE status_code IN ($2, $3)
		ORDER BY created_at ASC
		FOR UPDATE
	),
	updated AS (
		UPDATE submissions AS s
		SET status_code = $4,
			queued_at = COALESCE(s.queued_at, $1),
			started_at = NULL,
			updated_at = $1
		FROM unfinished
		WHERE s.token = unfinished.token
		RETURNING s.token, s.created_at
	)
	SELECT token
	FROM updated
	ORDER BY created_at ASC`,
		recoveredAt,
		status.Queued,
		status.Processing,
		status.Queued,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// CreateCallbackAttempt inserts one callback delivery attempt row.
func (r *SubmissionRepository) CreateCallbackAttempt(ctx context.Context, attempt submission.CallbackAttempt) error {
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx, `INSERT INTO callback_attempts (
		submission_token,
		attempt,
		status_code,
		error,
		created_at
	) VALUES ($1, $2, $3, $4, $5)`,
		attempt.SubmissionToken,
		attempt.Attempt,
		attempt.StatusCode,
		attempt.Error,
		attempt.CreatedAt,
	)
	return err
}

// ListCallbackAttempts returns callback attempts for token in attempt order.
func (r *SubmissionRepository) ListCallbackAttempts(ctx context.Context, token string) ([]submission.CallbackAttempt, error) {
	rows, err := r.db.Query(ctx, `SELECT
		id,
		submission_token,
		attempt,
		status_code,
		error,
		created_at
	FROM callback_attempts
	WHERE submission_token = $1
	ORDER BY attempt ASC, id ASC`, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []submission.CallbackAttempt
	for rows.Next() {
		var attempt submission.CallbackAttempt
		var statusCode pgtype.Int4
		var attemptError pgtype.Text
		if err := rows.Scan(
			&attempt.ID,
			&attempt.SubmissionToken,
			&attempt.Attempt,
			&statusCode,
			&attemptError,
			&attempt.CreatedAt,
		); err != nil {
			return nil, err
		}
		attempt.StatusCode = intPtr(statusCode)
		attempt.Error = textPtr(attemptError)
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	out := value.String
	return &out
}

func intPtr(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	out := int(value.Int32)
	return &out
}

func int64Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}

func boolValue(value pgtype.Bool) bool {
	return value.Valid && value.Bool
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}
