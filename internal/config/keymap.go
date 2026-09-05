package config

import (
	"fmt"
	"sort"
	"strings"
)

// Keymap remaps common actions to different keys, keyed by action name.
// Keys use Bubble Tea's KeyMsg string vocabulary: single runes like "q",
// named keys like "f5", and modifiers like "ctrl+r".
type Keymap map[string]string

// DefaultBindings is the built-in action vocabulary: every commonly
// remapped action and the keys bound to it out of the box. Actions
// outside this set (navigation, mutations) are structural and stay fixed
// in this phase.
var DefaultBindings = map[string][]string{
	"quit":      {"q"},
	"help":      {"?"},
	"refresh":   {"r"},
	"sort":      {"s"},
	"filter":    {"/"},
	"hidden":    {"."},
	"sidebar":   {"~"},
	"inspector": {"i"},
	"zoom_in":   {"+", "="},
	"zoom_out":  {"-", "_"},
	"open":      {"o"},
}

// reservedKeys are bound to structural behavior that remapping would
// break: movement, panel focus, mutations, and history.
var reservedKeys = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"home": true, "end": true, "pgup": true, "pgdown": true,
	"enter": true, "esc": true, "tab": true, "backspace": true, " ": true,
	"j": true, "k": true, "h": true, "l": true,
	"v": true, "y": true, "x": true, "p": true, "n": true, "R": true,
	"d": true, "D": true, "c": true, "e": true, "b": true, "B": true,
	"ctrl+a": true, "ctrl+c": true, "ctrl+d": true, "ctrl+u": true,
	"alt+left": true, "alt+right": true,
}

// Validate rejects unknown actions, empty keys, reserved keys, and keys
// that collide with another action's bindings.
func (k Keymap) Validate() error {
	// The key space each action owns out of the box.
	occupied := map[string]string{}
	for action, keys := range DefaultBindings {
		for _, key := range keys {
			occupied[key] = action
		}
	}

	actions := make([]string, 0, len(k))
	for action := range k {
		actions = append(actions, action)
	}
	sort.Strings(actions) // deterministic error order

	claimed := map[string]string{} // keys claimed by earlier overrides
	for _, action := range actions {
		key := k[action]
		switch {
		case DefaultBindings[action] == nil:
			return fmt.Errorf("config: unknown key action %q", action)
		case key == "":
			return fmt.Errorf("config: key action %q needs a key", action)
		case reservedKeys[key]:
			return fmt.Errorf("config: %q is reserved and cannot be remapped to %q", key, action)
		}

		if owner, clash := occupied[key]; clash && owner != action {
			return fmt.Errorf("config: key %q already belongs to %q", key, owner)
		}
		if owner, dup := claimed[key]; dup {
			return fmt.Errorf("config: key %q is mapped twice (%s and %s)", key, owner, action)
		}
		claimed[key] = action
	}

	return nil
}

// KeyFor returns the display key for an action: the override when one is
// configured, else the first default.
func (k Keymap) KeyFor(action string) string {
	if key, ok := k[action]; ok {
		return key
	}
	if keys := DefaultBindings[action]; len(keys) > 0 {
		return strings.Join(keys, " / ")
	}

	return ""
}
