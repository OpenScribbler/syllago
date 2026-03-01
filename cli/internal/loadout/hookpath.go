package loadout

import (
	"path/filepath"
	"strings"
)

// ResolveHookCommand reads a hook item's command field and returns the
// command string to write into settings.json.
// If the command starts with "./" or "../", it is treated as relative to the item directory
// and resolved to an absolute path. Otherwise, it is returned as-is.
func ResolveHookCommand(itemDir string, command string) string {
	if strings.HasPrefix(command, "./") || strings.HasPrefix(command, "../") {
		return filepath.Join(itemDir, command)
	}
	return command
}
