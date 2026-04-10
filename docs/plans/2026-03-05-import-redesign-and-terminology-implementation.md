# Import Redesign & Terminology Overhaul — Implementation Plan

**Goal:** Redesign syllago's content lifecycle with clear vocabulary, a global content library (`~/.syllago/content/`), provider-neutral canonical format, and complete verb coverage for every user workflow (add, remove, install, uninstall, convert, share, publish).

**Architecture:** Hub-and-spoke conversion with syllago-native canonical format. Content flows into `~/.syllago/content/` via `add`, activates in providers via `install`, and shares upstream via `share`/`publish`. All new CLI commands wrap existing converter/installer/promote internals.

**Design Doc:** `docs/plans/2026-03-04-import-redesign-and-terminology-design.md`

---

## Landmine Map

Before touching anything, understand these. Every one of them will cause silent breakage if missed.

### `item.Local` — used in 14+ places

The `Local bool` field on `ContentItem` drives the sidebar count, "My Tools" navigation, the `[LOCAL]` badge, the promote gate, the LLM-prompt copy feature, and the `filterBySource("local", ...)` in export. After Phase 3, `item.Local` becomes `item.Library` (or `item.Source == "global"`), and every downstream consumer must be updated simultaneously.

**Locations:**
- `cli/internal/catalog/types.go:82` — struct field definition
- `cli/internal/catalog/types.go:143,154,165` — `ByTypeLocal`, `ByTypeShared`, `CountLocal`
- `cli/internal/catalog/scanner.go:96,138,275,309` — `scanRoot` and `scanProviderDir` set `Local:local`
- `cli/internal/catalog/scanner.go:519` — source tagging in `ScanWithGlobalAndRegistries`
- `cli/internal/catalog/cleanup.go:30,37` — `CleanupPromotedItems` checks `item.Local`
- `cli/internal/catalog/precedence.go:7` — `itemPrecedence` gives local items priority 0
- `cli/internal/tui/sidebar.go:36,107,182` — `localCount`, `"My Tools"` render, `isMyToolsSelected()`
- `cli/internal/tui/app.go:197,204,891,992,1044,1121` — `localItems()`, all `MyTools` branching
- `cli/internal/tui/items.go:427` — `[LOCAL]` badge
- `cli/internal/tui/detail_render.go:36,52,81,244,516,723` — `[LOCAL]` badge, LLM prompt, promote gate, copy button
- `cli/internal/tui/detail.go:139,572,601` — LLM load, copy guard, promote key guard
- `cli/internal/tui/category_test.go:68` — test asserts `item.Local`
- `cli/cmd/syllago/export.go:536` — `filterBySource` checks `item.Local`
- `cli/cmd/syllago/main.go:127` — backfill skips local items

### Hook hardcoded path (line 391 in import.go)
```go
// cli/cmd/syllago/import.go:391
itemDir := filepath.Join(root, "local", string(catalog.Hooks), fromSlug, name)
```
This is separate from the `writeImportedContent` path logic at line 194-198. It must be updated independently to use `catalog.GlobalContentDir()`.

### `--base-dir` in export.go
The export command builds a `config.PathResolver` with `--base-dir` override (lines 107-125 in export.go). When absorbed into `install_cmd.go`, this flag and its resolver setup must be preserved intact.

### `filterBySource("local", ...)` in export.go
`export.go:536` — `filterBySource` treats source `"local"` as `item.Local == true`. After renaming, the library source filter must use `item.Source == "global"` instead.

### `MyTools` virtual ContentType
`catalog.MyTools ContentType = "local"` in `types.go:19`. All code that does `catalog.MyTools` must be updated when renaming to `catalog.Library`. This includes the items model display title in `items.go`.

### `catalog.CleanupPromotedItems`
This function in `cleanup.go` checks `!item.Local` to identify shared items and `item.Local` to identify the local copies to delete. After renaming, these guards must use the new field name (`Library`) or the `Source == "global"` check.

---

## Phase 1: Foundation (no user-facing changes)

Sets up infrastructure used by later phases. No existing behavior changes. Tests pass before and after each task.

### 1.1 — Add `SymlinkSupport` field to Provider struct

**File:** `cli/internal/provider/provider.go`

Add the `SymlinkSupport` map field. The install logic will consult it in Phase 2.

```go
// In provider.go, after the existing fields:
type Provider struct {
    Name      string
    Slug      string
    Detected  bool
    ConfigDir string
    InstallDir    func(homeDir string, ct catalog.ContentType) string
    Detect        func(homeDir string) bool
    DiscoveryPaths func(projectRoot string, ct catalog.ContentType) []string
    FileFormat    func(ct catalog.ContentType) Format
    EmitPath      func(projectRoot string) string
    SupportsType  func(ct catalog.ContentType) bool

    // SymlinkSupport maps content types to whether symlinks are supported.
    // If nil, symlinks are assumed supported for filesystem types.
    // Hooks and MCP are always false (JSON merge, not filesystem).
    SymlinkSupport map[catalog.ContentType]bool
}
```

**Test:** `go test ./cli/internal/provider/...` — still passes (additive change).

**Success:** Field exists; existing tests pass; no provider definitions changed yet.

---

### 1.2 — Populate `SymlinkSupport` in each provider definition

**Files:** All 11 provider files in `cli/internal/provider/`

For each provider, add a `SymlinkSupport` map. Hooks and MCP are always `false`. All filesystem content types are `true` unless there is a specific reason not to (Windows paths, embedded editors, etc.). Start with all `true` for filesystem types — correctness can be refined later.

**claude.go** (add after `SupportsType`):
```go
SymlinkSupport: map[catalog.ContentType]bool{
    catalog.Rules:    true,
    catalog.Skills:   true,
    catalog.Agents:   true,
    catalog.Commands: true,
    catalog.MCP:      false, // JSON merge
    catalog.Hooks:    false, // JSON merge
},
```

**cursor.go** (only supports Rules):
```go
SymlinkSupport: map[catalog.ContentType]bool{
    catalog.Rules: true,
},
```

Apply equivalent maps to: `gemini.go`, `windsurf.go`, `codex.go`, `copilot.go`, `zed.go`, `cline.go`, `roocode.go`, `opencode.go`, `kiro.go`. Consult each provider's `SupportsType` to know which content types to include. Filesystem-installed types get `true`; JSON merge types get `false`.

**Test:** `go test ./cli/internal/provider/...` — still passes.

**Success:** All 11 providers have `SymlinkSupport` populated.

---

### 1.3 — Add `CountLibrary` and `ByTypeLibrary` to catalog types

**File:** `cli/internal/catalog/types.go`

Add new methods alongside existing `CountLocal`/`ByTypeLocal`. Do NOT rename or remove the old methods yet — Phase 3 handles that.

```go
// CountLibrary returns the number of items sourced from the global content library.
func (c *Catalog) CountLibrary() int {
    count := 0
    for _, item := range c.Items {
        if item.Source == "global" {
            count++
        }
    }
    return count
}

// ByTypeLibrary returns items of the given type that came from the global library.
func (c *Catalog) ByTypeLibrary(ct ContentType) []ContentItem {
    var result []ContentItem
    for _, item := range c.Items {
        if item.Type == ct && item.Source == "global" {
            result = append(result, item)
        }
    }
    return result
}
```

Also add the `Library` virtual content type constant (alongside `MyTools`, keeping `MyTools` temporarily):
```go
// In types.go constants block:
Library ContentType = "library" // virtual type for global library items view
```

**Test:** `go test ./cli/internal/catalog/...` — still passes (additive only).

**Success:** New methods exist; `MyTools` still works; golden files unchanged.

---

### 1.4 — Add `GlobalContentDir` helper to scanner (already exists, verify)

**File:** `cli/internal/catalog/scanner.go`

`GlobalContentDir()` already exists at line 497-503. Verify it returns `~/.syllago/content`. No change needed if it does.

If `ScanWithGlobalAndRegistries` already scans `~/.syllago/content/` and tags items `Source = "global"` — it does (lines 530-555) — then no code change is needed here either. Just confirm the behavior is correct by reading lines 506-560.

**Test:** Write a quick test (if one doesn't exist) that creates a temp `~/.syllago/content/skills/test-skill/` structure and confirms `ScanWithGlobalAndRegistries` returns it with `Source == "global"`.

In `cli/internal/catalog/scanner_test.go`, add:
```go
func TestScanWithGlobalTagsSource(t *testing.T) {
    home := t.TempDir()
    globalDir := filepath.Join(home, ".syllago", "content")
    skillDir := filepath.Join(globalDir, "skills", "test-skill")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n"), 0644)

    // Patch GlobalContentDir to return our test dir
    // (needs refactor or env var injection — see note below)
    // For now, test ScanWithGlobalAndRegistries directly with a known global dir.
    cat, err := catalog.ScanWithGlobalAndRegistries(t.TempDir(), t.TempDir(), nil)
    // ... verify global items tagged correctly
}
```

Note: `GlobalContentDir()` uses `os.UserHomeDir()` — testing requires either dependency injection or a test that creates content under the real home. Defer this to integration testing if unit testing is awkward. The important thing is confirming the tagging path works.

**Success:** Understand and confirm the global content scan path before building on top of it.

---

### 1.4b — Move `globalContentDirOverride` test helper to Phase 1

**Note on test helper timing:** Phase 3.1 introduces the `globalContentDirOverride` package-level variable in `scanner.go`. However, Phase 1.4's scanner test (`TestScanWithGlobalTagsSource`) and any Phase 2 command tests that need to write to the global library also need this override to avoid polluting the real `~/.syllago/content/`. Move the override to Phase 1 so it is available immediately.

**File:** `cli/internal/catalog/scanner.go`

Add this before the existing `GlobalContentDir` function (wherever it lives, around line 497):

```go
// GlobalContentDirOverride is set in tests to redirect GlobalContentDir to a temp path.
// Do not set this in production code.
var GlobalContentDirOverride string

func GlobalContentDir() string {
    if GlobalContentDirOverride != "" {
        return GlobalContentDirOverride
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".syllago", "content")
}
```

**If `GlobalContentDir` already exists:** Simply prepend the override check to it.

In Phase 3.1's add_cmd_test.go, and any Phase 2 command tests that create library items, use:
```go
catalog.GlobalContentDirOverride = filepath.Join(t.TempDir(), ".syllago", "content")
t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })
```

**Test:** `go test ./cli/internal/catalog/...` — still passes (additive change).

**Success:** Override variable exported; existing behavior unchanged; test override pattern documented.

---

### 1.5 — Extend metadata.Meta with add-time fields

**File:** `cli/internal/metadata/metadata.go`

The `metadata.Meta` struct already exists with `SourceProvider` and `SourceFormat`. The design's `.syllago.yaml` schema (Decision 20) also requires `source_type`, `has_source`, `added_at`, and `added_by`. These fields are currently missing. Add them to the existing `Meta` struct.

Current struct (lines 25-43):
```go
type Meta struct {
    ID             string       `yaml:"id"`
    Name           string       `yaml:"name"`
    // ... existing fields ...
    SourceProvider string       `yaml:"source_provider,omitempty"`
    SourceFormat   string       `yaml:"source_format,omitempty"`
}
```

Add the following fields after `SourceFormat`:
```go
    SourceType     string     `yaml:"source_type,omitempty"`   // git | filesystem | registry | provider
    SourceURL      string     `yaml:"source_url,omitempty"`    // for future syllago update capability
    HasSource      bool       `yaml:"has_source,omitempty"`    // whether .source/ directory exists
    AddedAt        *time.Time `yaml:"added_at,omitempty"`      // when item was added to library
    AddedBy        string     `yaml:"added_by,omitempty"`      // e.g. "syllago v0.1.0"
```

**Note on existing fields:** `ImportedAt`/`ImportedBy` pre-date this redesign and remain for backwards compatibility with existing `.syllago.yaml` files. `AddedAt`/`AddedBy` are the new canonical names for new items. Do NOT remove `ImportedAt`/`ImportedBy` — that would require a migration.

**Note on `PromotedAt`/`PRBranch`:** These rename to share/publish terminology in Phase 5; leave them for now.

The complete addition to the struct:
```go
// In cli/internal/metadata/metadata.go, after line 42 (SourceFormat field):
SourceType     string     `yaml:"source_type,omitempty"`   // git | filesystem | registry | provider
SourceURL      string     `yaml:"source_url,omitempty"`    // for future syllago update capability
HasSource      bool       `yaml:"has_source,omitempty"`    // whether .source/ directory exists
AddedAt        *time.Time `yaml:"added_at,omitempty"`      // when content was added to library
AddedBy        string     `yaml:"added_by,omitempty"`      // e.g. "syllago v0.1.0"
```

**Test:** `go test ./cli/internal/metadata/...` — existing tests still pass (all new fields are `omitempty`).

**Success:** `meta.Meta` has all five new fields; existing tests pass; YAML round-trip works with new fields.

---

## Phase 2: New Commands

New CLI commands that can be developed and tested independently. These do not break existing commands.

### 2.1 — Create `install_cmd.go`

**File:** `cli/cmd/syllago/install_cmd.go` (NEW)

The `install` command activates library content in a provider. It harvests the core logic from `export.go`: the converter pipeline, JSON merge dispatch, resolver setup, and cross-provider rendering. The key differences from `export.go`:

1. Source is always the global library (`~/.syllago/content/`) — not `local/` or shared
2. Default method is symlink (not copy), with `--method` flag to override
3. Output is conversational (state change explanations + next-command suggestions)
4. No `--source` flag (library is the only source)

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/config"
    "github.com/OpenScribbler/syllago/cli/internal/converter"
    "github.com/OpenScribbler/syllago/cli/internal/installer"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
    "github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
    Use:   "install [name]",
    Short: "Activate library content in a provider",
    Long: `Install content from your library into a provider's location.

By default uses a symlink so edits in your library are reflected immediately.
Use --method copy to place a standalone copy instead.

Examples:
  syllago install my-skill --to claude-code
  syllago install my-skill --to cursor --method copy
  syllago install --to claude-code --type skills`,
    Args: cobra.MaximumNArgs(1),
    RunE: runInstall,
}

