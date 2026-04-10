# Git Registry System — Implementation Plan

**Goal:** Implement a decentralized git-based registry system so teams can distribute curated AI tool collections through any git host, with per-project config, global cache, and first-class TUI integration.

**Architecture:** Registry config stored in `.syllago/config.json` → cloned to `~/.syllago/registries/<name>/` via `git` CLI → scanned by `catalog.ScanWithRegistries()` → displayed in TUI alongside local/shared items with `[registry-name]` tags.

**Tech stack:** Go, Cobra (CLI), Bubbletea (TUI), Bubblezone (mouse), Lipgloss (styling). No new Go dependencies — uses `os/exec` to shell out to `git`.

**Design doc:** `docs/plans/2026-02-23-git-registry-system-design.md`

**Build:** `make build` | **Test:** `make test`

---

## Task 1 — Extend config schema with Registry struct

**Modifies:** `cli/internal/config/config.go`

**Dependencies:** None

Add the `Registry` struct and `Registries` field to `Config`. The `omitempty` tag preserves backward compatibility — existing configs without `registries` load cleanly since Go initializes the slice to nil, and `json.Unmarshal` leaves it nil if the key is absent.

```go
// Registry represents a git-based content source registered in this project.
type Registry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Ref  string `json:"ref,omitempty"` // branch/tag/commit, defaults to default branch
}

type Config struct {
	Providers   []string          `json:"providers"`
	Registries  []Registry        `json:"registries,omitempty"`
	Preferences map[string]string `json:"preferences,omitempty"`
}
```

**Success criteria:**
- [ ] `config.Load()` on a config without `registries` key returns `cfg.Registries == nil` (not error)
- [ ] `config.Save()` with empty registries slice omits the key from JSON output
- [ ] `make build` passes

**Command to verify:**
```bash
cd /home/hhewett/.local/src/syllago && make build
```

---

## Task 2 — Create registry package: cache paths and git operations

**Creates:** `cli/internal/registry/registry.go`

**Dependencies:** Task 1

This is the core registry package. All git operations shell out to `git` — no new Go dependencies. `CacheDir` and `CloneDir` are pure path helpers; the git functions wrap `exec.Command`. `NameFromURL` strips the `.git` suffix and takes the last path segment, matching how Homebrew taps derive names from URLs.

```go
package registry

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// CacheDir returns the global registry cache directory (~/.syllago/registries).
func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".syllago", "registries"), nil
}

// CloneDir returns the path where a named registry is cloned.
func CloneDir(name string) (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, name), nil
}

// IsCloned returns true if the registry clone directory exists.
func IsCloned(name string) bool {
	dir, err := CloneDir(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(dir)
	return err == nil
}

// NameFromURL derives a registry name from a git URL.
// Examples:
//   "git@github.com:acme/syllago-tools.git" → "syllago-tools"
//   "https://github.com/acme/syllago-tools"  → "syllago-tools"
func NameFromURL(url string) string {
	// Take the last path segment
	url = strings.TrimSuffix(url, "/")
	last := url
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		last = url[i+1:]
	}
	// Strip .git suffix
	return strings.TrimSuffix(last, ".git")
}

// checkGit returns an error if git is not on PATH.
func checkGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is required for registry operations but was not found on PATH")
	}
	return nil
}

// Clone clones the given URL into the registry cache as name.
// If ref is non-empty, checks out that branch/tag after cloning.
func Clone(url, name, ref string) error {
	if !catalog.IsValidItemName(name) {
		return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
	}
	if err := checkGit(); err != nil {
		return err
	}

	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("creating registry cache: %w", err)
	}

	args := []string{"clone", url, dir}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up partial clone
		os.RemoveAll(dir)
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Sync runs git pull --ff-only in the registry clone directory.
// Returns an error if the clone does not exist or git pull fails.
func Sync(name string) error {
	if err := checkGit(); err != nil {
		return err
	}
	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed for %q: %s\n(Hint: delete the clone at ~/.syllago/registries/%s and re-run `syllago registry add`)", name, strings.TrimSpace(string(out)), name)
	}
	return nil
}

// SyncResult holds the outcome of a single registry sync.
type SyncResult struct {
	Name string
	Err  error
}

// SyncAll syncs all registries concurrently (up to 4 at a time) and returns results.
func SyncAll(names []string) []SyncResult {
	results := make([]SyncResult, len(names))
	sem := make(chan struct{}, 4) // max 4 concurrent syncs

	done := make(chan struct{}, len(names))
	for i, name := range names {
		go func(i int, name string) {
			sem <- struct{}{}
			results[i] = SyncResult{Name: name, Err: Sync(name)}
			<-sem
			done <- struct{}{}
		}(i, name)
	}
	for range names {
		<-done
	}
	return results
}

// Remove deletes the registry clone directory.
func Remove(name string) error {
	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
```

**Success criteria:**
- [ ] `NameFromURL("git@github.com:acme/syllago-tools.git")` returns `"syllago-tools"`
- [ ] `NameFromURL("https://github.com/acme/syllago-tools")` returns `"syllago-tools"`
- [ ] `NameFromURL("https://github.com/acme/syllago-tools/")` returns `"syllago-tools"`
- [ ] `make build` passes

---

## Task 3 — Write registry package tests

**Creates:** `cli/internal/registry/registry_test.go`

**Dependencies:** Task 2

Tests for the pure functions only — no git operations (those require network/git). The test file lives in the same package.

```go
package registry

import "testing"

func TestNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:acme/syllago-tools.git", "syllago-tools"},
		{"https://github.com/acme/syllago-tools.git", "syllago-tools"},
		{"https://github.com/acme/syllago-tools", "syllago-tools"},
		{"https://github.com/acme/syllago-tools/", "syllago-tools"},
		{"git@github.com:acme/my_tools.git", "my_tools"},
	}
	for _, tt := range tests {
		got := NameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("NameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
```

**Command to run:**
```bash
cd /home/hhewett/.local/src/syllago && make test 2>&1 | grep -A5 "registry"
```

