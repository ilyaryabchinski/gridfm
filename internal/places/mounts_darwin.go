//go:build darwin

package places

import (
	"strings"

	"golang.org/x/sys/unix"
)

// Mounts returns user-visible external volumes mounted under /Volumes.
func Mounts() []Place {
	count, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil || count == 0 {
		return nil
	}

	stats := make([]unix.Statfs_t, count)
	count, err = unix.Getfsstat(stats, unix.MNT_NOWAIT)
	if err != nil {
		return nil
	}
	if count > len(stats) {
		count = len(stats)
	}

	var mounts []Place
	for _, stat := range stats[:count] {
		path := unix.ByteSliceToString(stat.Mntonname[:])
		if !strings.HasPrefix(path, "/Volumes/") || path == "/Volumes/" {
			continue
		}
		mounts = append(mounts, Place{Label: baseLabel(path), Path: path})
	}

	return mounts
}