func init() {
    installCmd.Flags().String("to", "", "Provider to install into (required)")
    installCmd.MarkFlagRequired("to")
    installCmd.Flags().String("type", "", "Filter to a specific content type")
    installCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
    installCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
    installCmd.Flags().String("base-dir", "", "Override base directory for content installation")
    installCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
    rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
    toSlug, _ := cmd.Flags().GetString("to")
    typeFilter, _ := cmd.Flags().GetString("type")
    methodStr, _ := cmd.Flags().GetString("method")
    dryRun, _ := cmd.Flags().GetBool("dry-run")
    baseDir, _ := cmd.Flags().GetString("base-dir")

    method := installer.MethodSymlink
    if methodStr == "copy" {
        method = installer.MethodCopy
    }

    prov := findProviderBySlug(toSlug)
    if prov == nil {
        slugs := providerSlugs()
        output.PrintError(1, "unknown provider: "+toSlug,
            "Available: "+strings.Join(slugs, ", "))
        return output.SilentError(fmt.Errorf("unknown provider: %s", toSlug))
    }

    // Build resolver
    globalCfg, err := config.LoadGlobal()
    if err != nil {
        return fmt.Errorf("loading global config: %w", err)
    }
    projectRoot, _ := findProjectRoot()
    projectCfg, err := config.Load(projectRoot)
    if err != nil {
        return fmt.Errorf("loading project config: %w", err)
    }
    mergedCfg := config.Merge(globalCfg, projectCfg)
    resolver := config.NewResolver(mergedCfg, baseDir)
    if err := resolver.ExpandPaths(); err != nil {
        return fmt.Errorf("expanding paths: %w", err)
    }

    // Scan global library only
    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        return fmt.Errorf("cannot determine home directory")
    }
    globalCat := &catalog.Catalog{RepoRoot: globalDir}
    // Use internal scan — may need to export scanRoot or use ScanWithGlobalAndRegistries
    // and filter to Source == "global". Use the latter for simplicity:
    fullCat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    var items []catalog.ContentItem
    for _, item := range fullCat.Items {
        if item.Source != "global" {
            continue
        }
        if len(args) == 1 && item.Name != args[0] {
            continue
        }
        if typeFilter != "" && string(item.Type) != typeFilter {
            continue
        }
        items = append(items, item)
    }

    if len(items) == 0 {
        if len(args) == 1 {
            return fmt.Errorf("No item named %q found in your library.\n  Hint: syllago list --type %s", args[0], typeFilter)
        }
        fmt.Fprintln(output.ErrWriter, "No items found in library matching filters.")
        return nil
    }

    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("getting home directory: %w", err)
    }
    _ = homeDir

    var installed, skipped int
    for _, item := range items {
        if dryRun {
            fmt.Fprintf(output.Writer, "[dry-run] Would install %s (%s) to %s\n", item.Name, item.Type.Label(), prov.Name)
            continue
        }
        desc, err := installer.Install(item, *prov, globalDir, method, baseDir)
        if err != nil {
            fmt.Fprintf(output.ErrWriter, "  skip %s: %s\n", item.Name, err)
            skipped++
            continue
        }
        if method == installer.MethodSymlink {
            fmt.Fprintf(output.Writer, "Symlinked %s to %s\n", item.Name, desc)
        } else {
            fmt.Fprintf(output.Writer, "Copied %s to %s\n", item.Name, desc)
        }
        installed++
    }

    if installed > 0 && !output.Quiet {
        fmt.Fprintf(output.Writer, "\n  Next: syllago install %s --to <other-provider>    (install to another provider)\n", firstArg(args))
        fmt.Fprintf(output.Writer, "        syllago convert %s --to <provider>           (convert for sharing)\n", firstArg(args))
    }

    return nil
}

func firstArg(args []string) string {
    if len(args) > 0 {
        return args[0]
    }
    return "<name>"
}
```

**Verification: installer.Install() decision tree (Gap 4 addressed)**

The quality review flagged that `installer.Install()` must implement the full decision tree from the design. It already does — verified in `cli/internal/installer/installer.go` lines 188-228:

```go
// The existing Install() already handles:
// 1. JSON merge dispatch for Hooks/MCP (lines 165-172)
// 2. Cross-provider rendering: if item.Provider != prov.Slug → render from canonical (lines 190-196)
// 3. Same-provider .source/ lossless install: if HasSourceFile && item.Provider == prov.Slug → use .source/ (lines 198-204)
// 4. Symlink vs copy fallthrough (lines 218-228)
// 5. Windows mount detection forces copy (lines 222-225)
```

No additional decision tree logic is needed in `install_cmd.go`. The `installer.Install(item, *prov, globalDir, method, baseDir)` call is sufficient.

The `item.Provider` field must be correctly set during the scan for same-provider detection to work. This is populated by the scanner from content directory structure (`rules/<provider>/<name>/`). Global library items scanned from `~/.syllago/content/rules/claude-code/my-rule/` will have `item.Provider = "claude-code"`. No additional wiring needed.

**Test:** `go test ./cli/cmd/syllago/... -run TestInstall` after writing tests in a new `install_cmd_test.go`.

Example test:
```go
func TestInstallRequiresTo(t *testing.T) {
    installCmd.Flags().Set("to", "")
    err := installCmd.RunE(installCmd, []string{})
    if err == nil {
        t.Error("install without --to should fail")
    }
}
```

**Success:** `syllago install --help` shows expected flags; `--to` required flag enforced; `make test` passes.

---

### 2.2 — Create `remove_cmd.go`

**File:** `cli/cmd/syllago/remove_cmd.go` (NEW)

The `remove` command deletes content from the global library and auto-uninstalls it from all detected providers.

```go
package main

import (
    "fmt"
    "os"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/installer"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
    "github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
    Use:   "remove <name>",
    Short: "Remove content from your library and uninstall from providers",
    Long: `Removes a content item from your library (~/.syllago/content/) and
uninstalls it from any providers where it is currently installed.

Examples:
  syllago remove my-skill
  syllago remove my-rule --type rules
  syllago remove my-skill --force`,
    Args: cobra.ExactArgs(1),
    RunE: runRemove,
}

func init() {
    removeCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
    removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
    removeCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
    rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
    name := args[0]
    typeFilter, _ := cmd.Flags().GetString("type")
    force, _ := cmd.Flags().GetBool("force")
    dryRun, _ := cmd.Flags().GetBool("dry-run")

    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        return fmt.Errorf("cannot determine home directory")
    }

    cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    // Find matching items in global library
    var matches []catalog.ContentItem
    for _, item := range cat.Items {
        if item.Source != "global" || item.Name != name {
            continue
        }
        if typeFilter != "" && string(item.Type) != typeFilter {
            continue
        }
        matches = append(matches, item)
    }

    if len(matches) == 0 {
        return fmt.Errorf("No item named %q found in your library.\n  Hint: syllago list    (show all library items)", name)
    }

    if len(matches) > 1 {
        var types []string
        for _, m := range matches {
            types = append(types, string(m.Type))
        }
        return fmt.Errorf("%q exists in multiple types: %s\n  Use --type to disambiguate: syllago remove %s --type <type>",
            name, strings.Join(types, ", "), name)
    }

    item := matches[0]

    // Find providers where it is installed
    home, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("getting home directory: %w", err)
    }
    _ = home

    var installedIn []string
    for _, prov := range provider.AllProviders {
        status := installer.CheckStatus(item, prov, globalDir)
        if status == installer.StatusInstalled {
            installedIn = append(installedIn, prov.Name)
        }
    }

    // Confirm unless --force or --dry-run
    if !force && !dryRun && isInteractive() {
        provList := "none"
        if len(installedIn) > 0 {
            provList = strings.Join(installedIn, ", ")
        }
        fmt.Fprintf(output.Writer, "This will remove %q from your library.\n", name)
        fmt.Fprintf(output.Writer, "  Type: %s\n", item.Type.Label())
        fmt.Fprintf(output.Writer, "  Installed in: %s\n", provList)
        fmt.Fprintf(output.Writer, "\nContinue? [y/N] ")

        var answer string
        fmt.Scanln(&answer)
        if strings.ToLower(strings.TrimSpace(answer)) != "y" {
            fmt.Fprintln(output.Writer, "Cancelled.")
            return nil
        }
    }

    if dryRun {
        fmt.Fprintf(output.Writer, "[dry-run] Would uninstall from: %s\n", strings.Join(installedIn, ", "))
        fmt.Fprintf(output.Writer, "[dry-run] Would remove from library: %s\n", item.Path)
        return nil
    }

    // Uninstall from each provider
    var uninstalledFrom []string
    for _, prov := range provider.AllProviders {
        status := installer.CheckStatus(item, prov, globalDir)
        if status != installer.StatusInstalled {
            continue
        }
        if _, err := installer.Uninstall(item, prov, globalDir); err != nil {
            fmt.Fprintf(output.ErrWriter, "  warning: failed to uninstall from %s: %s\n", prov.Name, err)
        } else {
            uninstalledFrom = append(uninstalledFrom, prov.Name)
        }
    }

    // Remove from library
    if err := os.RemoveAll(item.Path); err != nil {
        return fmt.Errorf("removing from library: %w", err)
    }

    if len(uninstalledFrom) > 0 {
        fmt.Fprintf(output.Writer, "Uninstalled from: %s\n", strings.Join(uninstalledFrom, ", "))
    }
    fmt.Fprintf(output.Writer, "Removed from library: %s (%s)\n", name, item.Type.Label())

    return nil
}
```

**Test:** Write `remove_cmd_test.go`:
```go
func TestRemoveRequiresName(t *testing.T) {
    err := removeCmd.Args(removeCmd, []string{})
    if err == nil {
        t.Error("remove without name should fail args validation")
    }
}

func TestRemoveDryRunDoesNotDelete(t *testing.T) {
    // Create a temp global library with one item
    // Run remove --dry-run
    // Verify the item directory still exists
}
```

**Success:** `syllago remove --help` works; `make test` passes.

---

### 2.3 — Create `convert_cmd.go`

**File:** `cli/cmd/syllago/convert_cmd.go` (NEW)

Ad-hoc format conversion: canonical → target provider format, output to stdout or `--output`.

```go
package main

import (
    "fmt"
    "os"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/converter"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
    Use:   "convert <name>",
    Short: "Convert library content to a provider format",
    Long: `Renders a library item to a target provider's format without installing it.
Output goes to stdout by default, or to a file with --output.

No state changes are made — this is purely for ad-hoc sharing.

Examples:
  syllago convert my-skill --to cursor
  syllago convert my-rule --to windsurf --output ./windsurf-rule.md`,
    Args: cobra.ExactArgs(1),
    RunE: runConvert,
}

