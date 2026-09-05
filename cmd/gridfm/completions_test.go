package main

import (
	"strings"
	"testing"
)

func TestCompletionForAllShells(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionFor(shell)
		if err != nil {
			t.Errorf("%s: %v", shell, err)

			continue
		}
		if len(script) < 100 {
			t.Errorf("%s: script suspiciously short (%d bytes)", shell, len(script))
		}
		if !strings.Contains(script, "gridfm") {
			t.Errorf("%s: script never mentions the binary", shell)
		}
		for _, flagName := range []string{"icons", "images", "sort", "completions"} {
			if !strings.Contains(script, flagName) {
				t.Errorf("%s: script missing flag %q", shell, flagName)
			}
		}
	}
}

func TestCompletionForRejectsUnknownShell(t *testing.T) {
	t.Parallel()

	if _, err := completionFor("powershell"); err == nil {
		t.Fatal("an unknown shell must be rejected")
	}
}
