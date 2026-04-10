# Release Readiness Phase 1: CI + Security Foundation — Implementation Plan

**Goal:** Establish CI quality gates and fix all security issues before the codebase goes public.

**Architecture:** Seven task groups: (1) CI gates — linting, race detection, go mod tidy, golangci-lint, Dependabot; (2-7) Security fixes — H1 script confirmation, H2 git clone protections, H3 symlink validation, H4 MCP name validation, M1 CleanStale race, M2 backup permissions, M3 temp path audit; (8) Registry scanning limits. Each security fix follows TDD: test first, then implementation.

**Tech Stack:** Go, GitHub Actions, golangci-lint, Dependabot

**Design Doc:** `docs/plans/2026-03-19-release-readiness-p1-design.md`

---

## Group 1: CI Quality Gates

### Task 1.1: Add go vet, race detection, and go mod tidy check to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

**Depends on:** Nothing

**Success Criteria:**
- [ ] CI runs `go vet ./...` as a separate step
- [ ] CI runs `go test -race ./...` instead of bare `go test ./...`
- [ ] CI verifies `go.mod` and `go.sum` are tidy
- [ ] All steps run in `cli` working directory

---

#### Step 1: Update `.github/workflows/ci.yml`

Replace the existing `Run tests` step and add new steps. Current file at line 25-27:

```yaml
      - name: Run tests
        working-directory: cli
        run: make test
```

New version of the relevant portion (replace lines 25-43, keeping surrounding structure):

```yaml
      - name: Run tests
        working-directory: cli
        run: go test ./...

      - name: Run tests with race detector
        working-directory: cli
        run: go test -race ./...

      - name: Run go vet
        working-directory: cli
        run: go vet ./...

      - name: Check go mod tidy
        working-directory: cli
        run: |
          go mod tidy
          git diff --exit-code go.mod go.sum

      - name: Build smoke check
        working-directory: cli
        run: go build ./cmd/syllago

      - name: Check commands.json freshness
        working-directory: cli
        run: |
          go build -o syllago-ci ./cmd/syllago
          ./syllago-ci _gendocs > commands.json.tmp
          grep -v '"generatedAt"' commands.json | grep -v '"syllagoVersion"' > commands.json.stable
          grep -v '"generatedAt"' commands.json.tmp | grep -v '"syllagoVersion"' > commands.json.tmp.stable
          diff -u commands.json.stable commands.json.tmp.stable
          rm -f syllago-ci commands.json.tmp commands.json.stable commands.json.tmp.stable
```

Note: Keep the two test steps separate so that the race detector failure is clearly labeled in CI output.

#### Step 2: Verify CI file is valid YAML

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo "OK"
```

Expected: `OK`

#### Step 3: Commit

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add go vet, race detector, and go mod tidy check"
```

---

### Task 1.2: Create golangci-lint configuration

**Files:**
- Create: `.golangci.yml`
- Modify: `.github/workflows/ci.yml`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `.golangci.yml` exists with minimal linter set (govet, gofmt, ineffassign, unused)
- [ ] CI runs `golangci-lint run ./...` in `cli/` directory
- [ ] Lint step fails fast (before tests) so feedback is immediate

---

#### Step 1: Create `.golangci.yml`

```yaml
# .golangci.yml — minimal baseline linter configuration
# Goal: consistency, not exhaustive analysis.
run:
  timeout: 5m
  modules-download-mode: readonly

linters:
  disable-all: true
  enable:
    - govet
    - gofmt
    - ineffassign
    - unused

linters-settings:
  govet:
    enable-all: false

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

#### Step 2: Add golangci-lint step to CI

In `.github/workflows/ci.yml`, add a new job before `test-and-build`:

```yaml
  lint:
    name: golangci-lint
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout repository
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - name: Set up Go
        uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version: '1.25.x'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          working-directory: cli
```

Also add `needs: lint` to the `test-and-build` job so lint runs first:

```yaml
  test-and-build:
    name: Go Test + Build
    needs: lint
    runs-on: ubuntu-latest
```

#### Step 3: Run locally to verify no existing violations

```bash
cd /home/hhewett/.local/src/syllago/cli
golangci-lint run ./...
```

Expected: No output (or warnings only, no errors). Fix any violations before committing.

#### Step 4: Commit

```bash
git add .golangci.yml .github/workflows/ci.yml
git commit -m "ci: add golangci-lint with minimal baseline configuration"
```

---

### Task 1.3: Create Dependabot configuration

**Files:**
- Create: `.github/dependabot.yml`

**Depends on:** Nothing (independent of 1.1 and 1.2)

**Success Criteria:**
- [ ] `.github/dependabot.yml` exists
- [ ] Covers `github-actions` ecosystem at root directory, weekly
- [ ] Covers `gomod` ecosystem at `/cli` directory, weekly

---

#### Step 1: Create `.github/dependabot.yml`

```yaml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10

  - package-ecosystem: "gomod"
    directory: "/cli"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
