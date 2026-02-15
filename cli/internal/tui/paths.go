package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandHome expands a leading ~/ in a path to the user's home directory.
// Returns an error if the home directory cannot be determined.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, path[2:]), nil
}
