//go:build unix

package preview

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// ownerGroup resolves the human names for an entry's owner and group,
// falling back to the numeric ids when the lookup fails.
func ownerGroup(path string, info os.FileInfo) (string, string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "?", "?"
	}

	uid := strconv.FormatUint(uint64(stat.Uid), 10)
	gid := strconv.FormatUint(uint64(stat.Gid), 10)

	if u, err := user.LookupId(uid); err == nil && u.Username != "" {
		uid = u.Username
	}
	if g, err := user.LookupGroupId(gid); err == nil && g.Name != "" {
		gid = g.Name
	}

	return uid, gid
}
