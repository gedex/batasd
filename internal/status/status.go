package status

type Status struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

const (
	Queued              = "queued"
	Processing          = "processing"
	Accepted            = "accepted"
	WrongAnswer         = "wrong_answer"
	CompilationError    = "compilation_error"
	RuntimeError        = "runtime_error"
	TimeLimitExceeded   = "time_limit_exceeded"
	MemoryLimitExceeded = "memory_limit_exceeded"
	OutputLimitExceeded = "output_limit_exceeded"
	InternalError       = "internal_error"
	SandboxError        = "sandbox_error"
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

func List() []Status {
	out := make([]Status, len(all))
	copy(out, all)
	return out
}

func Describe(code string) Status {
	for _, item := range all {
		if item.Code == code {
			return item
		}
	}
	return Status{Code: code, Description: code}
}