**Expected output contains:**
```
ok      github.com/OpenScribbler/syllago/cli/internal/registry
```

**Success criteria:**
- [ ] `make test` passes with no failures in the registry package

---

## Task 4 — Extend catalog types for registry items

**Modifies:** `cli/internal/catalog/types.go`

**Dependencies:** Task 2

Add `Registry string` field to `ContentItem` — empty means local/shared repo item, non-empty means this item came from a named registry. Add `RegistrySource` struct for passing registry metadata into `ScanWithRegistries`. Add `ByRegistry` helper to `Catalog` for filtering.

The `Registry` field is added after `Local` to keep related source-origin fields together.

```go
// RegistrySource describes a registry to include in a multi-source scan.
type RegistrySource struct {
	Name string // registry name (used to tag items)
	Path string // absolute path to the registry clone directory
}
```

Add to `ContentItem` struct after the `Local` field:
```go
Registry string // non-empty if item came from a git registry (value is the registry name)
```

Add to the `Catalog` type (new method alongside the existing ByType, ByTypeLocal, etc.):
```go
// ByRegistry returns all items from a specific named registry.
func (c *Catalog) ByRegistry(name string) []ContentItem {
	var result []ContentItem
	for _, item := range c.Items {
		if item.Registry == name {
			result = append(result, item)
		}
	}
	return result
}

// CountRegistry returns the number of items from a specific named registry.
func (c *Catalog) CountRegistry(name string) int {
	count := 0
	for _, item := range c.Items {
		if item.Registry == name {
			count++
		}
	}
	return count
}
```

**Success criteria:**
- [ ] `ContentItem` has `Registry string` field
- [ ] `RegistrySource` struct exists in catalog package
- [ ] `Catalog.ByRegistry("team-tools")` compiles and returns the right items
- [ ] `make build` passes

---

## Task 5 — Add ScanWithRegistries to catalog scanner

**Modifies:** `cli/internal/catalog/scanner.go`

**Dependencies:** Task 4

`ScanWithRegistries` calls the existing `Scan()` first (for local/shared items), then calls `scanRoot` once per registry source, tagging each item with the registry name. Scan errors per-registry are logged to stderr but do not fail the whole scan — a bad registry clone shouldn't break the TUI.

The `scanRoot` function signature does not change — we add a new wrapper that sets `item.Registry` after each registry scan by tagging newly-added items.

The cleanest approach: track the item count before each registry scan, then set `Registry` on all newly-appended items. This avoids changing `scanRoot`'s signature.

Add after the `Scan` function:

```go
// ScanWithRegistries scans the repo root (including my-tools/) plus any provided
// registry sources. Registry items are tagged with their registry name.
// Per-registry scan errors are logged to stderr but do not fail the overall scan.
func ScanWithRegistries(repoRoot string, registries []RegistrySource) (*Catalog, error) {
	// Start with the standard scan (local + shared repo items)
	cat, err := Scan(repoRoot)
	if err != nil {
		return nil, err
	}

	// Append items from each registry
	for _, reg := range registries {
		before := len(cat.Items)
		if err := scanRoot(cat, reg.Path, false); err != nil {
			fmt.Fprintf(os.Stderr, "warning: registry %q scan error: %s\n", reg.Name, err)
			continue
		}
		// Tag all newly-appended items with the registry name
		for i := before; i < len(cat.Items); i++ {
			cat.Items[i].Registry = reg.Name
		}
	}

	return cat, nil
}
```

**Success criteria:**
- [ ] `ScanWithRegistries(root, nil)` returns the same result as `Scan(root)`
- [ ] Items from a registry source have `item.Registry == registryName`
- [ ] Items from the local repo have `item.Registry == ""`
- [ ] A scan error in one registry does not prevent other registries from scanning
- [ ] `make build` passes

---

## Task 6 — Extend installer CheckStatus for registry paths

**Modifies:** `cli/internal/installer/installer.go`
**Modifies:** `cli/internal/installer/symlink.go`

**Dependencies:** Task 2, Task 4

`IsSymlinkedTo` currently checks if a symlink points into `repoRoot`. Registry items live at `~/.syllago/registries/<name>/...`, so the check needs to also accept registry cache directories.

**Approach:** Add `IsSymlinkedToAny(path string, roots []string) bool` to `symlink.go`, then modify `CheckStatus` to accept `registryPaths []string` and call `IsSymlinkedToAny`.

Since `CheckStatus` is called from many places, we use a variadic parameter to keep all existing call sites working without changes: `CheckStatus(item, prov, repoRoot, registryPaths...)`.

In `cli/internal/installer/symlink.go`, add:
```go
// IsSymlinkedToAny checks if path is a symlink pointing into any of the given roots.
func IsSymlinkedToAny(path string, roots []string) bool {
	for _, root := range roots {
		if IsSymlinkedTo(path, root) {
			return true
		}
	}
	return false
}
```

In `cli/internal/installer/installer.go`, modify `CheckStatus` signature:
```go
// CheckStatus checks whether an item is installed for a given provider.
// registryPaths contains additional valid symlink source roots (registry cache directories).
func CheckStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string, registryPaths ...string) Status {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return checkMCPStatus(item, prov, repoRoot)
		case catalog.Hooks:
			return checkHookStatus(item, prov, repoRoot)
		}
		return StatusNotAvailable
	}

	targetPath, err := resolveTarget(item, prov)
	if err != nil {
		return StatusNotAvailable
	}

	allRoots := append([]string{repoRoot}, registryPaths...)
	if IsSymlinkedToAny(targetPath, allRoots) {
		return StatusInstalled
	}

	// Also check if target exists as a regular file (e.g., installed via copy)
	if _, err := os.Lstat(targetPath); err == nil {
		return StatusInstalled
	}

	return StatusNotInstalled
}
```

Note: `checkMCPStatus` and `checkHookStatus` in `jsonmerge.go` also call `IsSymlinkedTo` internally. Those use config file paths, not item paths, so they don't need registry path extension — registry MCP/hook items install by merging into the same provider config files, and detection is by key presence, not symlink target.