func init() {
    convertCmd.Flags().String("to", "", "Target provider (required)")
    convertCmd.MarkFlagRequired("to")
    convertCmd.Flags().StringP("output", "o", "", "Write output to this file path (default: stdout)")
    rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
    name := args[0]
    toSlug, _ := cmd.Flags().GetString("to")
    outputPath, _ := cmd.Flags().GetString("output")

    prov := findProviderBySlug(toSlug)
    if prov == nil {
        return fmt.Errorf("unknown provider: %s\n  Available: %s", toSlug, providerSlugList())
    }

    globalDir := catalog.GlobalContentDir()
    cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    var item *catalog.ContentItem
    for i := range cat.Items {
        if cat.Items[i].Source == "global" && cat.Items[i].Name == name {
            item = &cat.Items[i]
            break
        }
    }
    if item == nil {
        return fmt.Errorf("No item named %q in your library.\n  Hint: syllago list    (show all library items)", name)
    }

    conv := converter.For(item.Type)
    if conv == nil {
        return fmt.Errorf("%s does not support format conversion", item.Type.Label())
    }

    contentFile := converter.ResolveContentFile(*item)
    if contentFile == "" {
        return fmt.Errorf("cannot locate content file for %s", name)
    }
    raw, err := os.ReadFile(contentFile)
    if err != nil {
        return fmt.Errorf("reading content: %w", err)
    }

    srcProvider := ""
    if item.Meta != nil {
        srcProvider = item.Meta.SourceProvider
    }
    if srcProvider == "" && item.Provider != "" {
        srcProvider = item.Provider
    }

    canonical, err := conv.Canonicalize(raw, srcProvider)
    if err != nil {
        return fmt.Errorf("canonicalizing content: %w", err)
    }

    rendered, err := conv.Render(canonical.Content, *prov)
    if err != nil {
        return fmt.Errorf("rendering to %s format: %w", prov.Name, err)
    }
    if rendered.Content == nil {
        return fmt.Errorf("%s is not compatible with %s format", name, prov.Name)
    }

    if outputPath != "" {
        if err := os.WriteFile(outputPath, rendered.Content, 0644); err != nil {
            return fmt.Errorf("writing output: %w", err)
        }
        if !output.Quiet {
            fmt.Fprintf(output.Writer, "Rendered %s as %s format to %s\n", name, prov.Name, outputPath)
        }
    } else {
        os.Stdout.Write(rendered.Content)
    }

    return nil
}

func providerSlugList() string {
    var slugs []string
    for _, p := range findAllProviders() {
        slugs = append(slugs, p.Slug)
    }
    return strings.Join(slugs, ", ")
}
```

Note: `findAllProviders()` is a helper that returns `provider.AllProviders` — add it to `helpers.go` if not present, or call `providerSlugs()` which already exists.

**Test:** Write `convert_cmd_test.go`:
```go
func TestConvertRequiresTo(t *testing.T) {
    convertCmd.Flags().Set("to", "")
    err := convertCmd.RunE(convertCmd, []string{"my-skill"})
    if err == nil {
        t.Error("convert without --to should fail")
    }
}
```

**Success:** Command wires up; `make test` passes; `syllago convert --help` works.

---

### 2.4 — Create `share_cmd.go`

**File:** `cli/cmd/syllago/share_cmd.go` (NEW)

Contributes library content to a team repo. Harvests the git workflow from `promote.go`'s `Promote()` function, but sources from `~/.syllago/content/` instead of `local/`.

```go
package main

import (
    "fmt"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/promote"
    "github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
    Use:   "share <name>",
    Short: "Contribute library content to a team repo",
    Long: `Copies a library item to your team repo, stages the change, and
optionally creates a branch and PR.

Examples:
  syllago share my-skill
  syllago share my-rule --type rules`,
    Args: cobra.ExactArgs(1),
    RunE: runShare,
}

func init() {
    shareCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
    shareCmd.Flags().BoolP("no-input", "", false, "Skip interactive git prompts, stage only")
    rootCmd.AddCommand(shareCmd)
}

func runShare(cmd *cobra.Command, args []string) error {
    name := args[0]
    typeFilter, _ := cmd.Flags().GetString("type")
    noInput, _ := cmd.Flags().GetBool("no-input")

    globalDir := catalog.GlobalContentDir()
    cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    item, err := findLibraryItem(cat, name, typeFilter)
    if err != nil {
        return err
    }

    // Find team repo root (the syllago repo the user is working in)
    root, err := findContentRepoRoot()
    if err != nil {
        return fmt.Errorf("could not find syllago team repo: %w\n  Use this command from inside a syllago team repo directory", err)
    }

    // Per CLI Design Standards: --no-input must suppress the "Create branch and PR?" interactive
    // prompt in promote.Promote even on a TTY. Pass noInput to Promote.
    // NOTE: If promote.Promote does not yet accept a noInput parameter, add one:
    //   func Promote(root string, item catalog.ContentItem, noInput bool) (PromoteResult, error)
    // and update all callers. Until then, promote.Promote uses TTY detection internally only,
    // which satisfies the "non-TTY → no prompts" case but not the "TTY + --no-input" case.
    // When wiring this up, replace the call below with:
    //   result, err := promote.Promote(root, *item, noInput)
    result, err := promote.Promote(root, *item)
    if err != nil {
        return fmt.Errorf("sharing failed: %w", err)
    }

    if result.PRUrl != "" {
        fmt.Fprintf(output.Writer, "Shared! PR: %s\n", result.PRUrl)
    } else if result.CompareURL != "" {
        fmt.Fprintf(output.Writer, "Shared! Branch %q pushed.\n  Open a PR: %s\n", result.Branch, result.CompareURL)
    } else {
        fmt.Fprintf(output.Writer, "Shared! Branch %q pushed.\n", result.Branch)
    }

    return nil
}

// findLibraryItem looks up an item by name in the global library.
func findLibraryItem(cat *catalog.Catalog, name, typeFilter string) (*catalog.ContentItem, error) {
    var matches []catalog.ContentItem
    for _, item := range cat.Items {
        if item.Source != "global" || item.Name != name {
            continue
        }
        if typeFilter != "" && string(item.Type) != typeFilter {
            continue
        }
        matches = append(matches, item)
    }
    if len(matches) == 0 {
        return nil, fmt.Errorf("No item named %q found in your library.", name)
    }
    if len(matches) > 1 {
        return nil, fmt.Errorf("%q exists in multiple types. Use --type to disambiguate.", name)
    }
    return &matches[0], nil
}
```

**Test:** Write `share_cmd_test.go` with a test that verifies unknown item returns a useful error.

**Success:** `syllago share --help` works; `make test` passes.

---

### 2.5 — Create `publish_cmd.go`

**File:** `cli/cmd/syllago/publish_cmd.go` (NEW)

Publishes library content to a registry. Wraps `promote.PromoteToRegistry`.

```go
package main

import (
    "fmt"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/promote"
    "github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
    Use:   "publish <name>",
    Short: "Contribute library content to a registry",
    Long: `Copies a library item to a registry clone, stages the change, and
optionally creates a branch and PR.

Examples:
  syllago publish my-skill --registry my-registry
  syllago publish my-rule --registry team-rules --type rules`,
    Args: cobra.ExactArgs(1),
    RunE: runPublish,
}

func init() {
    publishCmd.Flags().String("registry", "", "Registry name to publish to (required)")
    publishCmd.MarkFlagRequired("registry")
    publishCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
    publishCmd.Flags().Bool("no-input", false, "Skip interactive git prompts, stage only")
    rootCmd.AddCommand(publishCmd)
}

func runPublish(cmd *cobra.Command, args []string) error {
    name := args[0]
    registryName, _ := cmd.Flags().GetString("registry")
    typeFilter, _ := cmd.Flags().GetString("type")
    // noInput suppresses the "Create branch and PR?" interactive prompt even on a TTY.
    // Per CLI Design Standards, --no-input must work on publish.
    // NOTE: If promote.PromoteToRegistry does not yet accept a noInput parameter, add one:
    //   func PromoteToRegistry(root, registry string, item catalog.ContentItem, noInput bool) (PromoteResult, error)
    // and update all callers. When wiring: replace call below with PromoteToRegistry(..., noInput).
    noInput, _ := cmd.Flags().GetBool("no-input")
    _ = noInput // wire to PromoteToRegistry when that function supports the parameter

    globalDir := catalog.GlobalContentDir()
    cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    item, err := findLibraryItem(cat, name, typeFilter)
    if err != nil {
        return err
    }

    root, err := findContentRepoRoot()
    if err != nil {
        return fmt.Errorf("could not find syllago repo: %w", err)
    }

    result, err := promote.PromoteToRegistry(root, registryName, *item)
    if err != nil {
        return fmt.Errorf("publish failed: %w", err)
    }

    if result.PRUrl != "" {
        fmt.Fprintf(output.Writer, "Published! PR: %s\n", result.PRUrl)
    } else if result.CompareURL != "" {
        fmt.Fprintf(output.Writer, "Published! Branch %q pushed.\n  Open a PR: %s\n", result.Branch, result.CompareURL)
    } else {
        fmt.Fprintf(output.Writer, "Published! Branch %q pushed to registry %q.\n", result.Branch, registryName)
    }

    return nil
}
```

Note: `findLibraryItem` is already defined in `share_cmd.go`. Move it to `helpers.go` to share between the two files.

**Refactor after writing both:** Move `findLibraryItem` from `share_cmd.go` to `helpers.go` to avoid duplicate definition.

**Test:** `publishCmd` requires `--registry`; verify this compiles and tests pass.

**Success:** `syllago publish --help` works; `make test` passes.

---

### 2.6 — Create `uninstall_cmd.go`

**File:** `cli/cmd/syllago/uninstall_cmd.go` (NEW)

The `uninstall` command deactivates library content from a provider — the reverse of `install`. It removes the symlink (or copied file) or reverses a JSON merge, depending on content type. `installer.Uninstall()` already exists in `cli/internal/installer/installer.go` (line 230) and handles all three cases (symlink, copy, JSON merge). This command is a thin CLI wrapper around it.

```go
package main

import (
    "fmt"
    "os"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/installer"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
    "github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
    Use:   "uninstall <name>",
    Short: "Deactivate content from a provider",
    Long: `Removes installed content from a provider's location.

For symlinked content: removes the symlink.
For copied content: removes the copied file or directory.
For hooks/MCP: reverses the JSON merge from the provider's settings file.

The content remains in your library (~/.syllago/content/) and can be
reinstalled at any time with "syllago install".

Examples:
  syllago uninstall my-skill --from claude-code
  syllago uninstall my-rule --from cursor --force
  syllago uninstall my-agent                       (uninstall from all providers)`,
    Args: cobra.ExactArgs(1),
    RunE: runUninstall,
}

