package models

import (
	"fmt"
	"strconv"
	"strings"
)

type Metadata struct {
	FailedStatus  string  `json:"failed_status"`
	FailedMessage string  `json:"failed_message"`
	Time          float32 `json:"time"`
	WallTime      float32 `json:"wall_time"`
	Memory        int32   `json:"memory"`
}

/*
*max-rss*::

	Maximum resident set size of the process (in kilobytes).

*message*::

	Status message, not intended for machine processing.
	E.g., "Time limit exceeded."

*status*::

	Two-letter status code:
	* *RE* -- run-time error, i.e., exited with a non-zero exit code
	* *SG* -- program died on a signal
	* *TO* -- timed out
	* *XX* -- internal error of the sandbox

*time*::

	Run time of the program in fractional seconds.

*time-wall*::

	Wall clock time of the program in fractional seconds.
*/
func ParseMetadata(metadataStr string) (*Metadata, error) {
	var metadata Metadata
	for line := range strings.SplitSeq(metadataStr, "\n") {
		if strings.HasPrefix(line, "status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			metadata.FailedStatus = status
		}
		if strings.HasPrefix(line, "message:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "message:"))
			metadata.FailedMessage = status
		}
		if strings.HasPrefix(line, "time:") {
			time := strings.TrimSpace(strings.TrimPrefix(line, "time:"))
			time64, err := strconv.ParseFloat(time, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse time: %w", err)
			}
			metadata.Time = float32(time64)
		}
		if strings.HasPrefix(line, "time-wall:") {
			wallTime := strings.TrimSpace(strings.TrimPrefix(line, "time-wall:"))
			wallTime64, err := strconv.ParseFloat(wallTime, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse wall time: %w", err)
			}
			metadata.WallTime = float32(wallTime64)

		}
		if strings.HasPrefix(line, "max-rss:") {
			memory := strings.TrimSpace(strings.TrimPrefix(line, "max-rss:"))
			memoryInt, err := strconv.Atoi(memory)
			if err != nil {
				return nil, fmt.Errorf("cannot parse memory: %w", err)
			}
			metadata.Memory = int32(memoryInt)
		}
	}
	return &metadata, nil
}
