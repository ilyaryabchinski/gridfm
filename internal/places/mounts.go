package places

import (
	"io"
	"strconv"
	"strings"
)

// mountTypes are the local-disk filesystem types surfaced as mounts. Pseudo
// filesystems (proc, sysfs, tmpfs, cgroup, squashfs, ...) are excluded so
// only real volumes appear in the sidebar.
//
//nolint:gochecknoglobals // static lookup table, never mutated
var mountTypes = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "btrfs": true,
	"xfs": true, "zfs": true, "f2fs": true, "jfs": true, "reiserfs": true,
	"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true, "fuseblk": true,
	"udf": true, "iso9660": true,
}

// parseMounts reads mount table lines in /proc/mounts format and returns
// the local-disk volumes, deduplicated by mount point in table order. The
// root filesystem is listed first as "Root" when present.
func parseMounts(r io.Reader) []Place {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var out []Place
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if !mountTypes[fields[2]] {
			continue
		}

		point := unescapeMountPath(fields[1])
		if seen[point] {
			continue
		}
		seen[point] = true

		label := "Root"
		if point != "/" {
			label = baseLabel(point)
		}
		out = append(out, Place{Label: label, Path: point})
	}

	return out
}

// baseLabel names a mount point by its final component, falling back to the
// full path for shallow points like /mnt.
func baseLabel(point string) string {
	base := point
	for part := range strings.SplitSeq(strings.Trim(point, "/"), "/") {
		if part != "" {
			base = part
		}
	}
	if base == "" || base == point {
		return point
	}

	return base
}

// unescapeMountPath decodes the octal escapes /proc/mounts uses for
// whitespace and backslashes in mount paths (\040 space, \134 backslash).
func unescapeMountPath(path string) string {
	if !strings.Contains(path, "\\") {
		return path
	}

	var b strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+3 < len(path) {
			if v, err := strconv.ParseUint(path[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3

				continue
			}
		}
		b.WriteByte(path[i])
	}

	return b.String()
}
