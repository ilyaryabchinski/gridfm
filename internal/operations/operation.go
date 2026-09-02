// Package operations executes file mutations as explicit jobs with
// progress, cancellation, conflict questions, and per-item error
// accounting. Exactly one mutating job runs at a time; results always
// report partial completion accurately. The package never touches the
// terminal.
package operations

import "errors"

// Kind selects the mutation a job performs.
type Kind int

// Operation kinds, ordered by their numeric identity for stable behavior.
const (
	OpCopy Kind = iota
	OpMove
	OpRename
	OpCreateFile
	OpCreateDir
	OpTrash
	OpDelete
)

// String renders the kind for progress displays.
func (k Kind) String() string {
	switch k {
	case OpCopy:
		return "copy"
	case OpMove:
		return "move"
	case OpRename:
		return "rename"
	case OpCreateFile:
		return "create file"
	case OpCreateDir:
		return "create directory"
	case OpTrash:
		return "trash"
	case OpDelete:
		return "delete"
	}

	return "operation"
}

// Item is one source-target pair inside a job. Create and rename jobs use
// Source for the affected path (or the parent for creates).
type Item struct {
	Source string
	Target string
}

// ItemError pins a failure to the item that caused it.
type ItemError struct {
	Path string
	Err  error
}

// Result is the accurate outcome of a finished job, including partial
// completion and cancellation.
type Result struct {
	OpID      string
	Kind      Kind
	Succeeded int
	Skipped   int
	Failed    int
	NotRun    int
	Cancelled bool
	Failures  []ItemError
}

// ConflictAction decides what happens when a target already exists.
type ConflictAction int

const (
	// ConflictSkip leaves the existing target untouched.
	ConflictSkip ConflictAction = iota
	// ConflictReplace overwrites the existing target.
	ConflictReplace
	// ConflictRename writes to a generated unique name instead.
	ConflictRename
	// ConflictAbort cancels the whole job from this conflict onward.
	ConflictAbort
)

// Answer is the user's resolution of one conflict question.
type Answer struct {
	Action   ConflictAction
	ApplyAll bool
}

// ErrSkipped marks an item the user chose not to copy over; it is
// accounted as skipped rather than failed.
var ErrSkipped = errors.New("skipped by user")

// ErrAborted marks a job the user aborted from a conflict dialog.
var ErrAborted = errors.New("aborted by user")

// ErrDestinationInsideSource rejects a job whose target lies inside its
// own source tree.
var ErrDestinationInsideSource = errors.New("destination is inside source")

// ErrEmptyOperation rejects a job without items.
var ErrEmptyOperation = errors.New("operation has no items")

// ErrUnknownKind rejects an unrecognized operation kind.
var ErrUnknownKind = errors.New("unknown operation kind")
