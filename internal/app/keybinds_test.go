package app_test

import (
	"strings"
	"testing"

	"gridfm/internal/app"
	"gridfm/internal/config"
)

// remappedModel builds a model with quit bound to Q and the sort menu to
// ctrl+s.
func remappedModel(t *testing.T, root string) *app.Model {
	t.Helper()

	m := app.New(root, app.Options{
		Keys: config.Keymap{"quit": "Q", "sort": "ctrl+s"},
	})

	return resize(t, m, 80, 24)
}

// TestRemappedQuitReplacesDefault pins that an override replaces the
// action's default bindings: Q quits, and the old q does nothing.
func TestRemappedQuitReplacesDefault(t *testing.T) {
	t.Parallel()

	m := remappedModel(t, "/d")

	if _, cmd := m.Update(keyMsg("Q")); cmd == nil {
		t.Error("the remapped quit key must quit")
	}

	m = remappedModel(t, "/d")
	if _, cmd := m.Update(keyMsg("q")); cmd != nil {
		t.Error("the replaced default must no longer quit")
	}
}

// TestUnremappedActionsKeepDefaults pins that other actions stay on their
// default keys when only some actions are overridden.
func TestUnremappedActionsKeepDefaults(t *testing.T) {
	t.Parallel()

	m := remappedModel(t, "/d")

	next, _ := m.Update(keyMsg("ctrl+s"))
	m = next.(*app.Model)
	if !m.SortOpen() {
		t.Fatal("the remapped sort key must open the sort menu")
	}
}

// TestSortMenuClosesWithRemappedKeys pins that overlay close keys follow
// the remapping: the sort menu closes on its own key (ctrl+s) and the
// remapped quit key, but not the old q.
func TestSortMenuClosesWithRemappedKeys(t *testing.T) {
	t.Parallel()

	m := remappedModel(t, "/d")
	next, _ := m.Update(keyMsg("ctrl+s"))
	m = next.(*app.Model)

	next, _ = m.Update(keyMsg("ctrl+s"))
	m = next.(*app.Model)
	if m.SortOpen() {
		t.Fatal("the bound sort key must close the menu")
	}

	next, _ = m.Update(keyMsg("ctrl+s"))
	m = next.(*app.Model)
	next, _ = m.Update(keyMsg("Q"))
	m = next.(*app.Model)
	if m.SortOpen() {
		t.Fatal("the quit key must close the menu")
	}

	next, _ = m.Update(keyMsg("ctrl+s"))
	m = next.(*app.Model)
	next, _ = m.Update(keyMsg("q"))
	m = next.(*app.Model)
	if !m.SortOpen() {
		t.Fatal("the replaced default must not close the menu")
	}
}

// TestLegendShowsRemappedKeys pins that the help legend renders the
// actual bound keys, so documentation never drifts from behavior.
func TestLegendShowsRemappedKeys(t *testing.T) {
	t.Parallel()

	m := remappedModel(t, "/d")
	m = press(t, m, "?")
	out := m.View()

	if !strings.Contains(out, "Q") {
		t.Error("the legend must show the remapped quit key Q")
	}
	if !strings.Contains(out, "ctrl+s") {
		t.Error("the legend must show the remapped sort key ctrl+s")
	}
}
