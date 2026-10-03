package lifecycle

import (
	"errors"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

// UninstallRequest carries everything one Uninstall call needs.
type UninstallRequest struct {
	Item        catalog.ContentItem
	ProjectRoot string
	// Targets name where the item was installed. Each target's BaseDir and
	// Resolver must match the ones its Install used.
	Targets []Target
	// Method append removes a library rule from each target's monolithic
	// rule file in ProjectRoot. Any other value removes whatever Install
	// placed in the target's install directory or settings file.
	Method installer.InstallMethod
}

// Uninstall removes the item from every target and removes each removed
// placement from the item's install record. The record itself goes when its
// last placement does; the Library item stays.
// A failed target does not stop the others. The error joins the removal
// failures, or reports the lock when the call could not start; record
// failures leave the item uninstalled and appear only in Outcome.Failed.
func (m *Module) Uninstall(req UninstallRequest) (Outcome, error) {
	var out Outcome
	release, err := m.lock()
	if err != nil {
		out.Unattempted = append(out.Unattempted, req.Targets...)
		return out, err
	}
	defer release()

	coord := recordCoord(req.Item)
	storePath, storeErr := installstore.DefaultPath()
	var unplaceErrs []error
	for _, t := range req.Targets {
		pl, err := m.placer.unplace(req, t)
		if err != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StagePlace, Err: err})
			unplaceErrs = append(unplaceErrs, err)
			continue
		}
		out.Changed = true
		out.Completed = append(out.Completed, Step{Target: t, Placement: pl})

		recErr := storeErr
		if recErr == nil {
			recErr = installstore.RecordUninstall(storePath, coord, recordPlacement(t.Provider.Slug, pl), m.now())
		}
		if recErr != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StageRecord, Err: recordErr(recErr)})
		}
	}
	return out, errors.Join(unplaceErrs...)
}