func init() {
    uninstallCmd.Flags().String("from", "", "Provider to uninstall from (omit to uninstall from all)")
    uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
    uninstallCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
    uninstallCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
    rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
    name := args[0]
    fromSlug, _ := cmd.Flags().GetString("from")
    force, _ := cmd.Flags().GetBool("force")
    dryRun, _ := cmd.Flags().GetBool("dry-run")

    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        return fmt.Errorf("cannot determine home directory")
    }

    cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
    if err != nil {
        return fmt.Errorf("scanning library: %w", err)
    }

    // Find the item in the global library
    var item *catalog.ContentItem
    for i := range cat.Items {
        if cat.Items[i].Source == "global" && cat.Items[i].Name == name {
            item = &cat.Items[i]
            break
        }
    }
    if item == nil {
        return fmt.Errorf("No item named %q found in your library.\n  Hint: syllago list    (show all library items)", name)
    }

    // Determine which providers to uninstall from
    var targets []provider.Provider
    if fromSlug != "" {
        prov := findProviderBySlug(fromSlug)
        if prov == nil {
            slugs := providerSlugs()
            output.PrintError(1, "unknown provider: "+fromSlug,
                "Available: "+strings.Join(slugs, ", "))
            return output.SilentError(fmt.Errorf("unknown provider: %s", fromSlug))
        }
        // Verify it is actually installed there
        status := installer.CheckStatus(*item, *prov, globalDir)
        if status != installer.StatusInstalled {
            return fmt.Errorf("%q is not installed in %s", name, prov.Name)
        }
        targets = []provider.Provider{*prov}
    } else {
        // Uninstall from all providers where it is currently installed
        for _, prov := range provider.AllProviders {
            status := installer.CheckStatus(*item, prov, globalDir)
            if status == installer.StatusInstalled {
                targets = append(targets, prov)
            }
        }
        if len(targets) == 0 {
            return fmt.Errorf("%q is not installed in any provider", name)
        }
    }

    // Build a summary of what will be affected
    var targetNames []string
    for _, prov := range targets {
        targetNames = append(targetNames, prov.Name)
    }

    noInput, _ := cmd.Flags().GetBool("no-input")

    // Confirm unless --force, --dry-run, --no-input, or non-interactive
    if !force && !dryRun && !noInput && isInteractive() {
        fmt.Fprintf(output.Writer, "This will uninstall %q from: %s\n", name, strings.Join(targetNames, ", "))
        fmt.Fprintf(output.Writer, "  The content stays in your library.\n")
        fmt.Fprintf(output.Writer, "\nContinue? [y/N] ")
        var answer string
        fmt.Scanln(&answer)
        if strings.ToLower(strings.TrimSpace(answer)) != "y" {
            fmt.Fprintln(output.Writer, "Cancelled.")
            return nil
        }
    }

    if dryRun {
        for _, prov := range targets {
            fmt.Fprintf(output.Writer, "[dry-run] Would uninstall %q from %s\n", name, prov.Name)
        }
        return nil
    }

    // Perform uninstall
    var removedFrom []string
    for _, prov := range targets {
        desc, err := installer.Uninstall(*item, prov, globalDir)
        if err != nil {
            fmt.Fprintf(output.ErrWriter, "  warning: failed to uninstall from %s: %s\n", prov.Name, err)
            continue
        }
        // desc contains the path/detail of what was removed (e.g. "symlink: ~/.claude/agents/my-agent.md")
        // Use it for state change explanation per CLI Design Standards.
        if desc != "" {
            fmt.Fprintf(output.Writer, "Removed %s\n", desc)
        } else {
            fmt.Fprintf(output.Writer, "Removed from %s\n", prov.Name)
        }
        removedFrom = append(removedFrom, prov.Name)
    }

    if len(removedFrom) > 0 && !output.Quiet {
        fmt.Fprintf(output.Writer, "\n  %q is still in your library.\n", name)
        fmt.Fprintf(output.Writer, "  Remove with: syllago remove %s\n", name)
    }

    return nil
}
```

**What `installer.Uninstall()` does** (verified in `cli/internal/installer/installer.go` line 230-267):
- JSON merge types (Hooks/MCP): dispatches to `uninstallMCP`/`uninstallHook` which reverse the JSON merge
- Filesystem symlinks: `os.Remove(targetPath)`
- Copied files: `os.Remove(targetPath)` (regular file)
- Copied directories: `os.RemoveAll(targetPath)`

No additional logic is needed in `uninstall_cmd.go` — `installer.Uninstall()` is complete.

**Helper note:** `providerSlugs()` already exists in the codebase (called by `install_cmd.go`). If not, add to `helpers.go`:
```go
func providerSlugs() []string {
    var slugs []string
    for _, p := range provider.AllProviders {
        slugs = append(slugs, p.Slug)
    }
    return slugs
}
```

**Test:** Write `uninstall_cmd_test.go`:
```go
func TestUninstallRequiresName(t *testing.T) {
    err := uninstallCmd.Args(uninstallCmd, []string{})
    if err == nil {
        t.Error("uninstall without name should fail args validation")
    }
}

func TestUninstallNotInstalledReturnsError(t *testing.T) {
    globalDir := t.TempDir()
    catalog.GlobalContentDirOverride = globalDir
    t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })

    // No content in global dir → item not found error
    cmd := &cobra.Command{}
    cmd.Flags().String("from", "claude-code", "")
    cmd.Flags().Bool("force", false, "")
    cmd.Flags().Bool("dry-run", false, "")
    err := runUninstall(cmd, []string{"nonexistent"})
    if err == nil {
        t.Error("expected error for missing item")
    }
}

func TestUninstallDryRunDoesNotRemove(t *testing.T) {
    // Create a temp global library with one skill item
    // Create a temp provider dir with a symlink simulating an installed item
    // Run uninstall --dry-run
    // Verify the symlink still exists
}
```

**Success:** `syllago uninstall --help` shows expected flags; `make test` passes; `syllago uninstall my-skill --from claude-code` removes the installed symlink/file.

---

### 2.7 — Add deprecated command redirects

**File:** `cli/cmd/syllago/main.go` or a new `deprecated_cmds.go`

Per the design spec, old command names should return a helpful error instead of "command not found". Add three stub commands that print guidance and exit 1.

Create `cli/cmd/syllago/deprecated_cmds.go`:
```go
package main

import (
    "fmt"

    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/spf13/cobra"
)

// Deprecated command stubs. These exist solely to intercept old command names
// and provide guidance to users. They are not registered during normal usage.
// They ARE registered so Cobra doesn't respond with a generic "unknown command" error.

var deprecatedExportCmd = &cobra.Command{
    Use:    "export",
    Hidden: true,
    Short:  "(removed) use 'install' or 'convert'",
    RunE: func(cmd *cobra.Command, args []string) error {
        output.PrintError(1,
            "Unknown command 'export'.",
            "To install content into a provider: syllago install <name> --to <provider>\n  To convert for sharing:            syllago convert <name> --to <provider>")
        return output.SilentError(fmt.Errorf("export removed"))
    },
}

var deprecatedImportCmd = &cobra.Command{
    Use:    "import",
    Hidden: true,
    Short:  "(removed) use 'add'",
    RunE: func(cmd *cobra.Command, args []string) error {
        output.PrintError(1,
            "Unknown command 'import'.",
            "To add content to your library: syllago add <source>")
        return output.SilentError(fmt.Errorf("import removed"))
    },
}

var deprecatedPromoteCmd = &cobra.Command{
    Use:    "promote",
    Hidden: true,
    Short:  "(removed) use 'share' or 'publish'",
    RunE: func(cmd *cobra.Command, args []string) error {
        output.PrintError(1,
            "Unknown command 'promote'.",
            "To share with your team:        syllago share <name>\n  To publish to a registry: syllago publish <name> --registry <name>")
        return output.SilentError(fmt.Errorf("promote removed"))
    },
}

func init() {
    rootCmd.AddCommand(deprecatedExportCmd)
    rootCmd.AddCommand(deprecatedImportCmd)
    rootCmd.AddCommand(deprecatedPromoteCmd)
}
```

Note: In Phase 3, the real `export.go` and `promote_cmd.go` are deleted, and these stubs provide the redirect. Until then, they coexist but the real commands take precedence (Cobra picks the first match).

Actually — Cobra will error on duplicate command names. Defer adding these stubs until Phase 3 when the real commands are deleted.

**Revised plan:** Create `deprecated_cmds.go` but do NOT register the commands in `init()`. In Phase 3.2, after deleting export.go and promote_cmd.go, add the `rootCmd.AddCommand()` calls. See dependency graph for the Phase 2.7 → Phase 3.2 registration sequence.

**Success:** File created; not yet registered; compiles.

---

## Phase 3: Rename & Refactor

This phase breaks existing tests because it renames commands and paths. Work through tasks in order; run `make test` after each task and fix failures immediately.

### 3.1 — Rename `import.go` → `add_cmd.go`, command `import` → `add`

**File:** `cli/cmd/syllago/import.go` → `cli/cmd/syllago/add_cmd.go`

This is the core command rename. The logic is mostly unchanged; only destination paths and command strings change.

**Step 1:** Rename the file. In Go, the package is `package main` so no import path changes.
```bash
mv cli/cmd/syllago/import.go cli/cmd/syllago/add_cmd.go
mv cli/cmd/syllago/import_test.go cli/cmd/syllago/add_cmd_test.go
```

**Step 2:** In `add_cmd.go`, rename the Cobra command variable and text:

Line 21-53: Change `importCmd` → `addCmd`, update `Use`, `Short`, `Long`:
```go
var addCmd = &cobra.Command{
    Use:   "add [source]",
    Short: "Add content to your library from a provider, path, or git URL",
    Long: `Discovers content from a provider and adds it to your library (~/.syllago/content/).

Syllago handles format conversion automatically. Once added, content can be
installed to any supported provider with "syllago install --to <provider>".

Examples:
  syllago add --from claude-code                  Add all content from Claude Code
  syllago add --from claude-code --type skills    Add only skills
  syllago add --from cursor --name my-rule        Add a specific rule by name
  syllago add --from claude-code --preview        Preview what would be added (read-only)
  syllago add --from claude-code --dry-run        Show what would be written without writing

After adding, use "syllago install" to activate content in a provider.`,
    RunE: runAdd,
}
```

Line 41-53: `init()` — change `importCmd` → `addCmd`, `rootCmd.AddCommand(importCmd)` → `rootCmd.AddCommand(addCmd)`.

Line 56: `func runImport(...)` → `func runAdd(...)` and update the `addCmd.RunE` reference.

Line 175: `fmt.Fprintf(output.Writer, "\nImported %d file(s) to local/.\n", written)` → update to use global dir path.

**Step 3:** Change destination paths from `local/` to `~/.syllago/content/`:

In `writeImportedContent` (lines 187-253), change lines 193-198:
```go
// Before:
if ct.IsUniversal() {
    destDir = filepath.Join(projectRoot, "local", string(ct), name)
} else {
    destDir = filepath.Join(projectRoot, "local", string(ct), sourceProvider, name)
}

// After:
globalDir := catalog.GlobalContentDir()
if globalDir == "" {
    return "", fmt.Errorf("cannot determine home directory")
}
if ct.IsUniversal() {
    destDir = filepath.Join(globalDir, string(ct), name)
} else {
    destDir = filepath.Join(globalDir, string(ct), sourceProvider, name)
}
```

The `projectRoot` parameter can be removed from `writeImportedContent` signature, or kept unused — either is fine.

**Step 4:** Fix the hook hardcoded path at line 391 (now in `runAddHooks`/`addHooksFromLocation`):
```go
// Before (line 391):
itemDir := filepath.Join(root, "local", string(catalog.Hooks), fromSlug, name)

// After:
globalDir := catalog.GlobalContentDir()
itemDir := filepath.Join(globalDir, string(catalog.Hooks), fromSlug, name)
```

Also update `runImportHooks` → `runAddHooks` and `importHooksFromLocation` → `addHooksFromLocation` throughout.

**Step 5:** Update progress messages throughout `add_cmd.go`:
- `"Imported %d file(s) to local/."` → `"Added %d item(s) to library."`
- `"Imported %s -> %s"` → `"Added %s -> %s"`
- `"[dry-run] Would import %d file(s) to local/."` → `"[dry-run] Would add %d item(s) to library."`
- `"Imported %d hooks to local/hooks/%s/"` → `"Added %d hooks to library."`
- `printDiscoveryReport`: `"Import from %s:"` → `"Add from %s:"`

**Step 6:** Add `--force` and `--no-input` flags and overwrite confirmation to `add_cmd.go`:

Per the design's CLI standards ("Confirm before danger" table), `add` with an overwrite should prompt: "Overwrite existing <name>? [y/N]" and skip with `--force`. Per the flags table, `--no-input` applies to all commands.

In `init()`, add:
```go
addCmd.Flags().BoolP("force", "f", false, "Overwrite existing item without prompting")
addCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
```

In `writeImportedContent`, before writing the destination, check if it already exists:
```go
force, _ := cmd.Flags().GetBool("force")
noInput, _ := cmd.Flags().GetBool("no-input")
if _, err := os.Stat(destDir); err == nil && !force && !noInput && isInteractive() {
    fmt.Fprintf(output.Writer, "Overwrite existing %q? [y/N] ", name)
    var answer string
    fmt.Scanln(&answer)
    if strings.ToLower(strings.TrimSpace(answer)) != "y" {
        return destDir, fmt.Errorf("cancelled")
    }
}
```

Note: `isInteractive()` is defined in `helpers.go`. For non-interactive (no TTY), skip the prompt and always overwrite. `--no-input` explicitly disables the prompt even on a TTY (consistent with the design's TTY detection standard).

**Step 7 (renumbered from Step 7):** Add next-command suggestion at the end of `runAdd`:
```go
if written > 0 && !dryRun && !output.Quiet {
    fmt.Fprintf(output.Writer, "\n  Next: syllago install <name> --to <provider>\n")
}
```

**Step 7:** Update `add_cmd_test.go` (was `import_test.go`):
- Rename `TestImport*` → `TestAdd*`
- Fix variable references: `importCmd` → `addCmd`, `runImportHooks` → `runAddHooks`
- Fix path assertions: `local/rules/...` → use `catalog.GlobalContentDir()` or a temp home pattern
- Fix output assertions: `"would be imported"` → `"would be added"`

The existing test `TestImportWritesToLocal` checks `filepath.Join(tmp, "local", "rules", ...)` — this must change to check `~/.syllago/content/rules/...`. Since global content dir uses the real home, tests need to either:
  - Mock `GlobalContentDir` via a package-level var (cleanest)
  - Or use `os.Setenv("HOME", tmp)` before the test and restore after

**Recommended approach:** Add a package-level variable to `scanner.go`:
```go
// globalContentDirOverride is set in tests to redirect the global dir.
var globalContentDirOverride string

