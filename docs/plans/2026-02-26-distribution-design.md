# Distribution Workstream Design

*Design date: 2026-02-26*
*Status: Draft*

---

## 1. Design Summary

The Distribution workstream makes syllago installable for v1.0 launch. Without it, the only installation path is `git clone` + `make build` — a non-starter for general users. We are building: a CI pipeline that validates every PR, a release workflow that builds six cross-platform binaries, signs them with cosign (Sigstore keyless signing), and publishes them to GitHub Releases; an install script hosted on GitHub raw URLs; a Homebrew tap for macOS users; and a shared `internal/updater` package that handles binary self-update from GitHub Releases, consumed by both a new `syllago update` CLI command and the existing TUI update screen. The current TUI update screen is git-based and will be adapted to use the updater package for release builds (when `buildCommit` is empty), while the git path remains for dev builds.

---

## 2. Architecture

### CI Pipeline (test/build on PR)

Every pull request triggers `.github/workflows/ci.yml`. It runs `go test ./...` and a `go build` smoke check using Go 1.25.x. No matrix — single `ubuntu-latest` runner is sufficient. This workflow gates merges and gives confidence before release.

### Release Workflow (tag push → publish)

Pushing a `v*` tag triggers `.github/workflows/release.yml`. The workflow:

1. Checks out the repo with `fetch-depth: 0` (tags needed for version embedding).
2. Builds all six binaries using the existing `build-*` Makefile targets (after adding `build-windows-arm64`). The `VERSION` file and git tag are embedded via ldflags. Crucially, `buildCommit` is left **empty** for release builds — this is the signal that disables self-rebuild and switches the TUI updater to the GitHub Releases path.
3. Generates `checksums.txt` with SHA-256 hashes of all binaries using `sha256sum`.
4. Signs `checksums.txt` with cosign keyless signing using GitHub OIDC — no keys to manage.
5. Creates a GitHub Release via `gh release create`, attaches all binaries, `checksums.txt`, and the cosign bundle (`checksums.txt.bundle`).
6. Updates the Homebrew formula in `openscribbler/homebrew-tap` (see below).

The `LDFLAGS` for release builds omit `buildCommit`:
```
-X main.version=$(VERSION) -X main.repoRoot=
```
`repoRoot` is set empty so `findContentRepoRoot` falls through to CWD/binary-path walk (correct for installed binaries).

### Install Script (`install.sh`)

A shell script at the repo root, served via GitHub raw URL:
```
https://raw.githubusercontent.com/OpenScribbler/syllago/main/install.sh
```

The script:
1. Detects OS (`uname -s`) and architecture (`uname -m`), maps to binary name.
2. Fetches the latest release tag from `api.github.com/repos/OpenScribbler/syllago/releases/latest`.
3. Downloads the correct binary and `checksums.txt` from the release assets.
4. Verifies SHA-256 checksum locally with `sha256sum -c` or `shasum -a 256 -c`.
5. Installs to `${INSTALL_DIR:-$HOME/.local/bin}`.
6. If `~/.local/bin` is not on `$PATH`, prints friendly guidance (does not modify shell config).
7. No prompts. Prints what it's doing. Exits non-zero on any error.

### Homebrew Tap (`openscribbler/homebrew-tap`)

A separate GitHub repository (`openscribbler/homebrew-tap`) contains the formula. The release workflow commits an updated `Formula/syllago.rb` to that repo after publishing the GitHub Release. The formula:

- Downloads `syllago-darwin-arm64` or `syllago-darwin-amd64` based on `Hardware::CPU.arm?`.
- Verifies SHA-256.
- Installs to `bin/syllago`.
- No dependencies (static binary, CGO_ENABLED=0).

Install command: `brew install openscribbler/tap/syllago`

### `internal/updater` Package

New package at `cli/internal/updater/`. It contains two exported functions:

```go
type ReleaseInfo struct {
    Version     string // e.g. "0.5.0" (no "v" prefix)
    TagName     string // e.g. "v0.5.0"
    Body        string // release notes markdown
    UpdateAvail bool
}

func CheckLatest(currentVersion string) (ReleaseInfo, error)
func Update(currentVersion string, progress func(string)) error
```

**CheckLatest:** Calls `GET https://api.github.com/repos/OpenScribbler/syllago/releases/latest` with a 15-second timeout. Parses `tag_name` and `body`. Compares versions using semver logic.

