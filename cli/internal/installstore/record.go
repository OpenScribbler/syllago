package installstore

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// PlacementInput describes one installer action to record. Keys carries the
// JSON keys touched by a merge install (one stored Placement per key); an
// empty Keys means a single placement with no key (symlink/copy).
type PlacementInput struct {
	Provider  string
	Mechanism Mechanism
	Path      string
	Keys      []string
}

// RecordInstall loads the store at storePath, upserts the record for c, adds
// one placement per key (or one keyless placement), and saves. contentHash
// is computed from libraryPath via HashContent; a hash failure is returned
// as an error without writing. now stamps InstalledAt/UpdatedAt.
func RecordInstall(storePath string, c Coord, libraryPath string, p PlacementInput, now time.Time) error {
	contentHash, err := HashContent(libraryPath)
	if err != nil {
		return fmt.Errorf("hashing install content: %w", err)
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}

	rec := Record{
		Coord:       c,
		ContentHash: contentHash,
		LibraryPath: libraryPath,
		UpdatedAt:   now,
	}
	if existing := s.Find(c); existing != nil {
		rec.Placements = append([]Placement(nil), existing.Placements...)
		rec.MOAT = cloneMOAT(existing.MOAT)
	} else {
		rec.InstalledAt = now
	}
	s.Upsert(rec)

	keys := placementKeys(p)
	for _, key := range keys {
		placement := Placement{
			Provider:    p.Provider,
			Mechanism:   p.Mechanism,
			Path:        p.Path,
			Key:         key,
			InstalledAt: now,
		}
		if err := s.AddPlacement(c, placement); err != nil {
			return fmt.Errorf("adding install placement: %w", err)
		}
	}

	if err := s.Save(); err != nil {
		return fmt.Errorf("saving install store: %w", err)
	}
	return nil
}

// RecordUninstall loads the store at storePath, removes the matching
// placement(s) for c, and prunes the record entirely when no placements
// remain. Missing store file, missing record, or missing placement are all
// no-ops returning nil (idempotent). Saves only when something changed.
func RecordUninstall(storePath string, c Coord, p PlacementInput, now time.Time) error {
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat install store: %w", err)
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}

	changed := false
	for _, key := range placementKeys(p) {
		if s.RemovePlacement(c, p.Provider, p.Mechanism, p.Path, key) {
			changed = true
		}
	}
	if !changed {
		return nil
	}

	if rec := s.Find(c); rec != nil {
		if len(rec.Placements) == 0 {
			s.Remove(c)
		} else {
			rec.UpdatedAt = now
		}
	}
	if err := s.Save(); err != nil {
		return fmt.Errorf("saving install store: %w", err)
	}
	return nil
}

// ForgetRecord deletes the record for c regardless of remaining placements
// (used when the item is removed from the library). Missing store or record
// is a no-op returning nil. Saves only when something changed.
func ForgetRecord(storePath string, c Coord) error {
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat install store: %w", err)
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}
	if !s.Remove(c) {
		return nil
	}
	if err := s.Save(); err != nil {
		return fmt.Errorf("saving install store: %w", err)
	}
	return nil
}

func placementKeys(p PlacementInput) []string {
	if len(p.Keys) == 0 {
		return []string{""}
	}
	return p.Keys
}

func cloneMOAT(m *MOATProvenance) *MOATProvenance {
	if m == nil {
		return nil
	}
	clone := *m
	return &clone
}