**Success criteria:**
- [ ] Existing `CheckStatus(item, prov, repoRoot)` call sites compile unchanged (variadic)
- [ ] `IsSymlinkedToAny` returns true when symlink points to any of the provided roots
- [ ] `make test` passes (existing installer tests still pass)

---

## Task 7 — Wire ScanWithRegistries into TUI launch

**Modifies:** `cli/cmd/syllago/main.go`

**Dependencies:** Tasks 1, 5

Replace the `catalog.Scan(root)` call in `runTUI` with `catalog.ScanWithRegistries`, loading the project config to get the registry list and building the `[]catalog.RegistrySource` from the global cache.

Also add the auto-sync logic: if `registryAutoSync` preference is `"true"`, sync all registries with a 5-second timeout before scanning. On timeout or failure, fall back to whatever is in the cache.

Add this helper near the top of `runTUI`, after `findContentRepoRoot()`:

```go
import (
	// add these imports:
	"context"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/registry"
)
```

Replace in `runTUI`:
```go
// OLD:
cat, err := catalog.Scan(root)
if err != nil {
	return fmt.Errorf("catalog scan failed: %w", err)
}
```

with:
```go
// Load config to get registry list (and auto-sync preference)
cfg, cfgErr := config.Load(root)
if cfgErr != nil {
	cfg = &config.Config{}
}

// Auto-sync registries if enabled (5-second timeout; failure is non-fatal)
if cfgErr == nil && cfg.Preferences["registryAutoSync"] == "true" && len(cfg.Registries) > 0 {
	names := make([]string, len(cfg.Registries))
	for i, r := range cfg.Registries {
		names[i] = r.Name
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = ctx  // The goroutine and underlying git process are intentionally abandoned on timeout — git will finish on its own.
	syncDone := make(chan struct{})
	go func() {
		registry.SyncAll(names)
		close(syncDone)
	}()
	select {
	case <-syncDone:
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "Registry auto-sync timed out, using cached content\n")
	}
	cancel()
}

// Build registry sources from config
var regSources []catalog.RegistrySource
for _, r := range cfg.Registries {
	if registry.IsCloned(r.Name) {
		dir, _ := registry.CloneDir(r.Name)
		regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
	}
}

cat, err := catalog.ScanWithRegistries(root, regSources)
if err != nil {
	return fmt.Errorf("catalog scan failed: %w", err)
}
```

Also update the rescan points inside `runTUI` — the `importDoneMsg` and `promoteDoneMsg` and `updatePullMsg` handlers in `app.go` call `catalog.Scan(a.catalog.RepoRoot)`. These need to become `catalog.ScanWithRegistries`. However, they don't have access to cfg — we need to pass regSources into `App`.

**Simplest approach:** Add `registrySources []catalog.RegistrySource` to the `App` struct, pass it in `NewApp`, and use it in the rescan calls.

Changes to `cli/internal/tui/app.go`:

In `App` struct, add:
```go
registrySources []catalog.RegistrySource
```

In `NewApp`, add parameter and assignment:

Note: Task 14 will add a `cfg *config.Config` parameter to this signature. When implementing Task 14, update the `NewApp` call in both `runTUI` and the signature.

```go
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource) App {
	return App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		registrySources: registrySources,
		screen:          screenCategory,
		focus:           focusSidebar,
		sidebar:         newSidebarModel(cat, version),
		search:          newSearchModel(),
	}
}
```

In `runTUI` in `main.go`:
```go
app := tui.NewApp(cat, providers, version, autoUpdate, regSources)
```

In `app.go`, replace all occurrences of `catalog.Scan(a.catalog.RepoRoot)` with:
```go
catalog.ScanWithRegistries(a.catalog.RepoRoot, a.registrySources)
```

There are 4 occurrences: `promoteDoneMsg`, `importDoneMsg`, `updatePullMsg`, and the post-cleanup rescan. Search pattern: `catalog.Scan(a.catalog.RepoRoot)`.

**Success criteria:**
- [ ] `make build` passes
- [ ] `syllago` launches without errors when `.syllago/config.json` has no `registries` key
- [ ] `syllago` launches without errors when `registries` key is present but repos not yet cloned

---

## Task 8 — Create syllago registry CLI command

**Creates:** `cli/cmd/syllago/registry_cmd.go`

**Dependencies:** Tasks 1, 2, 5

Follows the pattern of `config_cmd.go`. The `registry` command has four subcommands: `add`, `remove`, `list`, `sync`. Security warnings follow the design doc exactly.

```go
package main

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage git-based content registries",
	Long:  "Add, remove, list, and sync git repositories as content registries in .syllago/config.json.",
}

var registryAddCmd = &cobra.Command{
	Use:   "add <git-url>",
	Short: "Add a git registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		url := args[0]

		nameFlag, _ := cmd.Flags().GetString("name")
		refFlag, _ := cmd.Flags().GetString("ref")

		name := nameFlag
		if name == "" {
			name = registry.NameFromURL(url)
		}
		if !catalog.IsValidItemName(name) {
			return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		// Check for duplicate name
		for _, r := range cfg.Registries {
			if r.Name == name {
				return fmt.Errorf("registry %q already exists (use a different --name or remove it first)", name)
			}
		}

		// Security warning: prominent box on first registry, brief reminder otherwise
		if len(cfg.Registries) == 0 {
			fmt.Fprintf(output.Writer, `
┌──────────────────────────────────────────────────────┐
│                   SECURITY NOTICE                    │
│                                                      │
│  Registries contain AI tool content (skills, rules,  │
│  hooks, prompts) that will be available for install.  │
│  This content can influence how AI tools behave.     │
│                                                      │
│  Syllago does not operate, verify, or audit any        │
│  registry. You are responsible for reviewing what    │
│  you install. Only add registries you trust.         │
│                                                      │
│  The syllago maintainers are not affiliated with and   │
│  accept no liability for any third-party registry.   │
└──────────────────────────────────────────────────────┘