**Update:**
1. Calls `CheckLatest`. If no update, returns early.
2. Identifies the correct asset name for `runtime.GOOS`/`runtime.GOARCH`.
3. Downloads binary and `checksums.txt` to temp dir.
4. Verifies SHA-256 of downloaded binary against checksums.
5. Replaces current binary: `os.Rename` on Unix (atomic), two-step on Windows.
6. Returns nil on success. Caller tells user to restart.

The package has no TUI dependencies. Progress is communicated via callback.

### `syllago update` CLI Command

New Cobra command in `main.go`. Calls `updater.Update(version, ...)`. For dev builds (`buildCommit != ""`), prints a message suggesting `make build` instead.

### TUI Update Screen Adaptation

The existing `update.go` is git-based. Adaptation is conditional on `isReleaseBuild` (derived from `buildCommit == "" && version != ""`):

- **Release build:** `checkForUpdate` calls `updater.CheckLatest` instead of git fetch. `startPull` calls `updater.Update` instead of git pull. Release notes come from the GitHub Release body.
- **Dev build:** Existing git-based behavior unchanged.

---

## 3. Implementation Tasks

Tasks are ordered by dependency. Each task is a self-contained unit of work.

---

### Task 1: CI Pipeline

**Files to create:**
- `.github/workflows/ci.yml`

**What it does:** Runs on every PR (and push to `main`). Single job: checkout, setup Go 1.25.x, `go test ./...`, `go build ./cmd/syllago` smoke check.

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
      - name: Test
        run: make test
        working-directory: cli
      - name: Build smoke check
        run: go build ./cmd/syllago
        working-directory: cli