func GlobalContentDir() string {
    if globalContentDirOverride != "" {
        return globalContentDirOverride
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".syllago", "content")
}
```

In tests:
```go
catalog.GlobalContentDirOverride = filepath.Join(tmp, ".syllago", "content")
t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })
```

**Step 8:** Enable the deprecated `import` redirect in `deprecated_cmds.go` by adding `rootCmd.AddCommand(deprecatedImportCmd)` to its `init()`.

**Step 9: `.source/` directory preservation (Decision 15)**

After `writeImportedContent` canonicalizes and saves the content, it must also preserve the original file in `.source/` if the source format differs from canonical. This is done in `writeImportedContent` in `add_cmd.go`.

Locate the point in `writeImportedContent` where the canonical content is written to `destDir`. After writing the canonical file, add:

```go
// In writeImportedContent, after writing the canonical content file:

// Preserve original in .source/ if source format differs from canonical.
// This enables lossless same-provider roundtrip (Decision 15).
sourceExt := filepath.Ext(srcPath)
// Canonical files use .md extension. If source extension differs, it came from a different format.
if sourceExt != "" && sourceExt != ".md" {
    sourceDir := filepath.Join(destDir, ".source")
    if err := os.MkdirAll(sourceDir, 0755); err != nil {
        return destDir, fmt.Errorf("creating .source/ directory: %w", err)
    }
    originalDest := filepath.Join(sourceDir, filepath.Base(srcPath))
    srcData, err := os.ReadFile(srcPath)
    if err != nil {
        return destDir, fmt.Errorf("reading source file for .source/ copy: %w", err)
    }
    if err := os.WriteFile(originalDest, srcData, 0644); err != nil {
        return destDir, fmt.Errorf("writing .source/ copy: %w", err)
    }
    hasSource = true
}
```

The `hasSource` bool is computed in this step and passed to Step 10's metadata writing.

For hook imports (the separate `addHooksFromLocation` path): hooks use JSON merge, not filesystem content, so `.source/` does not apply. No change needed to the hooks path.

**Step 10: Write `.syllago.yaml` metadata (Decision 20)**

After writing the canonical file and `.source/` (Steps 3 and 9), write the metadata file. This uses the `metadata` package which already has `Save()`.

Add a metadata write call at the end of `writeImportedContent`:

```go
// In writeImportedContent, after Step 9:

import (
    // add to existing imports in add_cmd.go:
    "time"
    "github.com/OpenScribbler/syllago/cli/internal/metadata"
    // ... existing imports
)

// Determine source_type from how the source was provided.
// The sourceType value comes from the calling context (CLI flag analysis):
// - "--from <provider>" → "provider"
// - A git URL argument → "git"
// - A filesystem path → "filesystem"
// The runAdd function knows which source was used; pass it as a parameter.
// For now, default to "provider" since that's the only add path implemented.
sourceType := "provider"  // TODO: update when git URL and filesystem add paths are implemented

now := time.Now()
ver := "syllago"  // replace with actual version string if available from build info
meta := &metadata.Meta{
    ID:             metadata.NewID(),
    Name:           name,
    Type:           string(ct),
    SourceProvider: sourceProvider,
    SourceFormat:   filepath.Ext(srcPath), // e.g. ".mdc", ".md"
    SourceType:     sourceType,
    HasSource:      hasSource,
    AddedAt:        &now,
    AddedBy:        ver,
}
if err := metadata.Save(destDir, meta); err != nil {
    // Non-fatal: warn but don't fail the add operation
    fmt.Fprintf(output.ErrWriter, "  warning: could not write metadata for %s: %s\n", name, err)
}
```

**Function signature note:** `writeImportedContent` currently returns `(string, error)`. The `hasSource` bool computed in Step 9 is local to the function and passed to `metadata.Save` in Step 10. No signature change needed.

**Test:** After implementing Steps 9-10, add to `add_cmd_test.go`:
```go
func TestAddWritesMetadata(t *testing.T) {
    globalDir := t.TempDir()
    catalog.GlobalContentDirOverride = globalDir
    t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })

    // Set up a fake Claude Code provider with one rule
    // Run runAdd with --from claude-code
    // Verify that destDir/.syllago.yaml exists and contains expected fields

    destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
    metaPath := filepath.Join(destDir, metadata.FileName)
    if _, err := os.Stat(metaPath); os.IsNotExist(err) {
        t.Error("expected .syllago.yaml to be written after add")
    }
    m, err := metadata.Load(destDir)
    if err != nil || m == nil {
        t.Fatalf("metadata load failed: %v", err)
    }
    if m.SourceProvider != "claude-code" {
        t.Errorf("expected source_provider=claude-code, got %q", m.SourceProvider)
    }
    if m.AddedAt == nil {
        t.Error("expected added_at to be set")
    }
}

func TestAddPreservesSourceForNonCanonicalFormat(t *testing.T) {
    // Set up a source file with .mdc extension (Cursor format)
    // Run add
    // Verify .source/rule.mdc exists alongside canonical rule.md
}
```

**Scope note — git URL and filesystem add modes:**

The design's command reference lists three `add` source types:
- `syllago add --from <provider>` — implemented by this plan (rename of existing import)
- `syllago add <git-url>` — **deferred to a follow-up plan** (requires git clone infrastructure that doesn't exist in the codebase)
- `syllago add <filesystem-path>` — **deferred to a follow-up plan** (requires file/directory content discovery from arbitrary paths)

This plan only implements `--from <provider>` because it is a refactor of existing functionality. The git URL and filesystem modes are net-new features. The `addCmd.Short` says "from a provider, path, or git URL" for accuracy, but the examples and `init()` only expose `--from`. Remove the reference to path/git-URL from the `Short` description to avoid advertising unimplemented features:

```go
Short: "Add content to your library from a provider",
```

The full command with all three modes will be implemented in a follow-up plan.

**Test:** `go test ./cli/cmd/syllago/... -run TestAdd` after each sub-step.

**Success:** `syllago add --help` shows correct text; `syllago import` prints redirect message; all `TestAdd*` tests pass; `.syllago.yaml` is written for each added item; `.source/` is created when source extension differs from `.md`.

---

### 3.2 — Delete `export.go` and `promote_cmd.go`

**Files to delete:**
- `cli/cmd/syllago/export.go`
- `cli/cmd/syllago/export_test.go`
- `cli/cmd/syllago/promote_cmd.go`
- `cli/cmd/syllago/promote_cmd_test.go`

Before deleting, verify that `install_cmd.go` (Phase 2.1) covers all functionality. The key functions in `export.go` that must be accounted for:

- `exportWithConverter` → ported into `install_cmd.go` (or its callee `installer.Install` already handles this)
- `effectiveProvider` → move to `helpers.go` (also used by install logic)
- `filterBySource` → move to `helpers.go` (still needed by `list.go`; update the `"local"` case to `"library"` in Phase 3.3 Step 29)
- `runExportAll` → not needed; `install_cmd.go` requires explicit `--to <provider>`; bulk-to-all-providers is not in scope for this plan
- `exportWarnMessage` → keep in `helpers.go` if still needed

**Step 1:** Move `effectiveProvider`, `exportWarnMessage`, **and `filterBySource`** to `helpers.go`. `list.go` calls `filterBySource` at line 74 — moving it here prevents a compile error when `export.go` is deleted.

**Step 2:** Delete the four files above.
```bash
rm cli/cmd/syllago/export.go
rm cli/cmd/syllago/export_test.go
rm cli/cmd/syllago/promote_cmd.go
rm cli/cmd/syllago/promote_cmd_test.go
```

**Step 3:** Enable the deprecated redirects by adding to `deprecated_cmds.go`'s `init()`:
```go
rootCmd.AddCommand(deprecatedExportCmd)
rootCmd.AddCommand(deprecatedPromoteCmd)
```

**Test:** `go build ./cli/cmd/syllago/` — must compile cleanly. `make test` — some tests will now fail (export_test.go gone), but compilation must succeed.

**Success:** Binary compiles; `syllago export` prints redirect message; `syllago promote` prints redirect message.

---

### 3.3 — Rename `Local` → `Library` in catalog types

**IMPORTANT:** This is the landmine. Every usage of `item.Local` must be updated simultaneously. Do not partially rename.

**File:** `cli/internal/catalog/types.go`

1. Change field definition (line 82):
   ```go
   // Before:
   Local bool // true if item lives in local/
   // After:
   Library bool // true if item lives in the global content library (~/.syllago/content/)
   ```

2. Update `ByTypeLocal` → `ByTypeLibrary` (lines 139-148):
   ```go
   func (c *Catalog) ByTypeLibrary(ct ContentType) []ContentItem {
       var result []ContentItem
       for _, item := range c.Items {
           if item.Type == ct && item.Library {
               result = append(result, item)
           }
       }
       return result
   }
   ```
   Note: `CountLibrary` and `ByTypeLibrary` were already added in Phase 1.3 using `Source == "global"`. Now that the field is `Library`, keep both the `Source == "global"` version (more semantically correct) and the `Library bool` version for scanRoot-tagged items. Reconcile: after 3.4 updates scanner to set `Library: true`, the `Library` field will match `Source == "global"` for global items.

3. Update `ByTypeShared` (line 151-159):
   ```go
   func (c *Catalog) ByTypeShared(ct ContentType) []ContentItem {
       var result []ContentItem
       for _, item := range c.Items {
           if item.Type == ct && !item.Library && item.Registry == "" {
               result = append(result, item)
           }
       }
       return result
   }
   ```

4. Update `CountLocal` → `CountLibrary` (already added in Phase 1.3, now make it use `item.Library`):
   ```go
   func (c *Catalog) CountLibrary() int {
       count := 0
       for _, item := range c.Items {
           if item.Library {
               count++
           }
       }
       return count
   }
   ```
   Keep or remove the old `CountLocal` — since it's gone from callers after this phase, remove it.

5. Rename the virtual type constant:
   ```go
   // Before:
   MyTools ContentType = "local"
   // After:
   Library ContentType = "library"
   ```

**File:** `cli/internal/catalog/scanner.go`

6. In `scanRoot` (line 96), change `local bool` parameter purpose comment and update all `Local: local` assignments → `Library: local`:
   - Line 138: `Local: local` → `Library: local`
   - Line 275: `Local: local` → `Library: local`
   - Line 309: `Library: local` (in `scanProviderDir`)

7. In `ScanWithGlobalAndRegistries` (line 509), update source tagging (line 519):
   ```go
   // Before:
   if cat.Items[i].Local {
       cat.Items[i].Source = "local"
   }
   // After:
   if cat.Items[i].Library {
       cat.Items[i].Source = "library"
   }
   ```
   Then for global items (line 551): `Source = "global"` remains unchanged.

8. Update `Scan` function comment (line 27-29): Remove `projectRoot/local/` references.

9. **Remove `local/` scan path from `Scan` and `ScanWithGlobalAndRegistries`** (design decision 3: "`local/` directory removed entirely"):

   The `Scan` function currently calls `scanRoot(cat, filepath.Join(projectRoot, "local"), true)` to scan the old local directory. Remove this call:
   ```go
   // Before (in Scan or ScanWithGlobalAndRegistries):
   // Scan local/ directory
   if err := scanRoot(cat, filepath.Join(projectRoot, "local"), true); err != nil { ... }

   // After: delete this scanRoot call entirely.
   // Global library scanning via GlobalContentDir() (already in ScanWithGlobalAndRegistries) replaces it.
   ```

   Also remove any directory creation of `local/` in the scan setup code.

   Verify: after removing this call, a project root with no `~/.syllago/content/` directory still returns an empty catalog rather than erroring.

**File:** `cli/internal/catalog/cleanup.go`

9. Lines 30, 37: `!item.Local` → `!item.Library`, `item.Local` → `item.Library`.

**File:** `cli/internal/catalog/precedence.go`

10. Line 7: `if item.Local` → `if item.Library`.

**File:** `cli/internal/tui/sidebar.go`

11. Line 36: `cat.CountLocal()` → `cat.CountLibrary()`
12. Line 107: `"My Tools"` → `"Library"`
13. Line 182: `isMyToolsSelected()` — rename to `isLibrarySelected()` (and update all callers in app.go)
14. Update `utilItems` label from `"Import"` → `"Add"` (line 130):
    ```go
    {"Add", len(m.types) + 1},
    ```

**File:** `cli/internal/tui/app.go`

15. `localItems()` function (line 200-208): rename to `libraryItems()`, change `item.Local` → `item.Library`.
16. Line 197 in `refreshSidebarCounts`: `a.sidebar.localCount = len(a.visibleItems(a.localItems()))` → `a.sidebar.localCount = len(a.visibleItems(a.libraryItems()))`
17. All occurrences of `item.Local` in the Update function (lines 891, 992, 1044, 1121): change to `item.Library`.
18. All occurrences of `catalog.MyTools` → `catalog.Library`.
19. `a.sidebar.isMyToolsSelected()` → `a.sidebar.isLibrarySelected()`.
20. Import done message (line 290): `"Imported %q successfully"` → `"Added %q to library"`.

**File:** `cli/internal/tui/items.go`

21. Line 427: `item.Local` → `item.Library`, `"[LOCAL]"` → `"[LIBRARY]"`.

**File:** `cli/internal/tui/detail.go`

22. Line 139: `item.Local` → `item.Library`.
23. Line 572: `m.item.Local` → `m.item.Library`.
24. Line 601: `m.item.Local` → `m.item.Library`.

**File:** `cli/internal/tui/detail_render.go`

25. Lines 36, 52, 81, 244, 516, 723: `m.item.Local` / `item.Local` / `ov.Local` → corresponding `.Library` references.
26. `"[LOCAL]"` → `"[LIBRARY]"`, `"local"` source string → `"library"`.
27. LLM prompt section (line 244): `m.item.Library && m.llmPrompt != ""`.

**File:** `cli/cmd/syllago/export.go`

Already deleted in Phase 3.2. If `filterBySource` was moved to `helpers.go`, update it:
```go
// Before:
case "local":
    return item.Local
