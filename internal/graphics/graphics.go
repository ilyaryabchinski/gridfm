// Package graphics detects which terminal image protocol can be used and
// resolves the user's preference against it. Detection is deliberately
// conservative: a wrong positive produces garbage on screen, a wrong
// negative only costs the thumbnails.
package graphics

import (
	"fmt"
	"strings"
)

// Protocol names a terminal image protocol this program can speak.
type Protocol string

// ProtocolKitty is the Kitty graphics protocol, spoken by kitty, foot,
// and Ghostty. It is currently the only backend.
const ProtocolKitty Protocol = "kitty"

// Mode is the user's preference for terminal graphics.
type Mode int

const (
	// ModeAuto detects support from the terminal environment.
	ModeAuto Mode = iota
	// ModeOn forces graphics on, taking responsibility for a terminal
	// that may not support them.
	ModeOn
	// ModeOff disables graphics entirely; rendering is byte-identical to
	// the pre-thumbnails behavior.
	ModeOff
)

// ParseMode maps a flag value to a Mode.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "auto":
		return ModeAuto, nil
	case "on":
		return ModeOn, nil
	case "off":
		return ModeOff, nil
	default:
		return ModeAuto, fmt.Errorf("invalid images mode %q (want auto, on, or off)", s)
	}
}

// String renders the Mode for status output.
func (m Mode) String() string {
	switch m {
	case ModeOn:
		return "on"
	case ModeOff:
		return "off"
	default:
		return "auto"
	}
}

// Detect reports which graphics protocol the running terminal speaks,
// judged from environment markers. Multiplexers hide or rewrite the
// real terminal's identity, so they are never auto-detected even when a
// capable client sits behind them; the user can still force ModeOn.
func Detect(env func(string) string) (Protocol, bool) {
	if env == nil {
		return "", false
	}

	// Multiplexers rewrite TERM and pass through escape sequences
	// unreliably; inside them, environment markers of the underlying
	// terminal are a lie.
	for _, mux := range []string{"TMUX", "ZELLIJ"} {
		if env(mux) != "" {
			return "", false
		}
	}

	// kitty exports its own markers.
	if env("KITTY_WINDOW_ID") != "" || env("KITTY_PID") != "" {
		return ProtocolKitty, true
	}

	// Ghostty exports a resources dir.
	if env("GHOSTTY_RESOURCES_DIR") != "" {
		return ProtocolKitty, true
	}

	term := env("TERM")
	switch {
	case strings.Contains(term, "kitty"), strings.Contains(term, "ghostty"):
		return ProtocolKitty, true
	case term == "foot" || strings.HasPrefix(term, "foot-"):
		return ProtocolKitty, true
	}

	return "", false
}

// Resolve combines the user's Mode with detection. ModeOff wins outright;
// ModeOn skips detection and trusts the user; ModeAuto defers to Detect.
func Resolve(mode Mode, env func(string) string) (Protocol, bool) {
	switch mode {
	case ModeOff:
		return "", false
	case ModeOn:
		return ProtocolKitty, true
	default:
		return Detect(env)
	}
}
