package submission

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("submission not found")

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

type AdditionalFiles struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

type Callback struct {
	URL string `json:"url"`
}

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

type Repository interface {
	Create(ctx context.Context, submission *Submission) error
	FindByToken(ctx context.Context, token string) (*Submission, error)
}

type Queue interface {
	Enqueue(ctx context.Context, token string) error
}