// After:
case "library":
    return item.Library
```

**File:** `cli/cmd/syllago/main.go`

28. Line 127: `if item.Local || item.Meta != nil` → `if item.Library || item.Meta != nil`.

**File:** `cli/cmd/syllago/list.go`

29. **Step 29 — Update `filterBySource` in `helpers.go` (after move from Phase 3.2 Step 1):**

   `filterBySource` has been moved from `export.go` to `helpers.go`. Now update its `"local"` case:
   ```go
   // In helpers.go filterBySource, change:
   case "local":
       return item.Local
   // To:
   case "library":
       return item.Library
   ```
   If the old `"local"` case is kept as a deprecated alias, that is fine — but the primary case must be `"library"`.

   In `sourceLabel()` in `list.go` (line 120–131), update the Local case:
   ```go
   // Before:
   case item.Local:
       return "local"
   // After:
   case item.Library:
       return "library"
   ```

   Update the `listCmd` flag definition (line 25):
   ```go
   // Before:
   listCmd.Flags().String("source", "all", "Filter by source: local, shared, registry, builtin, all")
   // After:
   listCmd.Flags().String("source", "all", "Filter by source: library, shared, registry, builtin, all")
   ```

   Update the `listCmd.Long` description: `"syllago list --source local"` → `"syllago list --source library"`.

**File:** `cli/cmd/syllago/list_test.go`

30. **Step 30 — Update `list_test.go`:**

   The test file uses `local/` paths and `"[local"` label assertions. Update to match the global library pattern:

   - Replace `filepath.Join(root, "local", ...)` content setup with the `catalog.GlobalContentDirOverride` pattern (same pattern as `add_cmd_test.go`):
     ```go
     globalDir := t.TempDir()
     catalog.GlobalContentDirOverride = globalDir
     t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })
     ```
   - Update content creation path from `filepath.Join(root, "local", "skills", ...)` to `filepath.Join(globalDir, "skills", ...)`
   - Update the `"[local"` assertion (line 76) to `"[library"`
   - Update `listCmd.Flags().Set("source", "local")` (line 123) to `"library"`
   - Update all related assertion strings that reference `"local"` as a source label

   **Note:** `list.go` uses `catalog.Scan(root, projectRoot)` internally (not `ScanWithGlobalAndRegistries`). After the global content dir change, verify that `catalog.Scan` either calls `ScanWithGlobalAndRegistries` or separately scans the global dir. If `catalog.Scan` does not include global library items, the test may need to call `catalog.ScanWithGlobalAndRegistries` instead, or the `runList` function itself needs to be updated to use `ScanWithGlobalAndRegistries`.

**Test:** `go build ./cli/...` must succeed. Then `make test`.

**Failures expected:**
- `category_test.go:68-69`: `item.Local` → `item.Library` (update test)
- `scanner_test.go:240,246,320,424`: update test assertions
- `detail_test.go:781,806`: update `item.Local` → `item.Library`
- `export_test.go`: already deleted
- `list_test.go`: updated in step 30 above
- Golden files: sidebar "My Tools" → "Library", "[LOCAL]" → "[LIBRARY]" tags — must regenerate

Fix all compilation failures, then fix test assertions, then regenerate golden files.

**Success:** `make test` passes; golden files updated.

---

### 3.4 — Update `promote/promote.go` to read from `~/.syllago/content/`

**File:** `cli/internal/promote/promote.go`

The `Promote` function currently reads from `local/` (via `item.Path` which was set during scan). After Phase 3.3, library items' `item.Path` points to `~/.syllago/content/...` because `scanRoot` was updated to scan global content and tag it `Library: true`. No code change needed in `promote.go` if the item path is correct.

Verify: `sharedPath` (line 128-133) uses `item.Provider` and `item.Type` to compute the destination in the repo. This is correct regardless of where the item came from.

The only needed change: update the PR body in `Promote` (line 107):
```go
// Before:
prBody := fmt.Sprintf("Promotes `%s` from local/ to shared.\n\nType: %s\nSource: %s", ...)
// After:
prBody := fmt.Sprintf("Contributes `%s` from library to shared.\n\nType: %s\nSource: %s", ...)
```

**Test:** `go test ./cli/internal/promote/...` — existing tests should still pass.

**Success:** No functional change; PR body updated; tests pass.

---

### 3.5 — Update `loadout_apply.go` to support provider-neutral manifests

**File:** `cli/cmd/syllago/loadout_apply.go`

Currently hardcoded to `provider.ClaudeCode` (line 164):
```go
prov := provider.ClaudeCode
```

The design says the `provider` field in loadout manifests should be optional. When no provider is specified in the manifest, use a provider selected via `--to` flag.

**Changes:**

1. Add `--to` flag and `--method` flag to `loadoutApplyCmd`:
   ```go
   // In init():
   loadoutApplyCmd.Flags().String("to", "", "Target provider (default: claude-code for backwards compat)")
   loadoutApplyCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
   ```

   The `--method` flag is required by the CLI Design Standards flags table: "Install method: `symlink` (default) or `copy` — `install`, `loadout apply`." Pass the selected method to the installer when applying each item in the loadout.

2. In `runLoadoutApply`, resolve provider and method:
   ```go
   toSlug, _ := cmd.Flags().GetString("to")
   methodStr, _ := cmd.Flags().GetString("method")
   method := installer.MethodSymlink
   if methodStr == "copy" {
       method = installer.MethodCopy
   }

   var prov provider.Provider
   if toSlug != "" {
       p := findProviderBySlug(toSlug)
       if p == nil {
           return fmt.Errorf("unknown provider: %s", toSlug)
       }
       prov = *p
   } else if manifest.Provider != "" {
       // Manifest specifies a provider
       p := findProviderBySlug(manifest.Provider)
       if p == nil {
           return fmt.Errorf("loadout manifest specifies unknown provider: %s", manifest.Provider)
       }
       prov = *p
   } else {
       // Default to ClaudeCode for backwards compatibility
       prov = provider.ClaudeCode
   }
   ```

   Pass `method` through to each `installer.Install` call within the loadout apply loop (wherever `loadout_apply.go` currently installs individual items).

3. Check `internal/loadout/` for the manifest struct to verify `Provider` field exists. If not, add it:
   - File: `cli/internal/loadout/loadout.go` (or wherever `Manifest` is defined)
   - Add `Provider string \`yaml:"provider,omitempty"\`` to the manifest struct

**Test:** `go test ./cli/cmd/syllago/... -run TestLoadout`

**Success:** `syllago loadout apply --to gemini-cli` works; `syllago loadout apply <name> --to cursor --method copy` applies all items via copy; existing behavior preserved when no `--to` or `--method` flags.

---

## Phase 4: TUI Updates

After Phase 3 renames, the TUI needs targeted changes. Golden file regeneration is required at the end of this phase.

### 4.1 — Sidebar: "Library" label, "Add" action label

**File:** `cli/internal/tui/sidebar.go`

After Phase 3.3, these changes should already be in place:
- `"My Tools"` → `"Library"` (line 107)
- `"Import"` → `"Add"` (line 130)
- `isMyToolsSelected()` → `isLibrarySelected()`

Verify these are complete. If Phase 3.3 was done correctly, this task is just a verification check.

Also update the `sidebar.localCount` field name and all references if it was renamed:
```go
// In sidebarModel struct (sidebar.go:19):
libraryCount int  // was: localCount
```

Update `newSidebarModel` (line 36): `localCount: cat.CountLocal()` → `libraryCount: cat.CountLibrary()`.

Update `refreshSidebarCounts` in app.go: `a.sidebar.localCount` → `a.sidebar.libraryCount`.

Update the render in sidebar.go View() (lines 105-117):
```go
// Change variable names and label:
libIdx := len(m.types)
libCountStr := fmt.Sprintf("%2d", m.libraryCount)
libLine := fmt.Sprintf("%-*s%s", inner-len(libCountStr)-2, "Library", libCountStr)
```

**Test:** `go test ./cli/internal/tui/ -run TestSidebar`

**Success:** Sidebar renders "Library" and "Add" correctly; tests pass.

---

### 4.2 — TUI import model: update destination paths

**File:** `cli/internal/tui/import.go`

The TUI import flow (which becomes the TUI "Add" flow) writes to `local/`. Update `destinationPath()` (line 922-928) and `batchDestForSource()` (line 1527-1533):

```go
// destinationPath computes where the content will be copied to (global library).
func (m importModel) destinationPath() string {
    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        globalDir = m.projectRoot // fallback
    }
    if m.contentType.IsUniversal() {
        return filepath.Join(globalDir, string(m.contentType), m.itemName)
    }
    return filepath.Join(globalDir, string(m.contentType), m.providerName, m.itemName)
}

// batchDestForSource computes the destination path for a batch source path.
func (m importModel) batchDestForSource(srcPath string) string {
    itemName := filepath.Base(srcPath)
    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        globalDir = m.projectRoot
    }
    if m.contentType.IsUniversal() {
        return filepath.Join(globalDir, string(m.contentType), itemName)
    }
    return filepath.Join(globalDir, string(m.contentType), m.providerName, itemName)
}
```

Also update the confirm step display text (line 827):
```go
// Before:
s += labelStyle.Render("To:    ") + valueStyle.Render(dest) + " " + helpStyle.Render("(local, not git-tracked)") + "\n"
// After:
s += labelStyle.Render("To:    ") + valueStyle.Render(dest) + " " + helpStyle.Render("(global library)") + "\n"
```

Update `discoverProviderDirs` (line 1690): this function checks both shared and `local/` for provider dirs. Update to check global library:
```go
func (m importModel) discoverProviderDirs(ct catalog.ContentType) []string {
    seen := make(map[string]bool)
    var names []string

    globalDir := catalog.GlobalContentDir()
    // Check shared content dir (the repo)
    checkDir(m.repoRoot, ct, seen, &names)
    // Check global library
    if globalDir != "" {
        checkDir(globalDir, ct, seen, &names)
    }
    return names
}
```

**Test:** `go test ./cli/internal/tui/ -run TestImport`

**Success:** TUI add flow writes to `~/.syllago/content/`; tests pass.

---

### 4.3 — TUI app: update promote handler to share/publish split, fix importer path

**File:** `cli/internal/tui/app.go`

The `handleConfirmAction` function (lines 129-151) handles `modalPromote` by calling `promote.Promote`. After the rename, this becomes the "share" action. The design spec says promote in TUI becomes "Share" and "Publish" options — but for now, map the existing promote key to "share" (the full share/publish split with separate buttons is a future TUI enhancement).

Changes to `handleConfirmAction` (line 136-142):
```go
// modalPromote → modalShare (rename the modal purpose constant)
case modalShare:
    repoRoot := a.detail.repoRoot
    item := a.detail.item
    return func() tea.Msg {
        result, err := promote.Promote(repoRoot, item)
        return shareDoneMsg{result: result, err: err}
    }
```

This requires:
1. Renaming `modalPromote` → `modalShare` in `modal.go` (or wherever modal purposes are defined)
2. Renaming `promoteDoneMsg` → `shareDoneMsg` in `detail.go` and wherever it's handled

In `app.go` (line 249-263), rename the handler:
```go
// Before:
case promoteDoneMsg:
// After:
case shareDoneMsg:
```

In `detail.go` (lines 31-34, 188-201, 600-607):
- `promoteDoneMsg` struct → `shareDoneMsg`
- `"Promote failed:"` → `"Share failed:"`
- `"Promoted! Branch: ..."` → `"Shared! Branch: ..."`
- `keys.Promote` → `keys.Share` (also update `keys.go`)
- `modalPromote` → `modalShare`
- `"Promote %q to shared?"` → `"Share %q to team repo?"`

