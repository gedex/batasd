// Package status defines submission status codes and descriptions.
package status

// Status is a public submission status.
type Status struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

const (
	// Queued means a submission is waiting for a worker.
	Queued = "queued"
	// Processing means a worker is executing the submission.
	Processing = "processing"
	// Accepted means output matched the expected output, or no expected output was supplied.
	Accepted = "accepted"
	// WrongAnswer means execution succeeded but output did not match expected output.
	WrongAnswer = "wrong_answer"
	// CompilationError means the compile command failed.
	CompilationError = "compilation_error"
	// RuntimeError means the run command exited with a non-zero status.
	RuntimeError = "runtime_error"
	// TimeLimitExceeded means execution exceeded the configured time limit.
	TimeLimitExceeded = "time_limit_exceeded"
	// MemoryLimitExceeded means execution exceeded the configured memory limit.
	MemoryLimitExceeded = "memory_limit_exceeded"
	// OutputLimitExceeded means stdout or stderr exceeded the configured output limit.
	OutputLimitExceeded = "output_limit_exceeded"
	// InternalError means batasd could not process the submission due to an internal problem.
	InternalError = "internal_error"
	// SandboxError means the sandbox failed before producing a submission result.
	SandboxError = "sandbox_error"
)

var all = []Status{
	{Code: Queued, Description: "Queued"},
	{Code: Processing, Description: "Processing"},
	{Code: Accepted, Description: "Accepted"},
	{Code: WrongAnswer, Description: "Wrong Answer"},
	{Code: CompilationError, Description: "Compilation Error"},
	{Code: RuntimeError, Description: "Runtime Error"},
	{Code: TimeLimitExceeded, Description: "Time Limit Exceeded"},
	{Code: MemoryLimitExceeded, Description: "Memory Limit Exceeded"},
	{Code: OutputLimitExceeded, Description: "Output Limit Exceeded"},
	{Code: InternalError, Description: "Internal Error"},
	{Code: SandboxError, Description: "Sandbox Error"},
}

// List returns every public status in stable order.
func List() []Status {
	out := make([]Status, len(all))
	copy(out, all)
	return out
}

// Describe returns the public status for code.
func Describe(code string) Status {
	for _, item := range all {
		if item.Code == code {
			return item
		}
	}
	return Status{Code: code, Description: code}
}
