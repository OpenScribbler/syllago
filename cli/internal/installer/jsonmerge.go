package installer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// readJSONFile reads a JSON file, returning empty object {} if file doesn't exist.
func readJSONFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return []byte("{}"), nil
	}
	return data, err
}

// writeJSONFile writes data to a JSON file atomically using temp-then-rename.
// The target file is never left in a partially-written state.
func writeJSONFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tempPath := path + ".tmp." + hex.EncodeToString(suffix)

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename (on POSIX, rename within same filesystem is atomic)
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// backupFile creates a .bak copy of a file before modifying it.
func backupFile(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // nothing to back up
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0644)
}
