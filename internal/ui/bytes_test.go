package ui

import "testing"

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"just under a k", 1023, "1023 B"},
		{"kilobytes", 1536, "1.5 KB"},
		{"rounds up into the next tier", 1_048_575, "1.0 MB"},
		{"megabytes", 3_600_000, "3.4 MB"},
		{"rounds up into the next tier 2", 1_073_741_823, "1.0 GB"},
		{"gigabytes", 5_100_000_000, "4.7 GB"},
		{"terabytes", 2 << 40, "2.0 TB"},
	}

	for _, tc := range cases {
		if got := FormatBytes(tc.in); got != tc.want {
			t.Errorf("%s: FormatBytes(%d) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
