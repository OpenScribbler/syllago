package lifecycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

// InstallRequest carries everything one Install call needs.
type InstallRequest struct {
	Item        catalog.ContentItem
	ProjectRoot string
	Targets     []Target
	// Method is symlink, copy or append. Append adds a library rule to each
	// target's monolithic rule file in ProjectRoot and ignores BaseDir and
	// Resolver.
	Method     installer.InstallMethod
	Scan       installer.ScanOptions
	Frozen     bool                         // pin the item after it installs
	Provenance *installstore.MOATProvenance // nil keeps any recorded provenance
	Source     string                       // who appended a rule: "manual", "tui"
	// Context, when set, is checked once the lock is held: a cancel during
	// the wait for the lock attempts nothing.
	Context context.Context
}

var errPinNotRegistry = errors.New("only registry items can be pinned")

// Install places the item in every target, records each placement in the
// install store, and pins the item when Frozen is set and any target took
// it.
// A failed target does not stop the others. The error joins the placement
// failures, or reports the lock when the call could not start; record and
// pin failures leave the item installed and appear only in Outcome.Failed.
func (m *Module) Install(req InstallRequest) (Outcome, error) {
	var out Outcome
	release, err := m.lock()
	if err != nil {
		out.Unattempted = append(out.Unattempted, req.Targets...)
		return out, err
	}
	defer release()
	if req.Context != nil {
		if err := req.Context.Err(); err != nil {
			out.Unattempted = append(out.Unattempted, req.Targets...)
			return out, err
		}
	}

	coord := recordCoord(req.Item)
	storePath, storeErr := installstore.DefaultPath()
	var placeErrs []error
	for _, t := range req.Targets {
		pl, err := m.placer.place(req, t)
		out.Notices = append(out.Notices, pl.Notices...)
		if err != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StagePlace, Err: err})
			placeErrs = append(placeErrs, err)
			continue
		}
		out.Changed = true
		out.Completed = append(out.Completed, Step{Target: t, Placement: pl})
		recErr := storeErr
		if recErr == nil {
			recErr = installstore.RecordInstallMeta(storePath, coord, req.Item.Path, recordPlacement(t.Provider.Slug, pl), installstore.InstallMeta{
				MOAT:      req.Provenance,
				SourceSHA: recordSourceSHA(req.Item),
			}, m.now())
		}
		if recErr != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StageRecord, Err: recordErr(recErr)})
		}
	}

	if req.Frozen && len(out.Completed) > 0 {
		var pinErr error
		switch {
		case coord.Registry == "":
			pinErr = errPinNotRegistry
		case storeErr != nil:
			pinErr = recordErr(storeErr)
		default:
			if err := installstore.SetPinned(storePath, coord, true, m.now()); err != nil {
				pinErr = recordErr(err)
			}
		}
		if pinErr != nil {
			out.Failed = append(out.Failed, Failure{Stage: StagePin, Err: pinErr})
		}
	}
	return out, errors.Join(placeErrs...)
}

func recordErr(err error) error {
	return fmt.Errorf("could not record install state: %w", err)
}
