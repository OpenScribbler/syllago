package installer

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// legacyInstalledRoot returns the global library dir where pre-v0.15 CLI
// installs recorded hook/MCP state, or "" when it is unavailable or equal
// to repoRoot.
func legacyInstalledRoot(repoRoot string) string {
	legacyRoot := catalog.GlobalContentDir()
	if legacyRoot == "" {
		return ""
	}

	legacyAbs, err := filepath.Abs(legacyRoot)
	if err != nil {
		return ""
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return ""
	}
	legacyAbs = filepath.Clean(legacyAbs)
	repoAbs = filepath.Clean(repoAbs)
	if legacyAbs == repoAbs {
		return ""
	}
	return legacyAbs
}
