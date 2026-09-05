package ui_test

import (
	"testing"

	"gridfm/internal/browser"
	"gridfm/internal/ui"
)

func BenchmarkComputeLayout(b *testing.B) {
	for b.Loop() {
		ui.ComputeLayoutWithInspector(120, 30, ui.ZoomNormal, true, false)
	}
}

func BenchmarkRenderCardNormal(b *testing.B) {
	e := browser.Entry{Name: "bench-file.txt", Path: "/d/bench-file.txt"}
	state := ui.CardState{Focused: true}
	for b.Loop() {
		ui.RenderCard(e, ui.ZoomNormal, state, ui.IconModeUnicode)
	}
}

func BenchmarkRenderCardDetailed(b *testing.B) {
	e := browser.Entry{Name: "bench-file.txt", Path: "/d/bench-file.txt"}
	state := ui.CardState{}
	for b.Loop() {
		ui.RenderCard(e, ui.ZoomDetailed, state, ui.IconModeUnicode)
	}
}
