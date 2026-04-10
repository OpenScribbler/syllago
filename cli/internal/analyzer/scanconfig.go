package analyzer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// ScanAsEntry is one path-to-type mapping in .syllago.yaml.
type ScanAsEntry struct {
	Type catalog.ContentType `yaml:"type"`
	Path string              `yaml:"path"`
}

// ScanAsConfig is the parsed content of .syllago.yaml.
type ScanAsConfig struct {
	ScanAs []ScanAsEntry `yaml:"scan-as"`
}

const scanAsConfigFile = ".syllago.yaml"

// LoadScanAsConfig reads .syllago.yaml from repoRoot.
// Returns an empty config if the file does not exist.
func LoadScanAsConfig(repoRoot string) (*ScanAsConfig, error) {
	path := filepath.Join(repoRoot, scanAsConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &ScanAsConfig{}, nil
	}
	if err != nil {
		return nil, err
	}

	// Intermediate struct with string type for validation.
	var raw struct {
		ScanAs []struct {
			Type string `yaml:"type"`
			Path string `yaml:"path"`
		} `yaml:"scan-as"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cfg := &ScanAsConfig{}
	for _, e := range raw.ScanAs {
		ct := catalog.ContentType(e.Type)
		if !IsValidContentType(ct) {
			return nil, fmt.Errorf(".syllago.yaml: unknown content type %q (valid: skills, agents, commands, rules, hooks, mcp)", e.Type)
		}
		cfg.ScanAs = append(cfg.ScanAs, ScanAsEntry{Type: ct, Path: e.Path})
	}
	return cfg, nil
}

// IsValidContentType checks whether ct is a recognized content type.
func IsValidContentType(ct catalog.ContentType) bool {
	for _, valid := range catalog.AllContentTypes() {
		if ct == valid {
			return true
		}
	}
	return false
}

// SaveScanAsConfig writes cfg to .syllago.yaml in repoRoot.
func SaveScanAsConfig(repoRoot string, cfg *ScanAsConfig) error {
	type rawEntry struct {
		Type string `yaml:"type"`
		Path string `yaml:"path"`
	}
	var raw struct {
		ScanAs []rawEntry `yaml:"scan-as"`
	}
	for _, e := range cfg.ScanAs {
		raw.ScanAs = append(raw.ScanAs, rawEntry{Type: string(e.Type), Path: e.Path})
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	dest := filepath.Join(repoRoot, scanAsConfigFile)
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// ToPathMap converts ScanAsConfig entries to the map format used by AnalysisConfig.
func (c *ScanAsConfig) ToPathMap() map[string]catalog.ContentType {
	m := make(map[string]catalog.ContentType, len(c.ScanAs))
	for _, e := range c.ScanAs {
		m[e.Path] = e.Type
	}
	return m
}
