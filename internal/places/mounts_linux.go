//go:build linux

package places

import "os"

// Mounts returns the local-disk volumes currently mounted, read from
// /proc/mounts. It never fails; unreadable tables yield nothing.
func Mounts() []Place {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer file.Close() //nolint:errcheck // read-only handle

	return parseMounts(file)
}
