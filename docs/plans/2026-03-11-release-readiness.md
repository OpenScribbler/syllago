# Syllago Release Readiness Plan

Status: DRAFT
Created: 2026-03-11
Updated: 2026-03-19

## Overview

Comprehensive checklist to prepare syllago for its first public release. Organized by workstream with priority ordering within each.

---

## 1. README Rewrite (Critical — First thing users see)

The README is substantially outdated. Commands, providers, content types, and TUI docs all need updating.

### Inaccuracies to fix:
- [ ] **Commands table**: `import`/`export` are deprecated — replace with `add`/`install` as primary commands
- [ ] **Missing 11+ commands**: add, install, uninstall, convert, create, list, inspect, remove, share, publish, loadout (with subcommands: apply, create, list, remove, status), config paths
- [ ] **Provider table**: Missing 5 providers — Zed, Cline, RooCode, OpenCode, Kiro (11 total, not 6)
- [ ] **Sidebar categories**: Lists Prompts/Apps (not in TUI); misses Loadouts
- [ ] **Content types**: Document 7 managed types: Skills, Agents, MCP, Rules, Hooks, Commands, Loadouts
- [ ] **Keyboard shortcuts**: Missing keys p, a, d, r, l, H
- [ ] **Sandbox subcommands**: Missing info, allow-env, deny-env, allow-port, deny-port, ports
- [ ] **Example workflows**: Update all examples to use add/install instead of import/export

### New sections needed:
- [ ] **Loadouts**: Explain what they are, how to use them
- [ ] **VHS GIF demos** (placeholders for now): converting content, creating loadouts, using registries

---

## 2. Missing Repo Files (High — Community/security expectations)

Files to create, modeled after phyllotaxis:

### 2a. SECURITY.md
Syllago-specific scope:
- Malicious content packages (rules, skills, agents, hooks)
- Path traversal during import/export
- MCP config injection via JSON merge
- Registry trust model (community registries are unverified)
- Hooks/MCP execute arbitrary code
- Contact: openscribbler.dev@pm.me
- 48-hour acknowledgment, 7-day fix target

### 2b. VERSIONING.md
- Semver policy (same structure as phyllotaxis)
- Pre-1.0 convention (additive minors)
- Release process: VERSION file + Go ldflags + `make release`

### 2c. CHANGELOG.md
- Backfill from existing release notes in `releases/` directory
- Follow Keep a Changelog format

### 2d. .github/dependabot.yml
```yaml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
  - package-ecosystem: "gomod"
    directory: "/cli"
    schedule:
      interval: "weekly"
```

### 2e. .github/workflows/pr-policy.yml
Auto-close external PRs with redirect to issue templates. Allow: holdenhewett, OpenScribblerOwner, dependabot[bot].

### 2f. Update CONTRIBUTING.md
Add "Why no code?" section and mention that external PRs will be auto-closed.

---

## 3. Security Fixes (High — Before public exposure)

### 3a. HIGH Priority

**H1. App install.sh execution without sandboxing**
- File: `cli/internal/tui/detail.go:784-800`
- Fix: Show script content and require explicit confirmation before executing app install scripts from registries

**H2. Git clone without submodule/hook protections**
- Files: `cli/internal/registry/registry.go:103-131`, `cli/internal/tui/import.go:1333`
- Fix: Add `--no-recurse-submodules` to all `git clone` invocations. Set `GIT_CONFIG_NOSYSTEM=1` and `-c core.hooksPath=/dev/null`

**H3. Symlinks in registry content not validated**
- File: `cli/internal/catalog/scanner.go`
- Fix: Add symlink checks when scanning registry content directories. Validate targets stay within registry root before reading files.

### 3b. MEDIUM Priority

**M1. CleanStale() symlink race (TOCTOU)**
- File: `cli/internal/sandbox/staging.go:55-65`
- Fix: Use `os.Lstat` before removal to verify entry is a directory

**M2. Backup file permissions**
- File: `cli/internal/installer/jsonmerge.go:73`
- Fix: Apply same `0600` permission logic from `writeJSONFile` to `backupFile`

**M3. Predictable temp path in loadout**
- File: `cli/internal/loadout/apply.go:472-486`
- Fix: Use random suffix for temp file (match pattern in `installer/jsonmerge.go`)

**M4. MCP item name as JSON key without validation**
- File: `cli/internal/installer/mcp.go:137-139`
- Fix: Validate item name with `IsValidItemName` before using as sjson key

---

## 4. CI/CD Hardening (Medium — Security posture)

### Already solid:
- Actions pinned to SHAs
- Permissions scoped correctly
- Cosign signing on releases
- CGO_ENABLED=0 for all builds
- PR actor guards on Claude workflows

