package places

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMountsFiltersPseudoFilesystems(t *testing.T) {
	t.Parallel()

	table := strings.NewReader(`
/dev/nvme0n1p2 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid,nodev 0 0
sysfs /sys sysfs rw 0 0
tmpfs /tmp tmpfs rw 0 0
/dev/nvme0n1p1 /boot vfat rw,flush 0 0
/dev/sda1 /run/media/user/Backup exfat rw 0 0
tmpfs /run/user/1000 tmpfs rw 0 0
cgroup /sys/fs/cgroup cgroup2 rw 0 0
overlay / overlay rw 0 0
/dev/sdb1 /mnt/with\040space ntfs3 rw 0 0
/dev/sdb1 /mnt/with\040space ntfs3 rw 0 0
`)

	got := parseMounts(table)
	want := []Place{
		{Label: "Root", Path: "/"},
		{Label: "boot", Path: "/boot"},
		{Label: "Backup", Path: "/run/media/user/Backup"},
		{Label: "with space", Path: "/mnt/with space"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseMounts = %+v, want %+v", got, want)
	}
}

func TestParseMountsEmptyAndGarbage(t *testing.T) {
	t.Parallel()

	if got := parseMounts(strings.NewReader("")); got != nil {
		t.Errorf("empty table = %+v, want nil", got)
	}
	if got := parseMounts(strings.NewReader("nonsense\nless\n")); got != nil {
		t.Errorf("garbage table = %+v, want nil", got)
	}
}
