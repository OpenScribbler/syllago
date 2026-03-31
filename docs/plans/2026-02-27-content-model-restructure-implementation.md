# Content Model Restructure - Implementation Plan

**Goal:** Restructure syllago's content model: config-aware content root, content/ umbrella directory, example classification, TUI badge/visibility, kitchen-sink examples, CLI fixes, registry security, content precedence, and polish.

**Architecture:** Six-phase rollout from foundation (content root resolution, directory move) through content model (example classification, badges, hide/show), kitchen-sink examples with CI coverage, CLI fixes (import writes, export all-source, create, list), registry/team features (allowedRegistries, inspect, precedence, promote), and polish (sync-and-export, first-run, aliases).

**Tech Stack:** Go, Bubble Tea, lipgloss, cobra, gopkg.in/yaml.v3

**Design Doc:** docs/plans/2026-02-27-content-model-restructure.md

---

---

## Phase 1: Foundation

---

## Task 1: Add `ContentRoot` field to Config struct

**Files:**
- Modify: `cli/internal/config/config.go`
- Test: `cli/internal/config/config_test.go`

**Depends on:** nothing

**Success Criteria:**
- [ ] `Config` struct has `ContentRoot string` field with JSON tag `content_root`
- [ ] `Save` + `Load` round-trip preserves the field
- [ ] Empty `ContentRoot` marshals as omitted (not `""` in JSON)

### Step 1: Write the failing test

Add to `cli/internal/config/config_test.go`:

```go
func TestConfigContentRoot(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

    cfg := &Config{
        Providers:   []string{"claude-code"},
        ContentRoot: "content",
    }
    if err := Save(dir, cfg); err != nil {
        t.Fatalf("Save failed: %v", err)
    }

    loaded, err := Load(dir)
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if loaded.ContentRoot != "content" {
        t.Errorf("ContentRoot = %q, want %q", loaded.ContentRoot, "content")
    }

    // Verify it's present in raw JSON when set
    data, _ := os.ReadFile(FilePath(dir))
    if !strings.Contains(string(data), "content_root") {
        t.Error("JSON should contain content_root key when set")
    }
}

func TestConfigContentRootOmittedWhenEmpty(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

    cfg := &Config{Providers: []string{"claude-code"}}
    if err := Save(dir, cfg); err != nil {
        t.Fatalf("Save failed: %v", err)
    }

    data, _ := os.ReadFile(FilePath(dir))
    if strings.Contains(string(data), "content_root") {
        t.Error("JSON should not contain content_root when empty")
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `cfg.ContentRoot undefined`

### Step 3: Write minimal implementation

In `cli/internal/config/config.go`, add the field to `Config`:

```go
type Config struct {
    Providers   []string          `json:"providers"`
    ContentRoot string            `json:"content_root,omitempty"`
    Registries  []Registry        `json:"registries,omitempty"`
    Preferences map[string]string `json:"preferences,omitempty"`
    Sandbox     SandboxConfig     `json:"sandbox,omitempty"`
}
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/internal/config/config.go cli/internal/config/config_test.go
git commit -m "feat: add ContentRoot field to Config struct"
```

---

## Task 2: Replace `findSkillsDir` with config-aware `findContentRepoRoot`

**Files:**
- Modify: `cli/cmd/syllago/main.go`
- Test: `cli/cmd/syllago/main_test.go`

**Depends on:** Task 1

**Success Criteria:**
- [ ] Resolution order: build-time embed → `.syllago/config.json` contentRoot → any content dir at project root → project root fallback
- [ ] `findSkillsDir` var is removed; existing `TestTUIErrorMessageContentRepoNotFound` continues to pass (now uses new override mechanism)
- [ ] New test verifies config-based resolution
- [ ] New test verifies content-dir-based fallback
- [ ] New test verifies bare project-root fallback (no content dirs)

### Step 1: Write the failing tests

Add to `cli/cmd/syllago/main_test.go`:

```go
func TestFindContentRepoRootConfigBased(t *testing.T) {
    tmp := t.TempDir()
    // Create project markers
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
    // Create .syllago/config.json with contentRoot
    os.MkdirAll(filepath.Join(tmp, ".syllago"), 0755)
    os.WriteFile(filepath.Join(tmp, ".syllago", "config.json"),
        []byte(`{"providers":[],"content_root":"content"}`), 0644)
    // Create the content dir so the path is valid
    os.MkdirAll(filepath.Join(tmp, "content"), 0755)

    oldFindProject := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    defer func() { findProjectRoot = oldFindProject }()

    oldRepoRoot := repoRoot
    repoRoot = ""
    defer func() { repoRoot = oldRepoRoot }()

    got, err := findContentRepoRoot()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    want := filepath.Join(tmp, "content")
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}

func TestFindContentRepoRootContentDirFallback(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
    // No config, but skills/ exists at project root
    os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

    oldFindProject := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    defer func() { findProjectRoot = oldFindProject }()

    oldRepoRoot := repoRoot
    repoRoot = ""
    defer func() { repoRoot = oldRepoRoot }()

    got, err := findContentRepoRoot()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got != tmp {
        t.Errorf("got %q, want %q", got, tmp)
    }
}

