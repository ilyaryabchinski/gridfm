//go:build !linux

package places

// Mounts has no implementation off Linux in version 1; the platform
// abstraction arrives with cross-platform parity work.
func Mounts() []Place { return nil }