`)
		} else {
			fmt.Fprintf(output.ErrWriter, "Warning: Registry content is unverified. Only add registries you trust.\n")
		}

		// Clone the registry
		fmt.Fprintf(output.Writer, "Cloning %s as %q...\n", url, name)
		if err := registry.Clone(url, name, refFlag); err != nil {
			return err
		}

		// Scan to verify it has content — warn but don't fail
		dir, _ := registry.CloneDir(name)
		hasDirs := false
		for _, ct := range catalog.AllContentTypes() {
			info, statErr := os.Stat(filepath.Join(dir, string(ct)))
			if statErr == nil && info.IsDir() {
				hasDirs = true
				break
			}
		}
		if !hasDirs {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain content directories (skills/, rules/, etc.). Added anyway.\n", name)
		}

		// Save to config
		cfg.Registries = append(cfg.Registries, config.Registry{
			Name: name,
			URL:  url,
			Ref:  refFlag,
		})
		if err := config.Save(root, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(output.Writer, "Added registry: %s\n", name)
		return nil
	},
}

var registryRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registry and delete its local clone",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		name := args[0]

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		found := false
		var filtered []config.Registry
		for _, r := range cfg.Registries {
			if r.Name == name {
				found = true
				continue
			}
			filtered = append(filtered, r)
		}
		if !found {
			return fmt.Errorf("registry %q not found in config", name)
		}

		cfg.Registries = filtered
		if err := config.Save(root, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if err := registry.Remove(name); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not delete clone for %q: %s\n", name, err)
		}

		fmt.Fprintf(output.Writer, "Removed registry: %s\n", name)
		return nil
	},
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered registries",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		if output.JSON {
			output.Print(cfg.Registries)
			return nil
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured. Run `syllago registry add <url>` to add one.")
			return nil
		}

		fmt.Printf("%-20s  %-8s  %s\n", "NAME", "STATUS", "URL")
		fmt.Printf("%-20s  %-8s  %s\n", strings.Repeat("─", 20), strings.Repeat("─", 8), strings.Repeat("─", 40))
		for _, r := range cfg.Registries {
			status := "missing"
			if registry.IsCloned(r.Name) {
				status = "cloned"
			}
			ref := r.Ref
			if ref == "" {
				ref = "default"
			}
			fmt.Printf("%-20s  %-8s  %s  [%s]\n", r.Name, status, r.URL, ref)
		}
		return nil
	},
}

var registrySyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Sync (git pull) one or all registries",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured.")
			return nil
		}

		// Single registry sync
		if len(args) == 1 {
			name := args[0]
			if !registry.IsCloned(name) {
				return fmt.Errorf("registry %q is not cloned locally — run `syllago registry add` first", name)
			}
			fmt.Fprintf(output.Writer, "Syncing %s...\n", name)
			if err := registry.Sync(name); err != nil {
				return err
			}
			fmt.Fprintf(output.Writer, "Synced: %s\n", name)
			return nil
		}

		// Sync all
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}

		results := registry.SyncAll(names)
		hasErrors := false
		for _, res := range results {
			if res.Err != nil {
				fmt.Fprintf(output.ErrWriter, "Error syncing %s: %s\n", res.Name, res.Err)
				hasErrors = true
			} else {
				fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
			}
		}
		if hasErrors {
			return fmt.Errorf("one or more registry syncs failed")
		}
		return nil
	},
}

func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd)
	rootCmd.AddCommand(registryCmd)
}
```

Note: add `"os"` and `"path/filepath"` to imports in `registry_cmd.go`.

Note: Task 16 will replace the `init()` function above with an expanded version that also registers `registryItemsCmd` and its `--type` flag. When implementing Task 16, delete this `init()` block and use the one from Task 16 instead.

**Success criteria:**
- [ ] `syllago registry --help` shows add/remove/list/sync subcommands
- [ ] `syllago registry list` outputs "No registries configured." when none exist
- [ ] `make build` passes

**Command to verify:**
```bash
cd /home/hhewett/.local/src/syllago && make build && ./cli/syllago registry --help
```

---

## Task 9 — Add registry items tag in TUI items view

**Modifies:** `cli/internal/tui/items.go`

**Dependencies:** Task 4

Registry items display a `[registry-name]` prefix in the description column, following the same pattern as the existing `[LOCAL]` tag. The prefix is rendered in `countStyle` (same as type tags in search results) to distinguish it visually from the description text.

In `items.go`, in the `View()` method, update the `localPrefix` block (around line 318) to also handle registry items:

Replace:
```go
// Build description with LOCAL prefix for local items
localPrefix := ""
localPrefixLen := 0
if item.Local {
	localPrefix = warningStyle.Render("[LOCAL]") + " "
	localPrefixLen = 8 // "[LOCAL] "
}
```

With:
```go
// Build description prefix: [LOCAL] for local items, [registry-name] for registry items
localPrefix := ""
localPrefixLen := 0
if item.Local {
	localPrefix = warningStyle.Render("[LOCAL]") + " "
	localPrefixLen = 8 // "[LOCAL] "
} else if item.Registry != "" {
	tag := "[" + item.Registry + "]"
	localPrefix = countStyle.Render(tag) + " "
	localPrefixLen = len(tag) + 1 // tag + space
}
```

**Success criteria:**
- [ ] Items from a registry show `[registry-name]` prefix in the description column
- [ ] Local items still show `[LOCAL]` prefix
- [ ] Shared repo items show no prefix
- [ ] `make build` passes

---

## Task 10 — Add registry source tag in TUI detail view

**Modifies:** `cli/internal/tui/detail_render.go`

**Dependencies:** Task 4, Task 9

Registry items show a `[registry-name]` tag in the detail view breadcrumb, next to the item name. This mirrors the `[LOCAL]` tag already there for local items.

In `detail_render.go`, in `renderContentSplit()`, update the breadcrumb block (around line 33):

Replace:
```go
current := titleStyle.Render(name)
if m.item.Local {
	current += " " + warningStyle.Render("[LOCAL]")
}
```

