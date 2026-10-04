// Package lifecycle installs content items to providers, uninstalls them,
// removes them from the Library, and keeps the install records that go with
// each placement. The CLI, the TUI and any later front end make one call per
// verb here instead of assembling the installer, the install store and the
// pin from separate steps. Nothing in this package
// prints: every notice and failure comes back in the Outcome.
package lifecycle

import (
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// Target is one provider an item is placed in.
type Target struct {
	Provider provider.Provider
	BaseDir  string               // empty means the provider default
	Resolver *config.PathResolver // nil means none; when set, BaseDir is ignored
	// File names the monolithic rule file an append Uninstall removes the
	// rule from. Empty means the first file installed.json records for the
	// provider.
	File string
}

// Stage names the part of a verb a Failure happened in.
type Stage string

const (
	// StagePlace is the provider-side write: placing the item on Install,
	// removing it on Uninstall. A failure here means the target did not
	// change.
	StagePlace Stage = "place"
	// StageRecord is the install store write after a successful
	// provider-side write. The target changed; only the bookkeeping is
	// missing.
	StageRecord Stage = "record"
	// StagePresence is the check for whether a target holds the item,
	// before Remove changes anything. A failure here means the check could
	// not tell, so Remove stopped.
	StagePresence Stage = "presence"
	// StagePin is the pin requested with InstallRequest.Frozen. It applies
	// to the item rather than one target, so its Failure has no Target.
	StagePin Stage = "pin"
)

// Step is one target a verb acted on.
type Step struct {
	Target    Target
	Placement installer.Placement
}

// Failure is one part of a verb that did not happen.
type Failure struct {
	Target Target
	Stage  Stage
	Err    error
}

// Outcome reports what a verb did. Every verb returns it alongside any
// error, because a call that fails partway can still have changed files.
type Outcome struct {
	Completed   []Step             // targets the verb changed
	Failed      []Failure          // in the order they happened
	Unattempted []Target           // targets the call stopped before reaching
	Notices     []installer.Notice // from every placement, failed ones included
	Changed     bool               // true when any provider-side file changed
}

// Warnings renders the failures that left the provider-side change in
// effect: a record not updated or a pin not applied. Placement and presence
// failures are not warnings; the verb's error reports them.
func (o Outcome) Warnings() []string {
	var out []string
	for _, f := range o.Failed {
		if f.Stage == StageRecord || f.Stage == StagePin {
			out = append(out, f.Err.Error())
		}
	}
	return out
}

// DecisionKind names a choice a verb needs from the user before it changes
// anything.
type DecisionKind string

const (
	// RemoveConfirm asks the user to confirm a Remove after seeing its
	// RemovePlan.
	RemoveConfirm DecisionKind = "remove_confirm"
	// OverwritePinned asks the user whether an Overwrite may replace each
	// pinned Library item it would write over.
	OverwritePinned DecisionKind = "overwrite_pinned"
	// TrustOnFirstUse asks the user to trust the signing profile a registry
	// presented the first time it was synced.
	TrustOnFirstUse DecisionKind = "trust_on_first_use"
	// PublisherWarn asks the user whether to install items their publisher
	// revoked.
	PublisherWarn DecisionKind = "publisher_warn"
	// PrivateSource asks the user whether to install items from a private
	// source repository.
	PrivateSource DecisionKind = "private_source"
)

// Decisions carries the choices the user already made, so a verb that would
// otherwise return DecisionRequired goes ahead.
type Decisions struct {
	RemoveConfirmed bool
	// Overwrite answers OverwritePinned for each pinned destination, by
	// Destination.Path: true replaces it and keeps the pin, false leaves it
	// as it is.
	Overwrite map[string]bool
}

// DecisionRequired is the error a verb returns when it needs a choice from
// the user first. Nothing has changed when it is returned. Context holds
// what the user decides on: a RemovePlan for RemoveConfirm, a
// []PinnedDestination for OverwritePinned, and for the registry install
// kinds the type moatinstall documents.
type DecisionRequired struct {
	Kind    DecisionKind
	Context any
}

func (d *DecisionRequired) Error() string {
	return fmt.Sprintf("decision required: %s", d.Kind)
}

// Module runs the lifecycle verbs. New builds the real one.
type Module struct {
	placer placer
	lock   func() (release func(), err error)
	now    func() time.Time
}

// New returns a Module that places items with the installer and serializes
// each call with the syllago-wide install lock.
func New() *Module {
	return &Module{
		placer: installerPlacer{},
		lock:   func() (func(), error) { return syllagolock.Acquire(syllagolock.DefaultTimeout) },
		now:    time.Now,
	}
}
