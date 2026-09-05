package browser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/browser"
)

// benchDir builds a directory with count real files so ReadDir measures
// genuine syscalls, not a synthetic slice.
func benchDir(b *testing.B, count int) string {
	b.Helper()

	dir := b.TempDir()
	for i := range count {
		name := filepath.Join(dir, fmt.Sprintf("bench-%04d.txt", i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	return dir
}

func BenchmarkReadDir100(b *testing.B) {
	dir := benchDir(b, 100)
	b.ResetTimer()
	for b.Loop() {
		if _, err := browser.ReadDir(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDir1000(b *testing.B) {
	dir := benchDir(b, 1000)
	b.ResetTimer()
	for b.Loop() {
		if _, err := browser.ReadDir(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDir10000(b *testing.B) {
	dir := benchDir(b, 10000)
	b.ResetTimer()
	for b.Loop() {
		if _, err := browser.ReadDir(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSortEntries10000(b *testing.B) {
	dir := benchDir(b, 10000)
	entries, err := browser.ReadDir(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		work := append([]browser.Entry(nil), entries...)
		browser.SortEntries(work, browser.SortBySize, browser.SortDescending)
	}
}
