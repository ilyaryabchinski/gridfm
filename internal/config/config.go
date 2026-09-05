// Package config loads the user's TOML configuration from
// $XDG_CONFIG_HOME/gridfm/config.toml (~/.config when unset).
//
// The loading philosophy serves the first-run experience: a missing or
// empty file is not an error — defaults apply and the app starts. A
// malformed file or an invalid value is an error naming the offending
// key, because silently ignoring a typo'd setting hides it from the user
// forever.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/BurntSushi/toml"
)

// Config is the decoded user configuration. Zero values mean "not set";
// resolution against defaults happens in Resolve so the app can layer
// command-line flags on top with flags winning.
type Config struct {
	// Icons selects the file type representation: labels, unicode, or
	// nerdfont.
	Icons string `toml:"icons"`
	// Images selects terminal thumbnail behavior: auto, on, or off.
	Images string `toml:"images"`
	// Sidebar shows the sidebar at start. nil means unset.
	Sidebar *bool `toml:"sidebar"`
	// Inspector opens the inspector panel at start. nil means unset.
	Inspector *bool `toml:"inspector"`
	// ShowHidden starts with dot-prefixed entries visible.
	ShowHidden bool `toml:"show_hidden"`
	// Sort is the initial sort mode: name, size, modified, or type.
	Sort string `toml:"sort"`
	// Order is the initial sort direction: asc or desc.
	Order string `toml:"order"`
	// Keys remaps common actions to different keys.
	Keys Keymap `toml:"keys"`
	// Theme overrides palette colors by role or category name; values
	// are ANSI 0-255 or #hex and are validated by the UI layer.
	Theme map[string]string `toml:"theme"`
	// Mouse enables mouse clicks and wheel scrolling.
	Mouse bool `toml:"mouse"`
}

// Path returns the configuration file location: $XDG_CONFIG_HOME, then
// the XDG default home fallback.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve config home: %w", err)
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, "gridfm", "config.toml"), nil
}

// Load reads and validates the configuration at path. A missing file
// yields the zero Config and no error: first run is a clean run. A
// present file with any invalid value is an error naming the key.
func Load(path string) (Config, error) {
	var cfg Config

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	if err := cfg.Keys.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Validate checks enum-valued fields, accepting empty as "unset".
func (c Config) Validate() error {
	for _, rule := range []struct {
		field string
		have  string
		want  []string
	}{
		{"icons", c.Icons, []string{"labels", "unicode", "nerdfont"}},
		{"images", c.Images, []string{"auto", "on", "off"}},
		{"sort", c.Sort, []string{"name", "size", "modified", "type"}},
		{"order", c.Order, []string{"asc", "desc"}},
	} {
		if rule.have == "" || contains(rule.want, rule.have) {
			continue
		}

		return fmt.Errorf("config: invalid %s %q (want one of %s)",
			rule.field, rule.have, join(rule.want))
	}

	return nil
}

// Resolved is Config layered over the built-in defaults, ready for the
// app to consume.
type Resolved struct {
	Icons      string
	Images     string
	Sidebar    bool
	Inspector  bool
	ShowHidden bool
	Sort       string
	Order      string
	// Keys carries the validated key remapping; use KeyFor to look up a
	// key by action.
	Keys Keymap
	// Theme carries raw color overrides for the UI layer to interpret.
	Theme map[string]string
	// Mouse mirrors the resolved mouse preference.
	Mouse bool
}

// Resolve applies defaults to every unset field.
func (c Config) Resolve() Resolved {
	r := Resolved{
		Icons:      "unicode",
		Images:     "auto",
		Sidebar:    true,
		Inspector:  false,
		ShowHidden: c.ShowHidden,
		Sort:       "name",
		Order:      "asc",
	}
	if c.Icons != "" {
		r.Icons = c.Icons
	}
	if c.Images != "" {
		r.Images = c.Images
	}
	if c.Sidebar != nil {
		r.Sidebar = *c.Sidebar
	}
	if c.Inspector != nil {
		r.Inspector = *c.Inspector
	}
	if c.Sort != "" {
		r.Sort = c.Sort
	}
	if c.Order != "" {
		r.Order = c.Order
	}
	r.Keys = c.Keys
	r.Theme = c.Theme
	r.Mouse = c.Mouse

	return r
}

func contains(valid []string, have string) bool {
	for _, v := range valid {
		if v == have {
			return true
		}
	}

	return false
}

func join(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += v
	}

	return out
}