```

#### Step 2: Commit

```bash
git add .github/dependabot.yml
git commit -m "ci: add Dependabot for GitHub Actions and Go modules"
```

---

## Group 2: H1 (CRITICAL) — App Script Execution Confirmation

**Context:** `cli/internal/tui/detail.go` has `runAppScript()` at line 733 which executes `install.sh` via `tea.ExecProcess`. This is called from `app.go` line 204 in `handleConfirmAction()` for `modalAppScript`. The `modalAppScript` purpose already exists in the enum (`modal.go` line 365), so infrastructure is partially in place. The `confirmModal` body field currently holds only a title and short body string — it does not display the full script content. The fix requires showing the full script in the confirmation body so the user can read it before executing.

The existing `loadScriptPreview` function at line 753 only reads the first 10 lines. We need to read the full script and display it in the confirmation modal.

### Task 2.1: Write failing test for script confirmation modal

**Files:**
- Modify: `cli/internal/tui/app_test.go` (or the nearest test file for app-level behavior)

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test verifies that pressing the install key on an App item opens a modal with script content
- [ ] Test verifies that cancelling does NOT call `runAppScript`
- [ ] Test runs and fails before implementation

---

#### Step 1: Find or create the test file

```bash
ls /home/hhewett/.local/src/syllago/cli/internal/tui/*test* 2>/dev/null | head -5
```

#### Step 2: Write failing test

Add to the existing TUI test file (check which test file covers detail behavior):

```go
func TestAppScriptConfirmationModal(t *testing.T) {
    // Create a temp directory with an install.sh script
    dir := t.TempDir()
    scriptContent := "#!/bin/bash\necho 'Installing...'\ncp some/file /dest\n"
    err := os.WriteFile(filepath.Join(dir, "install.sh"), []byte(scriptContent), 0755)
    if err != nil {
        t.Fatal(err)
    }

    item := catalog.ContentItem{
        Name: "test-app",
        Type: catalog.Apps,
        Path: dir,
    }

    // Build a detailModel for the app item
    m := newDetailModel(item, nil, dir, &catalog.Catalog{RepoRoot: dir})

    // Simulate pressing 'i' (install key) on the Install tab
    m.activeTab = tabInstall
    result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
    _ = result

    // The update should return an openModalMsg with modalAppScript purpose
    if cmd == nil {
        t.Fatal("expected a command to open a modal, got nil")
    }
    msg := cmd()
    openMsg, ok := msg.(openModalMsg)
    if !ok {
        t.Fatalf("expected openModalMsg, got %T", msg)
    }
    if openMsg.purpose != modalAppScript {
        t.Errorf("expected modalAppScript purpose, got %v", openMsg.purpose)
    }
    // The modal body must contain the actual script content
    if !strings.Contains(openMsg.body, "echo 'Installing...'") {
        t.Errorf("modal body should contain script content, got: %q", openMsg.body)
    }
}
```

#### Step 3: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/tui/ -run TestAppScriptConfirmationModal -v
```

Expected: FAIL — either the test panics (Apps type not handled), the modal body doesn't contain script content, or the command returns nil.

---

### Task 2.2: Implement script confirmation modal

**Files:**
- Modify: `cli/internal/tui/detail.go` (lines 733-751, `runAppScript` and surrounding logic)

**Depends on:** Task 2.1

**Success Criteria:**
- [ ] Pressing `i` on an App item in the Install tab sends `openModalMsg` with full script content in body
- [ ] Modal body includes the complete `install.sh` content (not truncated to 10 lines)
- [ ] Modal title is `"Run install.sh?"`
- [ ] Modal purpose is `modalAppScript`
- [ ] `runAppScript` is only called after modal confirmation (already wired in `app.go` line 203-204)
- [ ] The `confirmModal` renders the body in a scrollable viewport so long scripts are fully accessible (not silently clipped)

---

#### Step 1: Update the install key handler for App items

In `cli/internal/tui/detail.go`, find the `keys.Install` case starting at line 403. Currently it calls `m.startInstall()` for all non-loadout items. We need to intercept App items:

The current flow at lines 403-430:

```go
case key.Matches(msg, keys.Install):
    if m.activeTab != tabInstall {
        break
    }
    if m.item.Type == catalog.Loadouts {
        break
    }
    // Guard: no providers detected
    if len(m.detectedProviders()) == 0 {
        m.message = "No providers detected for this content type"
        m.messageIsErr = true
        return m, nil
    }
    // ... hook compat warning check ...
    return m, m.startInstall()
```

Add an Apps guard before the existing guard (after the Loadouts break, before the detectedProviders check):

```go
case key.Matches(msg, keys.Install):
    if m.activeTab != tabInstall {
        break
    }
    if m.item.Type == catalog.Loadouts {
        break
    }
    // Apps: show full script content for user confirmation before executing
    if m.item.Type == catalog.Apps {
        scriptPath := filepath.Join(m.item.Path, "install.sh")
        data, err := os.ReadFile(scriptPath)
        if err != nil {
            m.message = "No install.sh found for this app"
            m.messageIsErr = true
            return m, nil
        }
        body := string(data)
        return m, func() tea.Msg {
            return openModalMsg{
                purpose: modalAppScript,
                title:   "Run install.sh?",
                body:    body,
            }
        }
    }
    // Guard: no providers detected
    if len(m.detectedProviders()) == 0 {
```

#### Step 2: Remove the old `loadScriptPreview` function (lines 753-765)

The `loadScriptPreview` function only reads 10 lines and was a preview shortcut. Since the full script is now shown in the modal body, remove it to avoid dead code:

```go
// DELETE this entire function (lines 753-765):
// func loadScriptPreview(itemPath string) string { ... }
```

Also remove the `appScriptPreview string` field from `detailModel` at line 85 if it is no longer populated anywhere. Verify with a search before removing.

#### Step 3: Verify `confirmModal` body renders in a scrollable viewport

The `confirmModal` body is currently rendered as a plain string (check `modal.go` around the `renderConfirmModal` or equivalent function). If the body is passed directly to `lipgloss.NewStyle().Render(m.body)` without a viewport, long scripts will be clipped at the modal boundary.

```bash
grep -n "body\|viewport\|scroll" /home/hhewett/.local/src/syllago/cli/internal/tui/modal.go | head -30
```

If the body is **already** rendered inside a `viewport.Model`, no change is needed — confirm and move on.

If the body is rendered as a plain string:
- Add a `viewport.Model` field to `confirmModal` (or the relevant modal struct)
- Initialize it with the modal's available height (total modal height minus title, buttons, and border rows)
- Set `viewport.SetContent(m.body)` when the modal opens (in the `openModalMsg` handler in `app.go`)
- Render the viewport instead of `m.body` in the modal's `View()` method
- Forward `tea.KeyMsg` scroll events (`pgup`, `pgdown`, `up`, `down`) to the viewport in `Update()`

The goal: a user must be able to scroll through arbitrarily long scripts before deciding to confirm or cancel.

#### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/tui/ -run TestAppScriptConfirmationModal -v
```

Expected: PASS

#### Step 5: Build and verify compile

```bash
cd /home/hhewett/.local/src/syllago/cli
make build
```

Expected: Build succeeds with no errors.

#### Step 6: Commit

```bash
git add cli/internal/tui/detail.go cli/internal/tui/modal.go
git commit -m "security: require explicit confirmation before running app install.sh (H1)"
```

---

## Group 3: H2 (HIGH) — Git Clone Protections

**Context:** Two places run `git clone`:
1. `cli/internal/registry/registry.go` line 126-130: `args := []string{"clone", url, dir}`
2. `cli/internal/tui/import.go` line 1538: `cmd := exec.Command("git", "clone", "--depth", "1", url, tmpDir)`

Both need three protections:
- `--no-recurse-submodules` — prevents submodule hook execution during clone
- `GIT_CONFIG_NOSYSTEM=1` env var — prevents system git config hooks from running
- `-c core.hooksPath=/dev/null` — disables all git hooks for this invocation

### Task 3.1: Write failing test for registry Clone protections

**Files:**
- Modify: `cli/internal/registry/registry_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test verifies that `Clone` passes `--no-recurse-submodules` to git
- [ ] Test verifies that `Clone` passes `-c core.hooksPath=/dev/null` to git
- [ ] Test verifies that `Clone` sets `GIT_CONFIG_NOSYSTEM=1` in the command environment
- [ ] Test runs and fails before implementation

---

#### Step 1: Write failing test

The registry test file at `cli/internal/registry/registry_test.go` likely uses `CacheDirOverride`. Add a test that captures the git command args:

```go
func TestCloneSafeArgs(t *testing.T) {
    // We can't run a real git clone in unit tests, but we can verify
    // the Clone function passes the right args by checking CombinedOutput
    // against a real git command on a bare local repo.
    dir := t.TempDir()
    orig := CacheDirOverride
    CacheDirOverride = dir
    t.Cleanup(func() { CacheDirOverride = orig })

    // Create a bare repo to clone from
    bareDir := t.TempDir()
    if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
        t.Skip("git not available")
    }

    // Clone it — we're checking args indirectly by checking the process
    // doesn't follow submodules and doesn't trigger hooks.
    // Since we can't inspect exec.Command fields after the fact, we verify
    // behavior: a clone of a bare repo with no hooks should succeed.
    err := Clone("file://"+bareDir, "test-safe", "")
    if err != nil {
        // Clone of empty bare repo is expected to fail with "empty repo" error
        // but should NOT fail with a hooks-related error
        if strings.Contains(err.Error(), "hook") {
            t.Errorf("unexpected hook-related error: %v", err)
        }
    }
}
```

Note: A more precise test inspects the `cmd.Args` and `cmd.Env` fields. Since `Clone` builds and runs the command internally, we can test this via a wrapper variable (see implementation below) or by verifying the git arguments in an integration test. The integration test approach is cleaner here — see Task 3.1 alternative below.

Alternative approach using a command interceptor:

```go
// In registry_test.go — capture the git args via a process that prints them
func TestClonePassesSafeArgs(t *testing.T) {
    // Write a fake git that prints its args to stderr and exits 0
    fakeGit := filepath.Join(t.TempDir(), "git")
    script := "#!/bin/sh\necho \"$@\" > " + fakeGit + ".args\nexit 0\n"
    if err := os.WriteFile(fakeGit, []byte(script), 0755); err != nil {
        t.Fatal(err)
    }

    // PATH override so exec.LookPath finds our fake git first
    origPath := os.Getenv("PATH")
    os.Setenv("PATH", filepath.Dir(fakeGit)+":"+origPath)
    t.Cleanup(func() { os.Setenv("PATH", origPath) })

    dir := t.TempDir()
    orig := CacheDirOverride
    CacheDirOverride = dir
    t.Cleanup(func() { CacheDirOverride = orig })

    Clone("https://example.com/repo.git", "test-repo", "")

    argsData, err := os.ReadFile(fakeGit + ".args")
    if err != nil {
        t.Fatal("fake git was not invoked")
    }
    args := string(argsData)
    if !strings.Contains(args, "--no-recurse-submodules") {
        t.Errorf("expected --no-recurse-submodules in args, got: %s", args)
    }
    if !strings.Contains(args, "core.hooksPath=/dev/null") {
        t.Errorf("expected -c core.hooksPath=/dev/null in args, got: %s", args)
    }
}
```

#### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/registry/ -run TestClonePassesSafeArgs -v
```

Expected: FAIL — args captured from fake git do not contain the safety flags.

---

### Task 3.2: Implement clone protections in registry.go

**Files:**
- Modify: `cli/internal/registry/registry.go` (lines 126-131)

**Depends on:** Task 3.1

**Success Criteria:**
- [ ] `Clone()` passes `--no-recurse-submodules`, `-c core.hooksPath=/dev/null` to git
- [ ] `Clone()` sets `GIT_CONFIG_NOSYSTEM=1` in `cmd.Env`
- [ ] Test from Task 3.1 passes

---

#### Step 1: Update the Clone function

Current code at lines 126-131:

```go
args := []string{"clone", url, dir}
if ref != "" {
    args = append(args, "--branch", ref)
}
cmd := exec.Command("git", args...)
out, err := cmd.CombinedOutput()
```

Replace with:

```go
args := []string{
    "clone",
    "--no-recurse-submodules",
    "-c", "core.hooksPath=/dev/null",
    url, dir,
}
if ref != "" {
    args = append(args, "--branch", ref)
}
cmd := exec.Command("git", args...)
cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
out, err := cmd.CombinedOutput()
```

Also add `"os"` to the import block if it is not already present (it is — `os.MkdirAll` is used at line 122).

#### Step 2: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/registry/ -run TestClonePassesSafeArgs -v
```

Expected: PASS

#### Step 3: Commit

```bash
git add cli/internal/registry/registry.go
git commit -m "security: add --no-recurse-submodules and hooksPath=dev/null to registry clone (H2)"
```

---

### Task 3.3: Implement clone protections in import.go

**Files:**
- Modify: `cli/internal/tui/import.go` (line 1538 — the `startClone` function)

**Depends on:** Task 3.2

**Success Criteria:**
- [ ] `startClone()` in import.go passes same three protections as registry.Clone
- [ ] Build passes

---

#### Step 1: Locate the `startClone` function

The relevant code at line 1538:

```go
cmd := exec.Command("git", "clone", "--depth", "1", url, tmpDir)
return tea.ExecProcess(cmd, func(err error) tea.Msg {
    return importCloneDoneMsg{err: err, path: tmpDir}
})
```

Replace with:

```go
cmd := exec.Command("git",
    "clone",
    "--depth", "1",
    "--no-recurse-submodules",
    "-c", "core.hooksPath=/dev/null",
    url, tmpDir,
)
cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
return tea.ExecProcess(cmd, func(err error) tea.Msg {
    return importCloneDoneMsg{err: err, path: tmpDir}
})
```

Note: `os` is already imported in import.go (line 7).

#### Step 2: Build to verify compile

```bash
cd /home/hhewett/.local/src/syllago/cli
make build
```

Expected: Build succeeds.

#### Step 3: Commit

```bash
git add cli/internal/tui/import.go
git commit -m "security: add --no-recurse-submodules and hooksPath=dev/null to import clone (H2)"
```

---

## Group 4: H3 (HIGH) — Symlink Validation in Scanner

**Context:** `cli/internal/catalog/scanner.go` has multiple `os.ReadFile` calls inside `scanRoot`, `scanFromIndex`, `scanUniversal`, `scanProviderSpecific`, and `scanProviderDir`. These all read files from registry paths without validating that the path doesn't escape the registry root via symlinks. Copy operations in installer already have symlink protection; scanner reads do not.

The fix: add a `validateRegistryPath` helper that resolves symlinks and checks the result stays within the registry root. Call it before reading metadata files in `scanRoot`.

### Task 4.1: Write failing test for symlink escape prevention

**Files:**
- Modify: `cli/internal/catalog/scanner_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test creates a registry directory with a symlink pointing outside the registry root
- [ ] Test verifies that scanning the registry returns a warning (not an error) for the escaping symlink
- [ ] Test verifies the escaping item is NOT included in the catalog
- [ ] Test runs and fails before implementation

---

#### Step 1: Write the failing test

```go
func TestScanRejectsSymlinkEscape(t *testing.T) {
    // Create registry root
    registryRoot := t.TempDir()
    // Create a sensitive file OUTSIDE the registry
    outsideDir := t.TempDir()
    secretFile := filepath.Join(outsideDir, "secret.txt")
    if err := os.WriteFile(secretFile, []byte("secret"), 0644); err != nil {
        t.Fatal(err)
    }

    // Create a skill dir inside registry that symlinks to the outside directory
    skillsDir := filepath.Join(registryRoot, "skills")
    if err := os.MkdirAll(skillsDir, 0755); err != nil {
        t.Fatal(err)
    }
    escapingSkill := filepath.Join(skillsDir, "evil-skill")
    if err := os.Symlink(outsideDir, escapingSkill); err != nil {
        t.Skipf("symlinks not supported: %v", err)
    }

    // Also create a legitimate skill so we can verify partial success
    legitimateSkill := filepath.Join(skillsDir, "legit-skill")
    if err := os.MkdirAll(legitimateSkill, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(legitimateSkill, "SKILL.md"), []byte("# Legit"), 0644); err != nil {
        t.Fatal(err)
    }

    cat, err := Scan(registryRoot, registryRoot)
    if err != nil {
        t.Fatalf("Scan returned error: %v", err)
    }

    // The legitimate skill should be present
    found := false
    for _, item := range cat.Items {
        if item.Name == "legit-skill" {
            found = true
        }
        if item.Name == "evil-skill" {
            t.Error("escaping symlink item should not be in catalog")
        }
    }
    if !found {
        t.Error("legitimate skill should be in catalog")
    }

    // A warning should have been emitted for the escaping symlink
    foundWarning := false
    for _, w := range cat.Warnings {
        if strings.Contains(w, "evil-skill") && strings.Contains(w, "escapes") {
            foundWarning = true
        }
    }
    if !foundWarning {
        t.Errorf("expected a warning about symlink escape, got warnings: %v", cat.Warnings)
    }
}
```

#### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/catalog/ -run TestScanRejectsSymlinkEscape -v
```

Expected: FAIL — `evil-skill` is currently included in the catalog, and no warning is generated.

---

### Task 4.2: Implement symlink validation in scanner

**Files:**
- Modify: `cli/internal/catalog/scanner.go`

**Depends on:** Task 4.1

**Success Criteria:**
- [ ] New `validateRegistryPath(path, registryRoot string) error` helper function
- [ ] `scanUniversal` calls `validateRegistryPath` before creating an item for each directory entry
- [ ] `scanProviderSpecific` calls `validateRegistryPath` before processing each item directory
- [ ] `scanFromIndex` calls `validateRegistryPath` before reading each item path from the index (this is the third scanner entry point that reads registry files and was not covered in the initial audit)
- [ ] Validation failures produce a warning and skip the item (not a hard error)
- [ ] Test from Task 4.1 passes

---

#### Step 1: Add the `validateRegistryPath` helper

Add after the `shouldSkip` function (after line 536):

```go
// validateRegistryPath resolves symlinks in path and verifies the result stays
// within registryRoot. Returns an error if the path escapes the registry boundary.
// This prevents symlink-based path traversal attacks from malicious registries.
func validateRegistryPath(path, registryRoot string) error {
    resolved, err := filepath.EvalSymlinks(path)
    if err != nil {
        return fmt.Errorf("resolving symlinks in %q: %w", path, err)
    }
    // Normalize both paths for prefix comparison
    rootClean := filepath.Clean(registryRoot)
    resolvedClean := filepath.Clean(resolved)
    if !strings.HasPrefix(resolvedClean, rootClean+string(filepath.Separator)) && resolvedClean != rootClean {
        return fmt.Errorf("path escapes registry boundary: %q resolves to %q outside %q", path, resolved, registryRoot)
    }
    return nil
}
```

#### Step 2: Add validation in `scanUniversal`

In `scanUniversal` (lines 302-366), after the `shouldSkip` check and before creating the `item`, add the symlink check:

Current code at lines 305-314:

```go
for _, entry := range entries {
    if !entry.IsDir() || shouldSkip(entry.Name()) {
        continue
    }
    if !IsValidItemName(entry.Name()) {
        cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — name contains characters unsafe for JSON key paths", entry.Name()))
        continue
    }

    itemDir := filepath.Join(typeDir, entry.Name())
```

Add after the `IsValidItemName` check:

```go
    itemDir := filepath.Join(typeDir, entry.Name())
    if err := validateRegistryPath(itemDir, baseDir); err != nil {
        cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — %s", entry.Name(), err))
        continue
    }
```

Note: `baseDir` is not directly available in `scanUniversal` — it receives `typeDir`. The registry root is the parent of `typeDir` (one level up). Pass the registry root as a parameter, or use `filepath.Dir(typeDir)` as the boundary. Use `filepath.Dir(typeDir)` since that's the content root:

```go
    itemDir := filepath.Join(typeDir, entry.Name())
    registryRoot := filepath.Dir(typeDir)
    if err := validateRegistryPath(itemDir, registryRoot); err != nil {
        cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — %s", entry.Name(), err))
        continue
    }
```

#### Step 3: Add validation in `scanProviderSpecific`

In `scanProviderSpecific` (lines 368-436), in the directory item branch at line 395, add before calling `scanProviderDir`:

```go
            if child.IsDir() {
                childDir := filepath.Join(providerDir, child.Name())
                registryRoot := filepath.Dir(filepath.Dir(providerDir))
                if err := validateRegistryPath(childDir, registryRoot); err != nil {
                    cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — %s", child.Name(), err))
                    continue
                }
                // New directory-per-item format
                item, err := scanProviderDir(filepath.Join(providerDir, child.Name()), ct, providerName, local)
```

#### Step 3b: Add validation in `scanFromIndex`

`scanFromIndex` reads item paths directly from a manifest/index file and is the third scanner entry point that reads files from registry paths. Audit the function body (search for `os.ReadFile` or path construction calls within it):

```bash
grep -n "scanFromIndex\|ReadFile\|filepath.Join" /home/hhewett/.local/src/syllago/cli/internal/catalog/scanner.go | head -40
```

For each item path constructed from the index entries, add validation before reading the file or passing the path downstream:

```go
// Pattern to apply for each item path derived from the index:
if err := validateRegistryPath(itemPath, registryRoot); err != nil {
    cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping index item %q — %s", itemPath, err))
    continue
}
```

The exact insertion point depends on how `scanFromIndex` iterates its entries. The rule: any path derived from index data (user-controlled content) must be validated before use.

#### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/catalog/ -run TestScanRejectsSymlinkEscape -v
```

Expected: PASS

#### Step 5: Run full test suite

```bash
cd /home/hhewett/.local/src/syllago/cli
make test
```

Expected: All tests pass.

#### Step 6: Commit

```bash
git add cli/internal/catalog/scanner.go
git commit -m "security: validate symlinks in registry scanner to prevent path traversal (H3)"
```

---

## Group 5: H4 (HIGH) — MCP Item Name Validation

**Context:** `cli/internal/installer/mcp.go` at line 188-194, `installMCP` iterates over the entries from `ExtractServerEntries` and uses each `name` directly as an sjson key path:

```go
for name, configData := range entries {
    key := jsonKey + "." + name
    fileData, err = sjson.SetRawBytes(fileData, key, configData)
```

The `name` comes from `item.Name` (flat format) or from the keys in the nested config.json (nested format). sjson uses `.` as a path separator, so a name like `..` or `foo.bar.baz` creates unintended nested paths instead of a single server entry. `catalog.IsValidItemName` already blocks dots from item names in the scanner, but `ExtractServerEntries` trusts nested config.json keys without validation.

### Task 5.1: Write failing test for MCP name injection

**Files:**
- Modify: `cli/internal/installer/mcp_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test verifies that `ExtractServerEntries` rejects keys containing dots
- [ ] Test verifies that the flat format item name is validated via `IsValidItemName`
- [ ] Test runs and fails before implementation

---

#### Step 1: Write the failing test

```go
func TestExtractServerEntriesRejectsDotKeys(t *testing.T) {
    // Nested format with a malicious key containing dots
    maliciousConfig := []byte(`{
        "mcpServers": {
            "legit-server": {"command": "npx", "args": ["legit"]},
            "evil..server": {"command": "npx", "args": ["evil"]}
        }
    }`)

    entries, err := ExtractServerEntries(maliciousConfig, "item-name", "mcpServers")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // legit-server should be present
    if _, ok := entries["legit-server"]; !ok {
        t.Error("expected legit-server to be present")
    }
    // evil..server should be rejected (dots in name)
    if _, ok := entries["evil..server"]; ok {
        t.Error("expected evil..server to be rejected due to dot injection risk")
    }
}

func TestExtractServerEntriesRejectsInvalidFlatName(t *testing.T) {
    // Flat format where item name contains invalid characters
    flatConfig := []byte(`{"command": "npx", "args": ["tool"]}`)

    // Item name with dots — should be rejected
    _, err := ExtractServerEntries(flatConfig, "evil.dot.name", "mcpServers")
    if err == nil {
        t.Error("expected error for item name with dots, got nil")
    }
}
```

#### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/installer/ -run TestExtractServerEntries -v
```

Expected: FAIL — the dot-containing keys pass through without rejection.

---

### Task 5.2: Implement MCP name validation in ExtractServerEntries

**Files:**
- Modify: `cli/internal/installer/mcp.go` (around lines 120-154, the `ExtractServerEntries` function)

**Depends on:** Task 5.1

**Success Criteria:**
- [ ] Nested format: keys with invalid characters (dots, slashes, etc.) are skipped with no error
- [ ] Flat format: invalid item name returns an error
- [ ] Uses `catalog.IsValidItemName` for all name validation
- [ ] Tests from Task 5.1 pass

---

#### Step 1: Locate `ExtractServerEntries`

The function starts around line 107 (inferred from the nested format handling at lines 120-141 that was visible in the file read). Look at the full function header:

```go
func ExtractServerEntries(rawData []byte, itemName string, jsonKey string) (map[string][]byte, error) {
```

#### Step 2: Add validation for nested format keys

In the nested format branch (lines ~125-137), the `wrapper.ForEach` callback uses `key.String()` directly:

```go
wrapper.ForEach(func(key, value gjson.Result) bool {
    var cfg MCPConfig
    if err := json.Unmarshal([]byte(value.Raw), &cfg); err != nil {
        return true // skip malformed entries
    }
    cleaned, err := json.Marshal(cfg)
    if err != nil {
        return true
    }
    entries[key.String()] = cleaned
    return true
})
```

Add a name validation check before using the key:

```go
wrapper.ForEach(func(key, value gjson.Result) bool {
    name := key.String()
    if !catalog.IsValidItemName(name) {
        return true // skip keys that would create invalid sjson paths
    }
    var cfg MCPConfig
    if err := json.Unmarshal([]byte(value.Raw), &cfg); err != nil {
        return true // skip malformed entries
    }
    cleaned, err := json.Marshal(cfg)
    if err != nil {
        return true
    }
    entries[name] = cleaned
    return true
})
```

#### Step 3: Add validation for flat format item name

In the flat format branch (lines ~143-153):

```go
// Flat format — the entire config.json is a single server definition
var cfg MCPConfig
if err := json.Unmarshal(rawData, &cfg); err != nil {
    return nil, fmt.Errorf("parsing config.json: %w", err)
}
cleaned, err := json.Marshal(cfg)
if err != nil {
    return nil, fmt.Errorf("serializing config: %w", err)
}
entries[itemName] = cleaned
return entries, nil
```

Add validation before using `itemName`:

```go
// Flat format — the entire config.json is a single server definition
if !catalog.IsValidItemName(itemName) {
    return nil, fmt.Errorf("invalid MCP item name %q: must contain only letters, numbers, hyphens, and underscores", itemName)
}
var cfg MCPConfig
```

Note: The `catalog` package is already imported in mcp.go (line 10).

#### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/installer/ -run TestExtractServerEntries -v
```

Expected: PASS

#### Step 5: Run full test suite

```bash
cd /home/hhewett/.local/src/syllago/cli
make test
```

Expected: All tests pass.

#### Step 6: Commit

```bash
git add cli/internal/installer/mcp.go
git commit -m "security: validate MCP server names to prevent sjson path injection (H4)"
```

---

## Group 6: M1 (MEDIUM) — CleanStale TOCTOU Race

**Context:** `cli/internal/sandbox/staging.go` lines 55-65, `CleanStale()`:

```go
func CleanStale() {
    entries, err := os.ReadDir("/tmp")
    if err != nil {
        return
    }
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), "syllago-sandbox-") {
            _ = os.RemoveAll(filepath.Join("/tmp", e.Name()))
        }
    }
}
```

The race: between `os.ReadDir` returning entry `e` and `os.RemoveAll` running, an attacker can replace the directory with a symlink to a sensitive path. `os.RemoveAll` follows the symlink and deletes the target. Fix: use `os.Lstat` to verify the entry is a real directory (not a symlink) before removing.

### Task 6.1: Write failing test for CleanStale symlink safety

**Files:**
- Modify: `cli/internal/sandbox/staging_test.go` (create if it doesn't exist)

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test creates a `/tmp/syllago-sandbox-XXXX` symlink pointing to a temp directory
- [ ] Test verifies `CleanStale` does NOT remove the symlink target (only the symlink itself, or skips it)
- [ ] Test runs and fails (current code calls `os.RemoveAll` without Lstat check)

---

#### Step 1: Check if staging_test.go exists

```bash
ls /home/hhewett/.local/src/syllago/cli/internal/sandbox/
```

#### Step 2: Write the failing test

```go
package sandbox

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestCleanStaleDoesNotFollowSymlinks(t *testing.T) {
    // Create a temp directory that simulates a sensitive target
    sensitiveDir := t.TempDir()
    sensitiveFile := filepath.Join(sensitiveDir, "important.txt")
    if err := os.WriteFile(sensitiveFile, []byte("important"), 0644); err != nil {
        t.Fatal(err)
    }

    // Create a syllago-sandbox symlink in /tmp pointing to the sensitive dir
    symlinkName := "syllago-sandbox-test-toctou-" + filepath.Base(t.TempDir())
    symlinkPath := filepath.Join("/tmp", symlinkName)
    if err := os.Symlink(sensitiveDir, symlinkPath); err != nil {
        t.Skipf("cannot create symlink in /tmp: %v", err)
    }
    t.Cleanup(func() { os.Remove(symlinkPath) })

    CleanStale()

    // The sensitive file must still exist after CleanStale
    if _, err := os.Stat(sensitiveFile); os.IsNotExist(err) {
        t.Error("CleanStale followed a symlink and deleted the target directory contents")
    }

    // The symlink itself can be removed (that's fine) or left (also fine)
    // What's NOT acceptable is the target being deleted
}
```

#### Step 3: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/sandbox/ -run TestCleanStaleDoesNotFollowSymlinks -v
```

Expected: FAIL — current `os.RemoveAll` deletes via the symlink, removing the target directory contents.

---

### Task 6.2: Implement Lstat check in CleanStale

**Files:**
- Modify: `cli/internal/sandbox/staging.go` (lines 55-65)

**Depends on:** Task 6.1

**Success Criteria:**
- [ ] `CleanStale` uses `os.Lstat` to check entry type before removing
- [ ] Only entries that are regular directories (not symlinks) are removed
- [ ] Test from Task 6.1 passes

---

#### Step 1: Update `CleanStale`

Current code at lines 55-65:

```go
func CleanStale() {
    entries, err := os.ReadDir("/tmp")
    if err != nil {
        return
    }
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), "syllago-sandbox-") {
            _ = os.RemoveAll(filepath.Join("/tmp", e.Name()))
        }
    }
}
```

Replace with:

```go
func CleanStale() {
    entries, err := os.ReadDir("/tmp")
    if err != nil {
        return
    }
    for _, e := range entries {
        if !strings.HasPrefix(e.Name(), "syllago-sandbox-") {
            continue
        }
        fullPath := filepath.Join("/tmp", e.Name())
        // Use Lstat (not Stat) to avoid following symlinks.
        // Only remove actual directories — skip symlinks to prevent TOCTOU attacks.
        info, err := os.Lstat(fullPath)
        if err != nil {
            continue
        }
        if info.Mode()&os.ModeSymlink != 0 {
            continue // skip symlinks
        }
        if !info.IsDir() {
            continue // skip non-directories
        }
        _ = os.RemoveAll(fullPath)
    }
}
```

#### Step 2: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/sandbox/ -run TestCleanStaleDoesNotFollowSymlinks -v
```

Expected: PASS

#### Step 3: Build and run full test suite

```bash
cd /home/hhewett/.local/src/syllago/cli
make test
make build
```

Expected: All tests pass, build succeeds.

#### Step 4: Commit

```bash
git add cli/internal/sandbox/staging.go
git commit -m "security: use Lstat in CleanStale to prevent TOCTOU symlink race (M1)"
```

---

## Group 7: M2 (MEDIUM) — Backup File Permissions

**Context:** `cli/internal/installer/jsonmerge.go` line 73:

```go
func backupFile(path string) error {
    data, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) {
        return nil
    }
    if err != nil {
        return err
    }
    return os.WriteFile(path+".bak", data, 0644)  // line 73 — always 0644
}
```

The `writeJSONFile` function (lines 25-33) applies context-sensitive permissions: `0600` for home directory files, `0644` for project files. The `backupFile` function always writes `0644`, meaning backups of home-directory settings files (like `~/.claude/claude_desktop_config.json`) are world-readable when they should be `0600`. The fix: apply the same permission logic to the backup file.

### Task 7.1: Write failing test for backup file permissions

**Files:**
- Modify: `cli/internal/installer/jsonmerge_test.go` (create if it doesn't exist)

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test verifies backup of a home-directory file gets `0600` permissions
- [ ] Test verifies backup of a non-home-directory file gets `0644` permissions
- [ ] Test runs and fails (current code always writes `0644`)

---

#### Step 1: Check for existing test file

```bash
ls /home/hhewett/.local/src/syllago/cli/internal/installer/
```

#### Step 2: Write the failing test

Use `t.TempDir()` as the fake home directory and override `HOME` via env so `os.UserHomeDir()` returns it. This keeps all file I/O within the test's temp tree — no writes to the actual home directory.

```go
package installer

import (
    "os"
    "path/filepath"
    "testing"
)

func TestBackupFilePermissions(t *testing.T) {
    // Use a fake home directory to avoid writing to the real home dir.
    fakeHome := t.TempDir()
    t.Setenv("HOME", fakeHome) // overrides os.UserHomeDir() for this test

    // Create a file inside the fake home directory
    homeFile := filepath.Join(fakeHome, "settings.json")
    if err := os.WriteFile(homeFile, []byte(`{"key":"value"}`), 0600); err != nil {
        t.Fatal(err)
    }

    if err := backupFile(homeFile); err != nil {
        t.Fatalf("backupFile failed: %v", err)
    }

    info, err := os.Stat(homeFile + ".bak")
    if err != nil {
        t.Fatalf("backup file not created: %v", err)
    }
    perm := info.Mode().Perm()
    if perm != 0600 {
        t.Errorf("backup in home dir should have 0600 permissions, got %o", perm)
    }
}

func TestBackupFilePermissionsProjectFile(t *testing.T) {
    // Project file lives outside the home directory — use a separate temp dir.
    dir := t.TempDir()
    projectFile := filepath.Join(dir, "settings.json")
    if err := os.WriteFile(projectFile, []byte(`{"key":"value"}`), 0644); err != nil {
        t.Fatal(err)
    }

    if err := backupFile(projectFile); err != nil {
        t.Fatalf("backupFile failed: %v", err)
    }

    info, err := os.Stat(projectFile + ".bak")
    if err != nil {
        t.Fatalf("backup file not created: %v", err)
    }
    perm := info.Mode().Perm()
    if perm != 0644 {
        t.Errorf("backup in project dir should have 0644 permissions, got %o", perm)
    }
}
```

Note: `t.Setenv` (available since Go 1.17) automatically restores the original `HOME` value when the test completes — no manual `t.Cleanup` needed for the env var.

#### Step 3: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/installer/ -run TestBackupFilePermissions -v
```

Expected: FAIL — home directory backup gets `0644` instead of `0600`.

---

### Task 7.2: Implement context-sensitive backup file permissions

**Files:**
- Modify: `cli/internal/installer/jsonmerge.go` (lines 64-74)

**Depends on:** Task 7.1

**Success Criteria:**
- [ ] `backupFile` applies `0600` for home-directory files, `0644` for project files
- [ ] Uses the same logic as `writeJSONFile` (lines 25-33)
- [ ] Tests from Task 7.1 pass

---

#### Step 1: Update `backupFile`

Current code at lines 64-74:

```go
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
```

Replace with:

```go
func backupFile(path string) error {
    data, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) {
        return nil // nothing to back up
    }
    if err != nil {
        return err
    }
    perm := os.FileMode(0644)
    if home, err := os.UserHomeDir(); err == nil {
        if strings.HasPrefix(path, home+string(filepath.Separator)) {
            perm = 0600
        }
    }
    return os.WriteFile(path+".bak", data, perm)
}
```

Note: `strings` and `filepath` are already imported in jsonmerge.go (lines 10, 12).

#### Step 2: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/installer/ -run TestBackupFilePermissions -v
```

Expected: PASS

#### Step 3: Commit

```bash
git add cli/internal/installer/jsonmerge.go
git commit -m "security: apply 0600 permissions to backup files in home directory (M2)"
```

---

## Group 8: M3 (MEDIUM) — Predictable Temp Paths + Full File Creation Audit

**Context:** `cli/internal/loadout/apply.go` `writeJSONFileAtomic` at lines 477-491 uses:

```go
tmpPath := path + ".tmp"
```

This is a predictable temp path. The `installer/jsonmerge.go` pattern uses a random suffix:

```go
suffix := make([]byte, 8)
rand.Read(suffix)
tempPath := path + ".tmp." + hex.EncodeToString(suffix)
```

The loadout package also needs a full audit of all file write operations across the codebase.

### Task 8.1: Audit all file write operations for temp path safety

**Files:**
- Read-only audit: entire `cli/` directory

**Depends on:** Nothing

**Success Criteria:**
- [ ] List of all locations using predictable temp paths (`.tmp` suffix without random component)
- [ ] Verification that `writeJSONFileWithPerm` in jsonmerge.go (the gold standard) uses random suffix
- [ ] Audit results recorded in commit message

---

#### Step 1: Search for all temp file patterns

```bash
cd /home/hhewett/.local/src/syllago/cli
grep -rn '\.tmp"' --include="*.go" .
grep -rn 'os.WriteFile\|ioutil.WriteFile' --include="*.go" . | grep -v '_test.go'
grep -rn 'os.Rename' --include="*.go" . | grep -v '_test.go'
```

Review the results. Known locations:
- `internal/loadout/apply.go` line 482: `tmpPath := path + ".tmp"` — NEEDS FIX
- `internal/installer/jsonmerge.go` line 48: `tempPath := path + ".tmp." + hex.EncodeToString(suffix)` — ALREADY SAFE

Record any additional locations found for fixing in Task 8.2.

---

### Task 8.2: Write failing test for loadout atomic write

**Files:**
- Modify: `cli/internal/loadout/apply_test.go`

**Depends on:** Task 8.1

**Success Criteria:**
- [ ] Test verifies that `writeJSONFileAtomic` does NOT use a predictable temp path
- [ ] Specifically: no `.tmp` file without a random suffix exists during/after the write
- [ ] Test runs and fails before implementation

---

#### Step 1: Write the failing test

```go
func TestWriteJSONFileAtomicRandomSuffix(t *testing.T) {
    dir := t.TempDir()
    targetPath := filepath.Join(dir, "settings.json")

    // Collect any .tmp files created during the write
    var tmpFiles []string

    // Run writeJSONFileAtomic and immediately check for .tmp files
    err := writeJSONFileAtomic(targetPath, []byte(`{"key":"value"}`))
    if err != nil {
        t.Fatalf("writeJSONFileAtomic failed: %v", err)
    }

    // After successful write, no .tmp file should remain
    entries, _ := os.ReadDir(dir)
    for _, e := range entries {
        if strings.HasSuffix(e.Name(), ".tmp") {
            tmpFiles = append(tmpFiles, e.Name())
        }
    }
    if len(tmpFiles) > 0 {
        t.Errorf("predictable .tmp files left behind: %v", tmpFiles)
    }

    // The function should NOT use exactly "settings.json.tmp" (predictable)
    // We verify this by checking the implementation uses a random suffix.
    // Since we can't intercept the temp path from outside, we verify indirectly:
    // run two concurrent writes and verify they don't conflict on the temp name.
    var wg sync.WaitGroup
    errors := make([]error, 5)
    for i := range errors {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            errors[idx] = writeJSONFileAtomic(targetPath, []byte(fmt.Sprintf(`{"run":%d}`, idx)))
        }(i)
    }
    wg.Wait()
    // At least one should succeed (last writer wins); none should return an error
    // that's caused by temp file collision
    for i, err := range errors {
        if err != nil {
            t.Errorf("concurrent write %d failed: %v", i, err)
        }
    }
}
```

#### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/loadout/ -run TestWriteJSONFileAtomicRandomSuffix -v
```

Expected: FAIL or RACE — concurrent writes collide on the predictable `.tmp` path.

---

### Task 8.3: Fix predictable temp path in loadout/apply.go

**Files:**
- Modify: `cli/internal/loadout/apply.go` (lines 477-491, `writeJSONFileAtomic`)

**Depends on:** Task 8.2

**Success Criteria:**
- [ ] `writeJSONFileAtomic` uses a random hex suffix matching `installer/jsonmerge.go` pattern
- [ ] Function adds `crypto/rand` and `encoding/hex` imports if not already present
- [ ] Test from Task 8.2 passes

---

#### Step 1: Check current imports in apply.go

Current imports (lines 1-17):

```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
    ...
)
```

Add `"crypto/rand"` and `"encoding/hex"` to imports.

#### Step 2: Update `writeJSONFileAtomic`

Current code at lines 477-491:

```go
func writeJSONFileAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    tmpPath := path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }
    if err := os.Rename(tmpPath, path); err != nil {
        os.Remove(tmpPath)
        return err
    }
    return nil
}
```

Replace with:

```go
func writeJSONFileAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    // Use a random suffix to avoid predictable temp path collisions.
    suffix := make([]byte, 8)
    if _, err := rand.Read(suffix); err != nil {
        return fmt.Errorf("generating temp suffix: %w", err)
    }
    tmpPath := path + ".tmp." + hex.EncodeToString(suffix)
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }
    if err := os.Rename(tmpPath, path); err != nil {
        os.Remove(tmpPath)
        return err
    }
    return nil
}
```

#### Step 3: Fix any other predictable temp paths found in Task 8.1 audit

For each additional location found during the audit, apply the same pattern (random hex suffix). Apply fixes here before committing.

#### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/loadout/ -run TestWriteJSONFileAtomicRandomSuffix -v
```

Expected: PASS

#### Step 5: Run full test suite with race detector

```bash
cd /home/hhewett/.local/src/syllago/cli
go test -race ./...
```

Expected: All tests pass with no race conditions detected.

#### Step 6: Build

```bash
cd /home/hhewett/.local/src/syllago/cli
make build
```

Expected: Build succeeds.

#### Step 7: Commit

```bash
git add cli/internal/loadout/apply.go
# Add any other files fixed during audit
git commit -m "security: use random temp suffix in writeJSONFileAtomic to prevent TOCTOU (M3)"
```

---

## Group 9: Registry Scanning Limits

**Context:** `cli/internal/catalog/scanner.go` has no bounds on how much content a registry can push into the scanner. A malicious registry could exhaust memory or CPU by providing thousands of deeply nested directories. Fix: add configurable limits enforced during `scanRoot` with graceful degradation (warning + skip, not hard abort).

### Task 9.1: Write failing test for scanning limits

**Files:**
- Modify: `cli/internal/catalog/scanner_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Test creates a registry with 10,001 items
- [ ] Test verifies scan stops at 10,000 items and emits a warning
- [ ] Test runs and fails before implementation

---

#### Step 1: Write the failing test

```go
func TestScanEnforcesFileLimit(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping file limit test in short mode")
    }

    registryRoot := t.TempDir()
    skillsDir := filepath.Join(registryRoot, "skills")
    if err := os.MkdirAll(skillsDir, 0755); err != nil {
        t.Fatal(err)
    }

    // Create 10,001 skill directories (one over the default limit)
    limit := DefaultScanLimits().MaxFiles
    for i := 0; i <= limit; i++ {
        skillDir := filepath.Join(skillsDir, fmt.Sprintf("skill-%05d", i))
        if err := os.MkdirAll(skillDir, 0755); err != nil {
            t.Fatal(err)
        }
        if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
            t.Fatal(err)
        }
    }

    cat, err := Scan(registryRoot, registryRoot)
    if err != nil {
        t.Fatalf("Scan returned error: %v", err)
    }

    if len(cat.Items) > limit {
        t.Errorf("scan should stop at %d items, got %d", limit, len(cat.Items))
    }

    foundWarning := false
    for _, w := range cat.Warnings {
        if strings.Contains(w, "limit") {
            foundWarning = true
        }
    }
    if !foundWarning {
        t.Errorf("expected a limit warning, got: %v", cat.Warnings)
    }
}
```

#### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/catalog/ -run TestScanEnforcesFileLimit -v
```

Expected: FAIL — scan returns all 10,001 items without a warning.

---

### Task 9.2: Implement scanning limits

**Files:**
- Modify: `cli/internal/catalog/scanner.go`

**Depends on:** Task 9.1

**Success Criteria:**
- [ ] `ScanLimits` struct with `MaxFiles int`, `MaxDepth int`, `MaxBytes int64` fields
- [ ] `DefaultScanLimits()` returns `{MaxFiles: 10000, MaxDepth: 50, MaxBytes: 500 * 1024 * 1024}`
- [ ] `scanRoot` accepts a `ScanLimits` parameter and enforces limits
- [ ] Public `Scan` and `ScanWithRegistries` functions use `DefaultScanLimits()`
- [ ] When a limit is hit: emit a warning to `cat.Warnings`, stop scanning the current registry gracefully
- [ ] Test from Task 9.1 passes

---

#### Step 1: Add `ScanLimits` type and defaults

Add near the top of scanner.go, after the existing var declarations:

```go
// ScanLimits configures bounds on registry scanning to prevent runaway resource use.
type ScanLimits struct {
    MaxFiles int   // maximum total items across all content types
    MaxDepth int   // maximum directory nesting depth
    MaxBytes int64 // maximum total size of scanned files
}

// DefaultScanLimits returns conservative defaults for production use.
func DefaultScanLimits() ScanLimits {
    return ScanLimits{
        MaxFiles: 10000,
        MaxDepth: 50,
        MaxBytes: 500 * 1024 * 1024, // 500 MB
    }
}
```

#### Step 2: Add item counter to `scanRoot`

The cleanest approach is to pass a `*scanState` (mutable counter) through `scanRoot` and check it at the top of `scanUniversal` and `scanProviderSpecific` before appending each item.

Add a `scanState` struct:

```go
type scanState struct {
    itemCount int
    limits    ScanLimits
    limitHit  bool
}

func (s *scanState) canAdd() bool {
    if s.limits.MaxFiles > 0 && s.itemCount >= s.limits.MaxFiles {
        s.limitHit = true
        return false
    }
    return true
}

func (s *scanState) add() {
    s.itemCount++
}
```

#### Step 3: Thread `scanState` through scan functions

Update `scanRoot` signature to accept `*scanState`, and update `scanUniversal` and `scanProviderSpecific` similarly. The public `Scan` and `ScanWithRegistries` functions create the state:

```go
func scanRoot(cat *Catalog, baseDir string, local bool) error {
    state := &scanState{limits: DefaultScanLimits()}
    return scanRootWithState(cat, baseDir, local, state)
}

func scanRootWithState(cat *Catalog, baseDir string, local bool, state *scanState) error {
    items, _ := loadManifestItems(baseDir)
    if len(items) > 0 {
        return scanFromIndexWithState(cat, baseDir, items, local, state)
    }
    for _, ct := range AllContentTypes() {
        if state.limitHit {
            cat.Warnings = append(cat.Warnings, fmt.Sprintf("scan limit reached (%d items) — some content was skipped", state.limits.MaxFiles))
            return nil
        }
        // ... rest of existing loop ...
    }
    return nil
}
```

In `scanUniversal` and `scanProviderSpecific`, before `cat.Items = append(cat.Items, item)`:

```go
if !state.canAdd() {
    return nil // limit reached, stop scanning this type
}
state.add()
cat.Items = append(cat.Items, item)
```

Note: The exact threading of `state` through the call chain requires updating function signatures. The key contract: each public-facing scan function creates a fresh `scanState` with `DefaultScanLimits()`. The `limitHit` flag stops further scanning gracefully, and the warning is emitted once.

#### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/catalog/ -run TestScanEnforcesFileLimit -v
```

Expected: PASS

#### Step 5: Run full test suite

```bash
cd /home/hhewett/.local/src/syllago/cli
make test
make build
```

Expected: All tests pass, build succeeds.

#### Step 6: Commit

```bash
git add cli/internal/catalog/scanner.go
git commit -m "security: add registry scanning limits (10K files, 50 depth, 500MB) (M3-scan)"
```

---

## Final Verification

After all tasks are complete, run the full suite with race detector and verify CI would pass:

```bash
cd /home/hhewett/.local/src/syllago/cli
go test -race ./...
go vet ./...
make build
```

Expected: All clean.

```bash
# Verify git log shows all commits
git log --oneline -12
```

Expected: All 10+ commits present in correct logical order.
