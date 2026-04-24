// Package installcheck implements D16's rule-append verification scan:
// cross-references installed.json.RuleAppends against actual bytes on disk
// to produce (State, Reason) tuples consumed by D17 and the TUI.
package installcheck

// State is the per-record verification state (D16).
type State int

const (
	StateFresh    State = iota // no RuleAppend record exists — install proceeds
	StateClean                 // record exists AND recorded VersionHash bytes are in TargetFile
	StateModified              // record exists BUT recorded bytes are not found
)

// Reason carries the divergence type when State is Modified (D16).
type Reason int

const (
	ReasonNone       Reason = iota
	ReasonEdited            // file present, bytes don't match
	ReasonMissing           // ENOENT
	ReasonUnreadable        // EACCES/EIO/other I/O error or bad read
)

// PerTargetState is the value stored per (LibraryID, TargetFile) pair.
type PerTargetState struct {
	State  State
	Reason Reason
}

// VerificationResult is the output of a single scan (D16 "Input -> output contract").
type VerificationResult struct {
	PerRecord map[RecordKey]PerTargetState // one tuple per RuleAppend record
	MatchSet  map[string][]string          // LibraryID -> TargetFiles that are Clean (column projection)
	Warnings  []string                     // surfaced to stderr (CLI) / toast (TUI)
}

// RecordKey is the composite key (LibraryID, TargetFile).
type RecordKey struct {
	LibraryID  string
	TargetFile string
}