In `detail.go` line 601, gate on `item.Library` instead of `item.Local`:
```go
case key.Matches(msg, keys.Share):
    if m.item.Library {
        // same logic
    }
```

In `keys.go`, rename the `Promote` key binding to `Share`:
```go
// Before:
Promote key.Binding
// After:
Share key.Binding
```
Update the binding initialization and all references in help text.

In `app.go` (line 966): Update `newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)` — the `projectRoot` parameter is still valid but now less relevant since the add flow uses global dir. Keep as is for compatibility.

Status message on import done (line 290):
```go
// Before:
a.statusMessage = fmt.Sprintf("Imported %q successfully", msg.name)
// After:
a.statusMessage = fmt.Sprintf("Added %q to library", msg.name)
```

**Test:** `go test ./cli/internal/tui/ -run TestDetail`

**Success:** Promote key renamed to Share; Library filter works; tests pass.

---

### 4.4 — TUI detail view: remove "Export" action

**File:** `cli/internal/tui/detail.go`, `cli/internal/tui/detail_render.go`, `cli/internal/tui/keys.go`

The design specifies: "Export action: removed" from the detail view. After `export.go` is deleted in Phase 3.2, any TUI key binding that triggered export must also be removed.

**Step 1:** In `keys.go`, remove the `Export` key binding from the key map struct and its initialization.

**Step 2:** In `detail.go`, remove any case that matches `keys.Export` in the `Update` function (typically inside the key message switch):
```go
// Delete this block entirely:
case key.Matches(msg, keys.Export):
    // ... any export modal or action trigger
```

**Step 3:** In `detail_render.go`, remove any rendering of the Export key hint from the help bar (typically rendered alongside Promote/Install key hints).

**Step 4:** In `detail_render.go`, if there is an Export tab or Export section in the detail view, remove it.

**Test:** `go test ./cli/internal/tui/ -run TestDetail` — no Export key rendered; no compilation errors.

**Success:** Export action fully removed from TUI detail view; no orphan key bindings remain.

---

### 4.5 — TUI install modal: symlink default + SymlinkSupport-driven disable

**Files:** `cli/internal/tui/detail.go`, `cli/internal/tui/detail_render.go` (wherever the install modal is rendered)

The design specifies:
- Install modal default selection: symlink (pre-selected)
- Second option: copy
- Provider dropdown respects `SupportsSymlinks` — if the selected provider does not support symlinks for this content type, the symlink option is disabled with an explanation

**Step 1:** Find the install modal definition (the modal displayed when the user presses the install key in the detail view). Locate the install method selection UI — it currently may only offer copy, or may not offer a choice at all.

**Step 2:** Add symlink as the pre-selected default option. The method selection should render as:
```
Install method:
  ● Symlink (default — edits in library reflected immediately)
  ○ Copy
```

