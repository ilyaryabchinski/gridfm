# Grid TUI File Manager Plan

Status: Draft 1  
Working title: GridFM  
Primary platform: Linux  
Implementation language: Go  
Visual reference: `/home/ilya/Pictures/screenshot-2026-09-02_11-05-28.png`

## 1. Product Summary

GridFM is a keyboard-first terminal file manager that presents files as a responsive grid of visual cards. A persistent sidebar provides fast access to common folders, bookmarks, mounts, recent locations, and active operations. The product takes the broad spatial organization of graphical file managers such as GNOME Files, but adapts it to terminal strengths: speed, keyboard control, composability, low resource usage, and transparent filesystem operations.

The central product bet is that a spatial grid is easier to scan and more pleasant to use than the list and column layouts common in terminal file managers, especially when files have recognizable types or thumbnails.

## 2. Goals

### 2.1 Product Goals

- Make browsing a directory feel visual without requiring a desktop GUI.
- Provide predictable two-dimensional keyboard navigation.
- Keep common locations visible and reachable through a persistent sidebar.
- Remain responsive in large directories and during long filesystem operations.
- Make destructive operations explicit, recoverable where possible, and difficult to trigger accidentally.
- Work well in both a full-screen terminal and a narrow split pane.
- Offer useful visuals with plain terminal capabilities, then progressively enhance supported terminals with image previews.
- Ship as a single Go binary with minimal setup.

### 2.2 Engineering Goals

- Keep filesystem behavior independent from Bubble Tea rendering code.
- Model file mutations as explicit jobs with progress, cancellation, and per-item errors.
- Test navigation and operation semantics without requiring an interactive terminal.
- Avoid blocking the Bubble Tea update loop with filesystem I/O.
- Define stable internal boundaries before introducing plugins or extensibility.

### 2.3 Non-Goals for Version 1

- Replacing a desktop file manager for every workflow.
- Remote protocols such as SFTP, SMB, FTP, or cloud storage.
- An embedded text editor.
- A general plugin runtime.
- Full archive browsing as virtual directories.
- Full filesystem operation undo.
- Cross-platform parity with Windows and macOS.
- Perfect MIME detection for every file type.

## 3. Target User and Core Workflows

The initial user is a Linux developer or terminal-heavy user who wants a more spatial and discoverable file browser without leaving the terminal.

Core workflows:

1. Launch in the current directory and visually scan its contents.
2. Move through cards using arrows or Vim keys and enter a directory.
3. Jump to Home, Downloads, Projects, Pictures, a bookmark, or a mounted volume from the sidebar.
4. Filter the current directory by typing part of a name.
5. Select one or more files and copy, move, rename, trash, or open them.
6. Inspect file metadata without leaving the browser.
7. Monitor and cancel long copy or move operations.
8. Open a file with the configured editor or desktop application.

## 4. Product Principles

### 4.1 Spatial First

Files are cards in a grid, not rows disguised as cards. Navigation must preserve the user's horizontal intent when moving between incomplete rows. Resizing must keep the same entry focused even when the number of columns changes.

### 4.2 Keyboard First, Mouse Friendly

Every action must be available from the keyboard. Mouse support may improve accessibility and discoverability, but cannot be required.

### 4.3 Safe by Default

Delete means move to trash by default. Permanent deletion and overwrite require stronger confirmation. Errors identify the affected item and do not silently abort unrelated work.

### 4.4 Responsive by Construction

Directory reads, metadata enrichment, previews, filesystem watching, and mutations run outside the render/update path. Results return as typed messages and stale results are ignored.

### 4.5 Progressive Visual Enhancement

The base experience uses color and Unicode. Nerd Font icons are optional. Kitty graphics and Sixel previews are later enhancements with reliable text fallbacks.

### 4.6 Terminal Native

The interface borrows layout ideas from graphical file managers but does not imitate draggable windows, desktop menus, or other interactions that become awkward in a terminal.

## 5. Interface Design

### 5.1 Default Layout

