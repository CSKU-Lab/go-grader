package models

import (
	"log"
	"strconv"
	"strings"
)

type Metadata struct {
	FailedStatus  string  `json:"failed_status"`
	FailedMessage string  `json:"failed_message"`
	Time          float32 `json:"time"`
	WallTime      float32 `json:"wall_time"`
	Memory        int     `json:"memory"`
}

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
				log.Fatal("Cannot parse Time : ", err)
			}
			metadata.Time = float32(time64)
		}
		if strings.HasPrefix(line, "time-wall:") {
			wallTime := strings.TrimSpace(strings.TrimPrefix(line, "time-wall:"))
			wallTime64, err := strconv.ParseFloat(wallTime, 32)
			if err != nil {
				log.Fatal("Cannot parse Wall time : ", err)
			}
			metadata.WallTime = float32(wallTime64)

		}
		if strings.HasPrefix(line, "max-rss:") {
			memory := strings.TrimSpace(strings.TrimPrefix(line, "max-rss:"))
			memoryInt, err := strconv.Atoi(memory)
			if err != nil {
				log.Fatal("Cannot parse Memory : ", err)
			}
			metadata.Memory = memoryInt
		}
	}
	return &metadata, nil
}
