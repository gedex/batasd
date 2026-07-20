package submission

import (
	"time"

	"github.com/gedex/batasd/internal/status"
)

// Response is the public representation of a submission.
type Response struct {
	Token                  string        `json:"token"`
	Language               string        `json:"language,omitempty"`
	LanguageVersion        string        `json:"language_version,omitempty"`
	Status                 status.Status `json:"status"`
	Stdout                 *string       `json:"stdout"`
	Stderr                 *string       `json:"stderr"`
	StdoutTruncated        bool          `json:"stdout_truncated"`
	StderrTruncated        bool          `json:"stderr_truncated"`
	CompileOutput          *string       `json:"compile_output"`
	CompileOutputTruncated bool          `json:"compile_output_truncated"`
	Message                *string       `json:"message"`
	ExitCode               *int          `json:"exit_code"`
	ExitSignal             *string       `json:"exit_signal"`
	TimeMS                 *int64        `json:"time_ms"`
	WallTimeMS             *int64        `json:"wall_time_ms"`
	MemoryKB               *int64        `json:"memory_kb"`
	CreatedAt              string        `json:"created_at,omitempty"`
	QueuedAt               *string       `json:"queued_at"`
	StartedAt              *string       `json:"started_at"`
	FinishedAt             *string       `json:"finished_at"`
	Limits                 Limits        `json:"limits,omitempty"`
	Arguments              []string      `json:"arguments,omitempty"`
	CompilerOptions        []string      `json:"compiler_options,omitempty"`
}

// CallbackAttemptResponse is the public representation of a callback attempt.
type CallbackAttemptResponse struct {
	ID         int64   `json:"id"`
	Attempt    int     `json:"attempt"`
	StatusCode *int    `json:"status_code"`
	Error      *string `json:"error"`
	CreatedAt  string  `json:"created_at"`
}

// CallbackAttemptsResponse is the public response for callback attempts.
type CallbackAttemptsResponse struct {
	SubmissionToken string                    `json:"submission_token"`
	Attempts        []CallbackAttemptResponse `json:"attempts"`
}

// ListResponse is the public response for a submission list page.
type ListResponse struct {
	Submissions []Response     `json:"submissions"`
	Pagination  PaginationInfo `json:"pagination"`
}

// PaginationInfo describes how to fetch the next page, when one exists.
type PaginationInfo struct {
	Limit      int     `json:"limit"`
	NextBefore *string `json:"next_before"`
}

// ValidResponseField reports whether field can be selected with the fields query parameter.
func ValidResponseField(field string) bool {
	switch field {
	case "token",
		"language",
		"language_version",
		"status",
		"stdout",
		"stderr",
		"stdout_truncated",
		"stderr_truncated",
		"compile_output",
		"compile_output_truncated",
		"message",
		"exit_code",
		"exit_signal",
		"time_ms",
		"wall_time_ms",
		"memory_kb",
		"created_at",
		"queued_at",
		"started_at",
		"finished_at",
		"limits",
		"arguments",
		"compiler_options":
		return true
	default:
		return false
	}
}

// ToResponse converts sub into its public API shape.
func ToResponse(sub *Submission) Response {
	return Response{
		Token:                  sub.Token,
		Language:               sub.Language,
		LanguageVersion:        sub.LanguageVersion,
		Status:                 status.Describe(sub.StatusCode),
		Stdout:                 sub.Stdout,
		Stderr:                 sub.Stderr,
		StdoutTruncated:        sub.StdoutTruncated,
		StderrTruncated:        sub.StderrTruncated,
		CompileOutput:          sub.CompileOutput,
		CompileOutputTruncated: sub.CompileOutputTruncated,
		Message:                sub.Message,
		ExitCode:               sub.ExitCode,
		ExitSignal:             sub.ExitSignal,
		TimeMS:                 sub.TimeMS,
		WallTimeMS:             sub.WallTimeMS,
		MemoryKB:               sub.MemoryKB,
		CreatedAt:              sub.CreatedAt.Format(time.RFC3339Nano),
		QueuedAt:               formatTimePtr(sub.QueuedAt),
		StartedAt:              formatTimePtr(sub.StartedAt),
		FinishedAt:             formatTimePtr(sub.FinishedAt),
		Limits:                 sub.Limits,
		Arguments:              sub.Arguments,
		CompilerOptions:        sub.CompilerOptions,
	}
}

