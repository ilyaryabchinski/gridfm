package app

import (
	"slices"

	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"gridfm/internal/operations"

	tea "github.com/charmbracelet/bubbletea"
)

// This file owns the mutation workflows: staging, pasting, creating,
// renaming, trashing, and deleting, plus the overlays that gate them.
// Every mutation is validated and revalidated before it runs; the file
// system is only touched inside operation jobs. The one exception is the
// single Lstat in pathIsDirectory: deciding the strength of a permanent
// delete confirmation needs the entry's type even when the selection came
// from another directory, and one stat is cheap next to the destructive
// operation it guards.

// mutationSources returns the paths a mutation acts on: the selection when
// any exists, otherwise the focused entry.
func (m *Model) mutationSources() []string {
	if m.browser.SelectedCount() > 0 {
		return m.browser.SelectedPaths()
	}
	if path := m.FocusedPath(); path != "" {
		return []string{path}
	}

	return nil
}

// stageClipboard records the sources for the next paste.
func (m *Model) stageClipboard(kind ClipboardKind) (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}

	paths := m.mutationSources()
	if len(paths) == 0 {
		return m, nil
	}

	m.clipboard = kind
	m.clipboardPath = paths
	m.note = strconv.Itoa(len(paths)) + " staged for " + kind.String()

	return m, nil
}

// String renders the clipboard kind for notes.
func (c ClipboardKind) String() string {
	switch c {
	case ClipboardMove:
		return "move"
	case ClipboardCopy:
		return "copy"
	case ClipboardNone:
		return "nothing"
	}

	return "nothing"
}

// pasteClipboard enqueues one operation applying the staged paths to the
// browsed directory. Move pastes clear the clipboard; copies stay staged
// for repeated pastes.
func (m *Model) pasteClipboard() (tea.Model, tea.Cmd) {
	if m.region != RegionGrid || m.clipboard == ClipboardNone || len(m.clipboardPath) == 0 {
		return m, nil
	}

	kind := operations.OpCopy
	if m.clipboard == ClipboardMove {
		kind = operations.OpMove
	}

	items := make([]operations.Item, 0, len(m.clipboardPath))
	for _, src := range m.clipboardPath {
		dst := filepath.Join(m.browser.Path, filepath.Base(src))
		if src == dst {
			continue // pasting onto itself changes nothing
		}
		items = append(items, operations.Item{Source: src, Target: dst})
	}
	if len(items) == 0 {
		return m, nil
	}

	enqueueErr := m.ops.Enqueue(operations.Operation{ID: nextOpID(), Kind: kind, Items: items})
	if enqueueErr != nil {
		m.note = "cannot paste: " + enqueueErr.Error()

		return m, nil
	}

	m.note = kind.String() + " started: " + strconv.Itoa(len(items)) + " item(s)"
	if m.clipboard == ClipboardMove {
		m.clipboard = ClipboardNone
		m.clipboardPath = nil
	}

	return m, nil
}

// openCreateInput starts the create overlay in file mode; ctrl+d switches
// to directory mode before submitting.
func (m *Model) openCreateInput() (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}

	m.input = inputCreateFile
	m.inputValue = ""
	m.inputTarget = ""

	return m, nil
}

// openRenameInput starts the rename overlay prefilled with the focused
// entry's name.
func (m *Model) openRenameInput() (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}

	entry, ok := m.browser.Focused()
	if !ok {
		return m, nil
	}

	m.input = inputRename
	m.inputValue = entry.Name
	m.inputTarget = entry.Path

	return m, nil
}

// confirmMutation opens the confirmation overlay for trash or delete. A
// typed "yes" is required when deleting more than one item or any
// directory, so destructive actions are always deliberate.
func (m *Model) confirmMutation(kind confirmKind) (tea.Model, tea.Cmd) {
	if m.region != RegionGrid {
		return m, nil
	}

	paths := m.mutationSources()
	if len(paths) == 0 {
		return m, nil
	}

	detail := paths
	typed := kind == confirmDelete && m.requiresTypedConfirm(paths)

	m.confirm = kind
	m.confirmDetail = detail
	m.confirmTyped = typed
	m.confirmInput = ""

	return m, nil
}

