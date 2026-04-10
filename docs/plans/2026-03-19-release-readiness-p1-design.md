# Release Readiness Phase 1: CI + Security Foundation

*Design date: 2026-03-19*
*Status: Design Complete*
*Phase: 1 of 5*
*Dependencies: None — this phase unblocks everything else*

## Overview

Establish CI quality gates and fix all security issues before the codebase goes public. This is the "make it safe to publish" phase.

## Context

Four expert agent audits (coding, security, accessibility, UX) reviewed the release readiness plan. This phase addresses all findings from the coding and security audits that must land before public exposure.

---

## Work Items

### 1. CI Quality Gates

**1a. Linting and race detection in CI**

Add to `.github/workflows/ci.yml`:
- `go vet ./...` — catches common Go mistakes
- `go test -race ./...` — critical for concurrent TUI code (goroutines for registry sync, toast async updates, etc.)
- `go mod tidy` check — ensures go.mod/go.sum stay in sync with source

**1b. golangci-lint configuration**

Create `.golangci.yml` with minimal baseline:
- vet, gofmt, ineffassign, unused
- Not strict — consistency is the goal, not exhaustive analysis
- Run in CI alongside existing test job

**1c. Dependabot configuration**

Create `.github/dependabot.yml`:
- `github-actions` ecosystem (weekly, root directory)
- `gomod` ecosystem (weekly, `/cli` directory)

### 2. Security Fixes

**2a. H1 (CRITICAL): App install.sh execution without confirmation**
- File: `cli/internal/tui/detail.go` (~line 784-800)
- Current: Executes bash scripts from registry content silently
- Fix: Show full script content in a scrollable confirmation modal. Two buttons: "Cancel" (default, focused) and "Run Script". User must explicitly confirm.
- Test: Verify modal appears, can be dismissed, cancellation prevents execution.

**2b. H2 (HIGH): Git clone without protections**
- Files: `cli/internal/registry/registry.go` (~line 103-131), `cli/internal/tui/import.go` (~line 1333)
- Fix: Add three protections to all `git clone` invocations:
  - `--no-recurse-submodules` (prevents submodule hook execution)
  - `GIT_CONFIG_NOSYSTEM=1` env var (prevents system-level git hooks)
  - `-c core.hooksPath=/dev/null` (disables all git hooks during clone)

**2c. H3 (HIGH): Symlink validation in scanner**
- File: `cli/internal/catalog/scanner.go`
- Context: Copy operations (`copyFile`, `copyDir`) already have symlink protection. Scanner reads do not.
- Fix: Before reading metadata files from registry paths, validate with `filepath.EvalSymlinks()` + prefix check to ensure path doesn't escape registry root.
- Pattern:
  ```go
  resolved, err := filepath.EvalSymlinks(path)
  if err != nil || !strings.HasPrefix(resolved, registryRoot) {
      return fmt.Errorf("path escapes registry boundary: %s", path)
  }
  ```

**2d. H4 (HIGH, upgraded from M4): MCP item name validation**
- File: `cli/internal/installer/mcp.go` (~line 137-139)
- Risk: Item name used as sjson key without validation. Characters like `..` could create unintended JSON paths.
- Fix: Add `IsValidItemName(name)` check before the `sjson.SetRawBytes` call.

**2e. M1 (MEDIUM): CleanStale() TOCTOU race**
- File: `cli/internal/sandbox/staging.go` (~line 55-65)
- Fix: Use `os.Lstat` before `os.RemoveAll` to verify the entry is a directory, not a symlink.

**2f. M2 (MEDIUM): Backup file permissions**
- File: `cli/internal/installer/jsonmerge.go` (~line 73)
- Fix: Apply same permission logic from `writeJSONFile` (0600 for home-dir files, 0644 for project files) to `backupFile`.

**2g. M3 (MEDIUM): Predictable temp paths + full file creation audit**
- Known issue: `cli/internal/loadout/apply.go` (~line 472-486) uses `.tmp` suffix without random component
- Fix: Update to use random suffix matching `jsonmerge.go` pattern (`".tmp." + hex.EncodeToString(suffix)`)
- Audit: Search entire codebase for all file write operations. Verify all atomic writes use random suffixes and safe patterns. Fix any that don't.

### 3. Registry Scanning Limits

- Add configurable limits to registry scanning:
  - Max 10,000 files per registry
  - Max 50 directory depth
  - Max 500MB total size
- Defaults in code, overridable via `syllago.yaml` configuration
- Log a warning when limits are hit, skip remaining content gracefully

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| H1 severity | Upgraded to CRITICAL | Silent script execution is the worst finding |
| M4 severity | Upgraded to HIGH (H4) | JSON key injection is a real attack vector |
| H3 scope | Scanner reads only | Copy operations already protected |
| Temp audit scope | Full file creation audit | LLM effort is minimal; thoroughness is free |
| Scanning limits | Conservative defaults | 10K/50/500MB covers legitimate registries |
| golangci-lint strictness | Minimal baseline | Consistency, not exhaustive analysis |
| GoReleaser | Deferred to post-v1 | Current release.yml works fine |
| SBOM | Deferred to post-v1 | Cosign signing sufficient |

---

## Out of Scope

- GoReleaser adoption (deferred)
- SBOM generation (deferred)
- Scoop manifest (deferred)
- Code refactoring beyond security fixes
