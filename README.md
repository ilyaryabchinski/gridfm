# gridfm

A keyboard-first terminal file manager that presents the filesystem as a
responsive grid of visual cards. Directories are spaces you move through,
not lists you read: cards tile with gaps, resize with the terminal, and
zoom between compact, normal, and detailed densities.

![](docs/screenshot.png)

## Features

- **Spatial grid navigation** — arrows or vim keys, wrapping edges,
  focus preservation across sort, filter, and resize.
- **Safe mutations** — copy, move, rename, create, trash, and delete
  through a bounded serial queue with overwrite questions, partial
  completion reporting, and a trash path that restores `.trashinfo`
  metadata.
- **Live directories** — inotify-watched with debounced refreshes and a
  dirty latch that catches changes during a load.
- **Inspector panel** — size, permissions, ownership, timestamps, and a
  bounded text preview for small files, refreshed on every applied
  directory change.
- **Image thumbnails** — kitty-graphics-protocol rendering on kitty and
  Ghostty with a bounded, LRU-evicted disk cache; hard resource limits
  against oversized or hostile images; reliable icon fallback everywhere
  else.
- **Library sidebar** — places, bookmarks, mounts, and recent locations.
- **Theming and remapping** — `[theme]` color palettes and `[keys]`
  action remapping in TOML.

## Install

### From source

```
git clone <repository-url>
cd tui-files
go build -o bin/gridfm ./cmd/gridfm
install -Dm755 bin/gridfm ~/.local/bin/gridfm
```

### From release binaries

Download the archive and checksum file from the releases page, verify,
then:

```
tar xf gridfm_*_linux_amd64.tar.gz
install -Dm755 gridfm ~/.local/bin/gridfm
```

### Shell completions

```
gridfm --completions bash | sudo tee /etc/bash_completion.d/gridfm >/dev/null
gridfm --completions zsh > "${fpath[1]}/_gridfm"   # zsh
gridfm --completions fish > ~/.config/fish/completions/gridfm.fish
```

### Man page

```
install -Dm644 docs/gridfm.1 ~/.local/share/man/man1/gridfm.1
```

## Quick start

```
gridfm                 # browse the current directory
gridfm ~/projects      # browse somewhere else
gridfm --images on     # force thumbnails (e.g. a foot build with kitty graphics)
gridfm --hidden --sort size --order desc
```

## Configuration

`$XDG_CONFIG_HOME/gridfm/config.toml` (default `~/.config/...`). A
missing file is a clean first run; an invalid value is an error naming
the key — typos are never silently ignored. Command-line flags win.

```toml
icons = "unicode"        # labels | unicode | nerdfont
images = "auto"          # auto | on | off
sidebar = true
inspector = false
show_hidden = false
sort = "name"            # name | size | modified | type
order = "asc"            # asc | desc
mouse = false            # captures the pointer; disables text selection

[keys]                   # remap common actions (movement stays fixed)
quit = "q"
refresh = "r"
# full vocabulary: quit, help, refresh, sort, filter, hidden,
# sidebar, inspector, zoom_in, zoom_out, open

[theme]                  # ANSI 0-255 or #hex
accent = "12"            # titles and headers
muted = "8"              # panel borders
warning = "3"            # notes and confirmations
info = "6"               # filter and selection indicators
strong = "15"            # focused border and mode label
dir = "4"                # category colors: dir, go, image,
go = "14"                #   archive, media, text, other
```

## The keyboard

| Keys | Action |
|---|---|
| arrows, h j k l | move |
| enter / o | open |
| backspace | parent directory |
| alt+left / alt+right | history |
| tab | switch pane |
| / | filter |
| . | hidden files |
| s | sort menu |
| + / - | card size |
| i / ~ | inspector / sidebar |
| r | refresh |
| space / v / ctrl+a | select / range / select visible |
| y / x / p | stage copy / stage move / paste |
| n / ctrl+d / R | new file / new dir / rename |
| d / D | trash / delete (typed confirm) |
| b / B | bookmark add / remove |
| c / e | cancel job / last result |
| ? | legend |
| q | quit |

Press `?` in the app for the live legend — it reflects your `[keys]`
remaps.

## Terminals

- **kitty, Ghostty** — full experience including image thumbnails.
- **foot, alacritty, and others** — the complete icon browser;
  thumbnails require a terminal that speaks the kitty graphics protocol
  (force with `--images on` if yours does).
- **multiplexers (tmux, zellij)** — thumbnails auto-disable; everything
  else works.

## Development

```
make build   # bin/gridfm
make check   # vet + lint + test
go test -race ./...
```

## License

See the repository license.