With:
```go
current := titleStyle.Render(name)
if m.item.Local {
	current += " " + warningStyle.Render("[LOCAL]")
} else if m.item.Registry != "" {
	current += " " + countStyle.Render("["+m.item.Registry+"]")
}
```

Also update the metadata block below the tab bar to show registry source when present. After the `Type:` line block, add:

```go
if m.item.Registry != "" {
	pinned += labelStyle.Render("Registry: ") + valueStyle.Render(m.item.Registry) + "\n"
}
```

This goes in the pinned section, after the `Type:` line and before the `Path:` line.

**Success criteria:**
- [ ] Detail view breadcrumb shows `[registry-name]` for registry items
- [ ] Detail view metadata shows `Registry:` field for registry items
- [ ] Local items still show `[LOCAL]` in breadcrumb
- [ ] `make build` passes

---

## Task 11 — Add Registries sidebar entry and update sidebar counts

**Modifies:** `cli/internal/tui/sidebar.go`

**Dependencies:** Task 1, Task 4

Add "Registries" as the 4th entry in the Configuration section (after Settings). The sidebar needs to know the registry count — add `registryCount int` field to `sidebarModel` and pass it through from `App`.

Update `totalItems()` to return `len(m.types) + 5` (was 4, adding Registries).

Update `newSidebarModel` to accept `registryCount int`:
```go
func newSidebarModel(cat *catalog.Catalog, version string, registryCount int) sidebarModel {
	return sidebarModel{
		types:         catalog.AllContentTypes(),
		counts:        cat.CountByType(),
		localCount:    cat.CountLocal(),
		version:       version,
		registryCount: registryCount,
	}
}
```

Add `registryCount int` field to `sidebarModel`:
```go
type sidebarModel struct {
	types         []catalog.ContentType
	counts        map[catalog.ContentType]int
	localCount    int
	registryCount int  // number of configured registries
	cursor        int
	focused       bool
	height        int

	version         string
	remoteVersion   string
	updateAvailable bool
	commitsBehind   int
}
```

Update `totalItems()`:
```go
func (m sidebarModel) totalItems() int {
	return len(m.types) + 5 // content types + My Tools + Import + Update + Settings + Registries
}
```

Update the `utilItems` slice in `View()`:
```go
utilItems := []struct {
	label string
	index int
}{
	{"Import",      len(m.types) + 1},
	{"Update",      len(m.types) + 2},
	{"Settings",    len(m.types) + 3},
	{"Registries",  len(m.types) + 4},
}
```

For Registries, show the count in the sidebar. Update the rendering for Registries specifically in the loop — or render it separately after the loop to apply count formatting like content types:

Replace the simple `utilItems` loop with rendering that adds a count for Registries:

```go
for _, u := range utilItems {
	var rowContent string
	label := u.label
	// Registries shows item count like content types
	if u.label == "Registries" && m.registryCount > 0 {
		countStr := fmt.Sprintf("%2d", m.registryCount)
		line := fmt.Sprintf("%-*s%s", inner-len(countStr)-2, label, countStr)
		if u.index == m.cursor {
			rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, line))
		} else {
			rowContent = "  " + itemStyle.Render(line)
		}
	} else {
		if u.index == m.cursor {
			rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, label))
		} else {
			rowContent = "  " + itemStyle.Render(label)
		}
	}
	s += zone.Mark(fmt.Sprintf("sidebar-%d", u.index), rowContent) + "\n"
}
```

Add selector method:
```go
func (m sidebarModel) isRegistriesSelected() bool { return m.cursor == len(m.types)+4 }
```

Update the existing `isSettingsSelected`:
```go
func (m sidebarModel) isSettingsSelected() bool { return m.cursor == len(m.types)+3 }
```

(The index was already `len(m.types)+3`, so it stays the same.)

In `cli/internal/tui/app.go`, update `NewApp` to pass registry count to sidebar:
```go
sidebar: newSidebarModel(cat, version, len(registrySources)),
```

No update needed at other refresh points — `a.sidebar.registryCount` is set once in `NewApp` and registries do not change during a TUI session.

**Success criteria:**
- [ ] "Registries" appears in sidebar Configuration section
- [ ] When no registries configured, shows "Registries" with no count
- [ ] When registries present, shows count next to "Registries"
- [ ] Arrow key navigation reaches "Registries" (totalItems includes it)
- [ ] `make build` passes

---

## Task 12 — Add Registries entry to landing page

**Modifies:** `cli/internal/tui/app.go`

**Dependencies:** Task 11

Add a "Registries" card to the landing page Configuration section alongside Import, Update, Settings. The landing page `configItems` slice needs a 4th entry.

In `renderContentWelcome`, update the `configItems` slice:
```go
configItems := []struct {
	label  string
	desc   string
	zoneID string
}{
	{"Import",      "Import your own AI tools from local files or git repos",                "welcome-import"},
	{"Update",      "Check for updates and pull latest changes",                             "welcome-update"},
	{"Settings",    "Configure paths and providers",                                         "welcome-settings"},
	{"Registries",  "Manage git-based content sources from your team or organization",       "welcome-registries"},
}
```

In `renderWelcomeCards`, the config cards row currently renders 3 cards in a row. With 4 items, the layout needs to handle an even row. Update the multi-column config card rendering to use pairs (2 per row) when single-column is false:

```go
// Determine config card layout
singleColConfig := contentW < 56

if singleColConfig {
	for _, ci := range configItems {
		inner := labelStyle.Render(ci.label) + "\n" + helpStyle.Render(ci.desc)
		s += zone.Mark(ci.zoneID, configCardStyle.Render(inner)) + "\n"
	}
} else {
	// Render config cards in rows of 2 (4 items = 2 rows)
	configCardW := (contentW - 5) / 2
	if configCardW < 16 {
		configCardW = 16
	}
	configCardStyle2 := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(configCardW).
		Height(3).
		Padding(0, 1)
	for i := 0; i < len(configItems); i += 2 {
		left := labelStyle.Render(configItems[i].label) + "\n" + helpStyle.Render(configItems[i].desc)
		left = zone.Mark(configItems[i].zoneID, configCardStyle2.Render(left))
		var right string
		if i+1 < len(configItems) {
			rightInner := labelStyle.Render(configItems[i+1].label) + "\n" + helpStyle.Render(configItems[i+1].desc)
			right = zone.Mark(configItems[i+1].zoneID, configCardStyle2.Render(rightInner))
			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		} else {
			s += left + "\n"
		}
	}
}
```