```text
┌─ places ───────────┬─ Home / Projects ──────────────────────────────────────┐
│                    │                                                        │
│ > Home             │  ╭────────────╮  ╭────────────╮  ╭────────────╮         │
│   Recent           │  │     DIR    │  │     DIR    │  │     GO     │         │
│   Starred          │  │            │  │            │  │            │         │
│   Downloads        │  │  frontend  │  │  backend   │  │  main.go   │         │
│   Projects         │  ╰────────────╯  ╰────────────╯  ╰────────────╯         │
│   Pictures         │                                                        │
│   Videos           │  ╭────────────╮  ╭────────────╮  ╭────────────╮         │
│                    │  │    IMAGE   │  │    TEXT    │  │   ARCHIVE  │         │
│ ────────────────   │  │            │  │            │  │            │         │
│ Disk: 48 GB free   │  │  logo.png  │  │ README.md  │  source.zip  │         │
│                    │  ╰────────────╯  ╰────────────╯  ╰────────────╯         │
├────────────────────┴────────────────────────────────────────────────────────┤
│ NORMAL  2 selected                name ascending   hidden off   17 items   │
└─────────────────────────────────────────────────────────────────────────────┘
```

The default interface has four regions:

- Header: breadcrumbs, current location, navigation history, and transient search input.
- Sidebar: places, bookmarks, mounts, recent locations, and active operation indicators.
- Grid: file cards, selection, scrolling, and empty/loading/error states.
- Status bar: current mode, selection count, sort mode, hidden-file state, item count, and shortcut hints.

### 5.2 Responsive Layout

The layout responds to terminal dimensions rather than assuming a fixed size.

- Width 100 columns or more: sidebar and normal grid cards.
- Width 70-99: narrower sidebar and compact cards.
- Width below 70: sidebar becomes a toggleable overlay; grid occupies the viewport.
- Height below 20 rows: cards switch to compact mode and nonessential status details disappear.
- If the viewport is too small to operate safely, show a minimal resize message while retaining application state.

Initial sizing targets:

- Sidebar width: 20 cells, clamped between 16 and 28.
- Normal card width: 14 cells.
- Normal card height: 5 rows.
- Compact card width: 12 cells.
- Compact card height: 3 rows.
- Card gaps: 1 row and 2 columns where space permits.

Exact dimensions should be tuned through interactive use rather than treated as API guarantees.

### 5.3 Card Presentation

Each card can show:

- A file-type icon or short type label.
- A one- or two-line display name.
- Optional metadata in detailed zoom mode.
- Selection and cursor states.
- A marker for symlinks, hidden files, read-only files, or operation errors.

State styling:

- Focused: strong border and high-contrast foreground.
- Selected: filled accent background.
- Focused and selected: accent background plus distinct border.
- Cut: visually dimmed but readable.
- Unreadable or failed: warning color and marker.

File names are never used as terminal markup. All untrusted text must be sanitized for control characters before rendering.

### 5.4 Zoom Levels

The grid supports three density levels:

- Compact: icon and truncated name, optimized for high item density.
- Normal: larger icon/type marker and up to two lines for the name.
- Detailed: name plus size, permissions, and modified time.

The user changes zoom with `+` and `-`. Zoom changes layout only; it does not change sorting, selection, or focus.

### 5.5 Sidebar

Version 1 sidebar sections:

- Places: Home, Recent, Starred, Trash.
- User folders: Downloads, Projects, Pictures, Videos when present.
- Bookmarks: user-configured local paths.
- Mounts: mounted local volumes that can be detected reliably.

Later sidebar sections:

- Saved searches.
- Git repositories.
- Active operations.
- Recently visited paths.

The sidebar and grid share one focus model. `tab` switches focus; direct shortcuts can jump to common locations without changing focus first.

### 5.6 Inspector

The inspector is an optional right-side panel toggled with `i`. It is not required for the first browsing milestone.

It may show:

- Full file name and path.
- Type and MIME type.
- Size and allocation size where available.
- Permissions and ownership.
- Created, modified, and accessed times where available.
- Symlink target.
- Image dimensions.
- Git status when inside a repository.

Metadata loads asynchronously. The inspector immediately clears stale data when focus changes.

### 5.7 Overlays

Only one blocking overlay is active at a time:

- Command palette.
- Filter/search input.
- Rename/create input.
- Confirmation dialog.
- Error details.
- Help screen.

Background jobs do not use blocking overlays. They appear in the status bar or operation shelf.

## 6. Interaction Model

### 6.1 Modes

Keep modes limited and visible:

- Browse: normal navigation and actions.
- Select: range-oriented multi-selection.
- Input: search, rename, create, or path entry.
- Overlay: command palette, confirmation, help, or error details.

Avoid reproducing all Vim modes. Mode changes must have an obvious visual indicator and an easy `esc` path back to Browse.

### 6.2 Default Keybindings

Navigation:

| Key | Action |
| --- | --- |
| Arrow keys or `h/j/k/l` | Move spatially in the active region |
| `enter` or `l` on a directory | Enter directory |
| `backspace` or `h` at the left edge | Go to parent directory |
| `alt-left` | Back in location history |
| `alt-right` | Forward in location history |
| `g` then `h` | Go to Home |
| `g` then `d` | Go to Downloads |
| `g` then `p` | Go to Projects |
| `tab` | Switch between sidebar and grid |
| `~` | Toggle sidebar |

Browsing and display:

| Key | Action |
| --- | --- |
| `/` | Filter current directory |
| `.` | Toggle hidden files |
| `s` | Open sort menu |
| `+` / `-` | Increase/decrease card size |
| `i` | Toggle inspector |
| `r` | Refresh current directory |
| `?` | Open help |

Selection and actions:

| Key | Action |
| --- | --- |
| `space` | Toggle focused entry selection |
| `v` | Enter or leave Select mode |
| `ctrl-a` | Select all visible entries |
| `esc` | Clear transient state, then selection |
| `y` | Stage selected entries for copy |
| `x` | Stage selected entries for move |
| `p` | Paste into current directory |
| `n` | Create file or directory |
| `R` | Rename focused entry |
| `d` | Move selected entries to trash |
| `D` | Permanently delete with confirmation |
| `o` or `enter` on a file | Open file |
| `:` | Open command palette |
| `q` | Quit if no blocking confirmation is needed |

Potential conflicts such as `h/l` for both spatial movement and directory navigation must be tested. One possible rule is that `h/l` always move cards while `backspace/enter` change directories. This should be settled during the navigation prototype.

### 6.3 Mouse Support

Mouse support is a version 1 stretch goal:

- Click focuses a card or sidebar item.
- Double click opens an item.
- Mouse wheel scrolls the region under the pointer.
- Ctrl-click toggles selection.
- Right click opens contextual actions if Bubble Tea and the terminal provide reliable event data.

Drag and drop is out of scope.

### 6.4 Sorting and Filtering

Initial sort modes:

- Name.
- Modified time.
- Size.
- File type.

Each mode supports ascending and descending order. Directories are grouped first by default, with a configuration option planned later.

Filtering is incremental and case-insensitive by default. It changes visible entries but does not discard selections outside the filter. The status bar must indicate when hidden selections remain.

### 6.5 Opening Files

Opening behavior:

1. A configured extension or MIME association takes precedence.
2. Text files may open in `$VISUAL`, then `$EDITOR`.
3. Other files use `xdg-open` on Linux.
4. Failure returns a visible, nonfatal error.

GridFM should suspend or safely hand off terminal control when launching an interactive terminal application, then restore and refresh on return.

## 7. Filesystem Semantics

### 7.1 Directory Loading

Directory loading occurs in two stages:

1. Fast enumeration returns names, basic type information, and enough metadata to sort.
2. Optional enrichment loads expensive metadata and previews incrementally.

Every load has a request ID and path. Results are applied only if they still match the current request, preventing stale reads from replacing a newer directory.

### 7.2 Identity and Selection

Within a loaded directory, entries are identified by full path for version 1. Navigation and resize preserve focus by entry identity, not by numeric index. If an entry disappears, focus moves to the nearest surviving item.

Selections store normalized full paths. Selection behavior across directory changes must be explicit:

- Normal navigation keeps selections to support multi-directory operations.
- `esc` clears selections.
- The status bar shows total selected count even when selected items are not visible.

Before a mutation starts, each selected path is revalidated because the filesystem may have changed since selection.

### 7.3 Copy

Copy behavior must handle:

- Files and recursive directories.
- Symlinks copied as symlinks by default.
- Permission errors on individual entries.
- Existing destination names.
- Copying into a descendant of the source, which must be rejected.
- Partial destination cleanup or clear reporting after cancellation.
- Preservation of file mode and modified time where practical.