// SelectResponseFields returns a map containing only fields from response.
func SelectResponseFields(response Response, fields []string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		switch field {
		case "token":
			out[field] = response.Token
		case "language":
			out[field] = response.Language
		case "language_version":
			out[field] = response.LanguageVersion
		case "status":
			out[field] = response.Status
		case "stdout":
			out[field] = response.Stdout
		case "stderr":
			out[field] = response.Stderr
		case "stdout_truncated":
			out[field] = response.StdoutTruncated
		case "stderr_truncated":
			out[field] = response.StderrTruncated
		case "compile_output":
			out[field] = response.CompileOutput
		case "compile_output_truncated":
			out[field] = response.CompileOutputTruncated
		case "message":
			out[field] = response.Message
		case "exit_code":
			out[field] = response.ExitCode
		case "exit_signal":
			out[field] = response.ExitSignal
		case "time_ms":
			out[field] = response.TimeMS
		case "wall_time_ms":
			out[field] = response.WallTimeMS
		case "memory_kb":
			out[field] = response.MemoryKB
		case "created_at":
			out[field] = response.CreatedAt
		case "queued_at":
			out[field] = response.QueuedAt
		case "started_at":
			out[field] = response.StartedAt
		case "finished_at":
			out[field] = response.FinishedAt
		case "limits":
			out[field] = response.Limits
		case "arguments":
			out[field] = response.Arguments
		case "compiler_options":
			out[field] = response.CompilerOptions
		}
	}
	return out
}

// ToListResponse converts result into the public list API shape.
func ToListResponse(result ListResult) ListResponse {
	responses := make([]Response, 0, len(result.Submissions))
	for _, sub := range result.Submissions {
		responses = append(responses, ToResponse(sub))
	}
	return ListResponse{
		Submissions: responses,
		Pagination: PaginationInfo{
			Limit:      result.Limit,
			NextBefore: result.NextBefore,
		},
	}
}

// SelectListResponseFields returns a list response with filtered submission items.
func SelectListResponseFields(response ListResponse, fields []string) map[string]any {
	submissions := make([]map[string]any, 0, len(response.Submissions))
	for _, sub := range response.Submissions {
		submissions = append(submissions, SelectResponseFields(sub, fields))
	}
	return map[string]any{
		"submissions": submissions,
		"pagination":  response.Pagination,
	}
}

// ToCallbackAttemptsResponse converts attempts into the public API shape.
func ToCallbackAttemptsResponse(token string, attempts []CallbackAttempt) CallbackAttemptsResponse {
	responses := make([]CallbackAttemptResponse, 0, len(attempts))
	for _, attempt := range attempts {
		responses = append(responses, CallbackAttemptResponse{
			ID:         attempt.ID,
			Attempt:    attempt.Attempt,
			StatusCode: attempt.StatusCode,
			Error:      attempt.Error,
			CreatedAt:  attempt.CreatedAt.Format(time.RFC3339Nano),
		})
	}
	return CallbackAttemptsResponse{
		SubmissionToken: token,
		Attempts:        responses,
	}
}

// ToResultResponse converts sub plus result into the completed public API shape.
func ToResultResponse(sub *Submission, result Result) Response {
	completed := *sub
	completed.StatusCode = result.StatusCode
	completed.Stdout = result.Stdout
	completed.Stderr = result.Stderr
	completed.StdoutTruncated = result.StdoutTruncated
	completed.StderrTruncated = result.StderrTruncated
	completed.CompileOutput = result.CompileOutput
	completed.CompileOutputTruncated = result.CompileOutputTruncated
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