// requiresTypedConfirm reports whether a delete needs a typed "yes":
// whenever it affects multiple items or any directory. Directory checks
// use the cached entry list first; paths from other directories are
// checked against the filesystem, since selections persist across
// directories and the typed confirmation is the last safety net.
func (m *Model) requiresTypedConfirm(paths []string) bool {
	if len(paths) > 1 {
		return true
	}

	return slices.ContainsFunc(paths, m.pathIsDirectory)
}

// pathIsDirectory reports whether the path names a directory: from the
// cached entries when present there, otherwise from the filesystem itself.
// A path whose type cannot be determined counts as a directory, so the
// stronger confirmation never silently lapses.
func (m *Model) pathIsDirectory(path string) bool {
	for _, e := range m.browser.Entries {
		if e.Path == path {
			return e.IsDir
		}
	}

	info, err := os.Lstat(path)
	if err != nil {
		// A vanished path has nothing to confirm beyond the dialog; any
		// other stat failure keeps the cautious default.
		return !errors.Is(err, fs.ErrNotExist)
	}

	return info.IsDir()
}

// handleInputOverlayKeys drives the create/rename text overlay.
func (m *Model) handleInputOverlayKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyEsc:
		m.input = inputNone

		return m, nil
	case keyEnter:
		return m.submitInput()
	case keyBackspace:
		if m.inputValue != "" {
			runes := []rune(m.inputValue)
			m.inputValue = string(runes[:len(runes)-1])
		}

		return m, nil
	case "ctrl+u":
		m.inputValue = ""

		return m, nil
	case "ctrl+d":
		// Only the create overlay advertises the kind switch; in rename it
		// must not silently turn the operation into file creation.
		if m.input == inputCreateFile || m.input == inputCreateDir {
			m.input = toggleCreateKind(m.input)
		}

		return m, nil
	}

	if isNavigationKey(key) {
		return m, nil
	}

	if isSinglePrintable(key) {
		m.inputValue += key
	}

	return m, nil
}

// toggleCreateKind switches between the file and directory create modes.
func toggleCreateKind(kind inputKind) inputKind {
	if kind == inputCreateFile {
		return inputCreateDir
	}

	return inputCreateFile
}

// isNavigationKey reports whether the key is swallowed while a text input
// owns the keyboard.
func isNavigationKey(key string) bool {
	switch key {
	case keyUp, keyDown, keyLeft, keyRight, keyPgUp, keyPgDown, keyHome, keyEnd, keyTab:
		return true
	}

	return false
}

// isSinglePrintable reports whether the key adds exactly one visible rune.
func isSinglePrintable(key string) bool {
	runes := []rune(key)

	return len(runes) == 1 && runes[0] >= 0x20 && runes[0] != 0x7f
}

// submitInput applies the create or rename input after validating the
// name: no separators, no empty values, and renames must actually change
// something.
func (m *Model) submitInput() (tea.Model, tea.Cmd) {
	name := m.inputValue
	kind := m.input
	m.input = inputNone

	if name == "" {
		return m, nil
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		m.note = "invalid name: " + name

		return m, nil
	}

	op, ok := m.operationForInput(kind, name)
	if !ok {
		return m, nil
	}

	enqueueErr := m.ops.Enqueue(op)
	if enqueueErr != nil {
		m.note = "cannot apply: " + enqueueErr.Error()

		return m, nil
	}
	m.note = op.Kind.String() + " started"

	return m, nil
}

