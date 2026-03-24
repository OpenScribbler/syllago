package tui_v1

import "github.com/OpenScribbler/syllago/cli/internal/config"

// expandHome expands a leading ~/ in a path to the user's home directory.
// Delegates to the shared config.ExpandHome implementation.
func expandHome(path string) (string, error) {
	return config.ExpandHome(path)
}