**Step 3:** Consult `provider.SymlinkSupport` for the selected provider and content type. If `SymlinkSupport[item.Type] == false` (or the key is absent and it's a JSON-merge type like Hooks/MCP):
- Disable the symlink option (gray it out or remove it)
- Pre-select copy instead
- Show an explanation: e.g., `"Symlinks not supported for hooks — copy used automatically"`

```go
// In the install modal rendering code:
provSymlinkOK := prov.SymlinkSupport[item.Type] // false for Hooks, MCP; true for filesystem types
if !provSymlinkOK {
    // render copy only, with note
} else {
    // render symlink (pre-selected) and copy options
}
```

**Step 4:** Pass the selected method (`installer.MethodSymlink` or `installer.MethodCopy`) through to the install action.

**Test:** `go test ./cli/internal/tui/ -run TestInstall` or golden file check.

**Success:** Install modal pre-selects symlink by default; symlink option is disabled (with explanation) for hooks/MCP content types; copy is always available.

---

### 4.6 — Regenerate golden files

After all TUI changes, the golden baselines are stale. Regenerate them:

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden
```

Then review the diff of all golden files to confirm changes look correct:
- Sidebar should show "Library" instead of "My Tools"
- Sidebar should show "Add" instead of "Import"
- `[LOCAL]` badges should show `[LIBRARY]`
- No unintended visual regressions

If anything looks wrong, fix the underlying code and re-run `-update-golden`.

**Test:** After regenerating: `go test ./cli/internal/tui/` — must pass without `-update-golden`.

**Success:** All golden files regenerated; `make test` passes.

---

## Phase 5: Canonical Format Rebrand

Renaming comments and strings from "Claude Code format" to "syllago-native format". No functional changes.

### 5.1 — Update comments in converter files

**Scope:** All files in `cli/internal/converter/` that reference "Claude Code format" in comments or strings.

Search for references:
```bash
grep -r "Claude Code format\|claude.code format\|claude-code format" cli/internal/converter/
```

Update each occurrence. Examples:
- `"Claude Code format"` → `"syllago-native format"`
- `"Claude Code's native format"` → `"syllago canonical format"`
- Comments explaining that content is stored in "Claude Code format" → "syllago-native format"

**Do NOT change:**
- The string `"claude-code"` as a provider slug
- Provider-specific logic that targets Claude Code specifically
- The `Canonicalize` and `Render` function interfaces (unchanged)

**Test:** `go build ./cli/...` — just compilation check. No behavioral change to test.

**Success:** No "Claude Code format" references remain in converter comments.

---

### 5.2 — Update comments in scanner and types

Search:
```bash
grep -r "Claude Code format\|claude.code format" cli/internal/catalog/
```

Update any found references. Also update the `scanRoot` comment to remove `local/` references:
```go
// scanRoot scans a base directory for content items of all types.
// If library is true, discovered items are marked as library items (from ~/.syllago/content/).
func scanRoot(cat *Catalog, baseDir string, library bool) error {
```

**Test:** Compilation check only.

**Success:** No "Claude Code format" references in catalog package.

---

### 5.3 — Update `rootCmd` Long description in `main.go`

**File:** `cli/cmd/syllago/main.go` (lines 36-58)

Current text:
```go
Long: `Syllago manages AI tool configurations across providers.

Workflow:
  1. syllago import    Bring content in from a provider, path, or git URL
  2. syllago export    Send content out to any provider's install location
  3. syllago promote   Share local content to a registry (PR workflow)
```

Updated text:
```go
Long: `Syllago manages AI tool content across providers.

Workflow:
  1. syllago add       Bring content into your library from any source
  2. syllago install   Activate library content in a provider
  3. syllago share     Contribute to a team repo
  4. syllago publish   Contribute to a registry

Browse and manage content interactively with: syllago (no arguments)`,
```

**Test:** `syllago --help` shows updated text.

**Success:** Help text accurately describes the new workflow.

---

## Phase 6: CLI Design Standards

Polish tasks that implement the CLI UX standards from the design doc.

### 6.1 — Add `--json` output support to new commands

All new commands in Phase 2 should respect `output.JSON`. When `--json` is set, print structured JSON instead of prose. The `output.JSON` flag is already set by `PersistentPreRunE` in `main.go`.

**Files:** `install_cmd.go`, `remove_cmd.go`, `convert_cmd.go`

For each command, wrap prose output with `if !output.JSON { ... }` and add JSON output path using `output.Print(result)`.

Define result structs:
```go
// install_cmd.go
type installResult struct {
    Installed []installedItem `json:"installed"`
    Skipped   []skippedItem   `json:"skipped,omitempty"`
}

// remove_cmd.go
type removeResult struct {
    Name           string   `json:"name"`
    Type           string   `json:"type"`
    UninstalledFrom []string `json:"uninstalled_from"`
    RemovedPath    string   `json:"removed_path"`
}
```

**Test:** `syllago install my-skill --to claude-code --json` produces valid JSON.

**Success:** All new commands support `--json` flag.

---

### 6.2 — Add TTY detection to `remove_cmd.go` confirmation

**File:** `cli/cmd/syllago/remove_cmd.go`

The confirmation prompt in `runRemove` uses `isInteractive()` which already exists in `helpers.go`. Verify it handles non-TTY correctly.

Current logic (Phase 2.2):
```go
if !force && !dryRun && isInteractive() {
    // prompt
}
```

This is correct per the design spec: non-interactive (pipe/redirect) skips the prompt and treats as --force. But we should also handle `--no-input`:

```go
noInput, _ := cmd.Flags().GetBool("no-input")
// ...
if !force && !dryRun && !noInput && isInteractive() {
    // prompt
}
```

Add `--no-input` flag to `removeCmd.init()`:
```go
removeCmd.Flags().Bool("no-input", false, "Skip interactive prompts, use defaults")
```

**Test:** Pipe stdin: `echo "" | syllago remove nonexistent 2>/dev/null` — should fail cleanly (item not found), not hang on prompt.

**Success:** TTY handling correct; `--no-input` works.

---

### 6.3 — Add `--quiet` flag respect to new commands

Verify that all new commands check `output.Quiet` before printing non-essential output. The flag is already parsed by `PersistentPreRunE` in `main.go` and sets `output.Quiet = quiet`.

In `install_cmd.go`, the next-command suggestion block should be:
```go
if installed > 0 && !output.Quiet {
    fmt.Fprintf(output.Writer, "\n  Next: ...")
}
```

Verify `remove_cmd.go` and `convert_cmd.go` use `output.Quiet` consistently.

**Test:** `syllago add --from claude-code --quiet` — no summary output, only error messages.

**Success:** `--quiet` suppresses hints and summaries across all new commands.

---

### 6.4 — Color usage standard and `--no-color` flag

Per the design's CLI standards, output must use color consistently and be disableable.

**Color semantics** (apply across all new and refactored commands):
- Green (`output.Green` or equivalent): success indicators — "Added", "Installed", "Symlinked"
- Red (`output.Red`): errors and destructive action warnings
- Yellow (`output.Warn`): warnings, confirmations, deprecation notices
- Dim/gray (`output.Dim`): secondary information — paths, hints, next-command suggestions

**Disable conditions:** Output package must disable color when any of the following are true:
- `NO_COLOR` environment variable is set (any non-empty value)
- `TERM=dumb`
- `--no-color` flag is passed
- stdout is not a TTY

**Changes:**

**File:** `cli/internal/output/output.go` (or wherever color helpers live)

1. Add `NoColor bool` flag to the output package (alongside `Quiet`, `JSON`).
2. In `PersistentPreRunE` in `main.go`, detect and set `output.NoColor`:
   ```go
   noColor, _ := cmd.Flags().GetBool("no-color")
   output.NoColor = noColor ||
       os.Getenv("NO_COLOR") != "" ||
       os.Getenv("TERM") == "dumb" ||
       !isatty.IsTerminal(os.Stdout.Fd())
   ```
3. Add `--no-color` persistent flag in `main.go`.
4. Wrap all color calls in new command files (`add_cmd.go`, `install_cmd.go`, `remove_cmd.go`, `uninstall_cmd.go`, `convert_cmd.go`, `share_cmd.go`, `publish_cmd.go`) with the `NoColor` guard — or implement it centrally in the output package color functions.

Note: Check if `output` already has color support before adding new functions. Only add what is missing.

**Test:** `NO_COLOR=1 syllago add --from claude-code --dry-run` — output contains no ANSI escape codes.

**Success:** Color correctly applied to success/error/warning/secondary output; disabled by `NO_COLOR`, `TERM=dumb`, `--no-color`, or non-TTY.

---

### 6.5 — Progress indicators for long operations

Per the design's CLI standards, long operations must show activity within 100ms.

**Operations requiring spinners:**
- `add <git-url>` → spinner: "Cloning repository..."
- `install` with cross-provider conversion → spinner: "Converting to <provider> format..."
- `share` / `publish` → spinner: "Staging changes..."
- `loadout apply` → progress bar for multi-item installs (already designed; verify it exists)

**File:** Any command file listed above, using the existing spinner infrastructure (check if `output` or `tea` spinner is already used in the TUI or CLI).

For CLI commands (non-TUI), use a simple spinner from the `output` package or a lightweight spinner library. Spinner must:
- Start before the operation begins
- Stop immediately on completion or error
- Not render when `--quiet` is set
- Not render when stdout is not a TTY or `--no-input` is set

For the `add` git URL path specifically: the git clone may take several seconds. Add the spinner around the git clone call in `runAdd`.

For `install` with conversion: wrap the `installer.Install` call with a spinner when the item requires cross-provider rendering.

**Note:** If the existing codebase already has spinner support (e.g., via `bubbletea` in TUI), check whether a reusable spinner helper exists before introducing a new dependency.

**Test:** Manually verify that `syllago add <git-url>` shows spinner activity; `syllago add <git-url> --quiet` shows no spinner.

**Success:** All listed long operations show activity within 100ms; spinner suppressed in `--quiet` / non-TTY mode.

---

## Phase 7: Cleanup & Verification

Final pass to remove orphan code and ensure the full suite passes.

### 7.1 — Remove orphan code from deleted commands

After deleting `export.go` and `promote_cmd.go`, check for orphan helpers they used:

In `helpers.go`:
- `findItemByPath` — used by `promote_cmd.go`. Check if anything else uses it. If not, delete it.
- `effectiveProvider` — moved from `export.go` in Phase 3.2. Now used by `install_cmd.go` if applicable. Keep or remove based on usage.
- `filterBySource` — moved from `export.go` in Phase 3.2. Still used by `list.go`. Keep (do NOT delete). Updated to `"library"` case in Phase 3.3 Step 29.

Search:
```bash
grep -r "findItemByPath\|effectiveProvider\|filterBySource" cli/
```

Delete any functions that have zero callers.

**`create.go` destination path update:**

`destDirForCreate` (lines 73–77) writes to `filepath.Join(root, "local", ...)`. After the redesign, `local/` no longer exists and content should go to the global library. Update:

```go
func destDirForCreate(_ string, ct catalog.ContentType, name, providerSlug string) string {
    globalDir := catalog.GlobalContentDir()
    if globalDir == "" {
        globalDir = "."  // fallback, shouldn't happen in practice
    }
    if ct.IsUniversal() {
        return filepath.Join(globalDir, string(ct), name)
    }
    return filepath.Join(globalDir, string(ct), providerSlug, name)
}
```

Note: The `root` parameter is no longer used; rename it to `_` or remove it. Update the function signature and its call site in `runCreate`.

Also update:
- `createCmd.Short`: `"Scaffold a new content item in local/"` → `"Scaffold a new content item in the global library"`
- `createCmd.Long`: Replace all references to `local/` with `~/.syllago/content/`

Update `create_test.go` to use `catalog.GlobalContentDirOverride` for path assertions (same pattern as `add_cmd_test.go`):
```go
globalDir := t.TempDir()
catalog.GlobalContentDirOverride = globalDir
t.Cleanup(func() { catalog.GlobalContentDirOverride = "" })
```
Update destination path assertions from `filepath.Join(root, "local", ...)` to `filepath.Join(globalDir, ...)`.

**Test:** `go build ./cli/...` — no "declared but not used" errors (Go won't error on unused functions, but `go vet` might flag some cases). `go test ./cli/cmd/syllago/... -run TestCreate` — passes.

**Success:** No orphan helper functions remain. `create.go` writes to global library. `create_test.go` passes.

---

### 7.2 — Update `backfill` command in `main.go`

**File:** `cli/cmd/syllago/main.go` line 127:

```go
// Before:
if item.Local || item.Meta != nil {
    continue // skip local items and items that already have metadata
}
// After:
if item.Library || item.Meta != nil {
    continue // skip library items and items that already have metadata
}
```

Also update the comment. If `backfill` is still useful post-redesign, keep it. If it's only useful for the old `local/` workflow, hide it permanently.

**Test:** `go build ./cli/cmd/syllago/` — compiles.

**Success:** Backfill updated; no compilation errors.

---

### 7.3 — Update `CleanupPromotedItems` in `cleanup.go`

**File:** `cli/internal/catalog/cleanup.go`

This function was updated in Phase 3.3, but review it again to ensure the logic is sound after all changes:

```go
// CleanupPromotedItems removes library items whose ID, name, and type
// all match a shared item. This happens after a share/PR is merged and pulled —
// the shared copy now exists, so the library copy is redundant.
func CleanupPromotedItems(cat *Catalog) ([]CleanupResult, error) {
    sharedByID := make(map[string]sharedInfo)
    for _, item := range append(cat.Items, cat.Overridden...) {
        if !item.Library && item.Registry == "" && item.Meta != nil && item.Meta.ID != "" {
            sharedByID[item.Meta.ID] = sharedInfo{Name: item.Name, Type: item.Type}
        }
    }

    var cleaned []CleanupResult
    for _, item := range cat.Items {
        if !item.Library || item.Meta == nil || item.Meta.ID == "" {
            continue
        }
        // ... rest unchanged
    }
    return cleaned, nil
}
```

**Test:** `go test ./cli/internal/catalog/...`

**Success:** `CleanupPromotedItems` uses updated field names; tests pass.

---

### 7.4 — Full test suite run

Run the complete test suite and fix any remaining failures:

```bash
cd /home/hhewett/.local/src/syllago/cli && make test
```

Expected state at end of Phase 7:
- All non-golden tests pass
- All golden tests pass (baseline regenerated in Phase 4.6)
- No compilation errors
- `syllago --help` shows the updated workflow description
- `syllago add --help` works
- `syllago install --help` works
- `syllago uninstall --help` works
- `syllago remove --help` works
- `syllago convert --help` works
- `syllago share --help` works
- `syllago publish --help` works
- `syllago import` prints redirect message
- `syllago export` prints redirect message
- `syllago promote` prints redirect message

If any test fails, fix it before marking this phase complete.

**Success:** `make test` exits 0.

---

### 7.5 — Build and smoke test

```bash
make build
syllago --help
syllago add --help
syllago install --help
syllago uninstall --help
syllago remove --help
syllago convert --help
syllago share --help
syllago publish --help
syllago import   # should print redirect
syllago export   # should print redirect
syllago promote  # should print redirect
```

Also verify the TUI:
```bash
syllago  # launches TUI — confirm sidebar shows "Library" and "Add"
```

**Success:** All commands respond correctly; TUI renders correctly.

---

## Dependency Graph

```
Phase 1.1 (SymlinkSupport field)
  └→ Phase 1.2 (populate all providers)
      └→ Phase 2.1 (install_cmd, consults SymlinkSupport)

Phase 1.3 (CountLibrary, ByTypeLibrary, Library ContentType)
  └→ Phase 3.3 (rename Local→Library, uses new methods)
      └→ Phase 4.1 (sidebar verify)
      └→ Phase 4.3 (app.go promote→share)
      └→ Phase 4.6 (golden regen)

Phase 1.4 (GlobalContentDir verified) — prerequisite gate
  IF any issues found: fix before proceeding to Phase 2
  └→ Phase 2.1-2.6 (all new commands use GlobalContentDir)
  └→ Phase 3.1 (add_cmd uses GlobalContentDir)

Phase 1.4b (GlobalContentDirOverride test helper)
  └→ Phase 1.4 (scanner test uses it)
  └→ Phase 2.1 command tests (need override to avoid polluting real ~/.syllago)
  └→ Phase 2.6 uninstall_cmd tests (same)
  └→ Phase 3.1 add_cmd_test.go (same)

Phase 1.5 (metadata.Meta field extensions)
  └→ Phase 3.1 Step 10 (add_cmd writes new metadata fields: added_at, added_by, source_type, has_source)

Phase 2.1 (install_cmd) — depends on Phase 1.1, 1.2, 1.4
Phase 2.2 (remove_cmd) — depends on Phase 1.4; independent of 2.1
Phase 2.3 (convert_cmd) — depends on Phase 1.4; independent of 2.1-2.2
Phase 2.4 (share_cmd) — depends on Phase 1.4; independent of 2.1-2.3
Phase 2.5 (publish_cmd) — depends on Phase 2.4 (shares findLibraryItem helper)
Phase 2.6 (uninstall_cmd) — depends on Phase 1.4; independent of 2.1-2.5

Phase 2.7 (deprecated stubs) — IMPORTANT two-phase registration:
  Step 1: Write deprecated_cmds.go in Phase 2.7 (do NOT call rootCmd.AddCommand yet)
  Step 2: Add rootCmd.AddCommand(deprecatedExportCmd, deprecatedPromoteCmd) in Phase 3.2
          Add rootCmd.AddCommand(deprecatedImportCmd) in Phase 3.1 Step 8
  Reason: Cobra errors on duplicate command names. Real commands must be deleted first.

Phase 3.1 (import→add rename) — depends on Phase 1.4, 1.4b, 1.5, 2.7
  Includes: Steps 9-10 (.source/ preservation, metadata writing)
Phase 3.2 (delete export/promote) — depends on Phase 2.1, 2.4, 2.5, 2.7
  Also: registers deprecatedExportCmd and deprecatedPromoteCmd
  Also: MUST move filterBySource to helpers.go (Step 1) before deletion — list.go depends on it
Phase 3.3 (Local→Library) — depends on Phase 1.3, Phase 3.2 (filterBySource must be in helpers.go); MUST precede Phase 4
  Includes: Steps 29-30 (list.go and list_test.go updates)
Phase 3.4 (promote.go update) — depends on Phase 3.3
Phase 3.5 (loadout provider-neutral) — independent; can be done anytime

Phase 4.1-4.3 — depend on Phase 3.3
Phase 4.4 (Export action removal) — depends on Phase 3.2 (export.go deleted), Phase 3.3
Phase 4.5 (install modal symlink default) — depends on Phase 1.1, 1.2 (SymlinkSupport populated), Phase 3.3
Phase 4.6 — depends on Phase 4.1-4.5

Phase 5 (rebrand) — independent after Phase 3; pure comment/string changes
Phase 6.1-6.3 (UX polish) — independent after Phase 2; can overlap with Phase 4-5
Phase 6.4 (color standard) — independent; touches output package and all new command files
Phase 6.5 (progress indicators) — independent; requires commands exist (after Phase 2+3)

Phase 7.1-7.5 — must come last
```

---

## Test Commands by Phase

| Phase | Test Command | Expected Result |
|-------|-------------|-----------------|
| 1.1-1.2 | `go test ./cli/internal/provider/...` | Pass (additive only) |
| 1.3 | `go test ./cli/internal/catalog/...` | Pass (additive only) |
| 1.4b | `go test ./cli/internal/catalog/...` | Pass (additive: override var added) |
| 1.5 | `go test ./cli/internal/metadata/...` | Pass (additive: new fields omitempty) |
| 2.1 | `go test ./cli/cmd/syllago/... -run TestInstall` | New tests pass |
| 2.2 | `go test ./cli/cmd/syllago/... -run TestRemove` | New tests pass |
| 2.3 | `go test ./cli/cmd/syllago/... -run TestConvert` | New tests pass |
| 2.4-2.5 | `go build ./cli/cmd/syllago/` | Compiles |
| 2.6 | `go test ./cli/cmd/syllago/... -run TestUninstall` | New tests pass |
| 2.7 | `go build ./cli/cmd/syllago/` | Compiles (stubs not registered yet) |
| 3.1 | `go test ./cli/cmd/syllago/... -run TestAdd` | Renamed tests pass; metadata and .source/ tests pass |
| 3.2 | `go build ./cli/cmd/syllago/` | Compiles after deletes |
| 3.3 | `make test` | Fails until tests updated; fix all |
| 4.4 | `go test ./cli/internal/tui/ -run TestDetail` | Export key removed; no regressions |
| 4.5 | `go test ./cli/internal/tui/ -run TestInstall` | Symlink default; SymlinkSupport disable works |
| 4.6 | `go test ./cli/internal/tui/ -update-golden && go test ./cli/internal/tui/` | Pass |
| 7.4 | `make test` | All pass |

---

## Success Criteria

Per the design doc:
1. All seven commands work end-to-end: `add` (provider source), `remove`, `install`, `uninstall`, `convert`, `share`, `publish`
   - Note: `add <git-url>` and `add <filesystem-path>` are deferred to a follow-up plan
2. Loadouts can target any provider via `--to` flag; `--method` controls symlink vs copy
3. Content in `~/.syllago/content/` is visible in the TUI's "Library" section
4. No references to `export`, `promote`, `import`, or "My Tools" in user-facing strings
5. Canonical format documentation says "syllago-native format" not "Claude Code format"
6. `make test` passes completely

Additional verification:
- `syllago import` → helpful redirect message
- `syllago export` → helpful redirect message
- `syllago promote` → helpful redirect message
- Sidebar shows "Library" with count of global content items
- Sidebar shows "Add" in Configuration section
- `syllago add --from claude-code` writes to `~/.syllago/content/` and creates `.syllago.yaml`
- `syllago add --from cursor` of a `.mdc` file creates `.source/rule.mdc` alongside canonical `rule.md`
- `syllago install my-skill --to gemini-cli` activates content in Gemini CLI
- `syllago install my-cursor-rule --to cursor` uses `.source/` version (lossless roundtrip)
- `syllago uninstall my-skill --from claude-code` removes symlink/file from provider; content stays in library
- Each added item has a `.syllago.yaml` with `name`, `type`, `source_provider`, `source_format`, `source_type`, `has_source`, `added_at`, `added_by`
