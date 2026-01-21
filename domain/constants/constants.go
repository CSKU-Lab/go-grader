package constants

const (
	CONFIG_DIR        = "/var/local/lib/worker"
	RUNNER_DIR        = CONFIG_DIR + "/runners"
	COMPARE_DIR       = CONFIG_DIR + "/compares"
	COMPARE_LIST_PATH = CONFIG_DIR + "/compare_list.json"
	SANDBOX_PATH      = "/var/local/lib/isolate/%d"
	MAX_QUEUES        = 10
)