```

**Testing:** Open a PR with a deliberate test failure, verify CI blocks. Open a clean PR, verify CI passes.

---

### Task 2: Add `windows/arm64` Build Target + Naming Consistency

**Files to modify:**
- `cli/Makefile`

**Changes:**
- Add `build-windows-arm64` target.
- Fix `build-windows-amd64` output to `$(BINARY)-windows-amd64.exe` for naming consistency.
- Update `build-all` to include the new target.

**Testing:** Run `make build-all` locally. Verify all 6 binaries are produced with consistent names.

---

### Task 3: Release Workflow

**Files to create:**
- `.github/workflows/release.yml`

**Trigger:** Push to tags matching `v*`.

**Permissions:** `contents: write`, `id-token: write`

**Secrets needed:** `HOMEBREW_TAP_TOKEN` (PAT with write access to `openscribbler/homebrew-tap`)

**Steps:**
1. Checkout with `fetch-depth: 0`
2. Setup Go 1.25.x
3. Extract version from tag: `VERSION=${GITHUB_REF_NAME#v}`
4. Build all 6 targets with release ldflags (no `buildCommit`, no `repoRoot`)
5. Generate `checksums.txt` via `sha256sum`
6. Cosign keyless sign `checksums.txt`
7. Read release notes from `releases/v${VERSION}.md`
8. Create GitHub Release via `gh release create` with all assets
9. Update Homebrew formula (Task 8)

**Testing:** Push a test tag, verify all 6 binaries + checksums + cosign bundle appear in the release.

---

### Task 4: `internal/updater` Package

**Files to create:**
- `cli/internal/updater/updater.go`
- `cli/internal/updater/updater_test.go`

**Exports:** `CheckLatest(currentVersion string) (ReleaseInfo, error)`, `Update(currentVersion string, progress func(string)) error`

**Details:**
- GitHub API at `api.github.com/repos/OpenScribbler/syllago/releases/latest`
- Asset naming: `syllago-{os}-{arch}` with `.exe` suffix for Windows
- SHA-256 verification before binary replacement
- Atomic rename on Unix, two-step on Windows
- 15-second HTTP timeout
- `User-Agent: syllago/{version}` header

**Testing:** Mock HTTP server for `CheckLatest`. Checksum verification with bad hash. Asset name mapping table test for all 6 GOOS/GOARCH combos.

---

### Task 5: `syllago update` CLI Command

**Files to modify:**
- `cli/cmd/syllago/main.go`

**Changes:** Add `updateCmd` Cobra command, registered in `init()`. Calls `updater.Update(version, ...)`. For dev builds, prints guidance message instead.

**Testing:** Run on dev build, verify message. Integration test (manual) on older release binary.

---

### Task 6: Adapt TUI Update Screen for Release Builds

**Files to modify:**
- `cli/cmd/syllago/main.go` — derive and pass `isReleaseBuild`
- `cli/internal/tui/app.go` — accept `isReleaseBuild`, thread to updateModel
- `cli/internal/tui/update.go` — conditional behavior based on `isReleaseBuild`

**Changes:**
- `isReleaseBuild = buildCommit == "" && version != ""`
- `checkForUpdate` branches: release build → `updater.CheckLatest`, dev build → git fetch
- `startPull` branches: release build → `updater.Update`, dev build → git pull
- Release notes from GitHub Release body (release build) vs git log (dev build)

**Testing:** Unit tests with mock updater for release path. Verify no git exec calls when `isReleaseBuild=true`. Existing git-path tests unchanged.

---

### Task 7: Install Script

**Files to create:**
- `install.sh` (repo root)

**Features:**
- Detects OS (`uname -s`) and arch (`uname -m`)
- Fetches latest release from GitHub API
- Downloads binary and checksums
- Verifies SHA-256 (sha256sum or shasum)
- Installs to `${INSTALL_DIR:-$HOME/.local/bin}`
- PATH guidance if needed
- No prompts, clear output

**Testing:** Run on Linux amd64. Test `INSTALL_DIR` override. Test checksum failure with corrupted binary.

---

### Task 8: Homebrew Tap

**One-time setup:**
- Create `openscribbler/homebrew-tap` repo on GitHub
- Add `HOMEBREW_TAP_TOKEN` secret to syllago repo

**Files to create (in homebrew-tap repo):**
- `Formula/syllago.rb`

**Formula:** Downloads correct binary for macOS architecture, verifies SHA-256, installs to `bin/syllago`.

**Auto-update:** Release workflow extracts SHA-256s from checksums.txt, clones homebrew-tap, updates formula version and hashes, commits and pushes.

**Testing:** `brew install openscribbler/tap/syllago`, verify `syllago version` output.

---

## 4. Testing Strategy

| Component | Test Type | Details |
|-----------|-----------|---------|
| CI workflow | Integration | PR with failing test → blocks. Clean PR → passes. |
| Build targets | Local | `make build-all` produces 6 consistently-named binaries |
| `internal/updater` — CheckLatest | Unit | Mock HTTP server with known JSON response |
| `internal/updater` — checksum | Unit | Bad hash → error, temp file deleted |
| `internal/updater` — asset names | Unit | Table test for all 6 GOOS/GOARCH combos |
| `internal/updater` — Update | Manual | Older release binary → `syllago update` → version changes |
| `syllago update` — dev build | Unit | `buildCommit != ""` → prints dev message |
| TUI update — release build | Unit | Mock `updater.CheckLatest`, no git exec calls |
| Install script | Manual | Linux amd64, macOS arm64, INSTALL_DIR override, checksum failure |
| Homebrew | Manual | `brew install`, `brew test`, version check |
| Release workflow | Integration | Push test tag, verify all assets |

---

## 5. Security Considerations

**Cosign keyless signing (Sigstore):** `checksums.txt` signed via GitHub OIDC. Verification:
```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.bundle \
  --certificate-identity-regexp "https://github.com/OpenScribbler/syllago/.github/workflows/release.yml" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

**SHA-256 verification:** Both install script and `internal/updater` verify checksums before installing.

**Binary replacement atomicity:** `os.Rename` on Unix (atomic). Two-step pattern on Windows.

**No privilege escalation:** Install to `~/.local/bin` by default. No sudo required.

**Hardcoded API endpoint:** `api.github.com` only. No user-configurable update server.

**Pinned CI actions:** `actions/checkout@v4`, `actions/setup-go@v5`.

---

## 6. Open Questions

**Q1: Cosign verification in install script?**
Not included — requires `cosign` installed. Document as optional manual step for security-conscious users.

**Q2: Binary replacement on Windows while running?**
`os.Rename` fails on running executables. For v1, document that `syllago update` should be run from CLI mode on Windows (not TUI). Post-v1, consider `.pending-update` marker pattern.

**Q3: GitHub API rate limiting?**
60 requests/hour unauthenticated. Update checks are manual (not on every TUI launch), so this is fine for v1. Post-v1, cache last check time.

**Q4: Homebrew Linux support?**
macOS only for v1. Linux users use the install script. Add `on_linux` blocks post-v1.

**Q5: TUI update screen for dev builds?**
Existing git-based path remains unchanged. The `isReleaseBuild` flag cleanly separates the two code paths.