func TestFindContentRepoRootProjectRootFallback(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
    // No config, no content dirs — should fall back to project root

    oldFindProject := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    defer func() { findProjectRoot = oldFindProject }()

    oldRepoRoot := repoRoot
    repoRoot = ""
    defer func() { repoRoot = oldRepoRoot }()

    got, err := findContentRepoRoot()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got != tmp {
        t.Errorf("got %q, want %q", got, tmp)
    }
}
```

Also update `TestTUIErrorMessageContentRepoNotFound` — the test currently overrides `findSkillsDir`, which will be removed. Update it to override `findProjectRoot` to return an error:

```go
func TestTUIErrorMessageContentRepoNotFound(t *testing.T) {
    oldFindProject := findProjectRoot
    findProjectRoot = func() (string, error) {
        return "", fmt.Errorf("no project root")
    }
    defer func() { findProjectRoot = oldFindProject }()

    oldRepoRoot := repoRoot
    repoRoot = ""
    defer func() { repoRoot = oldRepoRoot }()

    err := runTUI(rootCmd, []string{})
    if err == nil {
        t.Fatal("expected error when content repo not found")
    }
    errMsg := err.Error()
    if strings.Contains(errMsg, "skills/") {
        t.Error("error message should not mention internal 'skills/' directory")
    }
    if !strings.Contains(errMsg, "syllago") {
        t.Error("error message should mention 'syllago'")
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `TestFindContentRepoRootConfigBased` fails because logic doesn't exist yet; `TestTUIErrorMessageContentRepoNotFound` may also fail once we remove `findSkillsDir`.

### Step 3: Write minimal implementation

Replace `findContentRepoRoot`, `findSkillsDir`, and `findSkillsDirImpl` in `cli/cmd/syllago/main.go`:

```go
// findContentRepoRoot returns the path syllago uses as its content root. It tries:
// 1. Build-time embedded path (via -ldflags)
// 2. Config-aware resolution from the project root
func findContentRepoRoot() (string, error) {
    if repoRoot != "" {
        if _, err := os.Stat(repoRoot); err == nil {
            return repoRoot, nil
        }
    }

    projectRoot, err := findProjectRoot()
    if err != nil {
        return "", fmt.Errorf("could not find syllago content repository")
    }

    return resolveContentRoot(projectRoot)
}

// resolveContentRoot applies the config-aware resolution order:
// 1. If .syllago/config.json exists with contentRoot → use <projectRoot>/<contentRoot>
// 2. Else if any content directory exists at project root → use project root
// 3. Else → use project root (scanner handles empty gracefully)
func resolveContentRoot(projectRoot string) (string, error) {
    cfg, err := config.Load(projectRoot)
    if err == nil && cfg.ContentRoot != "" {
        return filepath.Join(projectRoot, cfg.ContentRoot), nil
    }

    for _, ct := range catalog.AllContentTypes() {
        if _, err := os.Stat(filepath.Join(projectRoot, string(ct))); err == nil {
            return projectRoot, nil
        }
    }

    return projectRoot, nil
}
```

Remove the `findSkillsDir` var and `findSkillsDirImpl` function entirely. Also add the `config` import to `main.go`'s import block (it likely isn't imported there yet — check and add if needed).

Also add `"github.com/OpenScribbler/syllago/cli/internal/catalog"` if not already present. Looking at current imports in `main.go`, `catalog` is already imported.

Check if `config` is already imported in `main.go` — it is (`"github.com/OpenScribbler/syllago/cli/internal/config"`). So no import changes needed.

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/cmd/syllago/main.go cli/cmd/syllago/main_test.go
git commit -m "feat: replace findSkillsDir with config-aware content root resolution"
```

---

## Task 3: `syllago init` creates `local/` and `.gitignore` entries

**Files:**
- Modify: `cli/cmd/syllago/init.go`
- Test: `cli/cmd/syllago/init_test.go`

**Depends on:** Task 2

**Success Criteria:**
- [ ] `syllago init` creates a `local/` directory in the project root
- [ ] `syllago init` appends `local/` and `.syllago/registries/` to `.gitignore` if not already present
- [ ] Existing entries are not duplicated on re-run with `--force`
- [ ] If `.gitignore` does not exist, it is created

### Step 1: Write the failing tests

Add to `cli/cmd/syllago/init_test.go`:

```go
func TestInitCreatesLocalDir(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

    origDir, _ := os.Getwd()
    os.Chdir(tmp)
    defer os.Chdir(origDir)

    initCmd.Flags().Set("yes", "true")
    initCmd.Flags().Set("force", "false")
    if err := initCmd.RunE(initCmd, []string{}); err != nil {
        t.Fatalf("init failed: %v", err)
    }

    if _, err := os.Stat(filepath.Join(tmp, "local")); err != nil {
        t.Error("local/ directory should exist after init")
    }
}

func TestInitWritesGitignore(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

    origDir, _ := os.Getwd()
    os.Chdir(tmp)
    defer os.Chdir(origDir)

    initCmd.Flags().Set("yes", "true")
    initCmd.Flags().Set("force", "false")
    if err := initCmd.RunE(initCmd, []string{}); err != nil {
        t.Fatalf("init failed: %v", err)
    }

    data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
    if err != nil {
        t.Fatal(".gitignore should exist after init")
    }
    content := string(data)
    if !strings.Contains(content, "local/") {
        t.Error(".gitignore should contain local/")
    }
    if !strings.Contains(content, ".syllago/registries/") {
        t.Error(".gitignore should contain .syllago/registries/")
    }
}

func TestInitGitignoreNoDuplicates(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
    // Pre-populate .gitignore with one of the entries
    os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("local/\n"), 0644)

    origDir, _ := os.Getwd()
    os.Chdir(tmp)
    defer os.Chdir(origDir)

    initCmd.Flags().Set("yes", "true")
    initCmd.Flags().Set("force", "false")
    if err := initCmd.RunE(initCmd, []string{}); err != nil {
        t.Fatalf("init failed: %v", err)
    }

    data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
    count := strings.Count(string(data), "local/")
    if count != 1 {
        t.Errorf(".gitignore should contain local/ exactly once, got %d", count)
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `local/` directory not created, `.gitignore` not written

### Step 3: Write minimal implementation

Add a helper to `cli/cmd/syllago/init.go` and call it from `runInit`:

```go
// ensureGitignoreEntries appends gitignore entries that are not already present.
func ensureGitignoreEntries(projectRoot string, entries []string) error {
    gitignorePath := filepath.Join(projectRoot, ".gitignore")
    existing := ""
    data, err := os.ReadFile(gitignorePath)
    if err == nil {
        existing = string(data)
    }

    var toAdd []string
    for _, entry := range entries {
        if !strings.Contains(existing, entry) {
            toAdd = append(toAdd, entry)
        }
    }
    if len(toAdd) == 0 {
        return nil
    }

    // Ensure there's a trailing newline before appending
    if existing != "" && !strings.HasSuffix(existing, "\n") {
        existing += "\n"
    }
    content := existing + strings.Join(toAdd, "\n") + "\n"
    return os.WriteFile(gitignorePath, []byte(content), 0644)
}
```

In `runInit`, after `config.Save`, add:

```go
// Create local/ directory for user content
if err := os.MkdirAll(filepath.Join(root, "local"), 0755); err != nil {
    return fmt.Errorf("creating local/ directory: %w", err)
}

// Ensure .gitignore covers local content and registry cache
gitignoreEntries := []string{"local/", ".syllago/registries/"}
if err := ensureGitignoreEntries(root, gitignoreEntries); err != nil {
    // Non-fatal: warn but don't block init
    fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %s\n", err)
}
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/cmd/syllago/init.go cli/cmd/syllago/init_test.go
git commit -m "feat: init creates local/ directory and .gitignore entries"
```

---

## Task 4: Move repo content to `content/` and update config

**Files:**
- Shell: rename directories (git mv)
- Modify: `.syllago/config.json` (manual edit or `syllago init --force` equivalent)

**Depends on:** Tasks 1, 2, 3

**Success Criteria:**
- [ ] `skills/`, `agents/`, `rules/`, `hooks/`, `mcp/`, `prompts/`, `commands/`, `apps/` moved to `content/<each>`
- [ ] `.syllago/config.json` has `"content_root": "content"`
- [ ] `make test` passes (scanner finds items via config-based resolution)
- [ ] TUI launches and shows items

**Blocker:** After this task, `catalog.Scan(repoRoot)` hardcodes `local/` as `<contentRoot>/local/` — but `local/` lives at `<projectRoot>/local/`. Task 4a (below) fixes this before the filesystem move happens. **Do not execute this task until Task 4a is committed.**

**Note:** This task is a one-time repo restructure, not a code change. No new Go code is written. The implementation is mechanical filesystem operations.

### Step 1: Move directories with git

```bash
mkdir -p content
for dir in skills agents rules hooks mcp prompts commands apps; do
    if [ -d "$dir" ]; then
        git mv "$dir" "content/$dir"
    fi
done
```

### Step 2: Update `.syllago/config.json`

Edit `.syllago/config.json` to add `"content_root": "content"`. The existing file has `providers`, `registries`, etc. Add the field:

```json
{
  "providers": [...existing...],
  "content_root": "content",
  ...existing fields...
}
```

### Step 3: Run tests to verify scanner still works
Run: `make test`
Expected: PASS (scanner finds items at `content/skills/`, etc. via `resolveContentRoot`)

### Step 4: Verify TUI
Run: `make build && ./syllago` (in project root)
Expected: TUI shows all existing content items

### Step 5: Commit
```
git add content/ .syllago/config.json
git rm -r skills/ agents/ rules/ hooks/ mcp/ prompts/ commands/ apps/ 2>/dev/null || true
git commit -m "refactor: move content directories under content/ umbrella"
```

---

## Task 4a: Refactor `catalog.Scan` to accept separate `projectRoot` for `local/`

**Files:**
- Modify: `cli/internal/catalog/scanner.go`
- Modify: `cli/internal/catalog/scanner_test.go`
- Modify: `cli/cmd/syllago/main.go` (callers of `Scan` and `ScanWithRegistries`)
- Modify: `cli/cmd/syllago/init.go` (caller of `Scan`)
- Modify: `cli/cmd/syllago/export.go` (caller of `Scan`)
- Modify: `cli/internal/tui/app.go` (callers of `ScanWithRegistries`)

**Depends on:** Task 3 (local/ directory established)

**Problem being solved:** Currently `Scan(repoRoot)` resolves `local/` as `filepath.Join(repoRoot, "local")`. After Task 4 moves shared content under `content/`, `repoRoot` passed to `Scan` becomes `<project>/content/` — but `local/` stays at `<project>/local/`. Without this fix, all local items silently disappear after Task 4.

**Success Criteria:**
- [ ] `Scan` signature is `Scan(contentRoot string, projectRoot string) (*Catalog, error)`
- [ ] `ScanWithRegistries` signature is `ScanWithRegistries(contentRoot string, projectRoot string, registries []RegistrySource) (*Catalog, error)`
- [ ] `local/` is resolved as `filepath.Join(projectRoot, "local")`, not `filepath.Join(contentRoot, "local")`
- [ ] `cat.RepoRoot` is set to `contentRoot` (unchanged semantics for item path display)
- [ ] All callers updated to pass both roots
- [ ] Existing `TestScan` subtests updated to pass a second argument
- [ ] New test verifies that local items are found when `projectRoot != contentRoot`
- [ ] `make test` passes

### Step 1: Write the failing test

The new test goes at the bottom of `cli/internal/catalog/scanner_test.go`, inside the existing `TestScan` function as a new subtest, or as a new top-level test. Add it as a top-level test for clarity:

```go
func TestScanLocalRootSeparate(t *testing.T) {
    t.Parallel()
    projectRoot := t.TempDir()
    contentRoot := filepath.Join(projectRoot, "content")

    // Shared item under contentRoot
    writeFile(t, filepath.Join(contentRoot, "skills", "shared-skill", "SKILL.md"),
        "---\nname: Shared Skill\ndescription: A shared skill\n---\n")

    // Local item under projectRoot/local/ (NOT under contentRoot/local/)
    writeFile(t, filepath.Join(projectRoot, "local", "skills", "my-local-skill", "SKILL.md"),
        "---\nname: My Local Skill\ndescription: A local skill\n---\n")

    cat, err := Scan(contentRoot, projectRoot)
    if err != nil {
        t.Fatalf("Scan returned error: %v", err)
    }

    skills := cat.ByType(Skills)
    if len(skills) != 2 {
        var names []string
        for _, s := range skills {
            names = append(names, s.Name)
        }
        t.Fatalf("expected 2 skills (1 shared + 1 local), got %d: %v", len(skills), names)
    }

    // Find the local item and verify it's marked Local
    var foundLocal bool
    for _, s := range skills {
        if s.Name == "my-local-skill" {
            if !s.Local {
                t.Error("my-local-skill should be marked Local=true")
            }
            foundLocal = true
        }
    }
    if !foundLocal {
        t.Error("local skill was not discovered")
    }
}
```

Also update the existing `TestScan` subtests that call `Scan(root)` — they all pass a single temp dir for both roots, so update each call to `Scan(root, root)`. This keeps them passing because when `contentRoot == projectRoot`, the behavior is identical to the old implementation.

The affected subtests are:
- `"discovers universal items with frontmatter"` — line with `cat, err := Scan(root)`
- `"empty type directory does not error"` — line with `cat, err := Scan(root)`
- `"missing type directories are skipped"` — line with `cat, err := Scan(root)`
- `"rejects item names with sjson special characters"` — line with `cat, err := Scan(root)`
- `"discovers multiple content types"` — line with `cat, err := Scan(root)`

### Step 2: Run test to verify it fails

Run: `make test`
Expected: FAIL — `Scan(root, root)` wrong argument count; `TestScanLocalRootSeparate` fails because `Scan` only takes one argument.

### Step 3: Write minimal implementation

Update `cli/internal/catalog/scanner.go`. Change `Scan` and `ScanWithRegistries`:

```go
// Scan walks contentRoot and projectRoot/local/ to discover all content items.
// contentRoot is the directory containing shared content directories (skills/, agents/, etc.).
// projectRoot is the project root where local/ lives. Pass the same value for both when
// the project has not been restructured (contentRoot == projectRoot).
func Scan(contentRoot string, projectRoot string) (*Catalog, error) {
    cat := &Catalog{RepoRoot: contentRoot}

    // Scan shared content (git-tracked)
    if err := scanRoot(cat, contentRoot, false); err != nil {
        return nil, err
    }

    // Scan local content from projectRoot/local/ (gitignored, never under contentRoot)
    myToolsDir := filepath.Join(projectRoot, "local")
    if _, err := os.Stat(myToolsDir); err == nil {
        if err := scanRoot(cat, myToolsDir, true); err != nil {
            return nil, err
        }
    }

    return cat, nil
}

// ScanWithRegistries scans contentRoot (including projectRoot/local/) plus any provided
// registry sources. Registry items are tagged with their registry name.
// Per-registry scan errors are logged to stderr but do not fail the overall scan.
func ScanWithRegistries(contentRoot string, projectRoot string, registries []RegistrySource) (*Catalog, error) {
    // Start with the standard scan (local + shared repo items)
    cat, err := Scan(contentRoot, projectRoot)
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

`ScanRegistriesOnly` is unchanged — it never scans a local repo root, so no update needed.

**Update all callers** to pass both roots. For each caller, determine what to use as `projectRoot`:

**`cli/cmd/syllago/main.go`** — The TUI path already has both `root` (contentRoot from `findContentRepoRoot()`) and access to project root via `findProjectRoot()`. However, calling `findProjectRoot()` a second time is cheap. The cleanest approach: call `findProjectRoot()` once and thread it through. There are three call sites in this file:

1. `backfillCmd` (line ~108): `catalog.Scan(root)` → needs projectRoot. Add a `findProjectRoot()` call before this:
   ```go
   projectRoot, _ := findProjectRoot()
   if projectRoot == "" { projectRoot = root }
   cat, err := catalog.Scan(root, projectRoot)
   ```

2. `runTUI` (line ~241): `catalog.ScanWithRegistries(root, regSources)` → needs projectRoot. `root` comes from `findContentRepoRoot()`. Add:
   ```go
   projectRoot, _ := findProjectRoot()
   if projectRoot == "" { projectRoot = root }
   cat, err := catalog.ScanWithRegistries(root, projectRoot, regSources)
   ```
   Also update the rescan after cleanup (line ~253):
   ```go
   cat, err = catalog.ScanWithRegistries(root, projectRoot, regSources)
   ```
   Move the `projectRoot` declaration above both so both calls share it.

**`cli/cmd/syllago/init.go`** (line ~109): `catalog.Scan(repoRoot)`. Here `repoRoot` is the project root (init hasn't restructured anything yet, so `contentRoot == projectRoot`):
```go
cat, err := catalog.Scan(repoRoot, repoRoot)
```

**`cli/cmd/syllago/export.go`** (line ~92): `catalog.Scan(root)`. Here `root` comes from `findContentRepoRoot()`. Add a `findProjectRoot()` call:
```go
projectRoot, _ := findProjectRoot()
if projectRoot == "" { projectRoot = root }
cat, err := catalog.Scan(root, projectRoot)
```

**`cli/internal/tui/app.go`** — Three call sites, all using `a.catalog.RepoRoot` as the first argument. The `App` struct does not currently store `projectRoot` separately. The simplest fix: add a `projectRoot string` field to `App` and pass it through `NewApp`. Then update all three rescan sites:

```go
// In App struct:
projectRoot string

// In NewApp signature (add projectRoot parameter):
func NewApp(cat *catalog.Catalog, ..., projectRoot string) App {
    return App{
        ...
        projectRoot: projectRoot,
    }
}

// All three rescan sites:
cat, err := catalog.ScanWithRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
```

The `NewApp` caller in `runTUI` (`cli/cmd/syllago/main.go`) will also need to pass `projectRoot`.

**`cli/internal/tui/import.go`** (line ~176): `catalog.Scan(msg.path)` — this scans a freshly-cloned external repo for import preview. The cloned repo is a self-contained content repo; its `local/` (if any) should be scanned relative to itself. Pass `msg.path` for both arguments:
```go
cat, err := catalog.Scan(msg.path, msg.path)
```

### Step 4: Run test to verify it passes

Run: `make test`
Expected: PASS — `TestScanLocalRootSeparate` finds both shared and local items; all existing subtests pass with `Scan(root, root)`.

### Step 5: Commit
```
git add cli/internal/catalog/scanner.go cli/internal/catalog/scanner_test.go \
        cli/cmd/syllago/main.go cli/cmd/syllago/init.go cli/cmd/syllago/export.go \
        cli/internal/tui/app.go cli/internal/tui/import.go
git commit -m "refactor: Scan/ScanWithRegistries accept separate projectRoot for local/ resolution"
```

---

## Task 5: Add `Hidden` field to `Meta` struct

**Files:**
- Modify: `cli/internal/metadata/metadata.go`
- Test: `cli/internal/metadata/metadata_test.go`

**Depends on:** nothing (independent schema addition)

**Success Criteria:**
- [ ] `Meta` struct has `Hidden bool` field with YAML tag `hidden,omitempty`
- [ ] `Save` + `Load` round-trip preserves `Hidden: true`
- [ ] `Hidden: false` (zero value) is omitted from YAML output

### Step 1: Write the failing test

Add to `cli/internal/metadata/metadata_test.go`:

```go
func TestMetaHiddenField(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    itemDir := filepath.Join(dir, "test-item")

    m := &Meta{
        ID:     NewID(),
        Name:   "test-skill",
        Hidden: true,
    }
    if err := Save(itemDir, m); err != nil {
        t.Fatalf("Save failed: %v", err)
    }

    loaded, err := Load(itemDir)
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if !loaded.Hidden {
        t.Error("Hidden should be true after round-trip")
    }

    // Verify false (zero value) is omitted
    m2 := &Meta{ID: NewID(), Name: "visible"}
    if err := Save(filepath.Join(dir, "item2"), m2); err != nil {
        t.Fatalf("Save failed: %v", err)
    }
    data, _ := os.ReadFile(MetaPath(filepath.Join(dir, "item2")))
    if strings.Contains(string(data), "hidden") {
        t.Error("hidden field should be omitted when false")
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `m.Hidden undefined`

### Step 3: Write minimal implementation

In `cli/internal/metadata/metadata.go`, add the field to `Meta`:

```go
type Meta struct {
    ID             string       `yaml:"id"`
    Name           string       `yaml:"name"`
    Description    string       `yaml:"description,omitempty"`
    Version        string       `yaml:"version,omitempty"`
    Type           string       `yaml:"type,omitempty"`
    Author         string       `yaml:"author,omitempty"`
    Source         string       `yaml:"source,omitempty"`
    Tags           []string     `yaml:"tags,omitempty"`
    Hidden         bool         `yaml:"hidden,omitempty"`
    Dependencies   []Dependency `yaml:"dependencies,omitempty"`
    ImportedAt     *time.Time   `yaml:"imported_at,omitempty"`
    ImportedBy     string       `yaml:"imported_by,omitempty"`
    PromotedAt     *time.Time   `yaml:"promoted_at,omitempty"`
    PRBranch       string       `yaml:"pr_branch,omitempty"`
    SourceProvider string       `yaml:"source_provider,omitempty"`
    SourceFormat   string       `yaml:"source_format,omitempty"`
}
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/internal/metadata/metadata.go cli/internal/metadata/metadata_test.go
git commit -m "feat: add Hidden field to Meta struct"
```

---

## Phase 2: Content Model

---

## Task 6: Rename example items with `example-` prefix

**Files:**
- Shell: git mv content directories
- Modify: `content/skills/example-code-review/.syllago.yaml`
- Modify: `content/agents/example-code-reviewer/.syllago.yaml`
- Modify: `content/prompts/example-explain-code/.syllago.yaml`
- Modify: `content/prompts/example-write-tests/.syllago.yaml`

**Depends on:** Tasks 4, 5

**Success Criteria:**
- [ ] Directories renamed: `code-review` → `example-code-review`, `code-reviewer` → `example-code-reviewer`, `explain-code` → `example-explain-code`, `write-tests` → `example-write-tests`
- [ ] Each `.syllago.yaml` has `example` tag added and `hidden: true`
- [ ] `make test` passes

**Note:** This task is mechanical repo content changes, not Go code. The `.syllago.yaml` changes use the new `Hidden` field added in Task 5.

### Step 1: Rename directories

```bash
cd /path/to/syllago
git mv content/skills/code-review content/skills/example-code-review
git mv content/agents/code-reviewer content/agents/example-code-reviewer
git mv content/prompts/explain-code content/prompts/example-explain-code
git mv content/prompts/write-tests content/prompts/example-write-tests
```

### Step 2: Update each `.syllago.yaml`

`content/skills/example-code-review/.syllago.yaml`:
```yaml
name: example-code-review
description: Systematic code review workflow covering correctness, security, performance, and maintainability
version: "1.0"
hidden: true
tags:
  - example
  - code-review
  - quality
  - security
  - best-practices
```

`content/agents/example-code-reviewer/.syllago.yaml`:
```yaml
name: example-code-reviewer
description: AI agent personality for conducting thorough, constructive code reviews
version: "1.0"
hidden: true
tags:
  - example
  - agent
  - code-review
  - feedback
```

`content/prompts/example-explain-code/.syllago.yaml`:
```yaml
name: example-explain-code
description: Prompt template for explaining code to different audiences
version: "1.0"
hidden: true
tags:
  - example
  - prompts
  - explanation
```

`content/prompts/example-write-tests/.syllago.yaml`:
```yaml
name: example-write-tests
description: Prompt template for generating tests for existing code
version: "1.0"
hidden: true
tags:
  - example
  - prompts
  - testing
```

(Adjust `description` values to match what's actually in the existing files — read each `.syllago.yaml` before overwriting.)

### Step 3: Run tests
Run: `make test`
Expected: PASS

### Step 4: Verify TUI shows renamed items (they will be visible until Task 8 filters them)
Run: `make build && ./syllago` and navigate to Skills/Agents/Prompts.
Expected: Items listed as `example-code-review`, `example-code-reviewer`, etc.

### Step 5: Commit
```
git add content/
git commit -m "refactor: rename example content with example- prefix and mark hidden"
```

---

## Task 7: Add `IsExample()` method and `[EXAMPLE]` badge to TUI

**Files:**
- Modify: `cli/internal/catalog/types.go`
- Modify: `cli/internal/tui/items.go`
- Modify: `cli/internal/tui/styles.go`
- Test: `cli/internal/catalog/types_test.go` (create if it doesn't exist)
- Test: `cli/internal/tui/items_test.go`

**Depends on:** Task 5 (Hidden field added), Task 6 (example items exist)

**Success Criteria:**
- [ ] `ContentItem.IsExample()` returns true iff `Meta.Tags` contains `"example"`
- [ ] `exampleStyle` is defined in `styles.go` (dim purple, distinct from builtinStyle)
- [ ] Badge rendering in `items.go` shows `[EXAMPLE]` before `[BUILT-IN]` check
- [ ] `[EXAMPLE]` badge uses `exampleStyle`
- [ ] `localPrefixLen` for `[EXAMPLE]` is 10 (`"[EXAMPLE] "`)

### Step 1: Write the failing tests

In `cli/internal/catalog/types.go` test file (check if `cli/internal/catalog/types_test.go` exists; if not, create it):

```go
package catalog

import (
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestIsExample(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        item ContentItem
        want bool
    }{
        {
            name: "no meta",
            item: ContentItem{},
            want: false,
        },
        {
            name: "meta without example tag",
            item: ContentItem{Meta: &metadata.Meta{Tags: []string{"builtin"}}},
            want: false,
        },
        {
            name: "meta with example tag",
            item: ContentItem{Meta: &metadata.Meta{Tags: []string{"example", "skills"}}},
            want: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.item.IsExample(); got != tt.want {
                t.Errorf("IsExample() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

In `cli/internal/tui/items_test.go`, add:

```go
func TestExampleBadgeRendered(t *testing.T) {
    // Build a minimal catalog with one example item
    exampleItem := catalog.ContentItem{
        Name:        "example-code-review",
        Type:        catalog.Skills,
        Description: "A sample skill",
        Meta: &metadata.Meta{
            Tags: []string{"example"},
        },
    }
    m := newItemsModel(catalog.Skills, []catalog.ContentItem{exampleItem}, nil, t.TempDir())
    m.width = 120
    m.height = 40

    view := m.View()
    if !strings.Contains(view, "[EXAMPLE]") {
        t.Error("view should contain [EXAMPLE] badge for example items")
    }
}
```

(This test will need `metadata` imported in the test file — check whether it already is.)

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `IsExample undefined` on `ContentItem`

### Step 3: Write minimal implementation

In `cli/internal/catalog/types.go`, add after `IsBuiltin`:

```go
// IsExample returns true if this item is tagged as example content.
func (ci ContentItem) IsExample() bool {
    if ci.Meta == nil {
        return false
    }
    for _, tag := range ci.Meta.Tags {
        if tag == "example" {
            return true
        }
    }
    return false
}
```

In `cli/internal/tui/styles.go`, add after `builtinStyle`:

```go
// Example content badge (dim purple — lighter than builtin to indicate lower priority)
exampleStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#9D7ACC", Dark: "#A78BFA"})
```

In `cli/internal/tui/items.go`, update the badge block (currently lines 317-330):

```go
// Build description prefix: [EXAMPLE], [BUILT-IN], [LOCAL], or [registry-name]
localPrefix := ""
localPrefixLen := 0
if item.IsExample() {
    localPrefix = exampleStyle.Render("[EXAMPLE]") + " "
    localPrefixLen = 10 // "[EXAMPLE] "
} else if item.IsBuiltin() {
    localPrefix = builtinStyle.Render("[BUILT-IN]") + " "
    localPrefixLen = 11 // "[BUILT-IN] "
} else if item.Local {
    localPrefix = warningStyle.Render("[LOCAL]") + " "
    localPrefixLen = 8 // "[LOCAL] "
} else if item.Registry != "" {
    tag := "[" + item.Registry + "]"
    localPrefix = countStyle.Render(tag) + " "
    localPrefixLen = len(tag) + 1
}
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/internal/catalog/types.go cli/internal/tui/items.go cli/internal/tui/styles.go cli/internal/tui/items_test.go
git commit -m "feat: add IsExample() method and [EXAMPLE] badge to TUI"
```

---

## Task 8: Filter hidden items from display by default; `H` key to toggle

**Files:**
- Modify: `cli/internal/tui/keys.go`
- Modify: `cli/internal/tui/app.go`
- Modify: `cli/internal/tui/items.go` (status bar hint)
- Test: `cli/internal/tui/app_test.go`
- Test: `cli/internal/tui/items_test.go`

**Depends on:** Task 5 (Hidden field), Task 6 (example items marked hidden)

**Success Criteria:**
- [ ] Items where `Meta.Hidden == true` are excluded from list views by default
- [ ] `App` struct has `showHidden bool` field (session-only, not persisted)
- [ ] `H` key (lowercase) toggles `showHidden`
- [ ] When hidden items exist, items list footer shows "X hidden" hint
- [ ] When `showHidden` is true, hidden items are shown (without special badge; they already have `[EXAMPLE]` etc.)
- [ ] Hidden filter applied when building items for `screenItems` navigation

### Step 1: Write the failing tests

Add to `cli/internal/tui/app_test.go`:

```go
func TestHiddenItemsFilteredByDefault(t *testing.T) {
    // Build a catalog with one visible and one hidden item
    visibleItem := catalog.ContentItem{
        Name: "visible-skill",
        Type: catalog.Skills,
    }
    hiddenItem := catalog.ContentItem{
        Name: "hidden-skill",
        Type: catalog.Skills,
        Meta: &metadata.Meta{Hidden: true},
    }
    cat := &catalog.Catalog{
        Items:    []catalog.ContentItem{visibleItem, hiddenItem},
        RepoRoot: t.TempDir(),
    }
    app := NewApp(cat, nil, "", false, nil, &config.Config{}, false)
    app.width = 120
    app.height = 40

    // Navigate to Skills items
    m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
    app = m.(App)

    // Hidden item should not appear
    if len(app.items.items) != 1 {
        t.Errorf("expected 1 visible item, got %d", len(app.items.items))
    }
    if app.items.items[0].Name != "visible-skill" {
        t.Errorf("expected visible-skill, got %s", app.items.items[0].Name)
    }
}

func TestHKeyTogglesShowHidden(t *testing.T) {
    hiddenItem := catalog.ContentItem{
        Name: "hidden-skill",
        Type: catalog.Skills,
        Meta: &metadata.Meta{Hidden: true},
    }
    cat := &catalog.Catalog{
        Items:    []catalog.ContentItem{hiddenItem},
        RepoRoot: t.TempDir(),
    }
    app := NewApp(cat, nil, "", false, nil, &config.Config{}, false)
    app.width = 120
    app.height = 40

    // Navigate to Skills items (hidden item filtered out)
    m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
    app = m.(App)
    if len(app.items.items) != 0 {
        t.Errorf("expected 0 items before show-hidden, got %d", len(app.items.items))
    }

    // Press H to show hidden
    m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
    app = m.(App)
    if len(app.items.items) != 1 {
        t.Errorf("expected 1 item after show-hidden, got %d", len(app.items.items))
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `showHidden` undefined, hidden items not filtered

### Step 3: Write minimal implementation

In `cli/internal/tui/keys.go`, add `ToggleHidden` binding to the `keyMap` struct and `keys` var:

```go
// In keyMap struct:
ToggleHidden key.Binding

// In keys var:
ToggleHidden: key.NewBinding(
    key.WithKeys("H"),
    key.WithHelp("H", "show/hide hidden"),
),
```

In `cli/internal/tui/app.go`, add `showHidden bool` field to the `App` struct:

```go
showHidden bool // session-only: when true, show items with hidden:true
```

Add a helper method to `App` that filters hidden items:

```go
// visibleItems returns items from src, excluding hidden items unless showHidden is set.
func (a App) visibleItems(src []catalog.ContentItem) []catalog.ContentItem {
    if a.showHidden {
        return src
    }
    var result []catalog.ContentItem
    for _, item := range src {
        if item.Meta != nil && item.Meta.Hidden {
            continue
        }
        result = append(result, item)
    }
    return result
}
```

Wrap every `a.catalog.ByType(ct)` call that builds an `itemsModel` with `a.visibleItems(...)`. The primary call sites are:

1. In `screenCategory` Enter handling (line ~694):
```go
items := newItemsModel(ct, a.visibleItems(a.catalog.ByType(ct)), a.providers, a.catalog.RepoRoot)
```

2. In `screenItems` Back handling (line ~784):
```go
items := newItemsModel(ct, a.visibleItems(a.catalog.ByType(ct)), a.providers, a.catalog.RepoRoot)
```

3. The `MyTools` local items slice (line ~679-688):
```go
var localItems []catalog.ContentItem
for _, item := range a.catalog.Items {
    if item.Local {
        localItems = append(localItems, item)
    }
}
localItems = a.visibleItems(localItems)
items := newItemsModel(catalog.MyTools, localItems, a.providers, a.catalog.RepoRoot)
```

4. Live search filtering in `a.search.active` handler — wrap the `source` slice with `a.visibleItems(source)` before passing to `filterItems`.

Add `H` key handling in `screenItems` case of the Update switch, after the `Back` key check:

```go
if key.Matches(msg, keys.ToggleHidden) {
    a.showHidden = !a.showHidden
    ct := a.items.contentType
    var src []catalog.ContentItem
    switch ct {
    case catalog.MyTools:
        for _, item := range a.catalog.Items {
            if item.Local {
                src = append(src, item)
            }
        }
    case catalog.SearchResults:
        src = a.catalog.Items
    default:
        src = a.catalog.ByType(ct)
    }
    items := newItemsModel(ct, a.visibleItems(src), a.providers, a.catalog.RepoRoot)
    items.width = a.items.width
    items.height = a.items.height
    a.items = items
    return a, nil
}
```

Also add `H` key handling in `screenCategory` when focus is sidebar (so toggling works before drilling in):

```go
if key.Matches(msg, keys.ToggleHidden) {
    a.showHidden = !a.showHidden
    return a, nil
}
```

**Status bar hint** — in `items.go` `View()`, add a hint line before the final help line:

```go
// Count hidden items for status hint
hiddenCount := 0
for _, item := range allItemsForType {
    if item.Meta != nil && item.Meta.Hidden {
        hiddenCount++
    }
}
```

This requires `itemsModel` to have access to the full (unfiltered) item count. The simplest approach without restructuring: pass `hiddenCount` as a field on `itemsModel`. Add `hiddenCount int` to the struct and set it when building the model in `app.go`:

```go
// In app.go after building items model:
items.hiddenCount = countHidden(a.catalog.ByType(ct))

// Helper:
func countHidden(items []catalog.ContentItem) int {
    n := 0
    for _, item := range items {
        if item.Meta != nil && item.Meta.Hidden {
            n++
        }
    }
    return n
}
```

In `items.go` `View()`, update the footer line:

```go
hint := "up/down navigate • enter detail • esc back • / search • H show/hide hidden"
if m.hiddenCount > 0 {
    hint = fmt.Sprintf("up/down navigate • enter detail • esc back • / search • H toggle (%d hidden)", m.hiddenCount)
}
s += "\n" + helpStyle.Render(hint)
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/internal/tui/keys.go cli/internal/tui/app.go cli/internal/tui/items.go cli/internal/tui/app_test.go cli/internal/tui/items_test.go
git commit -m "feat: filter hidden items from TUI by default; H key to toggle"
```

---

## Task 9: Export/promote warnings for built-in and example content

**Files:**
- Modify: `cli/cmd/syllago/export.go`
- Modify: `cli/internal/tui/detail.go` (promote action)
- Test: `cli/cmd/syllago/export_test.go`

**Depends on:** Task 7 (IsExample method)

**Success Criteria:**
- [ ] `syllago export` prints a warning when exporting a built-in item, prompts for confirmation in interactive mode, proceeds in non-interactive mode
- [ ] The warning text is different for `IsExample()` items vs. `IsBuiltin()` non-example items
- [ ] Non-interactive mode (`isInteractive()` returns false) always proceeds without prompt

### Step 1: Write the failing tests

Add to `cli/cmd/syllago/export_test.go`:

```go
func TestExportBuiltinWarning(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

    // Create a local builtin item
    itemDir := filepath.Join(tmp, "local", "skills", "syllago-guide")
    os.MkdirAll(itemDir, 0755)
    os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("---\nname: Syllago Guide\n---\nBody."), 0644)
    os.WriteFile(filepath.Join(itemDir, ".syllago.yaml"), []byte("id: test\nname: syllago-guide\ntags:\n  - builtin\n"), 0644)

    // Non-interactive: should export with warning printed to stderr, not blocked
    origIsInteractive := isInteractive
    isInteractive = func() bool { return false }
    defer func() { isInteractive = origIsInteractive }()

    var errBuf bytes.Buffer
    origErrWriter := output.ErrWriter
    output.ErrWriter = &errBuf
    defer func() { output.ErrWriter = origErrWriter }()

    oldFindProject := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    defer func() { findProjectRoot = oldFindProject }()

    // ... (minimal provider setup to exercise the path) ...
    // The key assertion: warning appears in stderr
    // This is a smoke test — full provider setup would require more scaffolding.
    // The warning is printed before the export loop, so check for the warning message.
}
```

**Note:** Full integration of the export command test is heavy due to provider setup. The minimal test approach is to unit-test `shouldWarnOnExport` as a standalone function:

```go
func TestShouldWarnOnExport(t *testing.T) {
    tests := []struct {
        name     string
        item     catalog.ContentItem
        wantWarn bool
        wantMsg  string
    }{
        {
            name:     "example item",
            item:     catalog.ContentItem{Meta: &metadata.Meta{Tags: []string{"example", "builtin"}}},
            wantWarn: true,
            wantMsg:  "example",
        },
        {
            name:     "builtin non-example",
            item:     catalog.ContentItem{Meta: &metadata.Meta{Tags: []string{"builtin"}}},
            wantWarn: true,
            wantMsg:  "built-in",
        },
        {
            name:     "normal item",
            item:     catalog.ContentItem{Meta: &metadata.Meta{Tags: []string{"skills"}}},
            wantWarn: false,
        },
        {
            name:     "no meta",
            item:     catalog.ContentItem{},
            wantWarn: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            msg := exportWarnMessage(tt.item)
            if tt.wantWarn && msg == "" {
                t.Error("expected warning message, got empty")
            }
            if !tt.wantWarn && msg != "" {
                t.Errorf("expected no warning, got: %s", msg)
            }
            if tt.wantWarn && !strings.Contains(msg, tt.wantMsg) {
                t.Errorf("warning should contain %q, got: %s", tt.wantMsg, msg)
            }
        })
    }
}
```

### Step 2: Run test to verify it fails
Run: `make test`
Expected: FAIL - `exportWarnMessage undefined`

### Step 3: Write minimal implementation

Add to `cli/cmd/syllago/export.go`:

```go
// exportWarnMessage returns a warning string for built-in or example content.
// Returns "" if no warning is needed.
func exportWarnMessage(item catalog.ContentItem) string {
    if item.IsExample() {
        return fmt.Sprintf("%s is example content included for reference. Exporting it may conflict with your own tools.", item.Name)
    }
    if item.IsBuiltin() {
        return fmt.Sprintf("%s is a built-in syllago tool. Exporting it is unusual — it is already managed by syllago.", item.Name)
    }
    return ""
}
```

In `runExport`, inside the `for _, item := range items` loop, add before the provider-support check:

```go
if warnMsg := exportWarnMessage(item); warnMsg != "" {
    fmt.Fprintf(output.ErrWriter, "Warning: %s\n", warnMsg)
    if isInteractive() && os.Getenv("SYLLAGO_NO_PROMPT") != "1" && !output.JSON {
        fmt.Fprintf(output.Writer, "Continue exporting %s? [y/N] ", item.Name)
        var response string
        fmt.Scanln(&response)
        if strings.ToLower(response) != "y" {
            result.Skipped = append(result.Skipped, skippedItem{
                Name:   item.Name,
                Type:   string(item.Type),
                Reason: "skipped by user",
            })
            continue
        }
    }
}
```

### Step 4: Run test to verify it passes
Run: `make test`
Expected: PASS

### Step 5: Commit
```
git add cli/cmd/syllago/export.go cli/cmd/syllago/export_test.go
git commit -m "feat: warn on export of built-in and example content"
```

---

## Summary

| Task | Description | Files Changed | Phase |
|------|-------------|---------------|-------|
| 1 | Add `ContentRoot` to Config | `config/config.go` | 1 |
| 2 | Config-aware `findContentRepoRoot` | `cmd/syllago/main.go` | 1 |
| 3 | `init` creates `local/` + `.gitignore` | `cmd/syllago/init.go` | 1 |
| 4 | Move dirs to `content/`, update config | filesystem + `.syllago/config.json` | 1 |
| 4a | Refactor `Scan`/`ScanWithRegistries` for separate projectRoot | `catalog/scanner.go`, all callers | 1 |
| 5 | Add `Hidden` to `Meta` | `metadata/metadata.go` | 1 |
| 6 | Rename examples + add `hidden: true` | content `.syllago.yaml` files | 2 |
| 7 | `IsExample()` + `[EXAMPLE]` badge | `catalog/types.go`, `tui/items.go`, `tui/styles.go` | 2 |
| 8 | Hide/show toggle (`H` key) | `tui/keys.go`, `tui/app.go`, `tui/items.go` | 2 |
| 9 | Export warnings for built-in/example | `cmd/syllago/export.go` | 2 |

**Execution order:** Tasks 1-4a must be done in order — Task 4a must be committed before executing Task 4's filesystem move. Task 5 is independent and can be done in parallel with 1-4a. Tasks 6-9 depend on 1-5 being done. Tasks 7, 8, and 9 are independent of each other once Task 5 and 6 are in place.

**Critical sequencing for Tasks 4 and 4a:** Task 4a (the scanner refactor) must be committed first, because the test suite still passes with `contentRoot == projectRoot`. Once 4a is merged, executing Task 4 (the filesystem move + config update) becomes safe — `Scan` will look for `local/` at the project root, not under `content/`.

---

---

## Phase 3: Kitchen-Sink Examples

### Task 20 — Create kitchen-sink skill

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

**File:** `content/skills/example-kitchen-sink-skill/SKILL.md`

Create an example skill that populates every field in `SkillMeta` (from `cli/internal/converter/skills.go`):

```
content/skills/example-kitchen-sink-skill/
├── .syllago.yaml
├── SKILL.md
└── README.md
```

**`SKILL.md`:**
```markdown
---
name: Kitchen Sink Skill
description: Example skill demonstrating every available field for testing and documentation.
allowed-tools:
  - Read
  - Glob
  - Grep
disallowed-tools:
  - Bash
  - Write
context: fork
agent: Explore
model: claude-opus-4-5
disable-model-invocation: false
user-invocable: true
argument-hint: "<query> [--verbose]"
---

# Kitchen Sink Skill

This is the kitchen-sink example for skills. It populates every metadata field
to validate field coverage and converter round-trips.

## Usage

Invoke with `/kitchen-sink-skill <query>` to trigger this skill explicitly.

## What it does

Demonstrates all skill configuration options in a single example.
```

**`.syllago.yaml`:**
```yaml
id: example-kitchen-sink-skill
name: example-kitchen-sink-skill
description: Kitchen-sink skill example — all fields populated
tags:
  - builtin
  - example
hidden: true
```

**Verify:** `make test` passes; `catalog.Scan` picks up the item; `item.IsExample()` returns true (after Phase 2 adds that method).

---

### Task 21 — Create kitchen-sink agent

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

**File:** `content/agents/example-kitchen-sink-agent/agent.md`

Create an agent that populates every field in `AgentMeta` (from `cli/internal/converter/agents.go`). Note `Background: bool` defaults to false — use `true` to force a non-zero value. Same for Gemini-specific fields `Temperature`, `TimeoutMins`, `Kind`.

```
content/agents/example-kitchen-sink-agent/
├── .syllago.yaml
├── agent.md
└── README.md
```

**`agent.md`:**
```markdown
---
name: Kitchen Sink Agent
description: Example agent demonstrating every available field.
tools:
  - Read
  - Grep
  - Bash
disallowedTools:
  - Write
model: claude-opus-4-5
maxTurns: 10
permissionMode: acceptEdits
skills:
  - code-review
mcpServers:
  - filesystem
memory: project
background: true
isolation: worktree
temperature: 0.7
timeout_mins: 30
kind: remote
---

# Kitchen Sink Agent

This is the kitchen-sink example for agents. Every `AgentMeta` field is populated.

Handles complex multi-step tasks with full access controls configured.
```

**`.syllago.yaml`:**
```yaml
id: example-kitchen-sink-agent
name: example-kitchen-sink-agent
description: Kitchen-sink agent example — all fields populated
tags:
  - builtin
  - example
hidden: true
```

---

### Task 22 — Create kitchen-sink rules

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

Rules are provider-specific (`content/rules/<provider>/`). Create one item per provider that has a rules converter, covering all three `RuleMeta` activation modes: `alwaysApply`, glob-scoped, and description-activated.

```
content/rules/cursor/example-kitchen-sink-rules/
├── .syllago.yaml
├── rule.mdc
└── README.md
```

**`rule.mdc` (Cursor — glob-scoped, demonstrating all three RuleMeta fields):**
```markdown
---
description: Kitchen-sink rule example with all RuleMeta fields populated.
alwaysApply: false
globs:
  - "*.ts"
  - "*.tsx"
---

# Kitchen Sink Rule

This rule demonstrates every `RuleMeta` field: description, alwaysApply (false), and globs.

Apply TypeScript best practices.
```

Create three provider variants to cover all activation modes:

- `content/rules/cursor/example-kitchen-sink-rules/` — glob-scoped (all RuleMeta fields)
- `content/rules/windsurf/example-kitchen-sink-rules/` — always-on (alwaysApply: true)
- `content/rules/claude-code/example-kitchen-sink-rules/` — description-only (model_decision)

Each directory gets its own `.syllago.yaml` with `tags: [builtin, example]` and `hidden: true`.

---

### Task 23 — Create kitchen-sink hooks

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

```
content/hooks/claude-code/example-kitchen-sink-hooks/
├── .syllago.yaml
├── hooks.json
└── README.md
```

**`hooks.json`** must populate every field in `hooksConfig` / `hookEntry` / `hookMatcher`:
- Both `PreToolUse` and `PostToolUse` events (two entries in the top-level map)
- One matcher with a non-empty `Matcher` field, one without (empty matcher = all tools)
- Both `command` and `prompt` hook types
- All `hookEntry` fields: `Type`, `Command`, `Timeout`, `StatusMessage`, `Async`

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Pre-bash check'",
            "timeout": 5000,
            "statusMessage": "Running pre-bash validation",
            "async": false
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Post-tool check'",
            "timeout": 3000,
            "statusMessage": "Running post-tool audit"
          }
        ]
      },
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "prompt",
            "command": "Review the file that was just written for security issues.",
            "timeout": 30000
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST $WEBHOOK_URL -d '{\"text\":\"$NOTIFICATION\"}'",
            "timeout": 5000,
            "statusMessage": "Sending notification"
          }
        ]
      }
    ]
  }
}
```

Note on `Notification`: this is a third event type to ensure event-level coverage beyond just Pre/PostToolUse. The field coverage test for hooks is structure-based (all JSON keys populated), not reflect-based like the Go struct tests.

**`.syllago.yaml`:**
```yaml
id: example-kitchen-sink-hooks
name: example-kitchen-sink-hooks
description: Kitchen-sink hooks example — all event types and hook types
tags:
  - builtin
  - example