Update `renderWelcomeList` to include the Registries item in the compact list — it already iterates `configItems`, so no change needed there.

Add `"welcome-registries"` to the `welcomeConfigMap` in the mouse handler in `Update`:
```go
welcomeConfigMap := map[string]int{
	"welcome-import":      len(a.sidebar.types) + 1,
	"welcome-update":      len(a.sidebar.types) + 2,
	"welcome-settings":    len(a.sidebar.types) + 3,
	"welcome-registries":  len(a.sidebar.types) + 4,
}
```

**Success criteria:**
- [ ] "Registries" card appears on landing page alongside Import/Update/Settings
- [ ] Clicking Registries card navigates to Registries screen (after Task 14)
- [ ] `make build` passes

---

## Task 13 — Create registries TUI screen model

**Creates:** `cli/internal/tui/registries.go`

**Dependencies:** Tasks 1, 2, 5, 11

The registries screen lists all configured registries with name, URL, clone status, and item count. Navigation follows the same keyboard pattern as `update.go`. The screen is read-only — add/remove/sync is done via CLI.

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// registryEntry holds display data for one registry row.
type registryEntry struct {
	name      string
	url       string
	ref       string
	cloned    bool
	itemCount int
}

type registriesModel struct {
	entries  []registryEntry
	cursor   int
	width    int
	height   int
	repoRoot string
}

func newRegistriesModel(repoRoot string, cfg *config.Config, cat *catalog.Catalog) registriesModel {
	entries := make([]registryEntry, len(cfg.Registries))
	for i, r := range cfg.Registries {
		entries[i] = registryEntry{
			name:      r.Name,
			url:       r.URL,
			ref:       r.Ref,
			cloned:    registry.IsCloned(r.Name),
			itemCount: cat.CountRegistry(r.Name),
		}
	}
	return registriesModel{
		entries:  entries,
		repoRoot: repoRoot,
	}
}

func (m registriesModel) Update(msg tea.Msg) (registriesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			for i := range m.entries {
				if zone.Get(fmt.Sprintf("registry-row-%d", i)).InBounds(msg) {
					m.cursor = i
					// Synthesize Enter to drill in
					return m, func() tea.Msg {
						return tea.KeyMsg{Type: tea.KeyEnter}
					}
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	}
	return m, nil
}

func (m registriesModel) selectedName() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return ""
	}
	return m.entries[m.cursor].name
}

