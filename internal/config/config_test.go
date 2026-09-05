package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gridfm/internal/config"
)

func write(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadMissingFileIsCleanFirstRun(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("missing config must not error, got %v", err)
	}

	r := cfg.Resolve()
	if !r.Sidebar || r.Icons != "unicode" || r.Sort != "name" {
		t.Fatalf("resolved = %+v, want the documented defaults", r)
	}
}

func TestLoadFullFile(t *testing.T) {
	t.Parallel()

	path := write(t, `
icons = "labels"
images = "off"
sidebar = false
inspector = true
show_hidden = true
sort = "size"
order = "desc"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	r := cfg.Resolve()
	switch {
	case r.Icons != "labels" || r.Images != "off":
		t.Errorf("icons/images = %q/%q, want labels/off", r.Icons, r.Images)
	case r.Sidebar || !r.Inspector:
		t.Errorf("sidebar/inspector = %v/%v, want false/true", r.Sidebar, r.Inspector)
	case !r.ShowHidden:
		t.Error("show_hidden = false, want true")
	case r.Sort != "size" || r.Order != "desc":
		t.Errorf("sort/order = %q/%q, want size/desc", r.Sort, r.Order)
	}
}

func TestLoadInvalidValueNamesTheKey(t *testing.T) {
	t.Parallel()

	path := write(t, `sort = "colour"`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("an invalid enum value must be rejected")
	}
	for _, want := range []string{"sort", "colour", "name, size, modified, type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must mention %q", err.Error(), want)
		}
	}
}

func TestLoadMalformedTOMLIsAnError(t *testing.T) {
	t.Parallel()

	path := write(t, "this is [ not toml")

	if _, err := config.Load(path); err == nil {
		t.Fatal("malformed TOML must be rejected")
	}
}

func TestValidateAcceptsEmptyAsUnset(t *testing.T) {
	t.Parallel()

	if err := (config.Config{}).Validate(); err != nil {
		t.Fatalf("empty config must validate, got %v", err)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	t.Parallel()

	path := write(t, "show_hiden = true\nicons = \"labels\"\n")

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("a misspelled key must be rejected, not ignored")
	}
	if !strings.Contains(err.Error(), "show_hiden") {
		t.Errorf("error %q must name the unknown key", err.Error())
	}
}

func TestLoadAcceptsKnownSections(t *testing.T) {
	t.Parallel()

	path := write(t, `
[keys]
quit = "Q"

[theme]
accent = "12"
`)

	if _, err := config.Load(path); err != nil {
		t.Fatalf("known sections must decode: %v", err)
	}
}