hidden: true
```

---

### Task 24 — Create kitchen-sink MCP

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

```
content/mcp/example-kitchen-sink-mcp/
├── .syllago.yaml
├── mcp.json
└── README.md
```

**`mcp.json`** must populate all fields of `mcpServerConfig` across two servers — one stdio, one HTTP — to exercise every field path:

```json
{
  "mcpServers": {
    "kitchen-sink-stdio": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "env": {
        "MCP_API_KEY": "${MCP_API_KEY}",
        "MCP_LOG_LEVEL": "debug"
      },
      "cwd": "/home/user/projects",
      "type": "stdio",
      "autoApprove": ["list_directory", "read_file"],
      "disabled": false
    },
    "kitchen-sink-http": {
      "url": "https://mcp.example.com/v1",
      "headers": {
        "Authorization": "Bearer ${MCP_TOKEN}",
        "X-Client-ID": "syllago"
      },
      "type": "streamable-http",
      "trust": "trusted",
      "includeTools": ["search", "fetch"],
      "excludeTools": ["delete"],
      "disabled": false
    }
  }
}
```

**Why two servers:** `command`/`args`/`env`/`cwd` only appear on stdio; `url`/`headers`/`trust`/`includeTools`/`excludeTools` only appear on HTTP. A single server can't populate all fields without being incoherent. The field coverage test (Task 25) checks across all servers.

---

### Task 25 — Create kitchen-sink prompt

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

```
content/prompts/example-kitchen-sink-prompt/
├── .syllago.yaml
├── PROMPT.md
└── README.md
```

Prompts use `ParseFrontmatterWithBody` during catalog scan. The frontmatter fields come from `catalog/frontmatter.go`. Read what fields `ParseFrontmatterWithBody` extracts and populate all of them.

**`PROMPT.md`:**
```markdown
---
name: Kitchen Sink Prompt
description: Example prompt demonstrating all available frontmatter fields.
providers: [claude-code, gemini-cli, kiro]
---

# Kitchen Sink Prompt

This is a rich example prompt that demonstrates all metadata fields.

## Instructions

You are a {{role}} assistant. Your task is to {{task}}.

## Context

- User's goal: {{goal}}
- Constraints: {{constraints}}

## Output format

Provide your response as structured markdown with clear sections.
```

---

### Task 26 — Create kitchen-sink commands

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

Commands are provider-specific. Create per-provider examples that cover all `CommandMeta` fields and all command formats (markdown with frontmatter, TOML for Gemini, plain markdown for Codex).

```
content/commands/claude-code/example-kitchen-sink-commands/
├── .syllago.yaml
├── command.md
└── README.md

content/commands/gemini-cli/example-kitchen-sink-commands/
├── .syllago.yaml
├── command.toml
└── README.md

content/commands/codex/example-kitchen-sink-commands/
├── .syllago.yaml
├── command.md
└── README.md
```

**Claude Code `command.md`** (all `CommandMeta` fields):
```markdown
---
name: Kitchen Sink Command
description: Example command demonstrating every available field.
allowed-tools:
  - Read
  - Grep
context: fork
agent: Explore
model: claude-opus-4-5
disable-model-invocation: false
user-invocable: true
argument-hint: "<target> [--format json|text]"
---

# Kitchen Sink Command

Execute a comprehensive analysis of $ARGUMENTS.

## Steps

1. Read relevant files
2. Search for patterns
3. Report findings
```

**Gemini CLI `command.toml`:**
```toml
name = "kitchen-sink-command"
description = "Example Gemini CLI command with all fields."
prompt = "Analyze the following: {{args}}\n\nProvide a structured report."
```

---

### Task 27 — Create kitchen-sink app

**Depends on:** Tasks 1-5 (content/ directory structure exists, `.syllago.yaml` has `hidden` field, `example-` prefix convention established)

```
content/apps/example-kitchen-sink-app/
├── .syllago.yaml
├── README.md
└── install.sh
```

The App type uses `ParseFrontmatterWithBody` on `README.md`. All frontmatter fields for apps include `name`, `description`, and `providers`.

**`README.md`:**
```markdown
---
name: Kitchen Sink App
description: Example app demonstrating all app metadata fields and install script.
providers: [claude-code, gemini-cli, cursor, windsurf, kiro]
---

# Kitchen Sink App

A demonstration app that shows every metadata field for the `apps` content type.

## What it does

This app installs example configuration across all supported providers.

## Installation

Run `install.sh` to install. The script accepts these environment variables:

- `SYLLAGO_HOME` — Override the default syllago directory
- `PROVIDER` — Target a specific provider (default: all detected)

## Requirements

- syllago 1.0.0 or later
- At least one supported AI provider installed
```

**`install.sh`:**
```bash
#!/usr/bin/env bash
# Kitchen Sink App installer
# Demonstrates: environment variable reading, provider detection, error handling

set -euo pipefail

SYLLAGO_HOME="${SYLLAGO_HOME:-$HOME/.syllago}"
PROVIDER="${PROVIDER:-all}"

echo "Installing Kitchen Sink App..."
echo "SYLLAGO_HOME: $SYLLAGO_HOME"
echo "Target provider: $PROVIDER"

# Example: create config directory
mkdir -p "$SYLLAGO_HOME/apps/kitchen-sink"
echo "installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$SYLLAGO_HOME/apps/kitchen-sink/state"

echo "Kitchen Sink App installed successfully."
```

---

### Task 28 — Add reflect-based field coverage test

**Depends on:** Tasks 20-27 (all kitchen-sink examples must exist as test fixtures), Tasks 1-5 (content/ directory path is stable)

**File:** `cli/internal/converter/kitchen_sink_coverage_test.go`

This test is a CI guard: it fails if a kitchen-sink example is missing a field from the canonical struct. Uses reflection to enumerate exported fields and checks each is non-zero in the parsed example.

```go
package converter

import (
    "os"
    "path/filepath"
    "reflect"
    "runtime"
    "testing"
)

