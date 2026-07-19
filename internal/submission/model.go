// Package submission defines submission models, service contracts, and results.
package submission

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a submission token does not exist.
var ErrNotFound = errors.New("submission not found")

// Limits defines the execution resource limits for a submission.
type Limits struct {
	CPUTimeMS    int64 `json:"cpu_time_ms"`
	CPUExtraMS   int64 `json:"cpu_extra_time_ms"`
	WallTimeMS   int64 `json:"wall_time_ms"`
	MemoryKB     int64 `json:"memory_kb"`
	StackKB      int64 `json:"stack_kb"`
	MaxProcesses int   `json:"max_processes"`
	MaxOutputKB  int64 `json:"max_output_kb"`
	MaxFileKB    int64 `json:"max_file_kb"`
	Network      bool  `json:"network"`
	Runs         int   `json:"runs"`
}

// AdditionalFiles carries an archive of files to place in the execution directory.
type AdditionalFiles struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// Callback configures webhook delivery for a completed submission.
type Callback struct {
	URL string `json:"url"`
}

// CreateRequest is the API payload for creating a submission.
type CreateRequest struct {
	Source          string           `json:"source"`
	Language        string           `json:"language"`
	Input           string           `json:"input"`
	ExpectedOutput  *string          `json:"expected_output"`
	Arguments       []string         `json:"arguments"`
	CompilerOptions []string         `json:"compiler_options"`
	Limits          *Limits          `json:"limits"`
	AdditionalFiles *AdditionalFiles `json:"additional_files"`
	Callback        *Callback        `json:"callback"`
}

// Submission is a persisted code execution request and its current result state.
type Submission struct {
	Token           string
	Language        string
	Source          string
	Input           string
	ExpectedOutput  *string
	Arguments       []string
	CompilerOptions []string
	Limits          Limits
	AdditionalFiles *AdditionalFiles
	CallbackURL     *string
	StatusCode      string
	Stdout          *string
	Stderr          *string
	CompileOutput   *string
	Message         *string
	ExitCode        *int
	ExitSignal      *string
	TimeMS          *int64
	WallTimeMS      *int64
	MemoryKB        *int64
	CreatedAt       time.Time
	QueuedAt        *time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	UpdatedAt       time.Time
}

// Result is the final execution result stored for a submission.
type Result struct {
	StatusCode    string
	Stdout        *string
	Stderr        *string
	CompileOutput *string
	Message       *string
	ExitCode      *int
	ExitSignal    *string
	TimeMS        *int64
	WallTimeMS    *int64
	MemoryKB      *int64
	FinishedAt    time.Time
}

// CallbackAttempt is one delivery attempt for a submission callback.
type CallbackAttempt struct {
	ID              int64
	SubmissionToken string
	Attempt         int
	StatusCode      *int
	Error           *string
	CreatedAt       time.Time
}

// Repository persists submissions and their status transitions.
type Repository interface {
	Create(ctx context.Context, submission *Submission) error
	FindByToken(ctx context.Context, token string) (*Submission, error)
	MarkProcessing(ctx context.Context, token string, startedAt time.Time) error
	StoreResult(ctx context.Context, token string, result Result) error
	RecoverUnfinished(ctx context.Context, recoveredAt time.Time) ([]string, error)
	CreateCallbackAttempt(ctx context.Context, attempt CallbackAttempt) error
	ListCallbackAttempts(ctx context.Context, token string) ([]CallbackAttempt, error)
}

// Queue accepts submissions for asynchronous execution.
type Queue interface {
	Enqueue(ctx context.Context, token string) error
}
