package execution

type Status string

const (
	COMPILE_FAILED        Status = "compile_failed"
	RUN_PASSED            Status = "success"
	RUN_FAILED            Status = "run_failed"
	TIME_LIMIT_EXCEEDED   Status = "time_limit_exceeded"
	MEMORY_LIMIT_EXCEEDED Status = "memory_limit_exceeded"
	RUNTIME_ERROR         Status = "runtime_error"
	SIGNAL_ERROR          Status = "signal_error"
	GRADER_ERROR          Status = "grader_error"
)
