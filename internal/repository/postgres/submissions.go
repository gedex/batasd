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
			compile_output = $5,
			message = $6,
			exit_code = $7,
			exit_signal = $8,
			time_ms = $9,
			wall_time_ms = $10,
			memory_kb = $11,
			finished_at = $12,
			updated_at = $12
		WHERE token = $1`,
		token,
		result.StatusCode,
		result.Stdout,
		result.Stderr,
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

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}
