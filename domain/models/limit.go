package models

/*
* > Limit Unit <
* - CPUTime: The maximum execution time allowed for the code to run (seconds).
* - CPUExtraTime: The extra time allowed for the code to run (seconds).
* - WallTime: The maximum time allowed for the code to run (seconds).
* - Memory: The maximum memory allowed for the code to run (KB).
* - Stack: The maximum stack allowed for the code to run (KB).
* - MaxOpenFiles: The maximum number of files allowed to be opened by the code. (default: 64)
* - MaxFileSize: The maximum file size allowed to be created by the code. (KB)
* - NetworkAllow: Allow the code to access the network. (default: false)
 */
type Limit struct {
	CPUTime      float32 `json:"cpuTime" arg:"--time"`
	CPUExtraTime float32 `json:"cpuExtraTime" arg:"--extra-time"`
	WallTime     float32 `json:"wallTime" arg:"--wall-time"`
	Memory       int32   `json:"memory" arg:"--mem"`
	Stack        int32   `json:"stack" arg:"--stack"`
	MaxOpenFiles int32   `json:"maxOpenFiles" arg:"--open-files"`
	MaxFileSize  float32 `json:"maxFileSize" arg:"--fsize"`
	NetworkAllow bool    `json:"allowNetwork" arg:"--share-net"`
}

// System maximum safe limits applied when a field is zero or negative.
// These are DEFAULTS for tasks with no explicit limit; a configured task limit
// still overrides them. Wall-time is the effective hard timeout — isolate's CPU
// --time only accounts the directly-run process, so a forked runaway is bounded
// by --wall-time. Lowered from 15s/10s to 5s to fail runaway submissions fast.
const (
	SafeCPUTime      float32 = 5
	SafeCPUExtraTime float32 = 3
	SafeWallTime     float32 = 5
	SafeMemory       int32   = 524288
	SafeStack        int32   = 262144
	SafeMaxOpenFiles int32   = 10
	SafeMaxFileSize  float32 = 5120
)

// WithSafeLimits returns a copy of l with any field that is ≤ 0 replaced by
// the system maximum safe limit, ensuring isolate never runs unbounded.
func (l Limit) WithSafeLimits() Limit {
	if l.CPUTime <= 0 {
		l.CPUTime = SafeCPUTime
	}
	if l.CPUExtraTime <= 0 {
		l.CPUExtraTime = SafeCPUExtraTime
	}
	if l.WallTime <= 0 {
		l.WallTime = SafeWallTime
	}
	if l.Memory <= 0 {
		l.Memory = SafeMemory
	}
	if l.Stack <= 0 {
		l.Stack = SafeStack
	}
	if l.MaxOpenFiles <= 0 {
		l.MaxOpenFiles = SafeMaxOpenFiles
	}
	if l.MaxFileSize <= 0 {
		l.MaxFileSize = SafeMaxFileSize
	}
	return l
}
