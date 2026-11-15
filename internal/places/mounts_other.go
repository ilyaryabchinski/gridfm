//go:build !linux && !darwin

package places

func Mounts() []Place { return nil }