// TestKitchenSinkFieldCoverage verifies that each kitchen-sink example
// populates every exported field of its canonical metadata struct.
// This is a CI guard against struct additions that go undocumented.
func TestKitchenSinkFieldCoverage(t *testing.T) {
    // Locate the repo root relative to this test file.
    _, thisFile, _, _ := runtime.Caller(0)
    repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
    contentDir := filepath.Join(repoRoot, "content")

    t.Run("SkillMeta", func(t *testing.T) {
        skillPath := filepath.Join(contentDir, "skills",
            "example-kitchen-sink-skill", "SKILL.md")
        content, err := os.ReadFile(skillPath)
        if err != nil {
            t.Fatalf("reading kitchen-sink skill: %v", err)
        }
        meta, _, err := parseSkillCanonical(content)
        if err != nil {
            t.Fatalf("parsing kitchen-sink skill: %v", err)
        }
        assertAllFieldsPopulated(t, meta)
    })

    t.Run("AgentMeta", func(t *testing.T) {
        agentPath := filepath.Join(contentDir, "agents",
            "example-kitchen-sink-agent", "agent.md")
        content, err := os.ReadFile(agentPath)
        if err != nil {
            t.Fatalf("reading kitchen-sink agent: %v", err)
        }
        meta, _, err := parseAgentCanonical(content)
        if err != nil {
            t.Fatalf("parsing kitchen-sink agent: %v", err)
        }
        assertAllFieldsPopulated(t, meta)
    })

    t.Run("RuleMeta", func(t *testing.T) {
        rulePath := filepath.Join(contentDir, "rules", "cursor",
            "example-kitchen-sink-rules", "rule.mdc")
        content, err := os.ReadFile(rulePath)
        if err != nil {
            t.Fatalf("reading kitchen-sink rule: %v", err)
        }
        meta, _, err := parseCanonical(content)
        if err != nil {
            t.Fatalf("parsing kitchen-sink rule: %v", err)
        }
        assertAllFieldsPopulated(t, meta)
    })

    t.Run("CommandMeta", func(t *testing.T) {
        cmdPath := filepath.Join(contentDir, "commands", "claude-code",
            "example-kitchen-sink-commands", "command.md")
        content, err := os.ReadFile(cmdPath)
        if err != nil {
            t.Fatalf("reading kitchen-sink command: %v", err)
        }
        meta, _, err := parseCommandCanonical(content)
        if err != nil {
            t.Fatalf("parsing kitchen-sink command: %v", err)
        }
        assertAllFieldsPopulated(t, meta)
    })
}

// assertAllFieldsPopulated fails if any exported field of v has a zero value.
// Panics if v is not a struct.
func assertAllFieldsPopulated(t *testing.T, v any) {
    t.Helper()
    rv := reflect.ValueOf(v)
    rt := rv.Type()
    for i := 0; i < rt.NumField(); i++ {
        field := rt.Field(i)
        if !field.IsExported() {
            continue
        }
        fv := rv.Field(i)
        if fv.IsZero() {
            t.Errorf("field %s is zero — add it to the kitchen-sink example", field.Name)
        }
    }
}
```

**Why reflect here:** The structs (`SkillMeta`, `AgentMeta`, etc.) evolve as new providers add fields. A reflect-based test catches additions automatically without manual upkeep. The trade-off is that it's slightly magical, but the failure message (`field X is zero`) is clear and actionable.

**Note on `RuleMeta.AlwaysApply` (bool):** A `bool` is zero when `false`. The kitchen-sink rule uses `alwaysApply: false` for the glob-scoped variant, so `AlwaysApply` would be zero. Two options:
1. Test the always-on variant (Windsurf) for `AlwaysApply = true`
2. Skip `AlwaysApply` since `false` is a valid deliberate value

Use option 1: test both variants and skip fields where zero is a valid semantic value by adding a `skipFields` parameter. Initial implementation: skip `AlwaysApply` (its zero value is meaningful) and `Disabled bool` on `mcpServerConfig` (same reason). Add a comment documenting why.

Updated signature:
```go
func assertAllFieldsPopulated(t *testing.T, v any, skipFields ...string) {
    t.Helper()
    skip := map[string]bool{}
    for _, f := range skipFields {
        skip[f] = true
    }
    // ... same loop, check skip[field.Name]
}
```

---

### Task 29 — Add round-trip tests for skills and agents

**Depends on:** Tasks 20-21 (kitchen-sink skill and agent examples must exist), Task 28 (field coverage test confirms the kitchen-sink fixtures are complete — avoids testing incomplete fixtures)

**File:** `cli/internal/converter/kitchen_sink_roundtrip_test.go`

Round-trip tests verify: canonicalize(render(canonicalize(input))) produces no unexpected additional data loss compared to the first canonicalize pass. They use the kitchen-sink examples as canonical starting points.

```go
package converter

import (
    "os"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestKitchenSinkSkillRoundTrip(t *testing.T) {
    _, thisFile, _, _ := runtime.Caller(0)
    repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
    skillPath := filepath.Join(repoRoot, "content", "skills",
        "example-kitchen-sink-skill", "SKILL.md")

    original, err := os.ReadFile(skillPath)
    if err != nil {
        t.Fatalf("reading kitchen-sink skill: %v", err)
    }

    conv := &SkillsConverter{}

    // Step 1: canonicalize the original (already canonical format)
    canonical1, err := conv.Canonicalize(original, "claude-code")
    if err != nil {
        t.Fatalf("first canonicalize: %v", err)
    }
    meta1, body1, err := parseSkillCanonical(canonical1.Content)
    if err != nil {
        t.Fatalf("parsing canonical1: %v", err)
    }

    roundTripTargets := []struct {
        name     string
        target   provider.Provider
        src      string
        // wantWarnCount is the minimum expected warnings (data loss is expected
        // for providers that can't represent all fields structurally).
        wantWarnCount int
    }{
        {"claude-code", provider.ClaudeCode, "claude-code", 0},
        {"gemini-cli", provider.GeminiCLI, "gemini-cli", 0},
        {"kiro", provider.Kiro, "kiro", 0},
        {"opencode", provider.OpenCode, "opencode", 0},
    }

    for _, tt := range roundTripTargets {
        t.Run(tt.name, func(t *testing.T) {
            // Render to target format
            rendered, err := conv.Render(canonical1.Content, tt.target)
            if err != nil {
                t.Fatalf("render to %s: %v", tt.name, err)
            }

            // Canonicalize back
            canonical2, err := conv.Canonicalize(rendered.Content, tt.src)
            if err != nil {
                t.Fatalf("second canonicalize from %s: %v", tt.name, err)
            }
            meta2, body2, err := parseSkillCanonical(canonical2.Content)
            if err != nil {
                t.Fatalf("parsing canonical2: %v", err)
            }

            // The core fields that survive all round-trips must be preserved.
            if meta2.Name != meta1.Name {
                t.Errorf("Name: got %q, want %q", meta2.Name, meta1.Name)
            }
            if meta2.Description != meta1.Description {
                t.Errorf("Description: got %q, want %q", meta2.Description, meta1.Description)
            }
            if body2 == "" && body1 != "" {
                t.Errorf("body lost during round-trip to %s", tt.name)
            }

            // Providers that drop fields must emit warnings for each dropped field.
            if len(rendered.Warnings) < tt.wantWarnCount {
                t.Errorf("expected >= %d warnings for %s, got %d: %v",
                    tt.wantWarnCount, tt.name, len(rendered.Warnings), rendered.Warnings)
            }
        })
    }
}

func TestKitchenSinkAgentRoundTrip(t *testing.T) {
    _, thisFile, _, _ := runtime.Caller(0)
    repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
    agentPath := filepath.Join(repoRoot, "content", "agents",
        "example-kitchen-sink-agent", "agent.md")

    original, err := os.ReadFile(agentPath)
    if err != nil {
        t.Fatalf("reading kitchen-sink agent: %v", err)
    }

    conv := &AgentsConverter{}

    canonical1, err := conv.Canonicalize(original, "claude-code")
    if err != nil {
        t.Fatalf("first canonicalize: %v", err)
    }
    meta1, body1, err := parseAgentCanonical(canonical1.Content)
    if err != nil {
        t.Fatalf("parsing canonical1: %v", err)
    }

    targets := []struct {
        name          string
        target        provider.Provider
        src           string
        wantWarnCount int
    }{
        {"claude-code", provider.ClaudeCode, "claude-code", 0},
        {"gemini-cli", provider.GeminiCLI, "gemini-cli", 0},
        // roo-code drops many fields — must warn for each
        {"roo-code", provider.RooCode, "roo-code", 5},
        {"kiro", provider.Kiro, "kiro", 2},
        {"opencode", provider.OpenCode, "opencode", 1},
    }

    for _, tt := range targets {
        t.Run(tt.name, func(t *testing.T) {
            rendered, err := conv.Render(canonical1.Content, tt.target)
            if err != nil {
                t.Fatalf("render to %s: %v", tt.name, err)
            }

            if len(rendered.Warnings) < tt.wantWarnCount {
                t.Errorf("expected >= %d warnings for %s, got %d: %v",
                    tt.wantWarnCount, tt.name, len(rendered.Warnings), rendered.Warnings)
            }

            canonical2, err := conv.Canonicalize(rendered.Content, tt.src)
            if err != nil {
                t.Fatalf("second canonicalize from %s: %v", tt.name, err)
            }
            meta2, body2, err := parseAgentCanonical(canonical2.Content)
            if err != nil {
                t.Fatalf("parsing canonical2: %v", err)
            }

            if meta2.Name != meta1.Name {
                t.Errorf("Name: got %q, want %q", meta2.Name, meta1.Name)
            }
            if meta2.Description != meta1.Description {
                t.Errorf("Description: got %q, want %q", meta2.Description, meta1.Description)
            }
            if body2 == "" && body1 != "" {
                t.Errorf("body lost during round-trip to %s", tt.name)
            }
        })
    }
}
```

**Why these tests:** They serve as regression guards. When a converter is modified (e.g., a new field added, a provider rendering updated), the round-trip test catches unintended data loss that isn't already exercised by `field_preservation_test.go`. The `wantWarnCount` thresholds document expected lossy conversions explicitly — if a count drops, it means either a bug introduced silent loss, or a converter was improved (in which case update the threshold).

**Note on `provider.RooCode` etc.:** These variable names exist in `cli/internal/provider/` already (the package declares `var RooCode Provider` etc.). Confirm by checking `provider/roocode.go` if needed.

---

## Phase 4: CLI Fixes

### Task 30 — Fix import: write canonicalized content to `local/`

**Depends on:** Tasks 1-4 (content/ directory exists and `local/` is the target write path), Task 2 (resolveContentRoot is available for finding the repo root)

**File:** `cli/cmd/syllago/import.go`

**Current behavior:** After parsing, `runImport` builds a `parse.ImportResult` with `Sections` but does nothing with them — no files are written.

**Fix:** After parsing, for each section, write the canonicalized content file to `local/<type>/[<provider>/]<name>/` and create `.syllago.yaml` with import metadata. Rename the existing `--preview` flag logic to a new `--dry-run` flag for clarity (keep `--preview` as an alias or replace — check existing tests first).

The implementation needs two new capabilities:

1. **Write content from a `model.Section`** — sections come from `parse.ParseFile` and are `model.TextSection`, etc. But the *content to write* is the raw file parsed, not the section. The parsed sections are Claude Code's project analysis format (tech stack, dependencies, etc.), not the AI tool content. Looking at `import.go` more carefully: `parser.ParseFile(file)` returns `model.Section` slices, but for this import path (importing AI tool content like rules/skills/hooks), each `DiscoveredFile` *is* the content file. The write step should: read the raw file content, run it through the converter's `Canonicalize`, and write the output.

2. **Determine destination path** — universal types: `local/<type>/<name>/`; provider-specific types: `local/<type>/<provider>/<name>/`. The item name derives from the source file's base name or directory name.

**New flag:** Add `--dry-run` alongside (or replacing) `--preview`. `--dry-run` shows what *would* be written without writing. Keep `--preview` as a discovery-only mode (current behavior, no parsing). `--dry-run` parses and shows what would be written.

```go
func init() {
    importCmd.Flags().String("from", "", "Provider to import from (required)")
    importCmd.MarkFlagRequired("from")
    importCmd.Flags().String("type", "", "Limit to a single content type")
    importCmd.Flags().String("name", "", "Filter by name (substring, case-insensitive)")
    importCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
    importCmd.Flags().Bool("dry-run", false, "Parse and show what would be written, without writing")
    rootCmd.AddCommand(importCmd)
}
```

**Write logic** (new `writeImportedContent` helper):
```go
// writeImportedContent canonicalizes and writes a single discovered file to local/.
// Returns the destination directory path and any warnings.
func writeImportedContent(
    file parse.DiscoveredFile,
    prov provider.Provider,
    repoRoot string,
    dryRun bool,
) (dest string, warnings []string, err error) {
    content, err := os.ReadFile(file.Path)
    if err != nil {
        return "", nil, err
    }

    // Canonicalize if a converter exists
    var canonical []byte
    var canonFilename string
    conv := converter.For(file.ContentType)
    if conv != nil {
        result, err := conv.Canonicalize(content, prov.Slug)
        if err != nil {
            return "", nil, fmt.Errorf("canonicalize %s: %w", file.Path, err)
        }
        canonical = result.Content
        canonFilename = result.Filename
        warnings = result.Warnings
    } else {
        canonical = content
        canonFilename = filepath.Base(file.Path)
    }

    // Determine destination directory
    itemName := itemNameFromPath(file.Path, file.ContentType)
    var destDir string
    if file.ContentType.IsUniversal() {
        destDir = filepath.Join(repoRoot, "local", string(file.ContentType), itemName)
    } else {
        destDir = filepath.Join(repoRoot, "local", string(file.ContentType), prov.Slug, itemName)
    }

    dest = filepath.Join(destDir, canonFilename)

    if dryRun {
        return dest, warnings, nil
    }

    if err := os.MkdirAll(destDir, 0755); err != nil {
        return "", nil, fmt.Errorf("creating %s: %w", destDir, err)
    }
    if err := os.WriteFile(dest, canonical, 0644); err != nil {
        return "", nil, fmt.Errorf("writing %s: %w", dest, err)
    }

    // Write .syllago.yaml
    now := time.Now()
    m := &metadata.Meta{
        ID:             metadata.NewID(),
        Name:           itemName,
        SourceProvider: prov.Slug,
        SourceFormat:   strings.TrimPrefix(filepath.Ext(file.Path), "."),
        ImportedAt:     &now,
    }
    if err := metadata.Save(destDir, m); err != nil {
        return dest, warnings, fmt.Errorf("writing metadata: %w", err)
    }

    return dest, warnings, nil
}

// itemNameFromPath derives a canonical item name from the source file path.
// For directory-based items: the parent directory name.
// For single files: the base filename without extension.
func itemNameFromPath(path string, ct catalog.ContentType) string {
    base := filepath.Base(path)
    // Check if parent dir looks like an item (not a provider/type dir)
    parent := filepath.Base(filepath.Dir(path))
    if parent != string(ct) && parent != "." {
        return parent
    }
    // Strip extension
    return strings.TrimSuffix(base, filepath.Ext(base))
}
```

**Updated `runImport` flow:**
```go
// After parsing sections:
dryRun, _ := cmd.Flags().GetBool("dry-run")

var written, skipped int
for _, file := range report.Files {
    dest, warns, err := writeImportedContent(file, *prov, root, dryRun)
    if err != nil {
        fmt.Fprintf(output.ErrWriter, "  skipped %s: %v\n", filepath.Base(file.Path), err)
        skipped++
        continue
    }
    for _, w := range warns {
        fmt.Fprintf(output.ErrWriter, "  warning: %s\n", w)
    }
    if dryRun {
        fmt.Printf("  would write: %s\n", dest)
    } else {
        fmt.Printf("  imported: %s\n", dest)
    }
    written++
}

if dryRun {
    fmt.Printf("\nDry run: would import %d items.\n", written)
} else {
    fmt.Printf("\nImported %d items", written)
    if skipped > 0 {
        fmt.Printf(", %d skipped", skipped)
    }
    fmt.Println(".")
}
```

**Test:** `cli/cmd/syllago/import_test.go`

Add `TestImportWritesToLocal` and `TestImportDryRunDoesNotWrite`:

```go
func TestImportWritesToLocal(t *testing.T) {
    tmp := setupImportProject(t)  // already has claude-code rules

    origRoot := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    t.Cleanup(func() { findProjectRoot = origRoot })

    // Also need findSkillsDir to point to tmp for findContentRepoRoot
    origSkills := findSkillsDir
    findSkillsDir = func(dir string) (string, error) { return tmp, nil }
    t.Cleanup(func() { findSkillsDir = origSkills })

    output.SetForTest(t)

    importCmd.Flags().Set("from", "claude-code")
    defer importCmd.Flags().Set("from", "")
    importCmd.Flags().Set("type", "rules")
    defer importCmd.Flags().Set("type", "")

    if err := importCmd.RunE(importCmd, []string{}); err != nil {
        t.Fatalf("import failed: %v", err)
    }

    // Verify that files were written to local/rules/claude-code/<name>/
    entries, err := os.ReadDir(filepath.Join(tmp, "local", "rules", "claude-code"))
    if err != nil {
        t.Fatalf("local/rules/claude-code not created: %v", err)
    }
    if len(entries) == 0 {
        t.Error("expected items in local/rules/claude-code, got none")
    }

    // Verify .syllago.yaml was written
    for _, e := range entries {
        metaPath := filepath.Join(tmp, "local", "rules", "claude-code", e.Name(), ".syllago.yaml")
        if _, err := os.Stat(metaPath); err != nil {
            t.Errorf("expected .syllago.yaml in %s, got error: %v", e.Name(), err)
        }
    }
}

func TestImportDryRunDoesNotWrite(t *testing.T) {
    tmp := setupImportProject(t)

    origRoot := findProjectRoot
    findProjectRoot = func() (string, error) { return tmp, nil }
    t.Cleanup(func() { findProjectRoot = origRoot })

    origSkills := findSkillsDir
    findSkillsDir = func(dir string) (string, error) { return tmp, nil }
    t.Cleanup(func() { findSkillsDir = origSkills })

    stdout, _ := output.SetForTest(t)

    importCmd.Flags().Set("from", "claude-code")
    defer importCmd.Flags().Set("from", "")
    importCmd.Flags().Set("dry-run", "true")
    defer importCmd.Flags().Set("dry-run", "false")

    if err := importCmd.RunE(importCmd, []string{}); err != nil {
        t.Fatalf("import --dry-run failed: %v", err)
    }

    // local/ should not have been created
    if _, err := os.Stat(filepath.Join(tmp, "local")); err == nil {
        t.Error("--dry-run should not create local/ directory")
    }

    // Output should mention "would write"
    if !strings.Contains(stdout.String(), "would write") {
        t.Errorf("expected 'would write' in output, got: %s", stdout.String())
    }
}
```

---

### Task 31 — Fix export: support all sources, add `--source` flag

**Depends on:** Tasks 1-4 (content/ and local/ directory structure exists so catalog scan works correctly), Task 5 (hidden field on Meta — IsBuiltin() may check hidden+builtin tags)

**File:** `cli/cmd/syllago/export.go`

**Current bug (line 100):** `if !item.Local { continue }` — this skips all shared, registry, and builtin content.

**Fix:**

1. Remove the `item.Local` guard.
2. Replace `catalog.Scan(root)` with `catalog.ScanWithRegistries(root, regSources)` — load config to get registry list, same as `runTUI`.
3. Add `--source` flag: `local` | `shared` | `registry` | `builtin` | `all` (default: `local` to preserve existing behavior).
4. Add a confirmation prompt when `--source` includes built-in content (export of builtin items is unusual and potentially confusing).

**Why keep `local` as default:** Changing the default from "local only" to "all" would be a breaking behavior change for existing users. Making `all` opt-in via `--source all` is safer.

**New flag and filter logic:**

```go
func init() {
    exportCmd.Flags().String("to", "", "Provider slug to export to (required)")
    exportCmd.MarkFlagRequired("to")
    exportCmd.Flags().String("type", "", "Filter to a specific content type")
    exportCmd.Flags().String("name", "", "Filter by item name (substring match)")
    exportCmd.Flags().String("llm-hooks", "skip", "How to handle LLM-evaluated hooks: skip or generate")
    exportCmd.Flags().String("source", "local", "Source filter: local, shared, registry, builtin, all")
    rootCmd.AddCommand(exportCmd)
}
```

**Item filtering:**

```go
// filterBySource returns true if item should be included for the given source filter.
func filterBySource(item catalog.ContentItem, source string) bool {
    switch source {
    case "local":
        return item.Local
    case "shared":
        return !item.Local && item.Registry == "" && !item.IsBuiltin()
    case "registry":
        return item.Registry != ""
    case "builtin":
        return item.IsBuiltin()
    case "all":
        return true
    default:
        return item.Local
    }
}
```

Replace the existing filter block:
```go
// Old:
for _, item := range cat.Items {
    if !item.Local { continue }
    ...
}

// New:
source, _ := cmd.Flags().GetString("source")
for _, item := range cat.Items {
    if !filterBySource(item, source) { continue }
    ...
}
```

**Registry loading** (add before the scan):
```go
cfg, _ := config.Load(root)  // non-fatal; empty config = no registries
var regSources []catalog.RegistrySource
if cfg != nil {
    for _, r := range cfg.Registries {
        if registry.IsCloned(r.Name) {
            dir, _ := registry.CloneDir(r.Name)
            regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
        }
    }
}
cat, err := catalog.ScanWithRegistries(root, regSources)
```

**Builtin warning prompt:**
```go
if source == "builtin" || source == "all" {
    hasBuiltin := false
    for _, item := range items {
        if item.IsBuiltin() {
            hasBuiltin = true
            break
        }
    }
    if hasBuiltin && isInteractive() {
        fmt.Fprintf(output.ErrWriter, "Warning: you are exporting built-in content.\n")
        fmt.Fprintf(output.ErrWriter, "Built-in items are provided by syllago and may conflict with provider defaults.\n")
        fmt.Fprintf(output.ErrWriter, "Continue? [y/N] ")
        var resp string
        fmt.Scanln(&resp)
        if strings.ToLower(resp) != "y" {
            fmt.Fprintln(output.ErrWriter, "Export cancelled.")
            return nil
        }
    }
}
```

**Test:** `cli/cmd/syllago/export_test.go`

Add `TestExportSourceFilter` and `TestExportSharedSource`:

```go
func TestExportSourceFilter(t *testing.T) {
    root := setupExportRepo(t)
    withFakeRepoRoot(t, root)

    // Add a shared (non-local) skill
    sharedSkillDir := filepath.Join(root, "skills", "shared-skill")
    os.MkdirAll(sharedSkillDir, 0755)
    os.WriteFile(filepath.Join(sharedSkillDir, "SKILL.md"),
        []byte("# Shared Skill\nA shared skill.\n"), 0644)

    installBase := t.TempDir()
    addTestProvider(t, "test-provider", "Test Provider", installBase)

    // Default (--source local) should only export local items
    stdout, _ := output.SetForTest(t)
    exportCmd.Flags().Set("to", "test-provider")
    defer exportCmd.Flags().Set("to", "")
    exportCmd.Flags().Set("type", "skills")
    defer exportCmd.Flags().Set("type", "")

    if err := exportCmd.RunE(exportCmd, []string{}); err != nil {
        t.Fatalf("export failed: %v", err)
    }
    if strings.Contains(stdout.String(), "shared-skill") {
        t.Error("default --source local should not export shared-skill")
    }
}

func TestExportSharedSource(t *testing.T) {
    root := setupExportRepo(t)
    withFakeRepoRoot(t, root)

    // Add a shared (non-local) skill
    sharedSkillDir := filepath.Join(root, "skills", "shared-skill")
    os.MkdirAll(sharedSkillDir, 0755)
    os.WriteFile(filepath.Join(sharedSkillDir, "SKILL.md"),
        []byte("---\nname: shared-skill\ndescription: Shared.\n---\n\nShared skill.\n"), 0644)

    installBase := t.TempDir()
    addTestProvider(t, "test-provider", "Test Provider", installBase)

    stdout, _ := output.SetForTest(t)
    exportCmd.Flags().Set("to", "test-provider")
    defer exportCmd.Flags().Set("to", "")
    exportCmd.Flags().Set("type", "skills")
    defer exportCmd.Flags().Set("type", "")
    exportCmd.Flags().Set("source", "shared")
    defer exportCmd.Flags().Set("source", "local")

    if err := exportCmd.RunE(exportCmd, []string{}); err != nil {
        t.Fatalf("export --source shared failed: %v", err)
    }
    out := stdout.String()
    if !strings.Contains(out, "shared-skill") {
        t.Errorf("expected shared-skill in output, got: %s", out)
    }
    // Should not include local items
    if strings.Contains(out, "greeting") {
        t.Error("--source shared should not export local item 'greeting'")
    }
}
```

---

### Task 32a — Add `CreatedAt` field to `Meta` struct

**Depends on:** Task 5 (`hidden` field was added to `Meta` in Phase 2, establishing the pattern for adding fields)

**File:** `cli/internal/metadata/metadata.go`

**Decision:** Add a dedicated `CreatedAt *time.Time` field to `Meta`. This is semantically distinct from `ImportedAt` — `ImportedAt` records when content was imported from a provider (import provenance), while `CreatedAt` records when content was originally authored/scaffolded. A created item was never imported, so `ImportedAt` would be a semantic lie.

**Change to `Meta` struct:**

```go
// Meta holds metadata for a single content item.
type Meta struct {
    ID             string       `yaml:"id"`
    Name           string       `yaml:"name"`
    Description    string       `yaml:"description,omitempty"`
    Version        string       `yaml:"version,omitempty"`
    Type           string       `yaml:"type,omitempty"`
    Author         string       `yaml:"author,omitempty"`
    Source         string       `yaml:"source,omitempty"`
    Tags           []string     `yaml:"tags,omitempty"`
    Dependencies   []Dependency `yaml:"dependencies,omitempty"`
    CreatedAt      *time.Time   `yaml:"created_at,omitempty"`   // when item was scaffolded via syllago create
    ImportedAt     *time.Time   `yaml:"imported_at,omitempty"`  // when item was imported from a provider
    ImportedBy     string       `yaml:"imported_by,omitempty"`
    PromotedAt     *time.Time   `yaml:"promoted_at,omitempty"`
    PRBranch       string       `yaml:"pr_branch,omitempty"`
    SourceProvider string       `yaml:"source_provider,omitempty"` // provider slug content was imported from
    SourceFormat   string       `yaml:"source_format,omitempty"`   // original file extension (e.g. "mdc", "md")
}
```

**Test:** Add to `cli/internal/metadata/metadata_test.go`:

```go
func TestMetaCreatedAt(t *testing.T) {
    dir := t.TempDir()
    now := time.Now().UTC().Truncate(time.Second)
    m := &Meta{
        ID:        NewID(),
        Name:      "test",
        CreatedAt: &now,
    }
    if err := Save(dir, m); err != nil {
        t.Fatalf("Save: %v", err)
    }
    loaded, err := Load(dir)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if loaded.CreatedAt == nil {
        t.Fatal("CreatedAt was not persisted")
    }
    if !loaded.CreatedAt.Equal(now) {
        t.Errorf("CreatedAt: got %v, want %v", *loaded.CreatedAt, now)
    }
    // ImportedAt should be nil — CreatedAt and ImportedAt are independent
    if loaded.ImportedAt != nil {
        t.Error("ImportedAt should be nil for a created (not imported) item")
    }
}
```

---

### Task 32b — Implement `syllago create` command registration and template copy

**Depends on:** Tasks 1-4 (content/ and local/ directory structure established), Task 2 (resolveContentRoot available)

**File:** `cli/cmd/syllago/create.go` (new file)

This task covers command registration, argument validation, directory creation, and template file copying. Metadata generation is in Task 32c.

```go
// cli/cmd/syllago/create.go
package main

import (
    "fmt"
    "io"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
    Use:   "create <type> <name>",
    Short: "Scaffold a new content item in local/",
    Long: `Creates a new content item in local/ from the template for the given type.

Examples:
  syllago create skill my-review-skill
  syllago create agent my-helper
  syllago create rule --provider cursor my-cursor-rule
  syllago create prompt my-analysis-prompt
  syllago create mcp my-server
  syllago create hooks --provider claude-code my-hooks
  syllago create app my-app

The new item is placed in local/<type>/[<provider>/]<name>/ and populated
with template files. Edit the generated files to customize your content.`,
    Args: cobra.ExactArgs(2),
    RunE: runCreate,
}

func init() {
    createCmd.Flags().String("provider", "", "Provider slug (required for rules, hooks, commands)")
    rootCmd.AddCommand(createCmd)
}

// validateCreateArgs checks that typeName is a known content type and that
// --provider is supplied for provider-specific types. Returns the resolved
// ContentType or an error.
func validateCreateArgs(typeName, providerSlug string) (catalog.ContentType, error) {
    if !catalog.IsValidItemName(typeName) {
        return "", fmt.Errorf("unknown content type %q; valid types: skills, agents, rules, hooks, mcp, prompts, commands, apps", typeName)
    }
    ct := catalog.ContentType(typeName)
    valid := false
    for _, vt := range catalog.AllContentTypes() {
        if vt == ct {
            valid = true
            break
        }
    }
    if !valid {
        return "", fmt.Errorf("unknown content type %q; valid types: skills, agents, rules, hooks, mcp, prompts, commands, apps", typeName)
    }
    if !ct.IsUniversal() && providerSlug == "" {
        return "", fmt.Errorf("%s is a provider-specific type; use --provider <slug>", typeName)
    }
    return ct, nil
}

// destDirForCreate returns the destination directory for a new item.
func destDirForCreate(root, typeName, providerSlug, itemName string, ct catalog.ContentType) string {
    if ct.IsUniversal() {
        return filepath.Join(root, "local", typeName, itemName)
    }
    return filepath.Join(root, "local", typeName, providerSlug, itemName)
}

// scaffoldFromTemplate copies template files to destDir, or creates an empty
// directory if no template exists for the type.
func scaffoldFromTemplate(root, typeName, destDir string) error {
    templateDir := filepath.Join(root, "templates", typeName)
    if _, err := os.Stat(templateDir); err != nil {
        // No template for this type — create an empty directory; .syllago.yaml added by caller
        return os.MkdirAll(destDir, 0755)
    }
    return copyDir(templateDir, destDir)
}

// copyDir recursively copies src into dst.
func copyDir(src, dst string) error {
    return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }
        rel, err := filepath.Rel(src, path)
        if err != nil {
            return err
        }
        target := filepath.Join(dst, rel)
        if d.IsDir() {
            return os.MkdirAll(target, 0755)
        }
        return copyFile(path, target)
    })
}