### Additions:
- [ ] **dependabot.yml** (covered in section 2d)
- [ ] **SBOM generation**: Add `syft` step to release workflow, include in release artifacts
- [ ] **Homebrew auth modernization**: Optional — migrate git clone with PAT to gh cli auth (low priority)

---

## 5. Distribution Channels (Medium — Accessibility)

### Already have:
- GitHub Releases (6 targets: linux/darwin/windows x amd64/arm64)
- install.sh (curl-to-sh with checksum verification)
- Homebrew tap (auto-updated on release via release.yml)

### GoReleaser Decision

GoReleaser is the industry standard for Go CLI releases (used by gh, Helm, Trivy, Charm tools, Cosign). It replaces our manual cross-compilation with a single `.goreleaser.yml` that handles:
- Cross-compilation (all 6 targets)
- Archives: `.tar.gz` (Unix), `.zip` (Windows) with LICENSE/README included
- GitHub release creation with auto-generated checksums
- Cosign signing
- SBOM generation
- Homebrew formula auto-generation

**Recommendation**: Adopt GoReleaser. It eliminates script maintenance and the current manual release.yml build steps. Our existing release.yml is solid but GoReleaser would replace ~100 lines of YAML with a declarative config that's easier to extend.

**Alternative**: Keep current manual approach — it works, and GoReleaser adds a dev dependency. This is a polish decision, not a blocker.

### Binary Naming Convention

Current: `syllago-linux-amd64` (bare binary, no version, no archive)
Industry standard: `syllago-VERSION-OS-ARCH.tar.gz`

For v1:
- [ ] Include version in filenames: `syllago-0.7.0-linux-amd64.tar.gz`
- [ ] Archive binaries (tar.gz for Unix, zip for Windows) — includes LICENSE and README
- [ ] Keep bare binary option for install.sh compatibility (or update install.sh to extract from archive)

### For v1 release:
- [ ] **go install** support — verify `go install github.com/OpenScribbler/syllago/cli/cmd/syllago@latest` works
- [ ] **Scoop manifest** (Windows) — create scoop bucket or add to homebrew-tap repo
- [ ] Decide: GoReleaser adoption vs keeping manual release.yml

### Defer to post-v1 (demand-driven):
- AUR, apt/deb, Nix, Snap, Docker, winget

---

## 6. Update UX (Low — Already ahead of most Go CLIs)

### Current state is good:
- `syllago update` CLI command exists
- TUI update screen exists
- SHA-256 checksum verification on downloads

### Optional improvements (post-v1):
- [ ] Update notification on session start (check once per day, store timestamp)
- [ ] `syllago update --check` flag to preview without applying
- [ ] Consider `creativeprojects/go-selfupdate` library for more robust self-update

---

## 7. VHS Demo Tapes (Medium — Marketing/README)

Three showcase GIFs for the README:

### 7a. Converting content between providers
- Show adding a Cursor rule, then installing it to Claude Code
- Demonstrates the cross-provider value prop

### 7b. Creating and applying loadouts
- Show creating a loadout, then applying it
- Demonstrates the curated bundle concept

### 7c. Using the registry system
- Show adding a registry, browsing it in TUI, installing content
- Demonstrates the community content ecosystem

These need VHS `.tape` files. Charm's VHS tool records terminal sessions to GIF.

---

## Progress (as of 2026-03-19)

**Completed since creation:**
- CLI Reference infrastructure (not originally tracked here): `gendocs` command, `commands.json` manifest, Cobra Example fields on all 52 commands, Astro Starlight sync pipeline in `syllago-docs` repo
- TUI Polish and Registry Experience workstreams fully complete (see v1-release-strategy.md)

**Not yet started:** All items in sections 1-7 below remain open.

## Execution Order

1. **Security fixes** (H1-H3, M1-M4) — Fix before anyone clones the repo
2. **README rewrite** — Accurate first impression
3. **Missing repo files** (SECURITY.md, VERSIONING.md, CHANGELOG.md, dependabot, pr-policy)
4. **CI/CD hardening** (SBOM, dependabot)
5. **Distribution channels** (Scoop, go install verification)
6. **VHS demos** (after README structure is final)
7. **Update UX polish** (post-v1)

---

## Signing & Security Posture

**Current state**: Cosign keyless signing of checksums — this is sufficient and modern.

**Not needed for v1**: GPG signatures (Cosign's Sigstore transparency log is the modern equivalent), macOS notarization (not required for CLI tools distributed via Homebrew/direct download), reproducible builds (Cosign transparency log provides equivalent assurance).

**Consider for later**: SLSA provenance attestation (enterprise credibility).
