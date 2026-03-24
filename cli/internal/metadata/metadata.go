package metadata

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// FileName is the metadata file stored in each content item directory.
const FileName = ".syllago.yaml"

// CurrentFormatVersion is the latest .syllago.yaml format version.
// Bump this when the metadata schema changes in a way that older
// syllago versions cannot safely interpret.
const CurrentFormatVersion = 1

// Dependency represents a dependency on another content item.
type Dependency struct {
	Type string `yaml:"type"`
	Name string `yaml:"name"`
}

// Meta holds metadata for a single content item.
type Meta struct {
	FormatVersion    int          `yaml:"format_version,omitempty"` // syllago format version (1 = current)
	ID               string       `yaml:"id"`
	Name             string       `yaml:"name"`
	Description      string       `yaml:"description,omitempty"`
	Version          string       `yaml:"version,omitempty"`
	Type             string       `yaml:"type,omitempty"`
	Author           string       `yaml:"author,omitempty"`
	Source           string       `yaml:"source,omitempty"`
	Tags             []string     `yaml:"tags,omitempty"`
	Hidden           bool         `yaml:"hidden,omitempty"`
	Dependencies     []Dependency `yaml:"dependencies,omitempty"`
	CreatedAt        *time.Time   `yaml:"created_at,omitempty"` // when item was scaffolded via syllago create
	PromotedAt       *time.Time   `yaml:"promoted_at,omitempty"`
	PRBranch         string       `yaml:"pr_branch,omitempty"`
	SourceProvider   string       `yaml:"source_provider,omitempty"`   // provider slug content was imported from
	SourceFormat     string       `yaml:"source_format,omitempty"`     // original file extension (e.g. "mdc", "md")
	SourceType       string       `yaml:"source_type,omitempty"`       // git | filesystem | registry | provider
	SourceURL        string       `yaml:"source_url,omitempty"`        // for future syllago update capability
	HasSource        bool         `yaml:"has_source,omitempty"`        // whether .source/ directory exists
	SourceHash       string       `yaml:"source_hash,omitempty"`       // SHA-256 of source content at import time
	SourceRegistry   string       `yaml:"source_registry,omitempty"`   // registry name content was imported from (e.g. "acme/internal-rules")
	SourceVisibility string       `yaml:"source_visibility,omitempty"` // visibility at import time: "public", "private", "unknown"
	AddedAt          *time.Time   `yaml:"added_at,omitempty"`          // when content was added to library
	AddedBy          string       `yaml:"added_by,omitempty"`          // e.g. "syllago v0.1.0"
	SourceScope      string       `yaml:"source_scope,omitempty"`      // "global" or "project"
	SourceProject    string       `yaml:"source_project,omitempty"`    // project directory name (only when scope is "project")
}

// validateFormatVersion checks that the format version is supported.
// Zero (missing from old files) is treated as valid for backward compatibility.
func validateFormatVersion(m *Meta) error {
	if m.FormatVersion > CurrentFormatVersion {
		return fmt.Errorf("unsupported format version %d, max supported: %d", m.FormatVersion, CurrentFormatVersion)
	}
	return nil
}

// MetaPath returns the path to the metadata file in the given directory.
func MetaPath(itemDir string) string {
	return filepath.Join(itemDir, FileName)
}

// ProviderMetaPath returns the path to a provider-specific metadata file.
// Used for provider-specific content where multiple files share a directory.
func ProviderMetaPath(dir, filename string) string {
	return filepath.Join(dir, ".syllago."+filename+".yaml")
}

// Load reads .syllago.yaml from itemDir. Returns nil, nil if the file does not exist.
func Load(itemDir string) (*Meta, error) {
	data, err := os.ReadFile(MetaPath(itemDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", FileName, err)
	}
	var m Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	if err := validateFormatVersion(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return &m, nil
}

// LoadProvider reads a provider-specific metadata file. Returns nil, nil if not found.
func LoadProvider(dir, filename string) (*Meta, error) {
	path := ProviderMetaPath(dir, filename)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	var m Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	if err := validateFormatVersion(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return &m, nil
}

// Save writes .syllago.yaml to itemDir, creating directories as needed.
func Save(itemDir string, m *Meta) error {
	m.FormatVersion = CurrentFormatVersion
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	return os.WriteFile(MetaPath(itemDir), data, 0644)
}

// SaveProvider writes a provider-specific metadata file.
func SaveProvider(dir, filename string, m *Meta) error {
	m.FormatVersion = CurrentFormatVersion
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	return os.WriteFile(ProviderMetaPath(dir, filename), data, 0644)
}

// NewID generates a new UUID v4 string using crypto/rand.
func NewID() string {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant RFC4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
