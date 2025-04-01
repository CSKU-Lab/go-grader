package models

/*
> Limit Unit <
- CPUTime: The maximum execution time allowed for the code to run (seconds).
- CPUExtraTime: The extra time allowed for the code to run (seconds).
- WallTime: The maximum time allowed for the code to run (seconds).
- Memory: The maximum memory allowed for the code to run (KB).
- Stack: The maximum stack allowed for the code to run (KB).
- MaxOpenFiles: The maximum number of files allowed to be opened by the code. (default: 64)
- MaxFileSize: The maximum file size allowed to be created by the code. (KB)
- NetworkAllow: Allow the code to access the network. (default: false)
*/

type Limit struct {
	CPUTime      float32 `json:"cpuTime" arg:"--time"`
	CPUExtraTime float32 `json:"cpuExtraTime" arg:"--extra-time"`
	WallTime     float32 `json:"wallTime" arg:"--wall-time"`
	Memory       int     `json:"memory" arg:"--mem"`
	Stack        int     `json:"stack" arg:"--stack"`
	MaxOpenFiles int     `json:"maxOpenFiles" arg:"--open-files"`
	MaxFileSize  float32 `json:"maxFileSize" arg:"--fsize"`
	NetworkAllow bool    `json:"allowNetwork" arg:"--share-net"`
}