func copyFile(src, dst string) error {
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return err
    }
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()
    _, err = io.Copy(out, in)
    return err
}
```

**Test (scaffold logic only):** `cli/cmd/syllago/create_test.go`

```go
func TestCreateSkill(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)
    os.MkdirAll(filepath.Join(root, "templates", "skills"), 0755)
    os.WriteFile(filepath.Join(root, "templates", "skills", "SKILL.md"),
        []byte("---\nname: Skill Name\ndescription: \n---\n\n# Skill\n"), 0644)

    withFakeRepoRoot(t, root)
    output.SetForTest(t)

    createCmd.Flags().Set("provider", "")
    if err := createCmd.RunE(createCmd, []string{"skills", "my-test-skill"}); err != nil {
        t.Fatalf("create failed: %v", err)
    }

    destDir := filepath.Join(root, "local", "skills", "my-test-skill")
    if _, err := os.Stat(destDir); err != nil {
        t.Fatalf("expected destDir %s to exist: %v", destDir, err)
    }
    if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
        t.Error("expected SKILL.md to be created from template")
    }
}

func TestCreateRuleRequiresProvider(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)
    withFakeRepoRoot(t, root)
    output.SetForTest(t)

    createCmd.Flags().Set("provider", "")
    err := createCmd.RunE(createCmd, []string{"rules", "my-rule"})
    if err == nil {
        t.Error("create rules without --provider should fail")
    }
}

func TestCreateFailsIfExists(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)
    existing := filepath.Join(root, "local", "skills", "existing")
    os.MkdirAll(existing, 0755)

    withFakeRepoRoot(t, root)
    output.SetForTest(t)

    createCmd.Flags().Set("provider", "")
    err := createCmd.RunE(createCmd, []string{"skills", "existing"})
    if err == nil {
        t.Error("create should fail if item already exists")
    }
}

func TestCreateWithProvider(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)
    os.MkdirAll(filepath.Join(root, "templates", "rules"), 0755)
    os.WriteFile(filepath.Join(root, "templates", "rules", "rule.md"),
        []byte("# Rule\n\nRule content.\n"), 0644)

    withFakeRepoRoot(t, root)
    output.SetForTest(t)

    createCmd.Flags().Set("provider", "cursor")
    defer createCmd.Flags().Set("provider", "")
    if err := createCmd.RunE(createCmd, []string{"rules", "my-cursor-rule"}); err != nil {
        t.Fatalf("create rule failed: %v", err)
    }

    destDir := filepath.Join(root, "local", "rules", "cursor", "my-cursor-rule")
    if _, err := os.Stat(destDir); err != nil {
        t.Fatalf("expected %s to exist: %v", destDir, err)
    }
}
```

---

### Task 32c — Wire metadata generation into `syllago create` and complete `runCreate`

**Depends on:** Task 32a (`CreatedAt` field exists on `Meta`), Task 32b (scaffold helpers exist)

This task completes `runCreate` by adding the `.syllago.yaml` write step (using `CreatedAt`, not `ImportedAt`) and wiring the helpers from Task 32b into a full working command.

**Add to `cli/cmd/syllago/create.go`** (the `runCreate` function body and output):

```go
func runCreate(cmd *cobra.Command, args []string) error {
    typeName := args[0]
    itemName := args[1]

    if !catalog.IsValidItemName(itemName) {
        return fmt.Errorf("invalid item name %q: use only letters, numbers, hyphens, underscores", itemName)
    }

    providerSlug, _ := cmd.Flags().GetString("provider")
    ct, err := validateCreateArgs(typeName, providerSlug)
    if err != nil {
        return err
    }

    root, err := findContentRepoRoot()
    if err != nil {
        return err
    }

    destDir := destDirForCreate(root, typeName, providerSlug, itemName, ct)
    if _, err := os.Stat(destDir); err == nil {
        return fmt.Errorf("item already exists at %s", destDir)
    }

    if err := scaffoldFromTemplate(root, typeName, destDir); err != nil {
        return fmt.Errorf("copying template: %w", err)
    }

    // Write .syllago.yaml with CreatedAt (not ImportedAt — this was not imported)
    now := time.Now()
    m := &metadata.Meta{
        ID:        metadata.NewID(),
        Name:      itemName,
        CreatedAt: &now,
    }
    if err := metadata.Save(destDir, m); err != nil {
        return fmt.Errorf("writing metadata: %w", err)
    }

    if !output.JSON {
        fmt.Fprintf(output.Writer, "Created %s %q at %s\n", typeName, itemName, destDir)
        fmt.Fprintf(output.Writer, "Edit the files in %s to customize your content.\n", destDir)
    } else {
        output.Print(map[string]string{
            "type": typeName,
            "name": itemName,
            "path": destDir,
        })
    }
    return nil
}
```

**Test (metadata write):** Add to `cli/cmd/syllago/create_test.go`:

```go
func TestCreateWritesSyllagoYaml(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)

    withFakeRepoRoot(t, root)
    output.SetForTest(t)

    createCmd.Flags().Set("provider", "")
    if err := createCmd.RunE(createCmd, []string{"skills", "my-test-skill"}); err != nil {
        t.Fatalf("create failed: %v", err)
    }

    metaPath := filepath.Join(root, "local", "skills", "my-test-skill", ".syllago.yaml")
    if _, err := os.Stat(metaPath); err != nil {
        t.Fatalf("expected .syllago.yaml to be created: %v", err)
    }

    m, err := metadata.Load(filepath.Join(root, "local", "skills", "my-test-skill"))
    if err != nil {
        t.Fatalf("loading metadata: %v", err)
    }
    if m.Name != "my-test-skill" {
        t.Errorf("Name: got %q, want %q", m.Name, "my-test-skill")
    }
    if m.CreatedAt == nil {
        t.Error("CreatedAt should be set on created items")
    }
    if m.ImportedAt != nil {
        t.Error("ImportedAt should NOT be set on created items (use CreatedAt instead)")
    }
}
```

---

### Task 33 — Add `syllago list` command

**Depends on:** Task 31 (`filterBySource` helper is defined in `export.go` and reused here; Task 31 must be completed first so both files share the same function), Tasks 1-4 (content/ and local/ directory structure is stable for the catalog scan)

**File:** `cli/cmd/syllago/list.go`

This is a new file. `syllago list` provides a quick CLI inventory without the TUI.

```bash
syllago list                      # All content (all sources), grouped by type
syllago list --source local       # Only local items
syllago list --type skills        # Only skills
syllago list --source registry    # Only registry items
syllago list --json               # JSON output (uses --json global flag)
```

**Output format (human-readable):**
```
Skills (3)
  greeting          [local]  Says hello to the user.
  code-review       [shared] Comprehensive code review skill.
  syllago-guide       [builtin][hidden] Complete reference for using syllago.

Agents (1)
  code-reviewer     [shared] Automated code review agent.
```

**Implementation:**

```go
// cli/cmd/syllago/list.go
package main

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/config"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
    "github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List content items",
    Long: `Lists content items from local/, shared, registries, and built-ins.

Examples:
  syllago list                      List all content
  syllago list --source local       Only local items
  syllago list --type skills        Only skills
  syllago list --source registry    Only registry items
  syllago list --json               JSON output`,
    RunE: runList,
}

func init() {
    listCmd.Flags().String("source", "all", "Source filter: local, shared, registry, builtin, all")
    listCmd.Flags().String("type", "", "Filter to a single content type")
    rootCmd.AddCommand(listCmd)
}

type listItem struct {
    Name        string `json:"name"`
    Type        string `json:"type"`
    Source      string `json:"source"`
    Description string `json:"description,omitempty"`
    Hidden      bool   `json:"hidden,omitempty"`
    Provider    string `json:"provider,omitempty"`
    Registry    string `json:"registry,omitempty"`
}

func runList(cmd *cobra.Command, args []string) error {
    root, err := findContentRepoRoot()
    if err != nil {
        return fmt.Errorf("could not find syllago repo: %w", err)
    }

    sourceFilter, _ := cmd.Flags().GetString("source")
    typeFilter, _ := cmd.Flags().GetString("type")

    cfg, _ := config.Load(root)
    var regSources []catalog.RegistrySource
    if cfg != nil {
        for _, r := range cfg.Registries {
            if registry.IsCloned(r.Name) {
                dir, _ := registry.CloneDir(r.Name)
                regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
            }
        }
    }
    _ = context.WithTimeout // suppress unused import; registry sync is not done here
    _ = time.Second

    cat, err := catalog.ScanWithRegistries(root, regSources)
    if err != nil {
        return fmt.Errorf("scanning catalog: %w", err)
    }

    // Collect and filter items
    var items []listItem
    for _, item := range cat.Items {
        if !filterBySource(item, sourceFilter) {
            continue
        }
        if typeFilter != "" && string(item.Type) != typeFilter {
            continue
        }

        src := itemSourceLabel(item)
        li := listItem{
            Name:        item.Name,
            Type:        string(item.Type),
            Source:      src,
            Description: item.Description,
            Provider:    item.Provider,
            Registry:    item.Registry,
        }
        if item.Meta != nil {
            li.Hidden = item.Meta.Hidden
        }
        items = append(items, li)
    }

    if output.JSON {
        output.Print(items)
        return nil
    }

    if len(items) == 0 {
        fmt.Fprintln(output.Writer, "No items found.")
        return nil
    }

    // Group by type, print grouped
    byType := make(map[catalog.ContentType][]listItem)
    for _, item := range items {
        ct := catalog.ContentType(item.Type)
        byType[ct] = append(byType[ct], item)
    }

    for _, ct := range catalog.AllContentTypes() {
        group := byType[ct]
        if len(group) == 0 {
            continue
        }
        fmt.Fprintf(output.Writer, "\n%s (%d)\n", ct.Label(), len(group))
        for _, item := range group {
            badges := "[" + item.Source + "]"
            if item.Hidden {
                badges += "[hidden]"
            }
            desc := item.Description
            if desc == "" {
                desc = "-"
            }
            if len(desc) > 60 {
                desc = desc[:57] + "..."
            }
            fmt.Fprintf(output.Writer, "  %-30s %-20s %s\n", item.Name, badges, desc)
        }
    }
    fmt.Fprintln(output.Writer)

    return nil
}

