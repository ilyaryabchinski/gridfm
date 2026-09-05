package app

import (
	"strings"

	"gridfm/internal/config"
)

// actionQuit names the quit action across dispatch, overlays, and legend.
const actionQuit = "quit"

// keybinds resolves keys to actions. Out of the box every action owns its
// default keys; a configured override replaces that action's bindings
// with the single configured key. Dispatch switches on the action, never
// the key, so remaps take effect everywhere including the legend.
type keybinds struct {
	byKey   map[string]string // key -> action
	display map[string]string // action -> key or key pair, for the legend
}

// newKeybinds builds the table from defaults plus validated overrides.
func newKeybinds(overrides config.Keymap) *keybinds {
	kb := &keybinds{
		byKey:   map[string]string{},
		display: map[string]string{},
	}

	for action, keys := range config.DefaultBindings {
		for _, key := range keys {
			kb.byKey[key] = action
		}
		kb.display[action] = strings.Join(keys, " / ")
	}

	for action, key := range overrides {
		// An override replaces all of the action's default bindings.
		for k, a := range kb.byKey {
			if a == action {
				delete(kb.byKey, k)
			}
		}
		kb.byKey[key] = action
		kb.display[action] = key
	}

	return kb
}

// action names what a key does, or "" when the key is not bound.
func (kb *keybinds) action(key string) string { return kb.byKey[key] }

// keyFor renders the display key for an action, for the legend.
func (kb *keybinds) keyFor(action string) string { return kb.display[action] }
