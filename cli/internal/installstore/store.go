package installstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

const CurrentVersion = 1

// Coord identifies an installed item: identity is coordinates, not paths.
// Registry is the configured registry name; empty string means content that
// originated in the local library with no registry.
type Coord struct {
	Registry string `json:"registry,omitempty"`
	Type     string `json:"type"`
	Name     string `json:"name"`
}

// Mechanism is how a placement was materialized in a provider.
type Mechanism string

const (
	MechanismSymlink    Mechanism = "symlink"
	MechanismCopy       Mechanism = "copy"
	MechanismHookMerge  Mechanism = "hook_merge"
	MechanismMCPMerge   Mechanism = "mcp_merge"
	MechanismRuleAppend Mechanism = "rule_append"
)

// Placement records one materialization of a record in a provider.
type Placement struct {
	Provider    string    `json:"provider"`
	Mechanism   Mechanism `json:"mechanism"`
	Path        string    `json:"path"`
	Key         string    `json:"key,omitempty"`
	Scope       string    `json:"scope,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
}

// MOATProvenance is carried for MOAT-registry installs so trust can be
// re-verified later without re-deriving it from the lockfile.
type MOATProvenance struct {
	ManifestURI string    `json:"manifest_uri"`
	SourceURI   string    `json:"source_uri"`
	TrustTier   string    `json:"trust_tier,omitempty"`
	AttestedAt  time.Time `json:"attested_at,omitempty"`
}

// Record is one installed content item.
type Record struct {
	Coord
	ContentHash string          `json:"content_hash"`
	LibraryPath string          `json:"library_path"`
	MOAT        *MOATProvenance `json:"moat,omitempty"`
	InstalledAt time.Time       `json:"installed_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Placements  []Placement     `json:"placements,omitempty"`
}

// Store is the on-disk install-record store.
type Store struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`

	path string
}

// DefaultPath returns the global install-record store path.
func DefaultPath() (string, error) {
	dir, err := config.GlobalDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "installs.json"), nil
}

// Load reads the install-record store at path. A missing file returns an
// empty store bound to path so callers can save it directly.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{Version: CurrentVersion, Records: []Record{}, path: path}, nil
		}
		return nil, fmt.Errorf("reading install store %s: %w", path, err)
	}

	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("install store json %s: %w", path, err)
	}
	if s.Version > CurrentVersion {
		return nil, fmt.Errorf("install store version %d is newer than supported version %d", s.Version, CurrentVersion)
	}
	if s.Version == 0 {
		s.Version = CurrentVersion
	}
	if s.Records == nil {
		s.Records = []Record{}
	}
	s.path = path
	return &s, nil
}

// Save atomically writes the store to the path it was loaded from.
func (s *Store) Save() error {
	if s == nil {
		return errors.New("install store is nil")
	}
	if s.path == "" {
		return errors.New("install store path is empty")
	}
	if s.Version == 0 {
		s.Version = CurrentVersion
	}
	if s.Records == nil {
		s.Records = []Record{}
	}
	s.sortForSave()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling install store: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating install store dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "installs-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp install store: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp install store: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp install store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp install store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp install store: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("renaming install store into place: %w", err)
	}
	return nil
}

// Find returns the record matching c, or nil if none exists.
func (s *Store) Find(c Coord) *Record {
	if s == nil {
		return nil
	}
	for i := range s.Records {
		if s.Records[i].Coord == c {
			return &s.Records[i]
		}
	}
	return nil
}

// Upsert replaces the record with matching coordinates or appends it.
func (s *Store) Upsert(rec Record) {
	if s.Version == 0 {
		s.Version = CurrentVersion
	}
	for i := range s.Records {
		if s.Records[i].Coord == rec.Coord {
			if rec.InstalledAt.IsZero() {
				rec.InstalledAt = s.Records[i].InstalledAt
			}
			s.Records[i] = rec
			return
		}
	}
	s.Records = append(s.Records, rec)
}

// Remove deletes the record matching c and reports whether it existed.
func (s *Store) Remove(c Coord) bool {
	if s == nil {
		return false
	}
	for i := range s.Records {
		if s.Records[i].Coord != c {
			continue
		}
		copy(s.Records[i:], s.Records[i+1:])
		s.Records[len(s.Records)-1] = Record{}
		s.Records = s.Records[:len(s.Records)-1]
		return true
	}
	return false
}

// AddPlacement adds or replaces a placement for an existing record.
func (s *Store) AddPlacement(c Coord, p Placement) error {
	rec := s.Find(c)
	if rec == nil {
		return fmt.Errorf("install record not found for coord registry=%q type=%q name=%q", c.Registry, c.Type, c.Name)
	}
	for i := range rec.Placements {
		if samePlacementIdentity(rec.Placements[i], p.Provider, p.Mechanism, p.Path, p.Key) {
			rec.Placements[i] = p
			return nil
		}
	}
	rec.Placements = append(rec.Placements, p)
	return nil
}

// RemovePlacement removes a placement matching the placement identity.
func (s *Store) RemovePlacement(c Coord, provider string, m Mechanism, path, key string) bool {
	rec := s.Find(c)
	if rec == nil {
		return false
	}
	for i := range rec.Placements {
		if !samePlacementIdentity(rec.Placements[i], provider, m, path, key) {
			continue
		}
		copy(rec.Placements[i:], rec.Placements[i+1:])
		rec.Placements[len(rec.Placements)-1] = Placement{}
		rec.Placements = rec.Placements[:len(rec.Placements)-1]
		return true
	}
	return false
}

// ByProvider returns records with at least one placement for slug.
func (s *Store) ByProvider(slug string) []Record {
	if s == nil {
		return nil
	}
	var out []Record
	for _, rec := range s.Records {
		for _, p := range rec.Placements {
			if p.Provider != slug {
				continue
			}
			out = append(out, cloneRecord(rec))
			break
		}
	}
	return out
}

// ByRegistry returns records whose registry matches registry. An empty
// registry selects local-library records.
func (s *Store) ByRegistry(registry string) []Record {
	if s == nil {
		return nil
	}
	var out []Record
	for _, rec := range s.Records {
		if rec.Registry == registry {
			out = append(out, cloneRecord(rec))
		}
	}
	return out
}

func (s *Store) sortForSave() {
	sort.Slice(s.Records, func(i, j int) bool {
		a, b := s.Records[i], s.Records[j]
		if a.Registry != b.Registry {
			return a.Registry < b.Registry
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Name < b.Name
	})
	for i := range s.Records {
		sort.Slice(s.Records[i].Placements, func(a, b int) bool {
			left, right := s.Records[i].Placements[a], s.Records[i].Placements[b]
			if left.Provider != right.Provider {
				return left.Provider < right.Provider
			}
			if left.Mechanism != right.Mechanism {
				return left.Mechanism < right.Mechanism
			}
			if left.Path != right.Path {
				return left.Path < right.Path
			}
			return left.Key < right.Key
		})
	}
}

func samePlacementIdentity(p Placement, provider string, m Mechanism, path, key string) bool {
	return p.Provider == provider && p.Mechanism == m && p.Path == path && p.Key == key
}

func cloneRecord(rec Record) Record {
	if rec.MOAT != nil {
		moat := *rec.MOAT
		rec.MOAT = &moat
	}
	if rec.Placements != nil {
		rec.Placements = append([]Placement(nil), rec.Placements...)
	}
	return rec
}
