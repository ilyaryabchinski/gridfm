package places_test

import (
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/places"
)

func TestListIncludesHomeAndExistingUserDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // keep XDG lookups inside the temp dir

	err := os.Mkdir(filepath.Join(home, "Downloads"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(filepath.Join(home, "Pictures"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	// Missing dirs (Documents, Music, Videos, Projects) must be skipped.
	// A same-named file is not a place.
	err = os.WriteFile(filepath.Join(home, "Music"), []byte("x"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	list := places.List()
	if len(list) != 3 {
		t.Fatalf("places = %d, want 3 (Home, Downloads, Pictures): %+v", len(list), list)
	}
	if list[0].Label != "Home" || list[0].Path != home {
		t.Errorf("first place = %+v, want Home at %s", list[0], home)
	}
	if list[1].Label != "Downloads" {
		t.Errorf("second place = %+v, want Downloads", list[1])
	}
}

func TestListWithoutHomeIsEmpty(t *testing.T) {
	t.Setenv("HOME", "")

	if list := places.List(); list != nil {
		t.Errorf("places without a home = %+v, want empty", list)
	}
}

func TestHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := places.Home(); got != home {
		t.Errorf("Home() = %q, want %q", got, home)
	}
}
