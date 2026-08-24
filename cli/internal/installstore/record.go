package installstore

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrPinned is returned when an update would overwrite a pinned record.
var ErrPinned = errors.New("install record is pinned")

// PlacementInput describes one installer action to record. Keys carries the
// JSON keys touched by a merge install (one stored Placement per key); an
// empty Keys means a single placement with no key (symlink/copy).
type PlacementInput struct {
	Provider  string
	Mechanism Mechanism
	Path      string
	Keys      []string
}

// InstallMeta carries optional install-time metadata.
type InstallMeta struct {
	MOAT      *MOATProvenance // nil preserves existing provenance
	SourceSHA string          // "" preserves any existing SourceSHA
}

// RecordInstall loads the store at storePath, upserts the record for c, adds
// one placement per key (or one keyless placement), and saves. contentHash
// is computed from libraryPath via HashContent; a hash failure is returned
// as an error without writing. now stamps InstalledAt/UpdatedAt.
func RecordInstall(storePath string, c Coord, libraryPath string, p PlacementInput, now time.Time) error {
	return RecordInstallMeta(storePath, c, libraryPath, p, InstallMeta{}, now)
}

// RecordInstallMOAT is RecordInstall plus MOAT provenance. A non-nil moatProv
// replaces the record's MOAT field; nil preserves any existing provenance.
func RecordInstallMOAT(storePath string, c Coord, libraryPath string, p PlacementInput, moatProv *MOATProvenance, now time.Time) error {
	return RecordInstallMeta(storePath, c, libraryPath, p, InstallMeta{MOAT: moatProv}, now)
}

// RecordInstallMeta is RecordInstall plus optional install metadata. Non-nil
// MOAT provenance replaces the record's MOAT field; nil preserves it. A
// non-empty SourceSHA replaces the record's SourceSHA; empty preserves it.
func RecordInstallMeta(storePath string, c Coord, libraryPath string, p PlacementInput, meta InstallMeta, now time.Time) error {
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
		rec.SourceSHA = existing.SourceSHA
		rec.Pinned = existing.Pinned
		rec.PinnedAt = existing.PinnedAt
		rec.Previous = clonePrevious(existing.Previous)
	} else {
		rec.InstalledAt = now
	}
	if meta.MOAT != nil {
		rec.MOAT = cloneMOAT(meta.MOAT)
	}
	if meta.SourceSHA != "" {
		rec.SourceSHA = meta.SourceSHA
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

// SetPinned records whether c is held at its current content.
func SetPinned(storePath string, c Coord, pinned bool, now time.Time) error {
	if err := requireStoreFile(storePath); err != nil {
		return err
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}
	rec := s.Find(c)
	if rec == nil {
		return installRecordNotFound(c)
	}
	if rec.Pinned == pinned {
		return nil
	}

	rec.Pinned = pinned
	if pinned {
		rec.PinnedAt = now
	} else {
		rec.PinnedAt = time.Time{}
	}
	rec.UpdatedAt = now
	if err := s.Save(); err != nil {
		return fmt.Errorf("saving install store: %w", err)
	}
	return nil
}

// RecordUpdate updates c after its library copy has been overwritten.
func RecordUpdate(storePath string, c Coord, libraryPath string, newSourceSHA string, prevCopyPath string, now time.Time) error {
	if err := requireStoreFile(storePath); err != nil {
		return err
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}
	rec := s.Find(c)
	if rec == nil {
		return installRecordNotFound(c)
	}
	if rec.Pinned {
		return fmt.Errorf("%w for coord registry=%q type=%q name=%q", ErrPinned, c.Registry, c.Type, c.Name)
	}

	contentHash, err := HashContent(libraryPath)
	if err != nil {
		return fmt.Errorf("hashing install content: %w", err)
	}

	rec.Previous = &PreviousVersion{
		SourceSHA:   rec.SourceSHA,
		ContentHash: rec.ContentHash,
		CopyPath:    prevCopyPath,
		ReplacedAt:  now,
	}
	rec.ContentHash = contentHash
	rec.SourceSHA = newSourceSHA
	rec.UpdatedAt = now
	if err := s.Save(); err != nil {
		return fmt.Errorf("saving install store: %w", err)
	}
	return nil
}

// RecordRollback updates c after its previous library content was restored.
func RecordRollback(storePath string, c Coord, libraryPath string, now time.Time) error {
	if err := requireStoreFile(storePath); err != nil {
		return err
	}

	s, err := Load(storePath)
	if err != nil {
		return fmt.Errorf("loading install store: %w", err)
	}
	rec := s.Find(c)
	if rec == nil {
		return installRecordNotFound(c)
	}
	if rec.Previous == nil {
		return fmt.Errorf("no rollback data for coord registry=%q type=%q name=%q", c.Registry, c.Type, c.Name)
	}

	contentHash, err := HashContent(libraryPath)
	if err != nil {
		return fmt.Errorf("hashing install content: %w", err)
	}

	rec.SourceSHA = rec.Previous.SourceSHA
	rec.ContentHash = contentHash
	rec.Previous = nil
	rec.UpdatedAt = now
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

func requireStoreFile(storePath string) error {
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("install store not found: %s", storePath)
		}
		return fmt.Errorf("stat install store: %w", err)
	}
	return nil
}

func installRecordNotFound(c Coord) error {
	return fmt.Errorf("install record not found for coord registry=%q type=%q name=%q", c.Registry, c.Type, c.Name)
}
