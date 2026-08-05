package scale

import (
	"encoding/json"
	"fmt"
)

// OffsetWindow bounds topic inspection to records appended by one isolated run.
type OffsetWindow struct {
	start map[int32]int64
	end   map[int32]int64
}

// NewOffsetWindow validates broker offsets captured before and after a run.
// Empty snapshots retain the legacy full-topic behavior used by small tests.
func NewOffsetWindow(startJSON, endJSON string, partitions []int32, expected int64) (OffsetWindow, error) {
	if (startJSON == "") != (endJSON == "") {
		return OffsetWindow{}, fmt.Errorf("--start-offsets and --end-offsets must be provided together")
	}
	if startJSON == "" {
		return OffsetWindow{}, nil
	}
	var start, end map[int32]int64
	if err := json.Unmarshal([]byte(startJSON), &start); err != nil {
		return OffsetWindow{}, fmt.Errorf("decode start offsets: %w", err)
	}
	if err := json.Unmarshal([]byte(endJSON), &end); err != nil {
		return OffsetWindow{}, fmt.Errorf("decode end offsets: %w", err)
	}
	var records int64
	for _, partition := range partitions {
		startOffset, startExists := start[partition]
		endOffset, endExists := end[partition]
		if !startExists || !endExists || startOffset < 0 || endOffset < startOffset {
			return OffsetWindow{}, fmt.Errorf("invalid offset window for partition %d", partition)
		}
		records += endOffset - startOffset
	}
	if len(start) != len(partitions) || len(end) != len(partitions) {
		return OffsetWindow{}, fmt.Errorf("offset window does not match the topic partitions")
	}
	if records != expected {
		return OffsetWindow{}, fmt.Errorf("offset window contains %d records, want %d", records, expected)
	}
	return OffsetWindow{start: start, end: end}, nil
}

// Bounds returns the retained broker range or the validated run-specific range.
func (w OffsetWindow) Bounds(partition int32, oldest, newest int64) (int64, int64, error) {
	if w.start == nil {
		return oldest, newest, nil
	}
	start, startExists := w.start[partition]
	end, endExists := w.end[partition]
	if !startExists || !endExists || start < oldest || end > newest {
		return 0, 0, fmt.Errorf("offset window for partition %d is outside retained broker offsets", partition)
	}
	return start, end, nil
}
