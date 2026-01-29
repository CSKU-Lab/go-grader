package execution

type Status string

const (
	COMPILE_FAILED        Status = "COMPILE_FAILED"
	RUN_PASSED            Status = "RUN_PASSED"
	RUN_FAILED            Status = "RUN_FAILED"
	TIME_LIMIT_EXCEEDED   Status = "TIME_LIMIT_EXCEEDED"
	MEMORY_LIMIT_EXCEEDED Status = "MEMORY_LIMIT_EXCEEDED"
	RUNTIME_ERROR         Status = "RUNTIME_ERROR"
	SIGNAL_ERROR          Status = "SIGNAL_ERROR"
	GRADER_ERROR          Status = "GRADER_ERROR"
	FILE_NOT_FOUND        Status = "FILE_NOT_FOUND"
	BUILD_PASSED          Status = "BUILD_PASSED"
	QUEUED                Status = "QUEUED"
	RUNNING               Status = "RUNNING"
)
