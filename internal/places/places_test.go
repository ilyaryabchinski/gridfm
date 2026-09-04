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
	// Keep every XDG lookup inside the temp dir so the host environment
	// cannot leak a Trash directory into the assertions.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

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

func TestListIncludesTrashWhenItExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataHome := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", dataHome)

	// Without a trash directory there is no Trash place.
	if list := places.List(); len(list) != 1 {
		t.Fatalf("places without trash = %+v, want only Home", list)
	}

	trash := filepath.Join(dataHome, "Trash")
	if err := os.MkdirAll(filepath.Join(trash, "files"), 0o700); err != nil {
		t.Fatal(err)
	}

	list := places.List()
	if len(list) != 2 || list[1].Label != "Trash" || list[1].Path != trash {
		t.Fatalf("places = %+v, want Home then Trash at %s", list, trash)
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
