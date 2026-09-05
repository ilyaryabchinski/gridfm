//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package main

// cellSize reports zero on platforms without a known winsize ioctl; the
// sync loop falls back to a default cell size.
func cellSize() (w, h int) { return 0, 0 }
