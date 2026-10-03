package lifecycle

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// Destination is a Library item an Overwrite may write over. Path need not
// exist yet.
type Destination struct {
	Type catalog.ContentType
	Name string
	Path string
}

// PinnedDestination is a destination whose install record is pinned, with
// the source commit the pin holds it at ("" when the record has none).
type PinnedDestination struct {
	Destination
	SourceSHA string
}

// Written is one destination a Write attempted: the new source commit, and
// where the copy it replaced was saved ("" when it was not kept).
type Written struct {
	Path         string
	SourceSHA    string
	PreviousCopy string
}

// OverwriteRequest carries everything one Overwrite call needs.
type OverwriteRequest struct {
	Destinations []Destination
	// Write writes the new content to approved, a subset of Destinations,
	// and reports each destination it attempted, including one it failed
	// partway through. It never sees a pinned destination the user has not
	// approved.
	Write     func(approved []Destination) ([]Written, error)
	Decisions Decisions
}

// Overwrite writes new content over Library items without silently
// replacing a pinned one. It checks each destination against the install
// record of the item already there, whichever registry that item came from,
// rather than the record of the content about to land.
//
// When a pinned destination has no answer in Decisions.Overwrite, it
// changes nothing and returns *DecisionRequired carrying every such
// destination. When some destination already exists, it also changes
// nothing when the install records or a destination's metadata cannot be
// read, because then it cannot tell what is pinned. Otherwise it calls
// Write with the approved destinations, then moves the recorded version of
// each recorded item whose content changed to Previous, even when Write
// failed partway; an approved pinned record stays pinned. Record failures
// leave the new content in place and appear only in Outcome.Failed, which
// holds nothing else. Write's error is returned after the records.
func (m *Module) Overwrite(req OverwriteRequest) (Outcome, error) {
	var out Outcome
	release, err := m.lock()
	if err != nil {
		return out, err
	}
	defer release()

	// Only an item already in the Library can be pinned, so an Overwrite
	// that replaces nothing needs no install records.
	existing := false
	for _, d := range req.Destinations {
		_, err := os.Lstat(d.Path)
		if err == nil {
			existing = true
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return out, fmt.Errorf("cannot tell whether %s exists, so nothing was written: %w", d.Path, err)
		}
	}
	if len(req.Destinations) == 0 {
		return out, nil
	}
	if !existing {
		_, err := req.Write(req.Destinations)
		return out, err
	}

	path, err := installstore.DefaultPath()
	var store *installstore.Store
	if err == nil {
		store, err = installstore.Load(path)
	}
	if err != nil {
		return out, fmt.Errorf("cannot tell which items are pinned, so nothing was written: %w", recordErr(err))
	}

	coords := make(map[string]installstore.Coord, len(req.Destinations))
	var approved []Destination
	var undecided []PinnedDestination
	for _, d := range req.Destinations {
		meta, err := metadata.Load(d.Path)
		if err != nil {
			return out, fmt.Errorf("cannot tell whether %s is pinned, so nothing was written: %w", d.Path, err)
		}
		c := recordCoord(catalog.ContentItem{Type: d.Type, Name: d.Name, Meta: meta})
		coords[d.Path] = c
		if rec := store.Find(c); rec != nil && rec.Pinned {
			ok, decided := req.Decisions.Overwrite[d.Path]
			if !decided {
				undecided = append(undecided, PinnedDestination{Destination: d, SourceSHA: rec.SourceSHA})
				continue
			}
			if !ok {
				continue
			}
		}
		approved = append(approved, d)
	}
	if len(undecided) > 0 {
		return out, &DecisionRequired{Kind: OverwritePinned, Context: undecided}
	}
	if len(approved) == 0 {
		return out, nil
	}

	// A destination counts as replaced when its content changed, so a Write
	// that fails partway still records what it replaced.
	before := make(map[string]string, len(approved))
	for _, d := range approved {
		if rec := store.Find(coords[d.Path]); rec != nil {
			hash, err := installstore.HashContent(d.Path)
			if err != nil {
				hash = rec.ContentHash
			}
			before[d.Path] = hash
		}
	}
	written, writeErr := req.Write(approved)
	reported := make(map[string]Written, len(written))
	for _, w := range written {
		reported[w.Path] = w
	}
	changed := false
	for _, d := range approved {
		rec := store.Find(coords[d.Path])
		if rec == nil {
			continue
		}
		if after, err := installstore.HashContent(d.Path); err == nil && after == before[d.Path] {
			continue
		}
		w := reported[d.Path]
		if err := rec.Rotate(d.Path, w.SourceSHA, w.PreviousCopy, m.now()); err != nil {
			out.Failed = append(out.Failed, Failure{Stage: StageRecord, Err: recordErr(err)})
			continue
		}
		changed = true
	}
	if changed {
		if err := store.Save(); err != nil {
			out.Failed = append(out.Failed, Failure{Stage: StageRecord, Err: recordErr(err)})
		}
	}
	return out, writeErr
}