func (m registriesModel) View() string {
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	s := home + helpStyle.Render(" > ") + titleStyle.Render("Registries") + "\n\n"

	if len(m.entries) == 0 {
		s += helpStyle.Render("  No registries configured.") + "\n\n"
		s += helpStyle.Render("  Add one with: syllago registry add <git-url>") + "\n"
		s += "\n" + helpStyle.Render("esc back")
		return s
	}

	// Header
	s += tableHeaderStyle.Render(fmt.Sprintf("   %-20s  %-8s  %-6s  %s", "Name", "Status", "Items", "URL")) + "\n"
	s += helpStyle.Render("   " + strings.Repeat("─", 20) + "  " + strings.Repeat("─", 8) + "  " + strings.Repeat("─", 6) + "  " + strings.Repeat("─", 30)) + "\n"

	for i, entry := range m.entries {
		prefix := "   "
		nameStyle := itemStyle
		if i == m.cursor {
			prefix = " > "
			nameStyle = selectedItemStyle
		}

		status := helpStyle.Render("missing")
		if entry.cloned {
			status = installedStyle.Render("cloned")
		}

		countStr := fmt.Sprintf("%6d", entry.itemCount)
		url := entry.url
		if len(url) > 40 {
			url = url[:37] + "..."
		}

		row := fmt.Sprintf("%s%-20s  %s  %s  %s",
			prefix,
			nameStyle.Render(truncate(entry.name, 20)),
			status,
			helpStyle.Render(countStr),
			helpStyle.Render(url),
		)
		s += zone.Mark(fmt.Sprintf("registry-row-%d", i), row) + "\n"
	}

	s += "\n"
	s += helpStyle.Render("up/down navigate • enter browse items • esc back") + "\n"
	s += helpStyle.Render("add: syllago registry add <url> • sync: syllago registry sync • remove: syllago registry remove <name>") + "\n"

	return s
}
```

**Success criteria:**
- [ ] `registriesModel` compiles with no errors
- [ ] Empty state shows helpful CLI hint
- [ ] Each entry row shows name, clone status, item count, URL
- [ ] `make build` passes

---

## Task 14 — Wire registries screen into App navigation

**Modifies:** `cli/internal/tui/app.go`

**Dependencies:** Tasks 11, 12, 13, 7

Wire the new `screenRegistries` constant, `registriesModel` field, and navigation handlers into `App`.

Add to the `screen` enum:
```go
const (
	screenCategory screen = iota
	screenItems
	screenDetail
	screenImport
	screenUpdate
	screenSettings
	screenRegistries  // add this
)
```

Add to `App` struct:
```go
registries   registriesModel
registryCfg  *config.Config  // cached config for building registriesModel
```

In `NewApp`, store cfg for later use. The cleanest approach is to pass `cfg *config.Config` into `NewApp` and store it:

```go
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config) App {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		registrySources: registrySources,
		registryCfg:     cfg,
		screen:          screenCategory,
		focus:           focusSidebar,
		sidebar:         newSidebarModel(cat, version, len(cfg.Registries)),
		search:          newSearchModel(),
	}
}
```

Update `runTUI` in `main.go`:
```go
app := tui.NewApp(cat, providers, version, autoUpdate, regSources, cfg)
```

In `app.go` `Update`, handle `screenCategory` → Registries navigation. In the Enter/Right block in `screenCategory`, add before the content type handling:

```go
if a.sidebar.isRegistriesSelected() {
	a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
	a.registries.width = a.width - sidebarWidth - 1
	a.registries.height = a.panelHeight()
	a.screen = screenRegistries
	a.focus = focusContent
	return a, nil
}
```

Handle Esc from `screenRegistries`:
```go
case screenRegistries:
	if key.Matches(msg, keys.Back) {
		a.screen = screenCategory
		a.focus = focusSidebar
		return a, nil
	}
	// Enter on a registry → show its items
	if key.Matches(msg, keys.Enter) && len(a.registries.entries) > 0 {
		name := a.registries.selectedName()
		if name != "" {
			regItems := a.catalog.ByRegistry(name)
			items := newItemsModel(catalog.SearchResults, regItems, a.providers, a.catalog.RepoRoot)
			items.width = a.width - sidebarWidth - 1
			items.height = a.panelHeight()
			a.items = items
			a.screen = screenItems
			a.focus = focusContent
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.registries, cmd = a.registries.Update(msg)
	return a, cmd
```

Handle `WindowSizeMsg` for registries:
```go
a.registries.width = contentW
a.registries.height = ph
```

Handle mouse clicks for registry rows (in the `tea.MouseMsg` handler, after the item list zones check):
```go
if a.screen == screenRegistries {
	for i := range a.registries.entries {
		if zone.Get(fmt.Sprintf("registry-row-%d", i)).InBounds(msg) {
			a.registries.cursor = i
			a.focus = focusContent
			return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
}
```

Handle mouse wheel for registries screen (in the wheel event block):
```go
case screenRegistries:
	var cmd tea.Cmd
	a.registries, cmd = a.registries.Update(msg)
	return a, cmd
```

Add `screenRegistries` to the `View()` switch:
```go
case screenRegistries:
	contentView = a.registries.View()
```

Add to `breadcrumb()`:
```go
case screenRegistries:
	return "Registries"
```

Add to the `q` quit guard — `screenRegistries` should navigate back, not quit:
The existing `q` handling already synthesizes Esc for non-sidebar non-category screens, so this is handled automatically.

Add to the Tab/ShiftTab guard: exclude `screenRegistries` the same as import/update/settings:
```go
if a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && a.screen != screenRegistries {
```

**Success criteria:**
- [ ] Selecting "Registries" in sidebar and pressing Enter navigates to `screenRegistries`
- [ ] Esc from registries screen returns to category screen
- [ ] Entering a registry row shows its items in the standard items view
- [ ] Mouse click on registry row works
- [ ] `make build` passes

---

## Task 15 — Add registryAutoSync preference

**Modifies:** `cli/internal/config/config.go` (documentation comment only)
**Modifies:** `cli/internal/tui/settings.go`

**Dependencies:** Tasks 1, 7

The `registryAutoSync` preference is already read in Task 7's `runTUI` changes. This task exposes it in the Settings screen so users can toggle it from the TUI.

Add a third settings row to `settingsModel`. Update `settingsRowCount()`:
```go
func (m settingsModel) settingsRowCount() int {
	return 3 // auto-update, providers, registry-auto-sync
}
```

In `View()`, add row 2:
```go
// Row 2: Registry auto-sync
autoSyncVal := "off"
if m.cfg.Preferences["registryAutoSync"] == "true" {
	autoSyncVal = "on"
}
s += m.renderRow(2, "Registry auto-sync", autoSyncVal)
```

In `activateRow()`, add case 2:
```go
case 2: // registry auto-sync toggle
	if m.cfg.Preferences == nil {
		m.cfg.Preferences = make(map[string]string)
	}
	if m.cfg.Preferences["registryAutoSync"] == "true" {
		m.cfg.Preferences["registryAutoSync"] = "false"
	} else {
		m.cfg.Preferences["registryAutoSync"] = "true"
	}
	m.dirty = true
```

Add to `settingsDescriptions`:
```go
var settingsDescriptions = []string{
	"Pull updates automatically when a new version is detected on the remote.",
	"Providers are AI coding tools (Claude Code, Cursor, Gemini CLI, etc.).\nEnable the ones you use -- syllago imports their existing configs\nand can export your catalog items back to them.",
	"Sync git registries automatically when syllago launches (5-second timeout).\nRegistries must be added via `syllago registry add` first.",
}
```

**Success criteria:**
- [ ] Settings screen shows "Registry auto-sync" row
- [ ] Toggling saves `registryAutoSync: "true"` / `"false"` to config
- [ ] Description shows when row is selected
- [ ] `make build` passes

---

## Task 16 — Add syllago registry items subcommand

**Modifies:** `cli/cmd/syllago/registry_cmd.go`

**Dependencies:** Tasks 2, 5, 8

Add `syllago registry items [name]` for CLI browsing of registry content. Follows the `--json` pattern from `output.JSON`.

Add to the `registry_cmd.go` file:

```go
var registryItemsCmd = &cobra.Command{
	Use:   "items [name]",
	Short: "List items from a registry (or all registries)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured.")
			return nil
		}

		typeFilter, _ := cmd.Flags().GetString("type")

		// Build registry sources
		var sources []catalog.RegistrySource
		if len(args) == 1 {
			// Filter to specific registry
			name := args[0]
			found := false
			for _, r := range cfg.Registries {
				if r.Name == name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("registry %q not found in config", name)
			}
			if !registry.IsCloned(name) {
				return fmt.Errorf("registry %q not cloned — run `syllago registry sync %s` first", name, name)
			}
			dir, _ := registry.CloneDir(name)
			sources = append(sources, catalog.RegistrySource{Name: name, Path: dir})
		} else {
			for _, r := range cfg.Registries {
				if registry.IsCloned(r.Name) {
					dir, _ := registry.CloneDir(r.Name)
					sources = append(sources, catalog.RegistrySource{Name: r.Name, Path: dir})
				}
			}
		}

		// Scan only registry sources (no local repo scan needed here)
		cat, scanErr := catalog.ScanRegistriesOnly(sources)
		if scanErr != nil {
			return scanErr
		}

		// Filter by type if requested
		var items []catalog.ContentItem
		if typeFilter != "" {
			ct := catalog.ContentType(typeFilter)
			items = cat.ByType(ct)
		} else {
			items = cat.Items
		}

		if output.JSON {
			output.Print(items)
			return nil
		}

		if len(items) == 0 {
			fmt.Println("No items found.")
			return nil
		}

		fmt.Printf("%-20s  %-10s  %-15s  %s\n", "Name", "Type", "Registry", "Description")
		fmt.Printf("%-20s  %-10s  %-15s  %s\n", strings.Repeat("─", 20), strings.Repeat("─", 10), strings.Repeat("─", 15), strings.Repeat("─", 30))
		for _, item := range items {
			desc := item.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			fmt.Printf("%-20s  %-10s  %-15s  %s\n",
				truncateStr(item.Name, 20),
				truncateStr(string(item.Type), 10),
				truncateStr(item.Registry, 15),
				desc,
			)
		}
		return nil
	},
}

// truncateStr cuts a string to max length with "..." suffix.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")
	registryItemsCmd.Flags().String("type", "", "Filter by content type (skills, rules, hooks, etc.)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd)
	rootCmd.AddCommand(registryCmd)
}
```

This command only needs registry items — no local repo scan. `ScanWithRegistries` would error if given a non-existent `repoRoot`, so we use `ScanRegistriesOnly` instead, which skips the local scan entirely.

In `cli/internal/catalog/scanner.go`, add:
```go
// ScanRegistriesOnly scans only the provided registry sources without a local repo scan.
func ScanRegistriesOnly(registries []RegistrySource) (*Catalog, error) {
	cat := &Catalog{}
	for _, reg := range registries {
		before := len(cat.Items)
		if err := scanRoot(cat, reg.Path, false); err != nil {
			fmt.Fprintf(os.Stderr, "warning: registry %q scan error: %s\n", reg.Name, err)
			continue
		}
		for i := before; i < len(cat.Items); i++ {
			cat.Items[i].Registry = reg.Name
		}
	}
	return cat, nil
}
```

**Success criteria:**
- [ ] `syllago registry items` with no registries prints "No registries configured."
- [ ] `syllago registry items --type skills` filters to skills only
- [ ] `syllago registry items --json` outputs JSON
- [ ] `make build` passes

---

## Task 18 — Add security disclaimer to README

**Modifies:** `README.md`

**Dependencies:** None (can run any time)

Add a "Security" section covering registry trust boundaries. The design doc specifies this content explicitly (Design Step 11).

In `README.md`, add a new `## Security` section after the existing usage/install sections:

```markdown
## Security

Syllago does not operate any registry or marketplace. The built-in content comes from
the [syllago-tools](https://github.com/OpenScribbler/syllago-tools) repository, which you
can audit directly.

**Third-party registries are unverified.** When you run `syllago registry add <url>`,
you are trusting the owner of that repository. Review the content before installing
anything from it.

**Hooks and MCP configs can execute arbitrary code.** A hook is a shell script that
runs automatically in your AI coding session. An MCP server is a process that your AI
tool connects to. Before installing either, read the source.

The syllago maintainers are not affiliated with and accept no liability for any
third-party registry or its content.
```

**Success criteria:**
- [ ] `README.md` contains a `## Security` section
- [ ] Section covers: no official registry/marketplace, third-party content is unverified, hooks/MCP can execute code

---

## Task 17 — Full build and test verification

**Dependencies:** All previous tasks

Run the full build and test suite to verify all pieces integrate correctly.

```bash
cd /home/hhewett/.local/src/syllago && make build && make test
```

Then manually exercise the critical paths:

```bash
# Verify registry CLI help
./cli/syllago registry --help
./cli/syllago registry add --help
./cli/syllago registry list --help
./cli/syllago registry sync --help

# Verify list with no registries configured
./cli/syllago registry list

# Verify config schema (existing config loads without error)
./cli/syllago config list
```

**Expected output for `syllago registry list`:**
```
No registries configured. Run `syllago registry add <url>` to add one.
```

**Expected output for `syllago registry --help`:**
```
Manage git-based content registries

Usage:
  syllago registry [command]

Available Commands:
  add         Add a git registry
  items       List items from a registry (or all registries)
  list        List registered registries
  remove      Remove a registry and delete its local clone
  sync        Sync (git pull) one or all registries
```

**Success criteria:**
- [ ] `make build` passes with no errors
- [ ] `make test` passes with no failures
- [ ] `syllago registry list` works without a project config present
- [ ] `syllago` TUI launches without errors
- [ ] All 10 design doc success criteria are met

---

## Execution Order

Tasks are listed in dependency order. The recommended execution sequence:

1. **Task 1** — Config schema (foundation for everything)
2. **Task 2** — Registry package (foundation for CLI and TUI)
3. **Task 3** — Registry tests (immediately after Task 2)
4. **Task 4** — Catalog types extension
5. **Task 5** — ScanWithRegistries + ScanRegistriesOnly
6. **Task 6** — Installer CheckStatus extension
7. **Task 7** — Wire into TUI launch (first integration point)
8. **Task 8** — CLI registry command (can run after 1, 2, 5)
9. **Task 9** — Items view tag (requires Task 4)
10. **Task 10** — Detail view tag (requires Task 4)
11. **Task 11** — Sidebar Registries entry (requires Tasks 4, 7)
12. **Task 12** — Landing page Registries card (requires Task 11)
13. **Task 13** — Registries screen model (requires Tasks 1, 2, 5, 11)
14. **Task 14** — Wire registries screen into App (requires Tasks 11-13)
15. **Task 15** — registryAutoSync setting (requires Tasks 1, 7)
16. **Task 16** — registry items subcommand (requires Tasks 2, 5, 8)
17. **Task 18** — README security disclaimer (no dependencies, run any time)
18. **Task 17** — Final verification

Tasks 3, 6, 9, 10, 16, 18 can run in parallel with others once their dependencies are met.