Overwrite policy is never implicit. Conflicts produce a decision: skip, replace, rename copy, or apply a choice to all remaining conflicts.

### 7.4 Move

Moves on the same filesystem use rename where possible. Cross-filesystem moves become copy followed by source removal only after the copy succeeds. A partial cross-filesystem move must never delete a source item whose copy failed.

### 7.5 Trash and Delete

`d` uses the freedesktop.org trash specification where supported. Trash is preferred over permanent deletion.

Permanent deletion:

- Uses a distinct shortcut.
- Always presents a confirmation summarizing count and location.
- Requires stronger confirmation for directories or multiple items.
- Does not claim secure erasure.

### 7.6 Rename and Create

Rename uses an inline or centered input with the current name prefilled. Validation catches empty names, path separators, existing destinations, and platform-specific invalid values before submitting the operation.

Create supports a file and directory choice. Parent directories are not created implicitly unless a later explicit command provides that behavior.

### 7.7 Symlinks

- Directory browsing follows a symlink only when the user explicitly opens it.
- Recursive copy preserves symlinks by default rather than traversing them.
- The inspector displays the link target.
- Broken links remain visible and receive a distinct warning state.

### 7.8 Filesystem Changes

Version 1 supports manual refresh. Filesystem watching is added after basic navigation is stable.

When watching is enabled:

- Events are debounced.
- A refresh preserves focus, scroll position where possible, and selection.
- Watch errors fall back to periodic or manual refresh.
- The UI never assumes an event is complete or ordered.

## 8. Technical Architecture

### 8.1 Technology Stack

- Go: implementation language.
- Bubble Tea: application loop, commands, and terminal lifecycle.
- Lip Gloss: styling and layout.
- Bubbles: selected reusable controls such as text input, spinner, and progress.
- `os`, `io/fs`, `path/filepath`: initial filesystem implementation.
- `fsnotify`: filesystem watching after the first stable browser milestone.
- A maintained freedesktop trash package, selected after evaluation, or a small internal Linux adapter if existing packages are unsuitable.
- `golang.org/x/term`: terminal capability and size helpers if needed beyond Bubble Tea.

Dependencies should be kept modest. The grid, sidebar, focus model, and operation semantics are product-specific and should be implemented directly rather than hidden behind generic widget frameworks.

### 8.2 Package Layout

```text
cmd/gridfm/
    main.go                 CLI entry point

internal/app/
    model.go                top-level Bubble Tea model
    update.go               message routing and state transitions
    commands.go             async command construction
    messages.go             typed application messages

internal/browser/
    browser.go              location, entries, focus, history
    entry.go                entry model and display metadata
    grid.go                 spatial layout and movement
    sort.go                 deterministic sorting
    filter.go               current-directory filtering
    selection.go            cross-directory selection state

internal/places/
    places.go               standard folders and bookmarks
    mounts_linux.go         Linux mount discovery

internal/operations/
    operation.go            operation contracts and result types
    manager.go              queue, concurrency, cancellation
    copy.go
    move.go
    rename.go
    create.go
    trash_linux.go
    delete.go
    conflict.go             overwrite/conflict decisions

internal/open/
    opener.go               editor and xdg-open integration

internal/preview/
    metadata.go             inspector data
    text.go                 bounded text preview
    image.go                later image metadata/protocol support

internal/config/
    config.go
    keymap.go
    theme.go

internal/ui/
    layout.go
    header.go
    sidebar.go
    cards.go
    status.go
    inspector.go
    overlays.go
    sanitize.go

internal/platform/
    capabilities.go         terminal and OS capability detection

testdata/
```

Package boundaries may be collapsed during early implementation if they add ceremony without separation. In particular, avoid creating one-file packages merely to match this proposed tree.

### 8.3 Top-Level Model

The top-level state should contain domain state, view state, and active asynchronous requests without allowing render functions to mutate them.

Illustrative shape:

```go
type Model struct {
    Width  int
    Height int

    Browser    browser.Model
    Places     places.Model
    Operations operations.Model
    Inspector  preview.Model

    Focus   Focus
    Mode    Mode
    Overlay Overlay
    Theme   Theme

    LastError error
}
```

This is directional, not a required final API. The design should favor a small top-level update router and cohesive submodels over a single file with all behavior.

