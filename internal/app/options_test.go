package app_test

import (
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/browser"
	"gridfm/internal/graphics"
)

// TestOptionsShapeStartState pins that resolved configuration reaches the
// model: hidden files, sort mode, and the panel toggles all apply at
// start.
func TestOptionsShapeStartState(t *testing.T) {
	f := false
	tr := true

	m := app.New("/d", app.Options{
		ShowHidden: true,
		Sort:       "size",
		Order:      "desc",
		Sidebar:    &f,
		Inspector:  &tr,
		Images:     graphics.ModeOff,
	})

	if !m.ShowHidden() {
		t.Error("show_hidden from options must apply at start")
	}
	if m.SortMode() != browser.SortBySize || m.SortOrder() != browser.SortDescending {
		t.Errorf("sort = %v/%v, want size/desc", m.SortMode(), m.SortOrder())
	}
	if m.SidebarOn() {
		t.Error("sidebar = false must hide the sidebar at start")
	}
	if !m.InspectorOn() {
		t.Error("inspector = true must open the panel at start")
	}
}

// TestOptionsDefaultPanels pins the nil-panel defaults: sidebar shown,
// inspector hidden, matching the pre-config behavior every existing test
// relies on.
func TestOptionsDefaultPanels(t *testing.T) {
	t.Parallel()

	m := app.New("/d", app.Options{})

	if !m.SidebarOn() {
		t.Error("the sidebar must default to visible")
	}
	if m.InspectorOn() {
		t.Error("the inspector must default to hidden")
	}
	if m.ShowHidden() {
		t.Error("hidden entries must default to filtered")
	}
}
