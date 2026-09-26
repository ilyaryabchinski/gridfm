//go:build !darwin

package operations

func copyMetadata(_, _ string) error { return nil }
