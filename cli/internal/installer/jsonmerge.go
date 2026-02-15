package installer

import (
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

// writeJSONFile writes data to a JSON file, creating parent dirs if needed.
func writeJSONFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
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
