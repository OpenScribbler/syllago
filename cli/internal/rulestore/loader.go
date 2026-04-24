package rulestore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// Loaded is the in-memory form of a rule directory (D11).
// History is the direct map[canonical-hash]bytes used by scan (D16).
type Loaded struct {
	Dir     string
	Meta    metadata.RuleMetadata
	History map[string][]byte
}

// LoadRule reads a rule directory at dir, validates the orphan invariant
// (D11), and returns the Loaded view. Bodies in History are byte-equal to
// the on-disk .history/*.md files (no re-normalization on load per D21).
func LoadRule(dir string) (*Loaded, error) {
	meta, err := metadata.LoadRuleMetadata(filepath.Join(dir, metadata.FileName))
	if err != nil {
		return nil, err
	}
	historyDir := filepath.Join(dir, ".history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("reading history dir %s: %w", historyDir, err)
	}
	history := make(map[string][]byte, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		hash, ferr := filenameToHash(e.Name())
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", historyDir, ferr)
		}
		data, rerr := os.ReadFile(filepath.Join(historyDir, e.Name()))
		if rerr != nil {
			return nil, rerr
		}
		history[hash] = data
		seen[hash] = true
	}
	// Orphan-invariant checks (D11):
	// (a) every versions[].hash must have a .history file.
	for _, v := range meta.Versions {
		if !seen[v.Hash] {
			return nil, fmt.Errorf("%s: missing history file for version %s (rebuild from another machine's library or remove the orphan versions[] entry)", dir, v.Hash)
		}
	}
	// (b) every history file must have a versions[] entry.
	versioned := make(map[string]bool, len(meta.Versions))
	for _, v := range meta.Versions {
		versioned[v.Hash] = true
	}
	for hash := range history {
		if !versioned[hash] {
			return nil, fmt.Errorf("%s: orphan history file for %s (add a versions[] entry or delete the file)", dir, hash)
		}
	}
	return &Loaded{Dir: dir, Meta: *meta, History: history}, nil
}