### 8.4 Messages and Commands

Use typed messages for all asynchronous completion and external events. Examples:

```go
type DirectoryLoadedMsg struct {
    RequestID uint64
    Path      string
    Entries   []browser.Entry
    Err       error
}

type OperationProgressMsg struct {
    OperationID string
    Completed   int64
    Total       int64
    CurrentPath string
}

type OperationFinishedMsg struct {
    OperationID string
    Result      operations.Result
}
```

Rules:

- Rendering performs no I/O.
- Update handlers do not wait on filesystem work.
- Every long-running command supports context cancellation where practical.
- Results include enough identity to reject stale messages.
- Errors are values delivered to the state model, not direct terminal output.

### 8.5 Grid Algorithm

Inputs:

- Available width and height.
- Card dimensions for the active zoom level.
- Horizontal and vertical gaps.
- Visible entry count.
- Focused entry identity.

Derived values:

```text
columns = max(1, floor((availableWidth + horizontalGap) /
                       (cardWidth + horizontalGap)))
rowsVisible = max(1, floor((availableHeight + verticalGap) /
                           (cardHeight + verticalGap)))
pageCapacity = columns * rowsVisible
```

Movement rules:

- Left/right move one item without wrapping between rows by default.
- Up/down move by column count.
- Moving down into a shorter last row chooses the nearest valid column.
- Page up/down move by visible row count while preserving preferred column.
- Home/end move to the first/last visible entry.
- After sort, filter, refresh, or resize, locate focus by identity before falling back to the nearest index.

Track a preferred column so repeated vertical movement remains stable across incomplete rows.

### 8.6 Rendering Strategy

- Build each card as a fixed-width block.
- Join cards horizontally into rows, then rows vertically.
- Render only visible cards plus minimal overscan if needed.
- Compute display width with a Unicode-aware width library used consistently across all components.
- Sanitize tabs, newlines, escape sequences, and other control characters in file names.
- Cache expensive derived presentation such as icon classification, but do not add caching until profiling shows value.

### 8.7 Operation Manager

The operation manager owns:

- A queue of requested jobs.
- Stable operation IDs.
- Job status and progress.
- Cancellation functions.
- Conflict questions and responses.
- Completed result summaries.

Start with one mutating job at a time. Concurrent metadata reads may continue, but serializing mutations simplifies conflict handling and destination consistency. Parallel copy can be evaluated later through profiling.

### 8.8 Configuration

Initial configuration location:

```text
$XDG_CONFIG_HOME/gridfm/config.toml
```

If `XDG_CONFIG_HOME` is unset, use `~/.config/gridfm/config.toml`.

Configuration phases:

- Phase 1: command-line flags and built-in defaults only.
- Phase 2: bookmarks, theme, icon mode, opener rules, and basic key remapping.
- Phase 3: complete keymap customization and saved searches if needed.

Do not block the first functional release on a comprehensive config schema.

### 8.9 State Persistence

Potential persisted state:

- Bookmarks.
- Last visited location.
- Recent locations.
- Preferred zoom and sort modes.
- Sidebar visibility and width.

Persist only user preferences and navigation conveniences. Do not persist an unfinished mutation queue across application restarts in version 1.

## 9. Image and Preview Strategy

### 9.1 Phase 1

- File-type icon or text label.
- Image dimensions in the inspector.
- Bounded text preview for small text files.
- No terminal graphics protocol output.

### 9.2 Phase 2

Add protocol detection and one terminal image backend, preferably Kitty graphics because the target environment can be validated directly.

Requirements:

- Correct cleanup when cards scroll out of view.
- Correct behavior in alternate screen mode.
- Resize and zoom reflow without stale images.
- Stable image placement when overlays appear.
- Bounded decoding memory and file size.
- A disabled mode for multiplexers or unsupported terminals.

### 9.3 Phase 3

Evaluate Sixel and Unicode half-block previews based on actual demand. Do not create multiple image backends before one is reliable.

## 10. Error Handling and Safety

Errors have three presentation levels:

- Inline: an individual card or sidebar location is unavailable.
- Toast/status: a recoverable action failed and needs brief feedback.
- Details overlay: multiple files failed or the user requests complete diagnostics.

Safety requirements:

