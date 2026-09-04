package ui

import "strconv"

// FormatBytes renders a byte count in human-readable binary units:
// 0 B, 512 B, 1.5 KB, 3.4 MB. It is used for operation progress.
func FormatBytes(n int64) string {
	if n < 0 {
		return "?" // sizes are never negative; render defensively
	}
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}

	value := float64(n)
	unit := ""
	// Walk up one unit too far when rounding would cross the boundary:
	// 1048575 renders as 1.0 MB, not 1024.0 KB.
	for _, next := range []string{"KB", "MB", "GB", "TB", "PB", "EB"} {
		value /= 1024
		unit = next
		if value < 1023.5 {
			break
		}
	}

	return strconv.FormatFloat(value, 'f', 1, 64) + " " + unit
}