// itemSourceLabel returns a display label for where an item comes from.
func itemSourceLabel(item catalog.ContentItem) string {
    if item.IsBuiltin() {
        return "builtin"
    }
    if item.Local {
        return "local"
    }
    if item.Registry != "" {
        return "registry:" + item.Registry
    }
    return "shared"
}
```

Note: `filterBySource` is defined in `export.go` (Task 31). Since both files are in package `main`, it's accessible. If it needs to be shared across multiple command files, extract to `helpers.go`.

**Test:** `cli/cmd/syllago/list_test.go`

```go
package main

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/output"
)

func setupListRepo(t *testing.T) string {
    t.Helper()
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "skills"), 0755)  // repo marker

    // Shared skill
    shared := filepath.Join(root, "skills", "shared-skill")
    os.MkdirAll(shared, 0755)
    os.WriteFile(filepath.Join(shared, "SKILL.md"),
        []byte("---\nname: shared-skill\ndescription: A shared skill.\n---\n\nBody.\n"), 0644)

    // Local skill
    local := filepath.Join(root, "local", "skills", "my-local-skill")
    os.MkdirAll(local, 0755)
    os.WriteFile(filepath.Join(local, "SKILL.md"),
        []byte("---\nname: my-local-skill\ndescription: My local skill.\n---\n\nBody.\n"), 0644)

    return root
}

func TestListAll(t *testing.T) {
    root := setupListRepo(t)
    withFakeRepoRoot(t, root)

    stdout, _ := output.SetForTest(t)

    listCmd.Flags().Set("source", "all")
    defer listCmd.Flags().Set("source", "all")

    if err := listCmd.RunE(listCmd, []string{}); err != nil {
        t.Fatalf("list failed: %v", err)
    }

    out := stdout.String()
    if !strings.Contains(out, "shared-skill") {
        t.Errorf("expected shared-skill in output, got: %s", out)
    }
    if !strings.Contains(out, "my-local-skill") {
        t.Errorf("expected my-local-skill in output, got: %s", out)
    }
}

func TestListSourceLocal(t *testing.T) {
    root := setupListRepo(t)
    withFakeRepoRoot(t, root)

    stdout, _ := output.SetForTest(t)

    listCmd.Flags().Set("source", "local")
    defer listCmd.Flags().Set("source", "all")

    if err := listCmd.RunE(listCmd, []string{}); err != nil {
        t.Fatalf("list --source local failed: %v", err)
    }

    out := stdout.String()
    if strings.Contains(out, "shared-skill") {
        t.Error("--source local should not show shared-skill")
    }
    if !strings.Contains(out, "my-local-skill") {
        t.Errorf("expected my-local-skill in output, got: %s", out)
    }
}

func TestListTypeFilter(t *testing.T) {
    root := setupListRepo(t)
    withFakeRepoRoot(t, root)

    // Add an agent to ensure type filtering works
    agentDir := filepath.Join(root, "local", "agents", "my-agent")
    os.MkdirAll(agentDir, 0755)
    os.WriteFile(filepath.Join(agentDir, "agent.md"),
        []byte("---\nname: my-agent\ndescription: An agent.\n---\n\nBody.\n"), 0644)

    stdout, _ := output.SetForTest(t)

    listCmd.Flags().Set("type", "skills")
    defer listCmd.Flags().Set("type", "")
    listCmd.Flags().Set("source", "all")
    defer listCmd.Flags().Set("source", "all")

    if err := listCmd.RunE(listCmd, []string{}); err != nil {
        t.Fatalf("list --type skills failed: %v", err)
    }

    out := stdout.String()
    if strings.Contains(out, "my-agent") {
        t.Error("--type skills should not show agents")
    }
    if !strings.Contains(out, "my-local-skill") {
        t.Errorf("expected my-local-skill in --type skills output, got: %s", out)
    }
}

func TestListJSON(t *testing.T) {
    root := setupListRepo(t)
    withFakeRepoRoot(t, root)

    stdout, _ := output.SetForTest(t)
    output.JSON = true
    t.Cleanup(func() { output.JSON = false })

    listCmd.Flags().Set("source", "all")
    defer listCmd.Flags().Set("source", "all")

    if err := listCmd.RunE(listCmd, []string{}); err != nil {
        t.Fatalf("list --json failed: %v", err)
    }

    var items []listItem
    if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
        t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
    }
    if len(items) == 0 {
        t.Error("expected JSON output to contain items")
    }
}
```

---

## Execution Order

Tasks within a phase are independent (can be done in any order) unless noted below.

**Dependency graph:**
- **Tasks 20-27** (kitchen-sink content): all independent of each other; all require Phase 1-2 (Tasks 1-5) complete
- **Task 28** (field coverage test): depends on Tasks 20-27 (all kitchen-sink examples must exist first)
- **Task 29** (round-trip tests): depends on Tasks 20-21 (skill and agent examples) and Task 28
- **Task 30** (import write): independent; requires Phase 1-2 complete
- **Task 31** (export `--source` flag): independent; introduces `filterBySource` helper reused by Task 33
- **Task 32a** (`CreatedAt` field): independent; only touches `metadata.go`
- **Task 32b** (`syllago create` scaffold): requires Phase 1-2 complete
- **Task 32c** (metadata write in create): requires Task 32a (field) and Task 32b (helpers)
- **Task 33** (`syllago list`): requires Task 31 (`filterBySource` must exist first)

Recommended order:
1. Tasks 20-27 in parallel (kitchen-sink content creation — just files, no code changes)
2. Task 32a (`CreatedAt` field) — can run in parallel with Tasks 20-27
3. Tasks 28-29 (tests that read the kitchen-sink content)
4. Task 30 (import write) — independent, can run at any point after Phase 1-2
5. Task 31 (export `--source` flag — adds `filterBySource`, needed before Task 33)
6. Tasks 32b, 32c (create command — 32a must be done before 32c)
7. Task 33 (`syllago list` — needs `filterBySource` from Task 31)

---

## Verification Checklist

After each task:

```bash
make test   # must pass
make build  # must build cleanly
```

After Phase 3 complete:
```bash
# Verify kitchen-sink items appear in catalog
go run ./cmd/syllago --json | grep kitchen-sink  # should find items when hidden=false

# Verify field coverage test passes
go test ./cli/internal/converter/... -run TestKitchenSink -v
```

After Phase 4 complete:
```bash
# Test import write
syllago import --from claude-code --dry-run
syllago import --from claude-code --type rules

# Test export source filter
syllago export --to cursor --source shared --type rules

# Test create
syllago create skill test-skill
syllago create rule --provider cursor test-cursor-rule

# Test list
syllago list
syllago list --source local
syllago list --type skills --json
```

---

**Build:** `make build` | **Test:** `make test`

**Task numbering:** Phase 1-2 used Tasks 1-9; Phase 3-4 used Tasks 20-33. Tasks 10-19 and 34-39 are intentionally reserved as gap space so each phase group starts on a round number. Phase 5-6 starts at Task 40.

---

## Phase 5: Registry and Team Features

---

## Task 40 — Add `AllowedRegistries` to Config struct

**Modifies:** `cli/internal/config/config.go`

**Depends on:** Nothing — standalone config addition; safe to do first in Phase 5.

Add `AllowedRegistries []string` to `Config`. The `omitempty` tag preserves backward compatibility — existing configs without the key load cleanly with a nil slice, which means "any URL allowed."

This mirrors the existing `SandboxConfig.AllowedDomains` pattern: team leads commit a config with this field populated, and the project config enforces policy for everyone who clones the repo.

```go
type Config struct {
	Providers         []string          `json:"providers"`
	Registries        []Registry        `json:"registries,omitempty"`
	AllowedRegistries []string          `json:"allowedRegistries,omitempty"`
	Preferences       map[string]string `json:"preferences,omitempty"`
	Sandbox           SandboxConfig     `json:"sandbox,omitempty"`
}
```

Also add a helper so callers don't inline the allow-check everywhere:

```go
// IsRegistryAllowed returns true if url is permitted given the config.
// When AllowedRegistries is empty, any URL is allowed (solo-user default).
// When non-empty, url must appear in the list (exact string match).
func (c *Config) IsRegistryAllowed(url string) bool {
	if len(c.AllowedRegistries) == 0 {
		return true
	}
	for _, allowed := range c.AllowedRegistries {
		if allowed == url {
			return true
		}
	}
	return false
}
```

**Test:** `cli/internal/config/config_test.go`

```go
func TestIsRegistryAllowed(t *testing.T) {
	empty := &Config{}
	if !empty.IsRegistryAllowed("https://any-url.git") {
		t.Error("empty allowedRegistries should allow any URL")
	}

	restricted := &Config{
		AllowedRegistries: []string{"https://github.com/acme/tools.git"},
	}
	if !restricted.IsRegistryAllowed("https://github.com/acme/tools.git") {
		t.Error("allowed URL should pass")
	}
	if restricted.IsRegistryAllowed("https://github.com/random/other.git") {
		t.Error("non-allowed URL should be rejected")
	}
}
```

**Success criteria:**
- [ ] `IsRegistryAllowed` returns true for any URL when `AllowedRegistries` is empty
- [ ] `IsRegistryAllowed` returns false for a URL not in the list
- [ ] Config round-trips correctly through JSON marshal/unmarshal
- [ ] `make test` passes

---

## Task 41 — Enforce `allowedRegistries` in `syllago registry add`

**Modifies:** `cli/cmd/syllago/registry_cmd.go`

**Depends on:** Task 40 (`AllowedRegistries` field and `IsRegistryAllowed` helper must exist)

The enforcement happens in `registryAddCmd.RunE`, right after the duplicate-name check and before the security warning/clone. The error message tells the user what to do (contact their team lead), which is more helpful than just "not allowed."

```go
// After the duplicate-name check, add:

// Enforce allowedRegistries policy
if !cfg.IsRegistryAllowed(gitURL) {
    return fmt.Errorf("registry URL %q is not in the allowedRegistries list.\n"+
        "Your project config restricts which registries can be added.\n"+
        "Contact your team lead to add it to .syllago/config.json", gitURL)
}
```

This is a deliberate hard fail, not a warning — it defeats the policy if users can bypass it.

**Test:** `cli/cmd/syllago/registry_cmd_test.go` (or add to `testhelpers_test.go` if that pattern is used)

```go
func TestRegistryAddRejectsDisallowedURL(t *testing.T) {
	// Set up a project root with a config that has allowedRegistries
	root := t.TempDir()
	cfg := &config.Config{
		AllowedRegistries: []string{"https://github.com/allowed/only.git"},
	}
	if err := config.Save(root, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	// Override findProjectRoot to return our temp root
	orig := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	defer func() { findProjectRoot = orig }()

	cmd := registryCmd
	cmd.SetArgs([]string{"add", "https://github.com/not/allowed.git"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for disallowed URL, got nil")
	}
	if !strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("error %q does not mention allowedRegistries", err.Error())
	}
}
```

**Success criteria:**
- [ ] `syllago registry add <disallowed-url>` returns an error mentioning `allowedRegistries`
- [ ] `syllago registry add <allowed-url>` proceeds normally (no extra rejection)
- [ ] When `allowedRegistries` is absent, all URLs are accepted
- [ ] `make test` passes

---

## Task 42 — Add `RegistryManifest` type and loader to registry package

**Modifies:** `cli/internal/registry/registry.go`

**Depends on:** Nothing — purely additive, no changes to existing registry functions.

The registry manifest is optional metadata at the registry root. Its purpose is display-only — it gives teams a way to describe their registry in the TUI and CLI output. Not required: registries without a manifest still work.

```go
import "gopkg.in/yaml.v3"

// Manifest holds optional metadata from registry.yaml at the registry root.
type Manifest struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	Maintainers     []string `yaml:"maintainers,omitempty"`
	Version         string   `yaml:"version,omitempty"`
	MinSyllagoVersion string   `yaml:"min_syllago_version,omitempty"`
}

// LoadManifest reads registry.yaml from the registry clone directory.
// Returns nil, nil if the file does not exist (manifest is optional).
func LoadManifest(name string) (*Manifest, error) {
	dir, err := CloneDir(name)
	if err != nil {
		return nil, err
	}
	data, readErr := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if errors.Is(readErr, fs.ErrNotExist) {
		return nil, nil
	}
	if readErr != nil {
		return nil, fmt.Errorf("reading registry.yaml for %q: %w", name, readErr)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing registry.yaml for %q: %w", name, err)
	}
	return &m, nil
}
```

Add the `errors` and `io/fs` imports alongside the existing ones.

**Test:** `cli/internal/registry/registry_test.go`

```go
func TestLoadManifest_Missing(t *testing.T) {
	// A registry clone dir with no registry.yaml
	dir := t.TempDir()
	// Monkey-patch CloneDir by writing a test-only helper that returns dir for "test-reg"
	// (or test via the file system directly with a known name)
	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("loadManifestFromDir: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil manifest for missing file, got %+v", m)
	}
}

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	content := "name: my-registry\ndescription: Test registry\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("loadManifestFromDir: %v", err)
	}
	if m.Name != "my-registry" {
		t.Errorf("Name = %q, want %q", m.Name, "my-registry")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
}
```

Extract a `loadManifestFromDir(dir string)` helper that takes a path directly, so tests can call it without needing a real clone. `LoadManifest(name)` resolves the path and delegates.

**Success criteria:**
- [ ] `LoadManifest` returns nil, nil for registries without registry.yaml
- [ ] `LoadManifest` parses all fields correctly when the file exists
- [ ] Invalid YAML returns a descriptive error
- [ ] `make test` passes

---

## Task 43 — Show manifest in `syllago registry list`

**Modifies:** `cli/cmd/syllago/registry_cmd.go`

**Depends on:** Task 42 (`registry.LoadManifest` and `Manifest` type must exist)

Update `registryListCmd` to load and display manifest info. The current output is a simple table; we extend it to show the manifest description if present.

For plain text output, add a description column (or show as a second line indented under the name). For JSON output, include the manifest in the item struct.

> **Note:** Do not use `helpStyle` (or any lipgloss style) here. `helpStyle` is defined in `cli/internal/tui/styles.go` in the `tui` package and cannot be imported into the CLI command package (`package main`). Use plain `fmt.Printf` for description lines in CLI commands.

```go
// In registryListCmd.RunE, replace the print loop with:

type registryListItem struct {
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	URL         string          `json:"url"`
	Ref         string          `json:"ref"`
	Manifest    *registry.Manifest `json:"manifest,omitempty"`
}

var items []registryListItem
for _, r := range cfg.Registries {
	status := "missing"
	if registry.IsCloned(r.Name) {
		status = "cloned"
	}
	ref := r.Ref
	if ref == "" {
		ref = "default"
	}
	manifest, _ := registry.LoadManifest(r.Name) // ignore error; manifest is optional
	items = append(items, registryListItem{
		Name:     r.Name,
		Status:   status,
		URL:      r.URL,
		Ref:      ref,
		Manifest: manifest,
	})
}

if output.JSON {
	output.Print(items)
	return nil
}

fmt.Printf("%-20s  %-8s  %-8s  %s\n", "NAME", "STATUS", "VERSION", "URL / DESCRIPTION")
fmt.Printf("%-20s  %-8s  %-8s  %s\n",
	strings.Repeat("─", 20), strings.Repeat("─", 8),
	strings.Repeat("─", 8), strings.Repeat("─", 40))
for _, item := range items {
	version := "─"
	if item.Manifest != nil && item.Manifest.Version != "" {
		version = item.Manifest.Version
	}
	fmt.Printf("%-20s  %-8s  %-8s  %s\n",
		truncateStr(item.Name, 20), item.Status, version, item.URL)
	if item.Manifest != nil && item.Manifest.Description != "" {
		fmt.Printf("  %s\n", item.Manifest.Description)
	}
}
```

**Success criteria:**
- [ ] `syllago registry list` shows version column populated when manifest present
- [ ] Description line appears indented under registries that have one
- [ ] `syllago registry list --json` includes `manifest` field (null when absent)
- [ ] Output is unchanged for registries without manifest
- [ ] `make test` passes

---

## Task 44 — Show manifest in TUI registries screen

**Modifies:** `cli/internal/tui/registries.go`

**Depends on:** Task 42 (`registry.LoadManifest` and `Manifest` type must exist)

Extend `registryEntry` with manifest data and update `newRegistriesModel` to load it. The TUI registries screen currently shows name, status, count, URL. Add version and description from the manifest.

```go
type registryEntry struct {
	name        string
	url         string
	ref         string
	cloned      bool
	itemCount   int
	// Manifest fields (nil if no registry.yaml)
	version     string // from manifest.Version
	description string // from manifest.Description
}

func newRegistriesModel(repoRoot string, cfg *config.Config, cat *catalog.Catalog) registriesModel {
	entries := make([]registryEntry, len(cfg.Registries))
	for i, r := range cfg.Registries {
		entry := registryEntry{
			name:      r.Name,
			url:       r.URL,
			ref:       r.Ref,
			cloned:    registry.IsCloned(r.Name),
			itemCount: cat.CountRegistry(r.Name),
		}
		if manifest, err := registry.LoadManifest(r.Name); err == nil && manifest != nil {
			entry.version = manifest.Version
			entry.description = manifest.Description
		}
		entries[i] = entry
	}
	return registriesModel{entries: entries, repoRoot: repoRoot}
}
```

In `registriesModel.View()`, append the description below the row when present:

```go
// After the main row line:
s += zone.Mark(fmt.Sprintf("registry-row-%d", i), row) + "\n"
if entry.description != "" {
	s += helpStyle.Render("      " + entry.description) + "\n"
}
```

**Success criteria:**
- [ ] Registry description appears indented under the registry row in TUI
- [ ] Version appears in registry row when manifest present
- [ ] Registries without manifests display unchanged
- [ ] `make test` passes (golden tests should update if they cover the registries screen)

---

## Task 45 — Implement `RiskIndicators` for content items

**Creates:** `cli/internal/catalog/risk.go`

**Depends on:** Nothing — operates only on `ContentItem` fields that already exist in `catalog/types.go`. Independent of all other Phase 5 tasks.

Risk indicators are the "⚠ Runs commands" etc. labels shown in `syllago inspect` (Task 46) and the TUI detail view (Task 47). Centralizing the logic in the catalog package keeps it reusable.

The indicators are computed from the item's files and metadata — no new metadata fields required. We read the actual content files to detect patterns (hooks type, MCP env references, etc.).

```go
package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// RiskIndicator represents a security-relevant characteristic of a content item.
type RiskIndicator struct {
	Label       string // e.g. "Runs commands"
	Description string // e.g. "Hooks with type: command"
}

