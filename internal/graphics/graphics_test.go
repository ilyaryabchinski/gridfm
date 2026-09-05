package graphics_test

import (
	"testing"

	"gridfm/internal/graphics"
)

func envOf(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func TestDetectRecognizesCapableTerminals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"kitty window", map[string]string{"TERM": "xterm-256color", "KITTY_WINDOW_ID": "7"}, true},
		{"kitty pid", map[string]string{"TERM": "xterm-256color", "KITTY_PID": "42"}, true},
		{"kitty term", map[string]string{"TERM": "xterm-kitty"}, true},
		{"ghostty resources", map[string]string{"TERM": "xterm-256color", "GHOSTTY_RESOURCES_DIR": "/usr/share/ghostty"}, true},
		{"ghostty term", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"foot term", map[string]string{"TERM": "foot"}, false},
		{"foot extras term", map[string]string{"TERM": "foot-extra"}, false},
		{"plain xterm", map[string]string{"TERM": "xterm-256color"}, false},
		{"no term", map[string]string{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			proto, ok := graphics.Detect(envOf(tc.env))
			if ok != tc.want {
				t.Fatalf("Detect ok = %v, want %v", ok, tc.want)
			}
			if ok && proto != graphics.ProtocolKitty {
				t.Fatalf("protocol = %q, want kitty", proto)
			}
		})
	}
}

func TestDetectRejectsMultiplexers(t *testing.T) {
	t.Parallel()

	// Even with the real terminal's markers leaking through, a
	// multiplexer means auto-detection must stay off.
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"tmux", map[string]string{"TERM": "xterm-kitty", "KITTY_WINDOW_ID": "7", "TMUX": "/tmp/tmux-0/default,123"}},
		{"zellij", map[string]string{"TERM": "xterm-kitty", "ZELLIJ": "0"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, ok := graphics.Detect(envOf(tc.env)); ok {
				t.Fatal("multiplexer sessions must not auto-detect graphics")
			}
		})
	}
}
func TestResolveHonorsMode(t *testing.T) {
	t.Parallel()

	capable := envOf(map[string]string{"TERM": "xterm-kitty"})
	inMux := envOf(map[string]string{"TERM": "xterm-256color", "TMUX": "1"})

	if _, ok := graphics.Resolve(graphics.ModeOff, capable); ok {
		t.Error("mode off must win even on a capable terminal")
	}
	if _, ok := graphics.Resolve(graphics.ModeAuto, inMux); ok {
		t.Error("auto inside a multiplexer must stay off")
	}
	proto, ok := graphics.Resolve(graphics.ModeAuto, capable)
	if !ok || proto != graphics.ProtocolKitty {
		t.Errorf("auto on kitty = (%q, %v), want (kitty, true)", proto, ok)
	}
	proto, ok = graphics.Resolve(graphics.ModeOn, inMux)
	if !ok || proto != graphics.ProtocolKitty {
		t.Errorf("forced on inside a multiplexer = (%q, %v), want (kitty, true)", proto, ok)
	}
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	for s, want := range map[string]graphics.Mode{
		"auto": graphics.ModeAuto,
		"on":   graphics.ModeOn,
		"off":  graphics.ModeOff,
	} {
		got, err := graphics.ParseMode(s)
		if err != nil || got != want {
			t.Errorf("ParseMode(%q) = (%v, %v), want (%v, nil)", s, got, err, want)
		}
	}
	if _, err := graphics.ParseMode("sometimes"); err == nil {
		t.Error("ParseMode must reject unknown modes")
	}
}
