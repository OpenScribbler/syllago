package installer

// InstallCachedMOATToProvider — bridge between MOAT's source-cache directory
// and the existing provider-side install pipeline (bead syllago-kdxus).
//
// fetchAndRecord (cmd/syllago) downloads + verifies a MOAT item's tarball,
// extracts it under ~/.cache/syllago/moat-sources/<reg>/<item>/<sha>/, and
// records the install in the project lockfile. That cache hit alone is not
// a "real" install — the user's provider directory (~/.claude/skills/...)
// still has nothing in it.
//
// This function closes the gap by constructing a synthetic ContentItem
// rooted at the cache dir and delegating to the existing Install() pipeline.
// The caller has already verified content_hash + Rekor + signing identity,
// so we trust the cache contents and only validate the type/provider
// compatibility before invoking the filesystem side-effect.

import (
	"errors"
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// InstallCachedMOATToProvider installs an already-fetched MOAT cache
// directory to the given provider. cacheDir is the absolute path to the
// extracted source-artifact tree (the dir fetchAndRecord returned). entry
// supplies the manifest metadata; only Name, DisplayName, and Type are
// consulted — content verification is the caller's responsibility.
//
// Returns the provider-side install path (e.g. ~/.claude/skills/foo) on
// success. The signature mirrors Install() so callers using a baseDir
// override (--base-dir flag, project install root) get the same semantics.
func InstallCachedMOATToProvider(
	cacheDir string,
	entry *moat.ContentEntry,
	prov provider.Provider,
	repoRoot string,
	method InstallMethod,
	baseDir string,
) (string, error) {
	if entry == nil {
		return "", errors.New("InstallCachedMOATToProvider: entry is nil")
	}
	if cacheDir == "" {
		return "", errors.New("InstallCachedMOATToProvider: cacheDir is empty")
	}
	if _, err := os.Stat(cacheDir); err != nil {
		return "", fmt.Errorf("InstallCachedMOATToProvider: cacheDir %q: %w", cacheDir, err)
	}

	ct, ok := moat.FromMOATType(entry.Type)
	if !ok {
		return "", fmt.Errorf("InstallCachedMOATToProvider: unknown MOAT type %q", entry.Type)
	}

	if prov.SupportsType != nil && !prov.SupportsType(ct) {
		return "", fmt.Errorf("provider %q does not support %s", prov.Name, ct.Label())
	}

	item := catalog.ContentItem{
		Name:        entry.Name,
		DisplayName: entry.DisplayName,
		Type:        ct,
		Path:        cacheDir,
	}

	return Install(item, prov, repoRoot, method, baseDir)
}