// RiskIndicators analyzes a ContentItem and returns any applicable risk indicators.
// These are informational — they help users make informed install decisions.
func RiskIndicators(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator

	switch item.Type {
	case Hooks:
		risks = append(risks, hookRisks(item)...)
	case MCP:
		risks = append(risks, mcpRisks(item)...)
	case Apps:
		risks = append(risks, appRisks(item)...)
	case Skills, Agents:
		risks = append(risks, skillAgentRisks(item)...)
	}

	return risks
}

func hookRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	// Look for hook.json or any .json file
	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		// Claude Code hooks format: {"hooks": {"PostToolUse": [{"matcher":"...", "command":"..."}]}}
		// The outer ForEach iterates event names (e.g., "PostToolUse") → each value is the hook array.
		// The inner ForEach iterates the array entries → each is the actual hook object.
		// Entries do NOT have a "type" field; detect command hooks by the presence of "command",
		// and HTTP hooks by the presence of "url".
		gjson.GetBytes(data, "hooks").ForEach(func(_, eventHooks gjson.Result) bool {
			eventHooks.ForEach(func(_, hook gjson.Result) bool {
				if hook.Get("command").Exists() {
					risks = appendIfMissing(risks, RiskIndicator{
						Label:       "Runs commands",
						Description: "Hook executes shell commands on your machine",
					})
				}
				if hook.Get("url").Exists() {
					risks = appendIfMissing(risks, RiskIndicator{
						Label:       "Network access",
						Description: "Hook makes HTTP requests",
					})
				}
				return true
			})
			return true
		})
	}
	return risks
}

func mcpRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	risks = appendIfMissing(risks, RiskIndicator{
		Label:       "Network access",
		Description: "MCP server communicates over network",
	})
	// Check for env references
	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		// MCP config format: {"mcpServers": {"name": {"env": {...}}}}
		gjson.GetBytes(data, "mcpServers").ForEach(func(_, srv gjson.Result) bool {
			if srv.Get("env").Exists() {
				risks = appendIfMissing(risks, RiskIndicator{
					Label:       "Environment variables",
					Description: "MCP server reads environment variables",
				})
			}
			return true
		})
	}
	return risks
}

func appRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	for _, f := range item.Files {
		if strings.HasSuffix(f, "install.sh") || strings.HasSuffix(f, "setup.sh") {
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Runs commands",
				Description: "App has an install script that executes on your machine",
			})
			break
		}
	}
	return risks
}

func skillAgentRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	// Scan content files for "Bash" tool references
	for _, f := range item.Files {
		if filepath.Ext(f) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "Bash") {
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Bash access",
				Description: "Content references the Bash tool — can execute arbitrary commands",
			})
			break
		}
	}
	return risks
}

