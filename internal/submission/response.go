package submission

import (
	"time"

	"github.com/gedex/batasd/internal/status"
)

// Response is the public representation of a submission.
type Response struct {
	Token           string        `json:"token"`
	Language        string        `json:"language,omitempty"`
	Status          status.Status `json:"status"`
	Stdout          *string       `json:"stdout"`
	Stderr          *string       `json:"stderr"`
	CompileOutput   *string       `json:"compile_output"`
	Message         *string       `json:"message"`
	ExitCode        *int          `json:"exit_code"`
	ExitSignal      *string       `json:"exit_signal"`
	TimeMS          *int64        `json:"time_ms"`
	WallTimeMS      *int64        `json:"wall_time_ms"`
	MemoryKB        *int64        `json:"memory_kb"`
	CreatedAt       string        `json:"created_at,omitempty"`
	QueuedAt        *string       `json:"queued_at"`
	StartedAt       *string       `json:"started_at"`
	FinishedAt      *string       `json:"finished_at"`
	Limits          Limits        `json:"limits,omitempty"`
	Arguments       []string      `json:"arguments,omitempty"`
	CompilerOptions []string      `json:"compiler_options,omitempty"`
}

// ToResponse converts sub into its public API shape.
func ToResponse(sub *Submission) Response {
	return Response{
		Token:           sub.Token,
		Language:        sub.Language,
		Status:          status.Describe(sub.StatusCode),
		Stdout:          sub.Stdout,
		Stderr:          sub.Stderr,
		CompileOutput:   sub.CompileOutput,
		Message:         sub.Message,
		ExitCode:        sub.ExitCode,
		ExitSignal:      sub.ExitSignal,
		TimeMS:          sub.TimeMS,
		WallTimeMS:      sub.WallTimeMS,
		MemoryKB:        sub.MemoryKB,
		CreatedAt:       sub.CreatedAt.Format(time.RFC3339Nano),
		QueuedAt:        formatTimePtr(sub.QueuedAt),
		StartedAt:       formatTimePtr(sub.StartedAt),
		FinishedAt:      formatTimePtr(sub.FinishedAt),
		Limits:          sub.Limits,
		Arguments:       sub.Arguments,
		CompilerOptions: sub.CompilerOptions,
	}
}

// ToResultResponse converts sub plus result into the completed public API shape.
func ToResultResponse(sub *Submission, result Result) Response {
	completed := *sub
	completed.StatusCode = result.StatusCode
	completed.Stdout = result.Stdout
	completed.Stderr = result.Stderr
	completed.CompileOutput = result.CompileOutput
	completed.Message = result.Message
	completed.ExitCode = result.ExitCode
	completed.ExitSignal = result.ExitSignal
	completed.TimeMS = result.TimeMS
	completed.WallTimeMS = result.WallTimeMS
	completed.MemoryKB = result.MemoryKB
	completed.FinishedAt = &result.FinishedAt
	completed.UpdatedAt = result.FinishedAt
	return ToResponse(&completed)
}

// ToCreatedResponse returns the public response for a newly queued submission.
func ToCreatedResponse(sub *Submission) map[string]any {
	return map[string]any{
		"token":  sub.Token,
		"status": status.Describe(sub.StatusCode),
	}
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}