- Escape control characters from all filesystem-provided strings.
- Never construct shell command strings from file names; pass arguments directly.
- Revalidate source and destination paths before mutations.
- Reject copy or move operations where a destination is inside its source tree.
- Do not traverse symlinks during recursive operations by default.
- Do not overwrite without an explicit conflict policy.
- Clearly distinguish trash from permanent delete.
- Report partial completion accurately.
- Recover terminal state after panics where feasible through normal Bubble Tea lifecycle handling.

## 11. Performance Targets

Initial performance budgets on a typical developer laptop:

- Startup to first frame: under 100 ms excluding unusually slow current directories.
- First useful directory view for 1,000 entries: under 200 ms.
- Navigation input response: under 50 ms perceived latency.
- Resize reflow: under 50 ms for visible content.
- Memory for a 10,000-entry directory without thumbnails: under 100 MB.
- No dropped interaction for longer than one frame while copying files.

These are targets to guide measurement, not promises. Add benchmarks before optimizing algorithms.

Large-directory behavior:

- Show a loading state immediately.
- Allow incremental population if enumeration proves slow.
- Avoid stat calls beyond what sorting and presentation require.
- Render only the viewport.
- Disable or defer previews during rapid navigation.

## 12. Testing Strategy

### 12.1 Unit Tests

High-value unit test areas:

- Grid column and row calculations.
- Spatial navigation across complete and incomplete rows.
- Preferred-column preservation.
- Focus preservation after resize, sort, filter, and refresh.
- Selection behavior across directory changes and filtering.
- Deterministic sorting, including case and equal values.
- File-name sanitization and display-width truncation.
- Destination validation and descendant detection.
- Conflict policy application.
- Symlink handling rules.
- Config parsing and defaulting.

### 12.2 Filesystem Integration Tests

Use temporary directories to test:

- Directory enumeration.
- Copying files and nested directories.
- Moving on the same filesystem.
- Simulated cross-filesystem move logic at the operation boundary.
- Rename and collision behavior.
- Permission failures where the test environment permits them.
- Broken and cyclic symlinks.
- Cancellation and partial results.
- Trash behavior behind a platform adapter.

Tests must never operate on the developer's real files or trash without an isolated test environment.

### 12.3 Update-Loop Tests

Drive the Bubble Tea model with messages and assert state transitions for:

- Initial directory load.
- Stale directory results.
- Resize during loading.
- Overlay input routing.
- Operation progress and completion.
- Error acknowledgement.
- Quit behavior with active jobs.

### 12.4 Golden Rendering Tests

Use golden tests sparingly for stable components:

- Default full layout at selected terminal sizes.
- Compact/narrow mode.
- Focused and selected card states.
- Empty directory, loading, and error states.
- Confirmation and help overlays.

Avoid broad golden snapshots that fail after harmless spacing or styling changes.

### 12.5 Manual Test Matrix

- Terminal sizes: 60x15, 80x24, 120x30, 180x50.
- Terminals: Kitty and at least one non-Kitty terminal.
- Plain font and Nerd Font.
- Empty, small, 1,000-entry, and 10,000-entry directories.
- Long names, spaces, newlines/control characters, Unicode, and invalid UTF-8 names where representable.
- Hidden files, symlinks, broken links, permission-denied entries, sockets, and devices.
- Active copy during navigation, resize, and quit.

## 13. Delivery Milestones

Each milestone should end with a runnable vertical slice rather than a collection of disconnected abstractions.

### Milestone 0: Navigation Prototype

Purpose: validate that a grid file manager feels good before building mutation infrastructure.

Deliverables:

- Go module and `gridfm` executable.
- Bubble Tea alternate-screen application.
- Read current directory.
- Responsive card grid with placeholder type labels.
- Arrow and Vim-style spatial navigation.
- Enter directory and go to parent.
- Resize with focus preservation.
- Minimal status bar.
- Unit tests for grid layout and movement.

Exit criteria:

- Navigation remains predictable across incomplete rows.
- Resizing never changes the focused entry when it still exists.
- Browsing a 1,000-entry directory remains responsive.
- The team chooses final `h/l` behavior based on hands-on use.

### Milestone 1: Distinctive Browser

Purpose: deliver the recognizable product shape from the visual concept.

Deliverables:

