package capmon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const lastRunFile = "last-run.json"

// WriteRunManifest persists the run manifest to <cacheRoot>/last-run.json.
func WriteRunManifest(cacheRoot string, m RunManifest) error {
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return fmt.Errorf("mkdir cache root: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run manifest: %w", err)
	}
	path := filepath.Join(cacheRoot, lastRunFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write run manifest: %w", err)
	}
	return nil
}

// ReadLastRunManifest reads the most recent run manifest from disk.
func ReadLastRunManifest(cacheRoot string) (*RunManifest, error) {
	path := filepath.Join(cacheRoot, lastRunFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run manifest: %w", err)
	}
	var m RunManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse run manifest: %w", err)
	}
	return &m, nil
}
