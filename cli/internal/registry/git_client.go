package registry

// GitClient is the git-backed RegistryClient. It wraps the existing
// registry.Sync / registry.CloneDir / catalog.ScanRegistriesOnly code paths
// in the RegistryClient contract.
//
// This is Phase 1 of the RegistryClient rollout. It exists so the client
// abstraction is reachable from call sites today — Phase 2 will add
// MOATClient alongside, and callers that went through Open() will pick up
// the new backend without changes.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// GitClient implements RegistryClient by shelling out to git for sync and
// scanning the local clone directory for Items/FetchContent. Instances are
// created via NewGitClient or the Open factory in client.go.
type GitClient struct {
	name string // registry name (also the directory name under the cache)
	dir  string // absolute path to the clone directory
}

// NewGitClient constructs a GitClient for a registry whose clone lives at
// dir. The name is used for diagnostic messages and item tagging.
//
// The caller is responsible for ensuring dir exists and contains a valid
// git clone. This constructor does not validate — Sync will surface the
// error on first use if the directory is empty or not a git repo.
func NewGitClient(name, dir string) *GitClient {
	return &GitClient{name: name, dir: dir}
}

// Sync runs git pull --ff-only in the clone directory. ctx is currently
// unused because the existing Sync function spawns a synchronous git
// subprocess — wiring context cancellation into exec.Command is a separate
// follow-up (tracked implicitly by the interface contract requiring ctx
// support). For now, callers pass context and get the same behavior they
// had before; nothing regresses.
func (g *GitClient) Sync(ctx context.Context) error {
	// TODO(moat-phase2): honor ctx cancellation by switching to
	// exec.CommandContext. Not done now because package-level Sync is
	// still used by other call sites; changing its signature is a separate
	// migration.
	_ = ctx
	return Sync(g.name)
}

// Items scans the local clone directory and returns every content item
// discovered there. Equivalent to catalog.ScanRegistriesOnly with a single
// source — the catalog package already handles registry.yaml-indexed
// layouts, content-type directories, and provider-specific subdirs.
//
// Errors during scan are swallowed intentionally: the catalog package
// records them as warnings inside the Catalog. Returning an empty slice on
// scan failure matches the existing TUI behavior (a broken registry
// shouldn't wedge the rest of the CLI).
func (g *GitClient) Items() []catalog.ContentItem {
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{{
		Name: g.name,
		Path: g.dir,
	}})
	if err != nil || cat == nil {
		return nil
	}
	return cat.Items
}

// FetchContent copies item files from the local clone into dest. For git
// registries the content is already materialized on disk after Clone or
// Sync, so this is a straight recursive copy — no hash verification
// because git registries have no content_hash to verify against.
//
// If item.Path is a single file, dest will contain a copy at dest/<base>.
// If item.Path is a directory, its contents are copied recursively into
// dest.
func (g *GitClient) FetchContent(ctx context.Context, item catalog.ContentItem, dest string) error {
	_ = ctx

	if item.Path == "" {
		return fmt.Errorf("content item %q has no source path", item.Name)
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", item.Path, err)
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create dest %q: %w", dest, err)
	}

	if !info.IsDir() {
		return copyFile(item.Path, filepath.Join(dest, filepath.Base(item.Path)))
	}
	return copyDir(item.Path, dest)
}

// Type returns "git".
func (g *GitClient) Type() string {
	return TypeGit
}

// Trust returns nil. Git registries provide no cryptographic trust signal;
// representing that as nil (rather than an empty struct) lets the UI
// distinguish "no verification was attempted" from "verification produced
// empty metadata" (C9).
func (g *GitClient) Trust() *TrustMetadata {
	return nil
}

// copyFile copies src → dst. Destination parent directory must exist.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %q: %w", src, err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %q: %w", src, err)
	}
	if err := os.WriteFile(dst, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %q: %w", dst, err)
	}
	return nil
}

// copyDir recursively copies src → dst. dst must already exist.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", src, err)
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dp, 0755); err != nil {
				return fmt.Errorf("mkdir %q: %w", dp, err)
			}
			if err := copyDir(sp, dp); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sp, dp); err != nil {
			return err
		}
	}
	return nil
}
