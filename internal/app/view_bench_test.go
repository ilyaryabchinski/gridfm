package app_test

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gridfm/internal/app"
	"gridfm/internal/browser"
)

// benchEntries synthesizes count entries without touching the disk; View
// performance is dominated by rendering, not fixtures.
func benchEntries(root string, count int) []browser.Entry {
	entries := make([]browser.Entry, count)
	for i := range entries {
		name := fmt.Sprintf("%04d.txt", i)
		entries[i] = browser.Entry{Name: name, Path: filepath.Join(root, name)}
	}

	return entries
}

// BenchmarkView1000Entries measures one full frame over a large
// directory. The plan budgets 200 ms for the first useful view and 50 ms
// for a resize reflow; a single frame must stay well under both.
func BenchmarkView1000Entries(b *testing.B) {
	root := b.TempDir()
	m := app.New(root, app.Options{})
	m = resize(b, m, 120, 30)
	m = loaded(b, m, 1, root, benchEntries(root, 1000), nil)

	b.ResetTimer()
	for b.Loop() {
		_ = m.View()
	}
}

// BenchmarkMovementKey pins the navigation budget: one j press plus its
// re-render must be imperceptible.
func BenchmarkMovementKey(b *testing.B) {
	root := b.TempDir()
	m := app.New(root, app.Options{})
	m = resize(b, m, 120, 30)
	m = loaded(b, m, 1, root, benchEntries(root, 1000), nil)

	b.ResetTimer()
	for b.Loop() {
		next, _ := m.Update(keyMsg("j"))
		_ = next.(*app.Model).View()
	}
}

// BenchmarkResizeReflow measures the resize path: window size message
// plus a full repaint.
func BenchmarkResizeReflow(b *testing.B) {
	root := b.TempDir()
	m := app.New(root, app.Options{})
	m = resize(b, m, 120, 30)
	m = loaded(b, m, 1, root, benchEntries(root, 1000), nil)

	b.ResetTimer()
	for b.Loop() {
		next, _ := m.Update(tea.WindowSizeMsg{Width: 121, Height: 30})
		_ = next.(*app.Model).View()
	}
}