// operationForInput builds the mutation the submitted input describes. The
// bool is false for silent no-ops: unchanged rename names and unknown
// input kinds.
func (m *Model) operationForInput(kind inputKind, name string) (operations.Operation, bool) {
	target := filepath.Join(m.browser.Path, name)

	switch kind {
	case inputCreateFile:
		return operations.Operation{ID: nextOpID(), Kind: operations.OpCreateFile,
			Items: []operations.Item{{Target: target}}}, true
	case inputCreateDir:
		return operations.Operation{ID: nextOpID(), Kind: operations.OpCreateDir,
			Items: []operations.Item{{Target: target}}}, true
	case inputRename:
		if m.inputTarget == "" || target == m.inputTarget {
			return operations.Operation{}, false // unchanged name: silent no-op
		}

		return operations.Operation{ID: nextOpID(), Kind: operations.OpRename,
			Items: []operations.Item{{Source: m.inputTarget, Target: target}}}, true
	case inputNone:
	}

	return operations.Operation{}, false
}

// handleQuestionKeys answers the open conflict question. esc aborts the
// whole job.
func (m *Model) handleQuestionKeys(key string) (tea.Model, tea.Cmd) {
	answer := operations.Answer{}
	switch key {
	case "s":
		answer.Action = operations.ConflictSkip
	case "r":
		answer.Action = operations.ConflictReplace
	case "n":
		answer.Action = operations.ConflictRename
	case "a":
		m.applyAll = !m.applyAll

		return m, nil
	case keyEsc:
		answer.Action = operations.ConflictAbort
		m.ops.CancelActive()
	default:
		return m, nil
	}

	answer.ApplyAll = m.applyAll
	if answer.Action == operations.ConflictAbort {
		answer.ApplyAll = false
	}

	if ch := m.question.answerCh; ch != nil {
		ch <- answer
	}
	m.question = nil

	return m, nil
}

// handleConfirmKeys drives confirmation overlays. When a typed
// confirmation is required, y is just another character of the answer:
// only the exact word followed by enter can proceed.
func (m *Model) handleConfirmKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyEsc, "n", "N":
		m.confirm = confirmNone

		return m, nil
	case keyBackspace:
		if m.confirmInput != "" {
			runes := []rune(m.confirmInput)
			m.confirmInput = string(runes[:len(runes)-1])
		}

		return m, nil
	}

	if !m.confirmTyped {
		switch key {
		case "y", keyEnter:
			return m.applyConfirm()
		}
	} else if key == keyEnter {
		return m.applyConfirm()
	}

	if runes := []rune(key); len(runes) == 1 && runes[0] >= 0x20 && runes[0] != 0x7f {
		m.confirmInput += key
	}

	return m, nil
}

// applyConfirm executes the confirmed mutation.
func (m *Model) applyConfirm() (tea.Model, tea.Cmd) {
	kind := m.confirm
	m.confirm = confirmNone

	switch kind {
	case confirmQuit:
		return m, tea.Quit
	case confirmTrash, confirmDelete:
	case confirmNone:
		return m, nil
	}

	if m.confirmTyped && m.confirmInput != "yes" {
		m.note = "type yes to confirm the delete"
		m.confirm = kind // stay in the dialog until it is right

		return m, nil
	}

	opKind := operations.OpTrash
	if kind == confirmDelete {
		opKind = operations.OpDelete
	}

	items := make([]operations.Item, 0, len(m.confirmDetail))
	for _, path := range m.confirmDetail {
		items = append(items, operations.Item{Source: path})
	}

	enqueueErr := m.ops.Enqueue(operations.Operation{ID: nextOpID(), Kind: opKind, Items: items})
	if enqueueErr != nil {
		m.note = "cannot apply: " + enqueueErr.Error()

		return m, nil
	}

	// Selected sources are gone once the job lands; forget them up front so
	// stale paths do not accumulate across directories.
	if m.browser.SelectedCount() > 0 {
		m.browser.ClearSelection()
	}
	m.note = opKind.String() + " started: " + strconv.Itoa(len(items)) + " item(s)"

	return m, nil
}

// opCounter produces stable operation identifiers. It is atomic because
// parallel tests drive several models at once; the production Update loop
// runs on a single goroutine.
var opCounter atomic.Int64 //nolint:gochecknoglobals // process-wide identity counter

func nextOpID() string {
	return "op-" + strconv.FormatInt(opCounter.Add(1), 10)
}
