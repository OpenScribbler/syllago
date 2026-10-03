// Package lifecycle installs content items to providers, uninstalls them, and
// keeps the install records that go with each placement. The CLI, the TUI and any later front
// end make one call per verb here instead of assembling the installer, the
// install store and the pin from separate steps. Nothing in this package
// prints: every notice and failure comes back in the Outcome.
package lifecycle

import (
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
// effect: a record not updated or a pin not applied. Placement failures are
// not warnings; the verb's error reports them.
func (o Outcome) Warnings() []string {
	var out []string
	for _, f := range o.Failed {
		if f.Stage != StagePlace {
			out = append(out, f.Err.Error())
		}
	}
	return out
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
