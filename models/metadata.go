package models

import (
	"log"
	"strconv"
	"strings"
)

type Metadata struct {
	Status   string
	Message  string
	Time     float32
	WallTime float32
	Memory   int
}

func ParseMetadata(metadataStr string) (*Metadata, error) {
	var metadata Metadata
	for line := range strings.SplitSeq(metadataStr, "\n") {
		if strings.HasPrefix(line, "status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			metadata.Status = status
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
