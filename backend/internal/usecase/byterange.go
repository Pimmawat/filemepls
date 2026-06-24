package usecase

import (
	"fmt"
	"strconv"
	"strings"
)

// parseRange parses a single-range HTTP Range header value (e.g.
// "bytes=0-499", "bytes=500-", "bytes=-500") against totalSize. An empty
// rangeHeader means "whole file" (partial=false). Multi-range requests
// ("bytes=0-10,20-30") are not supported (out of scope: resume/video-seek
// only needs single-range). length <= 0 means "through EOF".
func parseRange(rangeHeader string, totalSize int64) (offset, length int64, partial bool, err error) {
	if rangeHeader == "" {
		return 0, 0, false, nil
	}

	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return 0, 0, false, fmt.Errorf("usecase: unsupported range unit in %q", rangeHeader)
	}
	spec := strings.TrimPrefix(rangeHeader, prefix)
	if strings.Contains(spec, ",") {
		return 0, 0, false, fmt.Errorf("usecase: multi-range requests are not supported")
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("usecase: malformed range %q", rangeHeader)
	}
	startStr, endStr := parts[0], parts[1]

	switch {
	case startStr == "" && endStr != "":
		// Suffix range: last N bytes.
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false, fmt.Errorf("usecase: malformed range %q", rangeHeader)
		}
		if n > totalSize {
			n = totalSize
		}
		return totalSize - n, n, true, nil

	case startStr != "":
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 || start >= totalSize {
			return 0, 0, false, fmt.Errorf("usecase: range not satisfiable for %q", rangeHeader)
		}
		if endStr == "" {
			return start, 0, true, nil // through EOF
		}
		end, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, fmt.Errorf("usecase: malformed range %q", rangeHeader)
		}
		if end >= totalSize {
			end = totalSize - 1
		}
		return start, end - start + 1, true, nil

	default:
		return 0, 0, false, fmt.Errorf("usecase: malformed range %q", rangeHeader)
	}
}
