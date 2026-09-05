package config_test

import (
	"strings"
	"testing"

	"gridfm/internal/config"
)

func TestKeymapValidateAcceptsGoodOverrides(t *testing.T) {
	t.Parallel()

	k := config.Keymap{"quit": "Q", "refresh": "f5", "sort": "ctrl+s"}
	if err := k.Validate(); err != nil {
		t.Fatalf("valid overrides rejected: %v", err)
	}
}

func TestKeymapValidateRejectsUnknownActions(t *testing.T) {
	t.Parallel()

	err := (config.Keymap{"explode": "x"}).Validate()
	if err == nil {
		t.Fatal("an unknown action must be rejected")
	}
	if !strings.Contains(err.Error(), "explode") {
		t.Errorf("error %q must name the action", err.Error())
	}
}

func TestKeymapValidateRejectsReservedKeys(t *testing.T) {
	t.Parallel()

	// j is movement: remapping quit onto it would break navigation.
	err := (config.Keymap{"quit": "j"}).Validate()
	if err == nil {
		t.Fatal("a reserved key must be rejected")
	}
}

func TestKeymapValidateRejectsCollisions(t *testing.T) {
	t.Parallel()

	// filter defaults to /: binding hidden to it would leave two actions
	// on one key.
	err := (config.Keymap{"hidden": "/"}).Validate()
	if err == nil {
		t.Fatal("a key owned by another action must be rejected")
	}

	// Two overrides fighting over the same key are equally broken.
	err = (config.Keymap{"quit": "x", "help": "x"}).Validate()
	if err == nil {
		t.Fatal("duplicate override keys must be rejected")
	}
}

func TestKeymapValidateRejectsEmptyKey(t *testing.T) {
	t.Parallel()

	if err := (config.Keymap{"quit": ""}).Validate(); err == nil {
		t.Fatal("an empty key must be rejected")
	}
}

func TestKeymapKeyForDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	k := config.Keymap{"quit": "Q"}
	if got := k.KeyFor("quit"); got != "Q" {
		t.Errorf("KeyFor(quit) = %q, want the override", got)
	}
	if got := k.KeyFor("zoom_in"); got != "+ / =" {
		t.Errorf("KeyFor(zoom_in) = %q, want the default pair", got)
	}
	if got := (config.Keymap{}).KeyFor("quit"); got != "q" {
		t.Errorf("KeyFor(quit) = %q, want the default", got)
	}
}

func TestLoadAppliesKeysSection(t *testing.T) {
	t.Parallel()

	path := write(t, `
[keys]
quit = "Q"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Keys.KeyFor("quit"); got != "Q" {
		t.Fatalf("quit key = %q, want Q from the [keys] section", got)
	}
}