- Places sidebar.
- Header with breadcrumbs and history.
- File-type colors and Unicode/Nerd Font icon modes.
- Compact, normal, and detailed card zoom.
- Sort menu.
- Hidden-file toggle.
- Incremental current-directory filter.
- Empty, loading, and error states.
- External opener support.
- Responsive narrow layout.

Exit criteria:

- The product is useful as a read-only daily browser.
- Every feature is keyboard accessible.
- Unsupported icons fall back cleanly.
- Opening terminal and graphical applications restores terminal state correctly.

### Milestone 2: Safe Basic Mutations

Purpose: make GridFM a practical local file manager.

Deliverables:

- Single and multi-selection.
- Create file and directory.
- Rename.
- Copy and move staging.
- Serial background operation queue.
- Progress and cancellation.
- Conflict handling.
- Trash support.
- Permanent delete with strong confirmation.
- Result summary for partial failures.

Exit criteria:

- Copy, move, rename, trash, and delete integration tests pass.
- The UI remains interactive during long operations.
- No tested failure mode silently loses a source file.
- Quitting with active jobs requires an explicit decision.

### Milestone 3: Live and Inspectable

Purpose: improve confidence and awareness while navigating changing filesystems.

Deliverables:

- Inspector panel.
- Text and metadata previews.
- Debounced filesystem watching.
- Focus-preserving refresh.
- Mount discovery.
- Bookmarks and recent locations.
- Operation shelf with completed/failed job summaries.

Exit criteria:

- Rapid focus changes cannot display stale inspector data.
- Watch events do not cause cursor jumps.
- Watch failures degrade to manual refresh without breaking browsing.

### Milestone 4: Rich Visuals

Purpose: provide a genuinely visual terminal file manager where the terminal permits it.

Deliverables:

- Kitty graphics capability detection.
- Image thumbnail generation and cache.
- Thumbnail placement, scrolling, resize, and cleanup.
- User setting to disable graphics.
- Reliable text/icon fallback.

Exit criteria:

- No stale image artifacts after navigation, overlays, resize, suspend, or exit.
- Large images are decoded within strict resource limits.
- Unsupported terminals behave exactly as they did before thumbnails.

### Milestone 5: Polish and Public Release

Deliverables:

- TOML configuration.
- Key remapping for common actions.
- Theme support.
- Mouse support if reliable.
- Shell completions.
- Man page or concise reference documentation.
- Release binaries and checksums.
- Installation instructions.
- Performance benchmark results.

Exit criteria:

- Clean installation and first-run experience on supported Linux distributions.
- No known critical data-loss defects.
- Core workflows and keybindings are documented in-app and externally.
- Release artifacts are reproducible through CI.

## 14. Proposed Ticket Breakdown

The milestones can be implemented as the following tracer-bullet tickets:

1. Bootstrap Go module and terminal lifecycle.
2. Enumerate a directory into stable entry models.
3. Render responsive cards from terminal dimensions.
4. Implement and test spatial grid navigation.
5. Add directory entry, parent navigation, and history.
6. Add status bar and loading/error states.
7. Add sidebar and focus switching.
8. Add breadcrumbs and direct path navigation.
9. Add file classification, icons, and safe name rendering.
10. Add zoom levels and narrow-layout behavior.
11. Add deterministic sorting and hidden-file toggle.
12. Add incremental filtering with selection preservation.
13. Add external opener and terminal suspend/resume behavior.
14. Add selection model and visual states.
15. Add create and rename operations.
16. Add operation manager and progress messages.
17. Add copy with conflict handling and cancellation.
18. Add same- and cross-filesystem move semantics.
19. Add freedesktop trash and permanent-delete confirmation.
20. Add result summaries and operation shelf.
21. Add inspector metadata with stale-request protection.
22. Add bounded text preview.
23. Add filesystem watching and focus-preserving refresh.
24. Add bookmarks, recent locations, and mounts.
25. Prototype Kitty image placement in an isolated branch or package.
26. Integrate thumbnail caching and lifecycle.
27. Add configuration, themes, and selected key remapping.
28. Add packaging, CI, release docs, and smoke tests.

Tickets 1-6 form the first prototype. Tickets 7-13 form the read-only browser. Tickets 14-20 form the safe mutation release. Thumbnail work deliberately begins only after the core state model and rendering lifecycle are stable.

