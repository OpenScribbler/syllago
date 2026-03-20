package installer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// readJSONFile reads a JSON file, returning empty object {} if file doesn't exist.
func readJSONFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return []byte("{}"), nil
	}
	return data, err
}

// writeJSONFile writes data to a JSON file atomically with appropriate permissions.
// Files in the home directory get 0600 (user-only), project files get 0644 (readable).
func writeJSONFile(path string, data []byte) error {
	perm := os.FileMode(0644)
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			perm = 0600
		}
	}
	return writeJSONFileWithPerm(path, data, perm)
}

// writeJSONFileWithPerm writes data to a JSON file atomically using temp-then-rename
// with the specified file permissions.
func writeJSONFileWithPerm(path string, data []byte, perm os.FileMode) error {
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

	// Write to temp file with specified permissions
	if err := os.WriteFile(tempPath, data, perm); err != nil {
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
	// Match permissions from writeJSONFile: 0600 for home-dir files, 0644 for project files
	perm := os.FileMode(0644)
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			perm = 0600
		}
	}
	return os.WriteFile(path+".bak", data, perm)
}
