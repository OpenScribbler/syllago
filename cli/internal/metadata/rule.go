package metadata

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

var canonicalHashRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// LoadRuleMetadata reads and validates a .syllago.yaml file as a RuleMetadata.
func LoadRuleMetadata(path string) (*RuleMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var m RuleMetadata
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	// Hash format invariant (D11).
	for i, v := range m.Versions {
		if !canonicalHashRe.MatchString(v.Hash) {
			return nil, fmt.Errorf("%s: invalid hash format in versions[%d]: %q (want sha256:<64-hex>)", path, i, v.Hash)
		}
	}
	if !canonicalHashRe.MatchString(m.CurrentVersion) {
		return nil, fmt.Errorf("%s: invalid hash format in current_version: %q (want sha256:<64-hex>)", path, m.CurrentVersion)
	}
	found := false
	for _, v := range m.Versions {
		if v.Hash == m.CurrentVersion {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: current_version references missing hash %q", path, m.CurrentVersion)
	}
	return &m, nil
}

// RuleSource is the provenance block for a library rule (D1, D13).
type RuleSource struct {
	Provider         string    `yaml:"provider"`
	Scope            string    `yaml:"scope"` // "project" | "global"
	Path             string    `yaml:"path"`
	Format           string    `yaml:"format"`
	Filename         string    `yaml:"filename"`
	Hash             string    `yaml:"hash"`         // canonical "<algo>:<64-hex>" per D11
	SplitMethod      string    `yaml:"split_method"` // h2|h3|h4|marker|single|llm
	SplitFromSection string    `yaml:"split_from_section,omitempty"`
	CapturedAt       time.Time `yaml:"captured_at"`
}

// RuleVersionEntry is one entry in the .syllago.yaml versions[] list (D13).
type RuleVersionEntry struct {
	Hash      string    `yaml:"hash"` // canonical "<algo>:<64-hex>" per D11
	WrittenAt time.Time `yaml:"written_at"`
}

// RuleMetadata is the on-disk .syllago.yaml shape for library rules (D13).
// Distinct from Meta because rule source metadata has ~9 fields that
// benefit from nesting under a source: block (D13 "Why nested source:").
type RuleMetadata struct {
	FormatVersion  int                `yaml:"format_version"`
	ID             string             `yaml:"id"`
	Name           string             `yaml:"name"`
	Description    string             `yaml:"description,omitempty"`
	Type           string             `yaml:"type"` // always "rule"
	AddedAt        time.Time          `yaml:"added_at"`
	AddedBy        string             `yaml:"added_by,omitempty"`
	Source         RuleSource         `yaml:"source"`
	Versions       []RuleVersionEntry `yaml:"versions"`
	CurrentVersion string             `yaml:"current_version"` // must match a versions[].hash
}