## 15. CI and Release

Initial CI checks:

- `go test ./...`
- `go vet ./...`
- Formatting check with `gofmt`.
- Static analysis with `staticcheck` once configured.
- Linux build on the supported Go version.

Release targets:

- Linux amd64.
- Linux arm64.

Potential later targets:

- macOS after platform abstractions for trash, opener, mounts, and filesystem metadata are defined.
- Package repositories such as AUR, Homebrew, or Nix after binary releases stabilize.

Use semantic versioning after the command, config, and keymap contracts become intentional. Before that point, publish `0.x` releases and call out breaking changes.

## 16. Documentation

Required documentation before public release:

- README with screenshot or terminal recording.
- Installation and supported terminal notes.
- In-app help with searchable keybindings.
- Safety semantics for trash, permanent delete, overwrite, cancellation, and partial operations.
- Configuration reference.
- Troubleshooting for icons, image previews, and terminal compatibility.
- Contributor guide describing architecture and test commands.

## 17. Risks and Mitigations

### Grid Navigation Feels Awkward

Mitigation: build Milestone 0 before mutation infrastructure and test multiple movement rules interactively. Preserve preferred columns and file identity rigorously.

### Terminal Image Protocols Destabilize Rendering

Mitigation: ship a complete icon-based browser first. Isolate graphics behind one interface and support an immediate off switch.

### Large Directories Cause Input Lag

Mitigation: asynchronous loading, viewport-only rendering, deferred previews, bounded metadata calls, and benchmarks with generated large directories.

### File Operations Cause Data Loss

Mitigation: trash by default, explicit overwrite policy, serial mutation queue, source revalidation, symlink-safe recursion, integration tests, and accurate partial-failure reporting.

### Unicode and Hostile File Names Break Layout

Mitigation: one shared sanitization and display-width layer, tests for control characters and complex Unicode, and no shell interpolation.

### Too Much Scope Before a Usable Release

Mitigation: treat the read-only grid browser as a complete milestone. Defer previews, watching, configuration breadth, plugins, and remote filesystems.

### Bubble Tea Layout Becomes Monolithic

Mitigation: keep filesystem and operation behavior independent from rendering, use typed messages, and split cohesive submodels only when their ownership is clear.

## 18. Open Decisions

These decisions should be answered through prototypes or direct product choice before their dependent milestones:

1. Does `h/l` move horizontally, navigate parent/child, or behave contextually?
2. Does the grid wrap left/right movement across rows?
3. Should selections survive directory changes by default?
4. Is Recent backed by GridFM history, desktop recent-file data, or both?
5. Which Nerd Font version and glyph set, if any, is the documented enhanced mode?
6. Which trash implementation is sufficiently maintained and specification-compliant?
7. Should opener associations be MIME-based, extension-based, or both?
8. Which Kitty environments, multiplexers, and remote sessions are supported for thumbnails?
9. Is mouse support part of version 1 or a post-release enhancement?
10. What final product name and executable name should replace GridFM?

## 19. Version 1 Definition of Done

Version 1 is complete when:

- The application launches in the requested or current directory.
- The responsive card grid works at supported terminal sizes.
- The sidebar provides common local places and bookmarks.
- Navigation, breadcrumbs, history, sorting, filtering, hidden files, and zoom are stable.
- Users can select, create, rename, copy, move, trash, and permanently delete files.
- Long operations show progress and can be cancelled where safe.
- Conflicts and partial failures are visible and actionable.
- The terminal remains responsive during all filesystem I/O.
- File names cannot inject terminal control sequences or shell syntax.
- Core navigation and filesystem semantics have automated coverage.
- Linux amd64 and arm64 binaries are produced by CI.
- Help and safety behavior are documented.

Image thumbnails, filesystem watching, inspector previews, mouse support, and deep customization improve the product but are not required to call the first safe file-management release complete unless they are explicitly promoted into version 1 scope.

## 20. Immediate Next Step

Implement Milestone 0 as a disposable-but-clean navigation prototype. Use real directory data, placeholder type labels, and no mutations. Run it in several terminal sizes and decide the `h/l`, row wrapping, card sizing, and focus-preservation rules before proceeding to the full browser.