func appendIfMissing(risks []RiskIndicator, r RiskIndicator) []RiskIndicator {
	for _, existing := range risks {
		if existing.Label == r.Label {
			return risks
		}
	}
	return append(risks, r)
}
```

**Test:** `cli/internal/catalog/risk_test.go`

```go
func TestRiskIndicators_Hook_Command(t *testing.T) {
	dir := t.TempDir()
	// Real Claude Code hook format: entries use "command" field, not "type"
	hookJSON := `{"hooks":{"PostToolUse":[{"matcher":"Write|Edit","command":"echo hi"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Hooks,
		Path:  dir,
		Files: []string{"hook.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Runs commands" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Runs commands")
	}
}

func TestRiskIndicators_MCP_WithEnv(t *testing.T) {
	dir := t.TempDir()
	mcpJSON := `{"mcpServers":{"db":{"command":"npx","env":{"DB_URL":"postgres://..."}}}}`
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  MCP,
		Path:  dir,
		Files: []string{"mcp.json"},
	}
	risks := RiskIndicators(item)
	labels := make(map[string]bool, len(risks))
	for _, r := range risks {
		labels[r.Label] = true
	}
	if !labels["Network access"] {
		t.Error("expected risk label 'Network access'")
	}
	if !labels["Environment variables"] {
		t.Error("expected risk label 'Environment variables'")
	}
}

func TestRiskIndicators_NoRisk(t *testing.T) {
	item := ContentItem{Type: Prompts, Path: t.TempDir(), Files: []string{"PROMPT.md"}}
	risks := RiskIndicators(item)
	if len(risks) != 0 {
		t.Errorf("expected no risks, got %d: %+v", len(risks), risks)
	}
}
```

**Success criteria:**
- [ ] `RiskIndicators` returns correct labels for hooks with a `"command"` field
- [ ] MCP items always get "Network access"; add "Environment variables" when env present
- [ ] Apps with `install.sh` get "Runs commands"
- [ ] Skills/agents with "Bash" in content get "Bash access"
- [ ] Items with no risk signals return empty slice
- [ ] `make test` passes

---

## Task 46 — Implement `syllago inspect` command

**Creates:** `cli/cmd/syllago/inspect.go`

**Depends on:** Task 45 (`catalog.RiskIndicators` function must exist); Task 2 (`findContentRepoRoot` used to locate the repo root); Task 31 (export all sources — `catalog.ScanWithRegistries` must handle registry sources)

`syllago inspect` is a pre-install auditing tool. It shows everything about an item before you install it: full content, metadata, file list, and risk indicators. The argument format mirrors how items are addressed: `<type>/<name>` or `<registry>/<type>/<name>`.

The command scans the catalog (same as the TUI) and looks up the item by the path argument. Output goes to stdout; risk indicators go to stderr when plain-text so they stand out as warnings.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <type>/<name>",
	Short: "Show full details and risk indicators for a content item",
	Long: `Display full metadata, file list, and risk indicators for a content item
before installing it. Useful for auditing registry content.

Examples:
  syllago inspect skills/my-skill
  syllago inspect company-tools/mcp/database-server
  syllago inspect rules/claude-code/coding-standards`,
	Args: cobra.ExactArgs(1),
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

type inspectResult struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Path        string                 `json:"path"`
	Description string                 `json:"description,omitempty"`
	Files       []string               `json:"files"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
	Risks       []catalog.RiskIndicator `json:"risks,omitempty"`
}

func runInspect(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago repo: %w", err)
	}

	cfg, _ := config.Load(root)
	if cfg == nil {
		cfg = &config.Config{}
	}

	var regSources []catalog.RegistrySource
	for _, r := range cfg.Registries {
		if registry.IsCloned(r.Name) {
			dir, _ := registry.CloneDir(r.Name)
			regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
		}
	}

	cat, err := catalog.ScanWithRegistries(root, regSources)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	// Parse the argument: [registry/]type/name
	item, err := findItemByPath(cat, args[0])
	if err != nil {
		return err
	}

	risks := catalog.RiskIndicators(*item)

	if output.JSON {
		result := buildInspectResult(*item, risks)
		output.Print(result)
		return nil
	}

	// Plain text output
	fmt.Fprintf(output.Writer, "Name:    %s\n", item.Name)
	fmt.Fprintf(output.Writer, "Type:    %s\n", item.Type.Label())
	source := "shared"
	if item.Local {
		source = "local"
	} else if item.Registry != "" {
		source = "registry:" + item.Registry
	}
	fmt.Fprintf(output.Writer, "Source:  %s\n", source)
	fmt.Fprintf(output.Writer, "Path:    %s\n", item.Path)
	if item.Description != "" {
		fmt.Fprintf(output.Writer, "Desc:    %s\n", item.Description)
	}
	if item.Meta != nil {
		if item.Meta.Author != "" {
			fmt.Fprintf(output.Writer, "Author:  %s\n", item.Meta.Author)
		}
		if item.Meta.Version != "" {
			fmt.Fprintf(output.Writer, "Version: %s\n", item.Meta.Version)
		}
		if len(item.Meta.Tags) > 0 {
			fmt.Fprintf(output.Writer, "Tags:    %s\n", strings.Join(item.Meta.Tags, ", "))
		}
	}
	fmt.Fprintf(output.Writer, "\nFiles:\n")
	for _, f := range item.Files {
		fmt.Fprintf(output.Writer, "  %s\n", f)
	}
	if len(risks) > 0 {
		fmt.Fprintf(output.ErrWriter, "\nRisk indicators:\n")
		for _, r := range risks {
			fmt.Fprintf(output.ErrWriter, "  ⚠  %s — %s\n", r.Label, r.Description)
		}
	} else {
		fmt.Fprintf(output.Writer, "\nNo risk indicators.\n")
	}
	return nil
}

// findItemByPath looks up an item by "type/name" or "registry/type/name" path.
func findItemByPath(cat *catalog.Catalog, path string) (*catalog.ContentItem, error) {
	parts := strings.Split(path, "/")
	switch len(parts) {
	case 2:
		// type/name
		ct := catalog.ContentType(parts[0])
		name := parts[1]
		for i := range cat.Items {
			if cat.Items[i].Type == ct && cat.Items[i].Name == name {
				return &cat.Items[i], nil
			}
		}
		return nil, fmt.Errorf("item %q not found", path)
	case 3:
		// Could be: registry/type/name or type/provider/name (for provider-specific)
		// Try registry first
		regName, ct, name := parts[0], catalog.ContentType(parts[1]), parts[2]
		for i := range cat.Items {
			if cat.Items[i].Registry == regName && cat.Items[i].Type == ct && cat.Items[i].Name == name {
				return &cat.Items[i], nil
			}
		}
		// Try provider-specific: type/provider/name
		for i := range cat.Items {
			if cat.Items[i].Type == ct && cat.Items[i].Provider == parts[1] && cat.Items[i].Name == name {
				return &cat.Items[i], nil
			}
		}
		return nil, fmt.Errorf("item %q not found (tried registry/type/name and type/provider/name)", path)
	default:
		return nil, fmt.Errorf("invalid item path %q (expected type/name or registry/type/name)", path)
	}
}
```

**Example output:**

```
$ syllago inspect skills/my-skill

Name:    my-skill
Type:    Skills
Source:  local
Path:    /home/user/syllago-workspace/local/skills/my-skill
Desc:    Reviews Go code for style and correctness
Author:  Holden
Version: 1.0.0
Tags:    code-review, go

Files:
  SKILL.md
  .syllago.yaml

No risk indicators.
```

```
$ syllago inspect company-tools/mcp/database-server

Name:    database-server
Type:    MCP Configs
Source:  registry:company-tools
Path:    /home/user/.syllago/registries/company-tools/mcp/database-server
Desc:    Read-only access to the Postgres analytics DB

Files:
  mcp.json
  README.md

Risk indicators:
  ⚠  Network access — MCP server communicates over network
  ⚠  Environment variables — MCP server reads environment variables
```

**Success criteria:**
- [ ] `syllago inspect skills/my-skill` shows metadata, files, and risk indicators
- [ ] `syllago inspect company-tools/mcp/database-server` works for registry items
- [ ] `syllago inspect --json` outputs valid JSON with `risks` array
- [ ] Clear error when item not found
- [ ] `make test` passes

---

## Task 47 — Show risk indicators in TUI detail view

**Modifies:** `cli/internal/tui/detail_render.go`

**Depends on:** Task 45 (`catalog.RiskIndicators` function must exist)

The detail view's Overview tab is the best place for risk indicators. They appear right after the description, before the file list. Using `warningStyle` (amber) makes them visually distinct without being alarming.

```go
// In renderContentSplit, in the Overview tab section, after description:

risks := catalog.RiskIndicators(m.item)
if len(risks) > 0 {
	body += "\n"
	for _, r := range risks {
		body += warningStyle.Render("⚠  "+r.Label) + "\n"
		body += helpStyle.Render("   "+r.Description) + "\n"
	}
}
```

The catalog import is already present in `detail_render.go`. No new imports needed.

**Success criteria:**
- [ ] Registry items with risks show warning indicators in the Overview tab
- [ ] Items with no risks show nothing extra
- [ ] Golden tests update (run `UPDATE_GOLDEN=1 make test` to regenerate)
- [ ] `make test` passes

---

## Task 48 — Implement content precedence deduplication in catalog

**Modifies:** `cli/internal/catalog/scanner.go`, `cli/internal/catalog/types.go`

**Depends on:** Nothing — pure post-processing logic operating on the existing `Catalog` struct. Can be done in parallel with Tasks 40-47.

Content precedence defines which version of a same-named item "wins" when multiple sources have it. The priority order (highest to lowest): Local → Shared → Registry → Built-in.

This is implemented as a post-processing step on the catalog after all sources are scanned. The key insight: "same item" means same `Name + Type` (case-insensitive). We keep the highest-precedence version in the main `Items` slice and move lower-precedence duplicates to `Overridden`.

```go
// In types.go, add to Catalog struct:

type Catalog struct {
	Items      []ContentItem
	Overridden []ContentItem // lower-precedence items shadowed by higher-precedence ones
	RepoRoot   string
}
```

```go
// In scanner.go, add:

// itemPrecedence returns the precedence level for an item (lower number = higher priority).
func itemPrecedence(item ContentItem) int {
	if item.Local {
		return 0 // highest
	}
	if item.Registry == "" && !item.IsBuiltin() {
		return 1 // shared
	}
	if item.Registry != "" {
		return 2 // registry
	}
	return 3 // built-in (lowest)
}

// applyPrecedence deduplicates items by (name, type), keeping the highest-precedence
// version in Items and moving others to Overridden.
func applyPrecedence(cat *Catalog) {
	type key struct {
		name string
		typ  ContentType
	}

	best := make(map[key]int) // key → index in cat.Items of winning item

	var kept []ContentItem
	var overridden []ContentItem

	for _, item := range cat.Items {
		k := key{strings.ToLower(item.Name), item.Type}
		winIdx, exists := best[k]
		if !exists {
			best[k] = len(kept)
			kept = append(kept, item)
			continue
		}
		challenger := itemPrecedence(item)
		current := itemPrecedence(kept[winIdx])
		if challenger < current {
			// Challenger wins — move existing to overridden, replace in kept
			overridden = append(overridden, kept[winIdx])
			best[k] = winIdx // index stays same, we replace in-place
			kept[winIdx] = item
		} else {
			// Current wins — challenger goes to overridden
			overridden = append(overridden, item)
		}
	}

	cat.Items = kept
	cat.Overridden = overridden
}
```

Call `applyPrecedence(cat)` at the end of both `ScanWithRegistries` and `Scan`.

Also add a helper to check if an item overrides something lower:

```go
// OverridesFor returns overridden items for a given (name, type) pair.
func (c *Catalog) OverridesFor(name string, ct ContentType) []ContentItem {
	var result []ContentItem
	lower := strings.ToLower(name)
	for _, item := range c.Overridden {
		if strings.ToLower(item.Name) == lower && item.Type == ct {
			result = append(result, item)
		}
	}
	return result
}
```

**Test:** `cli/internal/catalog/scanner_test.go`

```go
func TestApplyPrecedence_LocalWinsOverShared(t *testing.T) {
	cat := &Catalog{
		Items: []ContentItem{
			{Name: "my-skill", Type: Skills, Local: false, Registry: ""},  // shared
			{Name: "my-skill", Type: Skills, Local: true},                  // local
		},
	}
	applyPrecedence(cat)
	require.Len(t, cat.Items, 1)
	require.True(t, cat.Items[0].Local, "local item should win")
	require.Len(t, cat.Overridden, 1)
	require.False(t, cat.Overridden[0].Local, "shared item should be overridden")
}

func TestApplyPrecedence_DifferentTypes_NoDedup(t *testing.T) {
	cat := &Catalog{
		Items: []ContentItem{
			{Name: "my-item", Type: Skills},
			{Name: "my-item", Type: Rules},
		},
	}
	applyPrecedence(cat)
	require.Len(t, cat.Items, 2, "different types should not be deduplicated")
	require.Empty(t, cat.Overridden)
}
```

**Success criteria:**
- [ ] Local items shadow same-named shared items
- [ ] Shared items shadow registry items
- [ ] Registry items shadow built-in items
- [ ] Different types with same name are NOT deduplicated
- [ ] Case-insensitive name matching
- [ ] `make test` passes

---

## Task 49 — Show override info in TUI detail view

**Modifies:** `cli/internal/tui/detail_render.go`

**Depends on:** Task 48 (`Catalog.Overridden` field and `OverridesFor` method must exist)

When a detail view shows an item that has overridden a lower-precedence version, a note appears in the metadata block: "Overrides [BUILT-IN] version" or "Overrides [registry-name] version". This requires passing the catalog to the detail model, or computing it at item selection time.

The simplest approach: at the point where `newDetailModel` is called (in `app.go`), look up overrides and pass them in. This avoids giving `detailModel` a reference to the full catalog.

```go
// In detail.go, add field to detailModel:
overrides []catalog.ContentItem // lower-precedence items this one shadows

// Update newDetailModel signature in app.go call sites:
overrides := a.catalog.OverridesFor(item.Name, item.Type)
a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot, overrides)
```

```go
// In detail_render.go, in the metadata block (renderContentSplit):
if len(m.overrides) > 0 {
	for _, ov := range m.overrides {
		ovSource := "shared"
		if ov.IsBuiltin() {
			ovSource = "built-in"
		} else if ov.Registry != "" {
			ovSource = ov.Registry
		}
		pinned += helpStyle.Render(fmt.Sprintf("  Overrides [%s] version", ovSource)) + "\n"
	}
}
```

**Success criteria:**
- [ ] Detail view for a local item that shadows a shared item shows "Overrides [shared] version"
- [ ] Detail view for a shared item that shadows a registry item shows the registry name
- [ ] Items with no overrides show nothing extra
- [ ] `make test` passes

---

## Task 50 — Implement `syllago promote --to-registry` command

**Creates:** `cli/internal/promote/registry_promote.go`
**Modifies:** `cli/cmd/syllago/promote_cmd.go` (or wherever the promote command lives)

**Depends on:** Task 55 (`findItemByPath` extracted to `helpers.go`); Task 42 (`registry.CloneDir` and manifest structure stable); Tasks 40-41 (allowedRegistries enforcement must be in place before contributors can add new registries); Task 9 (export warnings for built-in/example content — same guards apply before promoting)

This is the reverse of the existing `Promote` function which promotes local → shared. `PromoteToRegistry` promotes a local or shared item to an external registry. It forks the registry repo if needed, creates a branch, copies the content, commits, pushes, and opens a PR.

The critical difference from `Promote`: the target repo is the registry clone at `~/.syllago/registries/<name>/`, not the user's project repo. The fork-and-PR workflow goes against the registry repo's origin.

> **Helper functions:** `gitRun`, `commandOutput`, `detectDefaultBranch`, `buildCompareURL`, and `copyForPromote` are already defined in `cli/internal/promote/promote.go` (same package). `registry_promote.go` lives in `package promote`, so it can use them directly — no need to re-implement or import them. Remove the `installer` import since `copyForPromote` is already implemented in `promote.go` and handles the file copy. Keep `os/exec` only for `exec.LookPath("gh")`.

```go
package promote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// RegistryResult holds the outcome of a promote-to-registry operation.
type RegistryResult struct {
	Branch     string
	PRUrl      string
	CompareURL string
	ForkURL    string // set if we forked the repo
}

// PromoteToRegistry copies an item to a registry and opens a PR.
// registryName must be a registered and cloned registry.
// item is the source content item (local or shared).
func PromoteToRegistry(repoRoot string, registryName string, item catalog.ContentItem) (*RegistryResult, error) {
	registryDir, err := registry.CloneDir(registryName)
	if err != nil {
		return nil, fmt.Errorf("finding registry clone: %w", err)
	}
	if _, statErr := os.Stat(registryDir); statErr != nil {
		return nil, fmt.Errorf("registry %q is not cloned — run `syllago registry sync %s` first", registryName, registryName)
	}

	// Determine destination path in registry
	var destDir string
	if item.Type.IsUniversal() {
		destDir = filepath.Join(registryDir, string(item.Type), item.Name)
	} else {
		destDir = filepath.Join(registryDir, string(item.Type), item.Provider, item.Name)
	}

	// Check if item already exists in registry
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("item %q already exists in registry %q", item.Name, registryName)
	}

	defaultBranch := detectDefaultBranch(registryDir)

	// Create contribution branch
	branchName := fmt.Sprintf("syllago/contribute/%s/%s", item.Type, item.Name)
	if err := gitRun(registryDir, "checkout", "-b", branchName); err != nil {
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
		if err := gitRun(registryDir, "checkout", "-b", branchName); err != nil {
			return nil, fmt.Errorf("creating branch in registry: %w", err)
		}
	}

	cleanup := func() {
		gitRun(registryDir, "checkout", defaultBranch)
	}

	// Copy content (exclude LLM-PROMPT.md and local scaffold artifacts)
	if err := copyForPromote(item.Path, destDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("copying content to registry: %w", err)
	}

	if err := gitRun(registryDir, "add", destDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("staging files: %w", err)
	}

	commitMsg := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
	if err := gitRun(registryDir, "commit", "-m", commitMsg); err != nil {
		cleanup()
		return nil, fmt.Errorf("committing to registry: %w", err)
	}

	// Push to fork (if gh available) or origin
	result := &RegistryResult{Branch: branchName}

	// Try to push and create PR via gh
	if ghPath, _ := exec.LookPath("gh"); ghPath != "" {
		// Push to fork (gh fork creates the fork if needed)
		if err := gitRun(registryDir, "push", "-u", "origin", branchName); err != nil {
			cleanup()
			return nil, fmt.Errorf("pushing to registry: %w", err)
		}

		prTitle := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
		prBody := fmt.Sprintf("Contributes `%s` (%s) to this registry.\n\nSubmitted via `syllago promote --to-registry %s`.",
			item.Name, item.Type, registryName)
		out, err := commandOutput(registryDir, "gh", "pr", "create",
			"--title", prTitle,
			"--body", prBody,
			"--base", defaultBranch)
		if err == nil {
			result.PRUrl = strings.TrimSpace(out)
		}
	} else {
		// No gh: just push and provide compare URL
		if err := gitRun(registryDir, "push", "-u", "origin", branchName); err != nil {
			cleanup()
			return nil, fmt.Errorf("pushing to registry: %w", err)
		}
	}

	result.CompareURL = buildCompareURL(registryDir, branchName)
	gitRun(registryDir, "checkout", defaultBranch)
	return result, nil
}
```

**CLI command** in `cli/cmd/syllago/promote_cmd.go`:

The command follows the project's subcommand pattern (same as `syllago registry add`, `syllago sandbox run`). A `promoteCmd` parent is registered at root, and `promoteToRegistryCmd` is a subcommand. The resulting invocation is `syllago promote to-registry <name> <type>/<name>`.

```go
var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote content to shared or registry",
	Long: `Share content with your team or contribute it to a registry.

Subcommands:
  to-registry   Contribute content to an external registry via PR`,
}

var promoteToRegistryCmd = &cobra.Command{
	Use:   "to-registry <registry-name> <type>/<name>",
	Short: "Contribute content to a registry via PR",
	Long: `Fork a registry and open a pull request to contribute your content.

Requires gh CLI for the PR workflow. Without gh, the branch is pushed
and a compare URL is printed for manual PR creation.

Examples:
  syllago promote to-registry company-tools skills/my-skill
  syllago promote to-registry syllago-tools rules/claude-code/my-rule`,
	Args: cobra.ExactArgs(2),
	RunE: runPromoteToRegistry,
}

func init() {
	promoteCmd.AddCommand(promoteToRegistryCmd)
	rootCmd.AddCommand(promoteCmd)
}

func runPromoteToRegistry(cmd *cobra.Command, args []string) error {
	// cobra.ExactArgs(2) enforces arg count; args[0]=registry name, args[1]=type/name path
	registryName := args[0]
	itemPath := args[1]

	root, err := findContentRepoRoot()
	if err != nil {
		return err
	}

	cat, err := catalog.Scan(root)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	item, err := findItemByPath(cat, itemPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(output.Writer, "Promoting %s (%s) to registry %q...\n", item.Name, item.Type, registryName)
	result, err := promote.PromoteToRegistry(root, registryName, *item)
	if err != nil {
		return err
	}

	if result.PRUrl != "" {
		fmt.Fprintf(output.Writer, "PR created: %s\n", result.PRUrl)
	} else if result.CompareURL != "" {
		fmt.Fprintf(output.Writer, "Branch pushed. Open PR at:\n  %s\n", result.CompareURL)
	}
	return nil
}
```

Note: `findItemByPath` is already defined in `inspect.go` (Task 46). Extract it to `helpers.go` so both commands can use it.

**Success criteria:**
- [ ] `syllago promote to-registry company-tools skills/my-skill` copies files to registry clone
- [ ] A branch `syllago/contribute/skills/my-skill` is created in the registry clone
- [ ] PR URL printed when `gh` is available
- [ ] Compare URL printed as fallback
- [ ] Error when item already exists in registry
- [ ] `make test` passes

---

## Phase 6: Polish

---

## Task 51 — Implement `syllago sync-and-export` command

**Creates:** `cli/cmd/syllago/sync_and_export.go`

**Depends on:** Task 31 (export must support all sources, not just local, so sync + export is meaningful); `registry.SyncAll` must exist (added in Phase 3-4 registry sync work)

This is a convenience command for team workflows and project setup scripts. It does exactly `syllago registry sync && syllago export --to <provider>`, but as a single atomic command with unified error reporting.

The implementation calls the underlying logic functions directly rather than shelling out to sub-processes — this is both more reliable and faster.

```go
package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

var syncAndExportCmd = &cobra.Command{
	Use:   "sync-and-export",
	Short: "Sync all registries then export to a provider",
	Long: `Pull latest content from all registries, then export to the specified provider.
Equivalent to: syllago registry sync && syllago export --to <provider>

Useful in project setup scripts and CI.

Examples:
  syllago sync-and-export --to cursor
  syllago sync-and-export --to claude-code --type skills`,
	RunE: runSyncAndExport,
}

func init() {
	syncAndExportCmd.Flags().String("to", "", "Provider slug to export to (required)")
	syncAndExportCmd.MarkFlagRequired("to")
	syncAndExportCmd.Flags().String("type", "", "Filter to a specific content type")
	syncAndExportCmd.Flags().String("name", "", "Filter by item name (substring match)")
	rootCmd.AddCommand(syncAndExportCmd)
}

func runSyncAndExport(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	// Step 1: Sync all registries
	if len(cfg.Registries) == 0 {
		fmt.Fprintln(output.Writer, "No registries configured — skipping sync.")
	} else {
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}
		fmt.Fprintln(output.Writer, "Syncing registries...")
		results := registry.SyncAll(names)
		syncErr := false
		for _, res := range results {
			if res.Err != nil {
				fmt.Fprintf(output.ErrWriter, "  Error syncing %s: %s\n", res.Name, res.Err)
				syncErr = true
			} else {
				fmt.Fprintf(output.Writer, "  Synced: %s\n", res.Name)
			}
		}
		if syncErr {
			return fmt.Errorf("one or more registry syncs failed; export aborted")
		}
	}

	// Step 2: Export — reuse runExport logic by synthesizing a cobra command context
	fmt.Fprintln(output.Writer, "Exporting...")
	toSlug, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")

	exportArgs := []string{"--to", toSlug}
	if typeFilter != "" {
		exportArgs = append(exportArgs, "--type", typeFilter)
	}
	if nameFilter != "" {
		exportArgs = append(exportArgs, "--name", nameFilter)
	}
	exportCmd.SetArgs(exportArgs)
	return exportCmd.RunE(exportCmd, nil)
}
```

**Success criteria:**
- [ ] `syllago sync-and-export --to cursor` syncs all registries then exports to Cursor
- [ ] If sync fails, export is aborted and error reported
- [ ] `--type` and `--name` flags are passed through to export
- [ ] When no registries configured, skips sync and proceeds to export
- [ ] `make build` passes

---

## Task 52 — Implement `syllago export --to all`

**Modifies:** `cli/cmd/syllago/export.go`

**Depends on:** Task 31 (export must already support all sources so `--to all` produces meaningful output across source types)

When `--to all` is specified, iterate through all providers in `provider.AllProviders` and run the export logic for each. Report a summary table at the end.

The key change: `findProviderBySlug("all")` currently returns nil (not found). We intercept that case before the existing nil check:

```go
func runExport(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")

	// Handle "all" provider as a special case
	if toSlug == "all" {
		return runExportAll(cmd)
	}

	// ... existing single-provider logic unchanged ...
}

type allExportResult struct {
	Provider string      `json:"provider"`
	Exported int         `json:"exported"`
	Skipped  int         `json:"skipped"`
	Err      string      `json:"error,omitempty"`
}

func runExportAll(cmd *cobra.Command) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago repo: %w", err)
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	_ = root // used by inner export logic

	var results []allExportResult

	for _, prov := range provider.AllProviders {
		// Run export for this provider by calling the inner function
		// Capture stdout to count items (or use a counting wrapper)
		exportArgs := []string{"--to", prov.Slug}
		if typeFilter != "" {
			exportArgs = append(exportArgs, "--type", typeFilter)
		}
		if nameFilter != "" {
			exportArgs = append(exportArgs, "--name", nameFilter)
		}

		// Use a temporary in-memory result by calling runExportForProvider directly
		result, exportErr := runExportForProvider(root, prov.Slug, typeFilter, nameFilter, cmd)
		r := allExportResult{Provider: prov.Slug}
		if exportErr != nil {
			r.Err = exportErr.Error()
		} else {
			r.Exported = len(result.Exported)
			r.Skipped = len(result.Skipped)
		}
		results = append(results, r)
	}

	if output.JSON {
		output.Print(results)
		return nil
	}

	// Summary table
	fmt.Printf("\n%-20s  %-10s  %-10s  %s\n", "PROVIDER", "EXPORTED", "SKIPPED", "STATUS")
	fmt.Printf("%-20s  %-10s  %-10s  %s\n",
		strings.Repeat("─", 20), strings.Repeat("─", 10),
		strings.Repeat("─", 10), strings.Repeat("─", 20))
	for _, r := range results {
		status := "ok"
		if r.Err != "" {
			status = "error: " + r.Err
		}
		fmt.Printf("%-20s  %-10d  %-10d  %s\n", r.Provider, r.Exported, r.Skipped, status)
	}
	return nil
}
```

To enable this cleanly, refactor the existing `runExport` to extract `runExportForProvider(root, slug, typeFilter, nameFilter string, cmd *cobra.Command) (exportResult, error)` — this is the function both the single-provider path and the `--to all` path call.

**Success criteria:**
- [ ] `syllago export --to all` iterates through all providers and reports results
- [ ] `syllago export --to all --json` outputs a JSON array with per-provider results
- [ ] Errors for individual providers are reported but don't abort other providers
- [ ] `make test` passes

---

## Task 53 — Implement first-run experience in TUI

**Modifies:** `cli/internal/tui/app.go`

**Depends on:** Task 3 (`syllago init` scaffolds workspace — the first-run screen tells users to run `syllago import` or `syllago registry add`, both of which require a proper workspace to exist). Standalone in terms of code dependencies.

The first-run screen appears when the TUI launches with no content items and no registries. It replaces `renderContentWelcome` output with a focused getting-started guide.

The detection condition: `len(cat.Items) == 0 && len(cfg.Registries) == 0`. This is specific: if someone has registries but no local content, they get the normal welcome screen (they know how to use syllago). Only total empty state gets the first-run screen.

```go
// In renderContentWelcome, add at the top:

func (a App) renderContentWelcome() string {
	contentW := a.width - sidebarWidth - 1
	if contentW < 30 {
		contentW = 30
	}

	// First-run: no content and no registries
	if (a.catalog == nil || len(a.catalog.Items) == 0) &&
		(a.registryCfg == nil || len(a.registryCfg.Registries) == 0) {
		return a.renderFirstRun(contentW)
	}

	// ... existing welcome rendering ...
}

func (a App) renderFirstRun(contentW int) string {
	var s string

	s += titleStyle.Render("Welcome to syllago!") + "\n\n"
	s += helpStyle.Render("No content found. Here's how to get started:") + "\n\n"

	steps := []struct {
		num  string
		head string
		cmd  string
	}{
		{"1.", "Import existing content:", "syllago import --from claude-code"},
		{"2.", "Add a community registry:", "syllago registry add syllago-tools"},
		{"3.", "Create new content:", "syllago create skill my-first-skill"},
	}

	for _, step := range steps {
		s += labelStyle.Render(step.num) + " " + valueStyle.Render(step.head) + "\n"
		s += "   " + helpStyle.Render(step.cmd) + "\n\n"
	}

	s += helpStyle.Render("Press ? for help, q to exit.") + "\n"
	return s
}
```

The `syllago-tools` alias (Task 54) will expand this automatically.

**Success criteria:**
- [ ] First-run screen appears when catalog is empty and no registries configured
- [ ] Normal welcome screen appears when content or registries exist
- [ ] The three steps are clearly formatted with commands visible
- [ ] `make test` passes (golden tests will need updating)

---

## Task 54 — Implement registry short aliases

**Modifies:** `cli/internal/registry/registry.go`
**Modifies:** `cli/cmd/syllago/registry_cmd.go`

**Depends on:** Task 41 (alias expansion runs in `registryAddCmd.RunE` right before the `IsRegistryAllowed` check — Task 41 must already be in place so the enforcement sees the expanded full URL, not the short alias)

Registry short aliases let users type `syllago registry add syllago-tools` instead of the full GitHub URL. This is the same pattern as Homebrew tap shortcuts (`brew tap homebrew/cask` instead of the full URL).

The alias expansion happens in `registryAddCmd.RunE` before any other processing — the URL passed to `Clone` is always the fully-expanded URL.

```go
// In registry/registry.go, add:

// KnownAliases maps short names to full git URLs.
// These are the official syllago registries. Users can always use full URLs.
var KnownAliases = map[string]string{
	"syllago-tools": "https://github.com/OpenScribbler/syllago-tools.git",
}

// ExpandAlias returns the full URL for a known alias, or the input unchanged if not an alias.
// An alias is identified as not containing "/" or ":" — these characters appear in all valid git URLs.
func ExpandAlias(input string) (url string, expanded bool) {
	if !strings.Contains(input, "/") && !strings.Contains(input, ":") {
		if full, ok := KnownAliases[input]; ok {
			return full, true
		}
	}
	return input, false
}
```

```go
// In registryAddCmd.RunE, at the top of the function, after getting gitURL:

gitURL := args[0]
if fullURL, wasExpanded := registry.ExpandAlias(gitURL); wasExpanded {
	fmt.Fprintf(output.Writer, "Expanding alias %q → %s\n", gitURL, fullURL)
	gitURL = fullURL
}
```

**Test:** `cli/internal/registry/registry_test.go`

```go
func TestExpandAlias_KnownAlias(t *testing.T) {
	url, expanded := ExpandAlias("syllago-tools")
	require.True(t, expanded)
	require.Equal(t, "https://github.com/OpenScribbler/syllago-tools.git", url)
}

func TestExpandAlias_FullURL_NotExpanded(t *testing.T) {
	input := "https://github.com/acme/tools.git"
	url, expanded := ExpandAlias(input)
	require.False(t, expanded)
	require.Equal(t, input, url)
}

func TestExpandAlias_UnknownShortName_NotExpanded(t *testing.T) {
	url, expanded := ExpandAlias("some-random-name")
	require.False(t, expanded)
	require.Equal(t, "some-random-name", url)
}
```

**Success criteria:**
- [ ] `syllago registry add syllago-tools` expands to the full OpenScribbler URL
- [ ] Full URLs pass through unchanged
- [ ] Unknown short names pass through unchanged (not an error)
- [ ] Expansion is logged to stdout so users can see what happened
- [ ] `make test` passes

---

## Task 55 — Extract `findItemByPath` to shared helpers

**Modifies:** `cli/cmd/syllago/helpers.go`

**Depends on:** Task 46 (`findItemByPath` initially defined in `inspect.go` — extract it here before Task 50 uses it)

`findItemByPath` is defined in `inspect.go` (Task 46) and also needed in the promote-to-registry command (Task 50). Extract it to `helpers.go` before implementing Task 50 to avoid duplication.

```go
// findItemByPath looks up an item by "type/name", "type/provider/name",
// or "registry/type/name" path. Returns an error if not found.
func findItemByPath(cat *catalog.Catalog, path string) (*catalog.ContentItem, error) {
	parts := strings.Split(path, "/")
	switch len(parts) {
	case 2:
		ct := catalog.ContentType(parts[0])
		name := parts[1]
		for i := range cat.Items {
			if cat.Items[i].Type == ct && cat.Items[i].Name == name {
				return &cat.Items[i], nil
			}
		}
		return nil, fmt.Errorf("item %q not found (type=%s, name=%s)", path, ct, name)
	case 3:
		// Try registry/type/name first
		regName, ct, name := parts[0], catalog.ContentType(parts[1]), parts[2]
		for i := range cat.Items {
			if cat.Items[i].Registry == regName && cat.Items[i].Type == ct && cat.Items[i].Name == name {
				return &cat.Items[i], nil
			}
		}
		// Try type/provider/name (for provider-specific content)
		for i := range cat.Items {
			if cat.Items[i].Type == catalog.ContentType(parts[0]) &&
				cat.Items[i].Provider == parts[1] &&
				cat.Items[i].Name == parts[2] {
				return &cat.Items[i], nil
			}
		}
		return nil, fmt.Errorf("item %q not found (tried registry/type/name and type/provider/name)", path)
	default:
		return nil, fmt.Errorf("invalid item path %q: expected type/name, type/provider/name, or registry/type/name", path)
	}
}
```

Remove the duplicate definition from `inspect.go` and reference the one in `helpers.go`.

**Success criteria:**
- [ ] `inspect.go` and `promote_cmd.go` both use the shared `findItemByPath`
- [ ] No duplicate function definitions
- [ ] `make build` passes

---

## Sequence Summary

| Task | Feature | File(s) | Deps |
|------|---------|---------|------|
| 40 | `AllowedRegistries` in Config + `IsRegistryAllowed` | config/config.go | — |
| 41 | Enforce in `registry add` | registry_cmd.go | 40 |
| 42 | `RegistryManifest` loader | registry/registry.go | — |
| 43 | Manifest in `registry list` | registry_cmd.go | 42 |
| 44 | Manifest in TUI registries screen | tui/registries.go | 42 |
| 45 | `RiskIndicators` function | catalog/risk.go | — |
| 46 | `syllago inspect` command | cmd/syllago/inspect.go | 45 |
| 47 | Risk indicators in TUI detail | tui/detail_render.go | 45 |
| 48 | Content precedence deduplication | catalog/scanner.go, types.go | — |
| 49 | Override info in TUI detail | tui/detail_render.go | 48 |
| 50 | `syllago promote --to-registry` | promote/registry_promote.go | 42, 46→55 |
| 51 | `syllago sync-and-export` | cmd/syllago/sync_and_export.go | — |
| 52 | `syllago export --to all` | cmd/syllago/export.go | — |
| 53 | First-run TUI experience | tui/app.go | — |
| 54 | Registry short aliases | registry/registry.go, registry_cmd.go | — |
| 55 | Extract `findItemByPath` to helpers | cmd/syllago/helpers.go | 46 |

**Parallel opportunities:**
- Tasks 40-41 are independent from 42-44 — can be done in either order
- Tasks 45-47 (risk indicators) and 48-49 (precedence) are completely independent
- Tasks 51-54 (Phase 6) can all start in parallel after their deps are met
- Task 55 should be done immediately after Task 46, before Task 50
