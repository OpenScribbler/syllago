# provmon drift detection — Implementation Plan

**Goal:** Replace `ErrUnimplementedDetectionMethod` in `cli/internal/provmon/CheckVersion` with working `source-hash` detection for windsurf, kiro, cursor, amp, and copilot-cli. Migrate `ProviderVersion` → `ChangeDetection.Baseline` in the same bead to clean up schema semantics.

**Architecture:** Two commits. Commit 1 is a mechanical schema rename + method unification (`content-hash` + `github-commits` → `source-hash`, `ProviderVersion` → `ChangeDetection.Baseline`) with no behavior change. Commit 2 wires up the new detection: provmon imports capmon's `LoadFormatDoc`, `FetchSource`, `FetchChromedp`, `SHA256Hex`, and `ValidateContentResponse`, loops over each manifest source, and emits per-source `SourceDrift` with statuses `stable | drifted | skipped | fetch_failed | content_invalid`. Provmon is read-only against FormatDoc — seeding is always via `capmon extract` (backfill for 4 providers runs inside commit 2).

**Tech Stack:** Go 1.23 (WSL) / Go 1.25 (CI); `gopkg.in/yaml.v3`; `net/http/httptest`; `github.com/OpenScribbler/syllago/cli/internal/capmon` (direct cross-package import). No new dependencies.

**Design Doc:** [docs/plans/2026-04-21-provmon-drift-detection-design.md](./2026-04-21-provmon-drift-detection-design.md)

**Bead:** syllago-5gthn

---

## Task Index

### Commit 1 — Schema migration (no behavior change)

| # | Task |
|---|------|
| 1 | Add `Baseline` field to `ChangeDetection`; rename `VersionDrift.ManifestVersion` → `.Baseline`; update `CheckReport.ProviderVersion` → `CheckReport.Baseline` |
| 2 | Unify `content-hash` + `github-commits` method names to `source-hash` in Go types and docs |
| 3 | Update `CheckVersion` github-releases branch to read from `Baseline` field |
| 4 | Update provmon tests (`checker_test.go`, `manifest_test.go`) for migrated schema |
| 5 | Update `cli/cmd/provider-monitor/main.go` output formatting + struct reads |
| 6 | Update JSON schema (`docs/provider-sources/manifest.schema.json`) |
| 7 | Migrate 16 YAML manifests in `docs/provider-sources/` (including `_template.yaml`) |
| 8 | Run full test suite + `make fmt`; commit commit 1 |

### Commit 2 — Detection implementation

| # | Task |
|---|------|
| 9 | Backfill capmon FormatDocs: windsurf |
| 10 | Backfill capmon FormatDocs: amp |
| 11 | Backfill capmon FormatDocs: cursor (chromedp) |
| 12 | Resolve copilot-cli URL drift + re-extract |
| 13 | Add `SourceDriftStatus` constants + `SourceDrift` struct + `VersionDrift.Sources` |
| 14 | Write RED test: source-hash happy path (all sources stable) |
| 15 | Write RED test: source-hash drift path |
| 16 | Write RED test: source-hash skipped paths (3 reasons) |
| 17 | Write RED test: source-hash fetch_failed path |
| 18 | Write RED test: source-hash content_invalid path |
| 19 | Implement `checkSourceHash` in new file `checker_source_hash.go` (makes tests 14–18 GREEN) |
| 20 | Delete `TestCheckVersion_UnimplementedMethods` + `ErrUnimplementedDetectionMethod` sentinel |
| 21 | Update `RunCheck` to populate `VersionDrift` via the new branch |
| 22 | Write RED test: `--fail-on` flag default = drifted only |
| 23 | Write RED test: `--fail-on=drifted,fetch_failed` opt-in |
| 24 | Implement `--fail-on` flag parsing + exit-code computation |
| 25 | Update `printReports` for per-source DRIFT lines |
| 26 | Add `capmon extract` sequencing note to `docs/adding-a-provider.md` |
| 27 | End-to-end smoke run (opt-in via `SYLLAGO_TEST_NETWORK=1`) |
| 28 | Run full test suite + `make fmt`; commit commit 2 |

**Total: 28 tasks.** Each task is a single focused action (2–5 minutes of work for a human, one bead for Beads tracking).

---

# Commit 1 — Schema Migration

## Task 1: Add `Baseline` field to `ChangeDetection`; rename drift-result field

**Files:**
- Modify: `cli/internal/provmon/manifest.go` (lines 20–36, 47–51)
- Modify: `cli/internal/provmon/checker.go` (lines 39–58, 50)

**Depends on:** none

### Success Criteria
- `cd cli && go build ./internal/provmon/...` → pass — package compiles with new fields
- `cd cli && go vet ./internal/provmon/...` → pass — no vet warnings
- `grep -q 'Baseline string' cli/internal/provmon/manifest.go` → pass — new field added
- `grep -q 'ManifestVersion' cli/internal/provmon/checker.go` → fail — old field name gone

### Step 1: Edit `manifest.go`

Remove `ProviderVersion` from the `Manifest` struct (line 23). Add `Baseline` to `ChangeDetection`:

```go
// ChangeDetection defines how to detect when provider content changes.
type ChangeDetection struct {
	Method   string `yaml:"method"`             // github-releases | source-hash
	Endpoint string `yaml:"endpoint,omitempty"` // informational for source-hash; required for github-releases
	Baseline string `yaml:"baseline,omitempty"` // version tag (github-releases) — opaque comparison reference
}
```

(Note: the narrower `Method` comment is applied in Task 2 when `source-hash` is introduced. For this task, retain the original comment text but drop `ProviderVersion`.)

### Step 2: Edit `checker.go`

Rename `VersionDrift.ManifestVersion` → `.Baseline`. Rename `CheckReport.ProviderVersion` → `CheckReport.Baseline`:

```go
// CheckReport is the full report for one provider manifest.
type CheckReport struct {
	Slug         string
	DisplayName  string
	Status       string
	FetchTier    string
	URLResults   []URLResult
	VersionDrift *VersionDrift
	TotalURLs    int
	FailedURLs   int
	LastVerified string
	Baseline     string // was ProviderVersion
}

// VersionDrift describes when the provider's latest version differs from what was last verified.
type VersionDrift struct {
	Baseline      string // what the manifest records (version tag, for github-releases)
	LatestVersion string // what the API says
	Drifted       bool
}
```

Update the two `CheckVersion` writes (lines 156–160, `ManifestVersion: m.ProviderVersion` → `Baseline: m.ChangeDetection.Baseline`; `m.ProviderVersion != ""` → `m.ChangeDetection.Baseline != ""`; compare against `m.ChangeDetection.Baseline`).

Update the `RunCheck` report assignment (line 183): `ProviderVersion: m.ProviderVersion` → `Baseline: m.ChangeDetection.Baseline`.

### Step 3: Verify compilation

```bash
cd cli && go build ./internal/provmon/...
```

Expected: builds without error. Tests will fail — that's Task 4.

---

## Task 2: Unify `content-hash` + `github-commits` → `source-hash`

**Files:**
- Modify: `cli/internal/provmon/manifest.go` (line 49 comment only)
- Modify: `cli/internal/provmon/checker.go` (lines 14–22, 112–125)

**Depends on:** Task 1

### Success Criteria
- `grep -q '"source-hash"' cli/internal/provmon/checker.go` → pass — new method string added
- `grep -q 'content-hash\|github-commits' cli/internal/provmon/checker.go` → fail — old method strings purged (in switch; comments may still reference in migration context, removed in Task 20)
- `cd cli && go build ./internal/provmon/...` → pass — package compiles

### Step 1: Update comment on `ChangeDetection.Method` in `manifest.go:49`

```go
Method   string `yaml:"method"` // github-releases | source-hash
```

### Step 2: Update the CheckVersion switch in `checker.go:121`

Replace the single line `case "content-hash", "github-commits":` with `case "source-hash":`. Do not touch the rest of the function body — only the case label changes in this task. Keep the sentinel `ErrUnimplementedDetectionMethod` in place for now; Task 19 wires up real detection, Task 20 deletes the sentinel.

Old (line 121):
```go
	case "content-hash", "github-commits":
```

New (line 121):
```go
	case "source-hash":
```

All other lines of `CheckVersion` (117–161) remain byte-for-byte unchanged.

### Step 3: Verify compilation

```bash
cd cli && go build ./internal/provmon/...
```

---

## Task 3: Update `CheckVersion` github-releases branch reads

**Files:**
- Modify: `cli/internal/provmon/checker.go` (lines 156–160, 183)

**Depends on:** Task 1, Task 2

### Success Criteria
- `grep -q 'm.ChangeDetection.Baseline' cli/internal/provmon/checker.go` → pass — new field read
- `grep -q 'm.ProviderVersion' cli/internal/provmon/checker.go` → fail — no references to removed field
- `cd cli && go build ./internal/provmon/...` → pass — builds

### Step 1: Replace reads in `CheckVersion` (lines 156–160)

Old:
```go
return &VersionDrift{
	ManifestVersion: m.ProviderVersion,
	LatestVersion:   release.TagName,
	Drifted:         m.ProviderVersion != "" && release.TagName != m.ProviderVersion,
}, nil
```

New:
```go
return &VersionDrift{
	Baseline:      m.ChangeDetection.Baseline,
	LatestVersion: release.TagName,
	Drifted:       m.ChangeDetection.Baseline != "" && release.TagName != m.ChangeDetection.Baseline,
}, nil
```

### Step 2: Replace read in `RunCheck` (line 183)

Old:
```go
ProviderVersion: m.ProviderVersion,
```

New:
```go
Baseline: m.ChangeDetection.Baseline,
```

### Step 3: Verify

```bash
cd cli && go build ./internal/provmon/...
```

---

## Task 4: Update provmon tests for migrated schema

**Files:**
- Modify: `cli/internal/provmon/checker_test.go` (lines 86–93, 101–108, 125–132, 145–181, 228–238)
- Modify: `cli/internal/provmon/manifest_test.go` (lines 12–54, 97–119, 161–186)

**Depends on:** Task 3

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run 'TestCheckVersion_GitHubReleases|TestCheckVersion_NoDrift|TestCheckVersion_UnimplementedMethods|TestRunCheck|TestLoadManifest'` → pass — all existing tests green with new schema
- `grep -q 'ManifestVersion' cli/internal/provmon/checker_test.go` → fail — old field name removed from tests
- `grep -q 'provider_version:' cli/internal/provmon/manifest_test.go` → fail — old YAML key removed
- `grep -Eq '"content-hash"|"github-commits"' cli/internal/provmon/checker_test.go` → fail — test fixtures purged of old method strings

### Step 1: Migrate `checker_test.go` Manifest fixtures

For `TestCheckVersion_GitHubReleases` (line 86), `TestCheckVersion_NoDrift` (line 125), and `TestRunCheck` (line 228), change:

```go
ProviderVersion: "v1.0.0",
ChangeDetection: ChangeDetection{
	Method:   "github-releases",
	Endpoint: server.URL + "/releases/latest",
},
```

to:

```go
ChangeDetection: ChangeDetection{
	Method:   "github-releases",
	Endpoint: server.URL + "/releases/latest",
	Baseline: "v1.0.0",
},
```

### Step 2: Migrate assertions (lines 104–105)

```go
if drift.Baseline != "v1.0.0" {
	t.Errorf("Baseline = %q, want %q", drift.Baseline, "v1.0.0")
}
```

### Step 3: Migrate `manifest_test.go` YAML fixtures

In the inline YAML in `TestLoadManifest` (lines 12–54), remove `provider_version: "v1.0.0"` from top level and add `baseline: "v1.0.0"` under `change_detection`:

```yaml
change_detection:
  method: github-releases
  endpoint: https://api.github.com/repos/test/repo/releases/latest
  baseline: "v1.0.0"
```

The `TestLoadManifest_MissingSlugs` and `TestLoadAllManifests` fixtures (lines 97, 161) use `method: content-hash` — change both to `method: source-hash` for forward consistency with commit 1's schema. They are otherwise untouched.

### Step 4: Run tests

```bash
cd cli && go test ./internal/provmon/ -run 'TestCheckVersion_GitHubReleases|TestCheckVersion_NoDrift|TestRunCheck|TestLoadManifest' -v
```

Expected: all named tests PASS.

**Also migrate `TestCheckVersion_UnimplementedMethods`** (lines 152–181 in `checker_test.go`): after Task 2, `content-hash` / `github-commits` strings fall to the `default` case (returns `nil, nil`), so the test would start failing. Since the sentinel moved to `source-hash`, update the `cases` slice to pin that single method:

```go
cases := []struct{ name, method string }{
    {"source-hash stub until Task 19 lands", "source-hash"},
}
```

This keeps the sentinel contract tested through commit 1. Task 20 deletes the test wholesale once `source-hash` is fully implemented.

---

## Task 5: Update `cli/cmd/provider-monitor/main.go` output + reads

**Files:**
- Modify: `cli/cmd/provider-monitor/main.go` (lines 114–117)

**Depends on:** Task 1, Task 3

### Success Criteria
- `cd cli && go build ./cmd/provider-monitor` → pass — binary builds
- `grep -q 'baseline=' cli/cmd/provider-monitor/main.go` → pass — updated label
- `grep -q 'manifest=%s' cli/cmd/provider-monitor/main.go` → fail — old label gone

### Step 1: Update DRIFT printf

Old (lines 115–116):
```go
fmt.Printf("  DRIFT   manifest=%s  latest=%s\n",
	r.VersionDrift.ManifestVersion, r.VersionDrift.LatestVersion)
```

New:
```go
fmt.Printf("  DRIFT   baseline=%s  latest=%s\n",
	r.VersionDrift.Baseline, r.VersionDrift.LatestVersion)
```

### Step 2: Verify build

```bash
cd cli && go build ./cmd/provider-monitor
```

Expected: builds cleanly.

---

## Task 6: Update JSON schema

**Files:**
- Modify: `docs/provider-sources/manifest.schema.json` (lines 7–17, 30–33, 94–110)

**Depends on:** none (parallel with Task 1)

### Success Criteria
- `jq '.required | index("provider_version")' docs/provider-sources/manifest.schema.json` → outputs `null` — removed from required list
- `jq '.properties.provider_version' docs/provider-sources/manifest.schema.json` → outputs `null` — top-level property removed
- `jq '.properties.change_detection.properties.baseline.type' docs/provider-sources/manifest.schema.json` → outputs `"string"` — new field present
- `jq '.properties.change_detection.properties.method.enum' docs/provider-sources/manifest.schema.json` → outputs array containing exactly `github-releases` and `source-hash`

### Step 1: Remove top-level `provider_version` property

Delete lines 30–33:
```json
"provider_version": {
  "type": "string",
  "description": "Provider version at time of last verification. Empty string if no public versioning."
},
```

### Step 2: Relax `change_detection.required`

Change line 97:
```json
"required": ["method"],
```

(was `["method", "endpoint"]` — endpoint becomes optional because `source-hash` providers carry baselines in FormatDoc, not the manifest endpoint.)

### Step 3: Update method enum + add baseline property

Replace lines 99–110 with:
```json
"properties": {
  "method": {
    "type": "string",
    "description": "Detection strategy.",
    "enum": ["github-releases", "source-hash"]
  },
  "endpoint": {
    "type": "string",
    "description": "URL for this method. Required for github-releases (points at releases API); informational for source-hash (primary docs landing URL — actual per-source baselines live in the FormatDoc).",
    "format": "uri"
  },
  "baseline": {
    "type": "string",
    "description": "Opaque comparison reference. For github-releases: the pinned release tag (e.g. 'v2.1.86'). Empty for source-hash (per-source hashes live in docs/provider-formats/<slug>.yaml)."
  }
}
```

### Step 4: Verify

```bash
jq . docs/provider-sources/manifest.schema.json > /dev/null
```

Expected: no errors. Schema is valid JSON.

---

## Task 7: Migrate 16 YAML manifests

**Files:**
- Modify: all 16 files in `docs/provider-sources/*.yaml`, including `_template.yaml`

**Depends on:** Task 6

### Success Criteria
- `grep -l '^provider_version:' docs/provider-sources/*.yaml` → exit non-zero (no matches) — top-level field removed from every manifest
- `grep -h 'method: source-hash' docs/provider-sources/*.yaml | wc -l` → outputs `5` — windsurf, kiro, cursor, amp, copilot-cli migrated (5 of 15 real providers use source-hash; the other 10 use github-releases)
- `grep -l 'method: content-hash\|method: github-commits' docs/provider-sources/*.yaml` → exit non-zero — old method strings purged everywhere
- `cd cli && go test ./internal/provmon/ -run TestLoadAllManifests_RealManifests` → pass — every real manifest loads cleanly

### Step 1: Inventory

Run to list the 16 files:

```bash
ls docs/provider-sources/*.yaml
```

Expected: 16 files (15 providers + `_template.yaml`).

### Step 2: Migrate each manifest

For every file, perform these edits:

1. **Remove** the top-level line `provider_version: "<value>"`.
2. **If method is `content-hash` or `github-commits`:** change to `source-hash`.
3. **Add** `baseline: "<value>"` under `change_detection` (indented 2 spaces). Use:
   - For `github-releases` providers (10 of 15 real providers): the value that used to be in `provider_version`.
   - For `source-hash` providers (5: windsurf, kiro, cursor, amp, copilot-cli): omit the field (`baseline:` absent) — per-source hashes live in FormatDoc.
4. **Update trailing comments** that reference `content-hash`/`github-commits`/"Re-fetch llms.txt periodically and compare page list"-style mechanics. Replace with one line: `# method: source-hash — per-source baselines in docs/provider-formats/<slug>.yaml`.

Example diff for `docs/provider-sources/claude-code.yaml` (github-releases provider):
```diff
-provider_version: "v2.1.86"
-
 change_detection:
   method: github-releases
   endpoint: https://api.github.com/repos/anthropics/claude-code/releases/latest
+  baseline: "v2.1.86"
```

Example diff for `docs/provider-sources/windsurf.yaml` (source-hash provider):
```diff
-provider_version: ""
-
 change_detection:
   method: source-hash
   endpoint: https://docs.windsurf.com/llms.txt
-  # Re-fetch llms.txt periodically and compare page list. Then hash individual pages.
+  # method: source-hash — per-source baselines in docs/provider-formats/windsurf.yaml
```

### Step 3: Verify

```bash
cd cli && go test ./internal/provmon/ -run TestLoadAllManifests_RealManifests -v
```

Expected: PASS. All 15 real manifests load; required fields validated.

```bash
grep -l '^provider_version:' docs/provider-sources/*.yaml
```

Expected: empty output (exit 1).

```bash
grep -l 'method: content-hash\|method: github-commits' docs/provider-sources/*.yaml
```

Expected: empty output (exit 1).

---

## Task 8: Run full suite + commit commit 1

**Files:**
- None new. All prior changes.

**Depends on:** Tasks 1–7

### Success Criteria
- `cd cli && make test` → pass — full Go test suite green
- `cd cli && make fmt` → pass — no formatting diffs
- `cd cli && make vet` → pass — no vet warnings
- `git log -1 --format=%s` → outputs subject line matching pattern `refactor(provmon): migrate.*source-hash` — commit created

### Step 1: Run full test suite

```bash
cd cli && make test
```

Expected: PASS across all packages. Any failure here means a dependent package references the renamed fields — fix, don't skip.

### Step 2: Format + vet

```bash
cd cli && make fmt && make vet
```

Expected: no output (clean).

### Step 3: Stage and commit

```bash
git add \
  cli/internal/provmon/manifest.go \
  cli/internal/provmon/checker.go \
  cli/internal/provmon/checker_test.go \
  cli/internal/provmon/manifest_test.go \
  cli/cmd/provider-monitor/main.go \
  docs/provider-sources/manifest.schema.json \
  docs/provider-sources/*.yaml
git commit -m "$(cat <<'EOF'
refactor(provmon): migrate ProviderVersion → ChangeDetection.Baseline and unify source-hash method

Schema migration only — no behavior change. Follows syllago-5gthn design doc.

- Adds Baseline field to ChangeDetection; removes top-level ProviderVersion
- Collapses content-hash + github-commits method strings into source-hash
- Renames VersionDrift.ManifestVersion → Baseline; CheckReport.ProviderVersion → Baseline
- Migrates all 16 YAML manifests (+ _template.yaml) and the JSON schema
- Updates provider-monitor DRIFT output label: manifest= → baseline=

Detection implementation for source-hash follows in the next commit.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

### Step 4: Verify commit

```bash
git log -1 --stat
```

Expected: commit present with ~22 files modified (16 YAMLs + 5 Go files + 1 JSON schema).

---

# Commit 2 — Detection Implementation

## Task 9: Backfill FormatDoc for windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (capmon extract writes the diff)

**Depends on:** Task 8 (commit 1 merged)

### Success Criteria
- `grep -c 'content_hash: sha256:' docs/provider-formats/windsurf.yaml` → outputs `6` — all 6 sources have populated hashes
- `grep -q 'workflows.md' docs/provider-formats/windsurf.yaml` → pass — the missing source is now listed

### Step 1: Run capmon fetch-extract

```bash
cd cli && go run ./cmd/syllago capmon run --stage fetch-extract --provider=windsurf
```

Expected: exits 0. Writes to `docs/provider-formats/windsurf.yaml`.

### Step 2: Verify coverage

```bash
grep -c 'content_hash: sha256:' docs/provider-formats/windsurf.yaml
```

Expected: `6` (was 5 before extract — workflows.md added).

### Step 3: Commit backfill

```bash
git add docs/provider-formats/windsurf.yaml
git commit -m "$(cat <<'EOF'
chore(capmon): backfill windsurf FormatDoc with workflows.md

Prerequisite for provmon source-hash detection (syllago-5gthn).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Backfill FormatDoc for amp

**Files:**
- Modify: `docs/provider-formats/amp.yaml`

**Depends on:** Task 8

### Success Criteria
- `grep -c 'content_hash: sha256:' docs/provider-formats/amp.yaml` → outputs `9` — all 9 sources populated
- `grep -q 'uri: "https://ampcode.com/manual"' docs/provider-formats/amp.yaml` → pass — chromedp source listed as a uri (not just as docs_url)

### Step 1: Set CHROMEDP_URL if available

Amp's `ampcode.com/manual` is a chromedp source. If you have a local sidecar running:

```bash
export CHROMEDP_URL=ws://localhost:9222/devtools/browser/<id>
```

Otherwise, capmon will launch a local Chrome instance (requires `google-chrome` or `chromium` on PATH).

### Step 2: Run capmon fetch-extract

```bash
cd cli && go run ./cmd/syllago capmon run --stage fetch-extract --provider=amp
```

Expected: exits 0. Chromedp fetch takes 15–30s per source.

### Step 3: Verify + commit

```bash
grep -c 'content_hash: sha256:' docs/provider-formats/amp.yaml
git add docs/provider-formats/amp.yaml
git commit -m "$(cat <<'EOF'
chore(capmon): backfill amp FormatDoc with ampcode.com/manual (chromedp)

Prerequisite for provmon source-hash detection (syllago-5gthn).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Backfill FormatDoc for cursor

**Files:**
- Modify: `docs/provider-formats/cursor.yaml`

**Depends on:** Task 8

### Success Criteria
- `grep -c 'content_hash: sha256:' docs/provider-formats/cursor.yaml` → outputs `6` — all 6 empty hashes populated
- `grep -c 'content_hash: ""' docs/provider-formats/cursor.yaml` → outputs `0` — no empty hashes remain

### Step 1: Set CHROMEDP_URL

Cursor is fully chromedp + rate-limits aggressively. Remote sidecar strongly preferred:

```bash
export CHROMEDP_URL=ws://localhost:9222/devtools/browser/<id>
```

### Step 2: Run capmon fetch-extract (chromedp — requires CHROMEDP_URL set or local chrome)

```bash
cd cli && go run ./cmd/syllago capmon run --stage fetch-extract --provider=cursor
```

Expected: exits 0. Cursor may 429 — if it does, wait 2–5 minutes and retry. This is the known rate-limit risk from Decision 5.

### Step 3: Verify + commit

```bash
grep -c 'content_hash: sha256:' docs/provider-formats/cursor.yaml
# expect: 6
grep -c 'content_hash: ""' docs/provider-formats/cursor.yaml
# expect: 0

git add docs/provider-formats/cursor.yaml
git commit -m "$(cat <<'EOF'
chore(capmon): backfill cursor FormatDoc with all 6 content hashes

Prerequisite for provmon source-hash detection (syllago-5gthn).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Resolve copilot-cli URL drift + re-extract

**Files:**
- Modify: either `docs/provider-sources/copilot-cli.yaml` or `docs/provider-formats/copilot-cli.yaml` (whichever is stale)

**Depends on:** Task 8

### Success Criteria
- `grep -r 'add-skills.md\|create-skills.md' docs/provider-sources/copilot-cli.yaml docs/provider-formats/copilot-cli.yaml` → outputs the same URL in both files — URL drift resolved
- `grep -c 'content_hash: sha256:' docs/provider-formats/copilot-cli.yaml` → outputs `15` — all 15 sources populated

### Step 1: Check upstream URL

```bash
curl -sI https://raw.githubusercontent.com/github/copilot-cli-examples/main/docs/add-skills.md | head -1
curl -sI https://raw.githubusercontent.com/github/copilot-cli-examples/main/docs/create-skills.md | head -1
```

(Substitute actual upstream path — read `copilot-cli.yaml`'s sources to confirm the repo + path structure.)

Decide which URL is current based on which returns HTTP 200.

### Step 2: Update the stale reference

If manifest is stale (add-skills.md renamed upstream to create-skills.md):
- Edit `docs/provider-sources/copilot-cli.yaml`: change `add-skills.md` → `create-skills.md`.

If FormatDoc is stale (FormatDoc still has old URL from prior extract):
- Nothing to edit directly — `syllago capmon run --stage fetch-extract` will normalize it below.

### Step 3: Re-run extract

```bash
cd cli && go run ./cmd/syllago capmon run --stage fetch-extract --provider=copilot-cli
```

Expected: exits 0. Writes updated `docs/provider-formats/copilot-cli.yaml`.

### Step 4: Verify + commit

```bash
grep -c 'content_hash: sha256:' docs/provider-formats/copilot-cli.yaml
# expect: 15
```

```bash
git add docs/provider-sources/copilot-cli.yaml docs/provider-formats/copilot-cli.yaml
git commit -m "$(cat <<'EOF'
chore(capmon): resolve copilot-cli skills URL drift

Upstream path changed (add-skills.md → create-skills.md).
Re-extracted FormatDoc to normalize.

Prerequisite for provmon source-hash detection (syllago-5gthn).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Add `SourceDriftStatus` + `SourceDrift` types; extend `VersionDrift.Sources`

**Files:**
- Modify: `cli/internal/provmon/checker.go` (after the `VersionDrift` struct, around line 58)

**Depends on:** Task 8 (commit 1 merged)

### Success Criteria
- `grep -q 'type SourceDriftStatus string' cli/internal/provmon/checker.go` → pass — type declared
- `grep -q 'StatusFetchFailed' cli/internal/provmon/checker.go` → pass — constant declared
- `grep -q 'Sources  *\[\]SourceDrift' cli/internal/provmon/checker.go` → pass — field added to VersionDrift
- `cd cli && go build ./internal/provmon/...` → pass — package compiles

### Step 1: Extend `VersionDrift`

Replace the existing `VersionDrift` with:

```go
// VersionDrift describes the outcome of change detection for one manifest.
// For github-releases, Baseline and LatestVersion are set and Sources is empty.
// For source-hash, Sources carries per-source results and Baseline/LatestVersion are empty.
type VersionDrift struct {
	Method        string // "github-releases" | "source-hash"
	Baseline      string // github-releases only
	LatestVersion string // github-releases only
	Drifted       bool   // aggregate: true if any source drifted or version changed
	Sources       []SourceDrift // source-hash only
}
```

### Step 2: Add status taxonomy and `SourceDrift`

Place immediately after `VersionDrift`:

```go
// SourceDriftStatus enumerates the possible outcomes for one source in a source-hash check.
type SourceDriftStatus string

const (
	StatusStable         SourceDriftStatus = "stable"
	StatusDrifted        SourceDriftStatus = "drifted"
	StatusSkipped        SourceDriftStatus = "skipped"
	StatusFetchFailed    SourceDriftStatus = "fetch_failed"
	StatusContentInvalid SourceDriftStatus = "content_invalid"
)

// SourceDrift is the result of checking a single source URL against its baseline.
type SourceDrift struct {
	ContentType string            // rules | hooks | mcp | skills | agents | commands
	URL         string
	Status      SourceDriftStatus
	Baseline    string // from FormatDoc; empty for skipped/fetch_failed
	Current     string // freshly computed; empty for skipped/fetch_failed
	Reason      string // human-readable detail for non-stable statuses
}
```

### Step 3: Set `Method` in the github-releases branch

In `CheckVersion` (around line 156), include `Method: "github-releases"` in the returned `VersionDrift`:

```go
return &VersionDrift{
	Method:        "github-releases",
	Baseline:      m.ChangeDetection.Baseline,
	LatestVersion: release.TagName,
	Drifted:       m.ChangeDetection.Baseline != "" && release.TagName != m.ChangeDetection.Baseline,
}, nil
```

### Step 4: Verify

```bash
cd cli && go build ./internal/provmon/...
```

Expected: builds. Existing tests still pass — the new `Method` field is additive.

---

## Task 14: RED test — source-hash happy path (all stable)

**Files:**
- Create: `cli/internal/provmon/checker_source_hash_test.go`

**Depends on:** Task 13

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Stable` → fail — test exists and fails with "status mismatch" or "method not implemented" (sentinel still in switch from commit 1)
- `grep -q 'func TestCheckVersion_SourceHash_Stable' cli/internal/provmon/checker_source_hash_test.go` → pass — test function present

### Step 1: Create test file

Full contents:

```go
package provmon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// newHashedServer returns a test server that serves fixed content for each configured path.
// Returns the server + a map of URL → expected SHA256Hex hash.
func newHashedServer(t *testing.T, paths map[string]string) (*httptest.Server, map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range paths {
		bodyCopy := body
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte(bodyCopy))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	hashes := make(map[string]string, len(paths))
	for path, body := range paths {
		hashes[server.URL+path] = capmon.SHA256Hex([]byte(body))
	}
	return server, hashes
}

// writeFormatDocFixture writes a minimal FormatDoc YAML that provmon will read.
func writeFormatDocFixture(t *testing.T, dir, slug string, sources []fixtureSource) {
	t.Helper()
	path := filepath.Join(dir, slug+".yaml")
	content := fmt.Sprintf("provider: %s\ndocs_url: https://example.invalid\ncategory: ide-extension\ncontent_types:\n  rules:\n    status: supported\n    sources:\n", slug)
	for _, s := range sources {
		content += fmt.Sprintf("      - uri: %q\n        type: docs\n        fetch_method: %q\n        content_hash: %q\n        fetched_at: \"2026-04-21T00:00:00Z\"\n",
			s.URI, s.FetchMethod, s.ContentHash)
	}
	content += "    canonical_mappings: {}\n    provider_extensions: []\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

type fixtureSource struct {
	URI         string
	FetchMethod string
	ContentHash string
}

func TestCheckVersion_SourceHash_Stable(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	paths := map[string]string{
		"/rules.md": "# rules body",
		"/hooks.md": "# hooks body",
	}
	server, hashes := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: hashes[server.URL+"/rules.md"]},
		{URI: server.URL + "/hooks.md", FetchMethod: "http", ContentHash: hashes[server.URL+"/hooks.md"]},
	})

	m := &Manifest{
		Slug: "test-provider",
		ChangeDetection: ChangeDetection{
			Method:   "source-hash",
			Endpoint: server.URL,
		},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
			Hooks: ContentType{Sources: []SourceEntry{{URL: server.URL + "/hooks.md", Type: "docs", Format: "markdown"}}},
		},
	}

	ctx := context.Background()
	drift, err := CheckVersionWithFormats(ctx, m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if drift.Method != "source-hash" {
		t.Errorf("Method = %q, want source-hash", drift.Method)
	}
	if drift.Drifted {
		t.Error("expected no drift")
	}
	if len(drift.Sources) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(drift.Sources))
	}
	for _, s := range drift.Sources {
		if s.Status != StatusStable {
			t.Errorf("source %s: status = %q, want stable (reason: %q)", s.URL, s.Status, s.Reason)
		}
	}
}
```

### Step 2: Stub `CheckVersionWithFormats`

The test references `CheckVersionWithFormats(ctx, m, formatsDir)` which Task 19 implements. To make the test compile-and-fail (RED) rather than compile-error, add the stub to `checker.go`:

```go
// CheckVersionWithFormats is CheckVersion variant that accepts an explicit
// formats directory for source-hash checks. Default callers use the repo-relative
// docs/provider-formats/. Stub pending Task 19 implementation.
func CheckVersionWithFormats(ctx context.Context, m *Manifest, formatsDir string) (*VersionDrift, error) {
	// TEMPORARY: Task 19 replaces this with real source-hash logic.
	return nil, fmt.Errorf("CheckVersionWithFormats not implemented")
}
```

### Step 3: Run the test — it must fail

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Stable
```

Expected: FAIL with message `"CheckVersionWithFormats not implemented"` or equivalent.

---

## Task 15: RED test — source-hash drift path

**Files:**
- Modify: `cli/internal/provmon/checker_source_hash_test.go`

**Depends on:** Task 14

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Drifted` → fail — test exists and fails (stub still panics)
- `grep -q 'func TestCheckVersion_SourceHash_Drifted' cli/internal/provmon/checker_source_hash_test.go` → pass

### Step 1: Append test

Append to `checker_source_hash_test.go`:

```go
func TestCheckVersion_SourceHash_Drifted(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server returns "new content"; FormatDoc records baseline for "old content".
	paths := map[string]string{"/rules.md": "# new content"}
	server, _ := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	oldHash := capmon.SHA256Hex([]byte("# old content"))

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: oldHash},
	})

	m := &Manifest{
		Slug: "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if !drift.Drifted {
		t.Error("expected Drifted=true")
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	if drift.Sources[0].Status != StatusDrifted {
		t.Errorf("status = %q, want drifted (reason: %q)", drift.Sources[0].Status, drift.Sources[0].Reason)
	}
	if drift.Sources[0].Baseline != oldHash {
		t.Errorf("Baseline mismatch: got %q", drift.Sources[0].Baseline)
	}
	if drift.Sources[0].Current == "" || drift.Sources[0].Current == oldHash {
		t.Errorf("Current hash unset or equals baseline: %q", drift.Sources[0].Current)
	}
}
```

### Step 2: Run — must fail

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Drifted
```

Expected: FAIL.

---

## Task 16: RED test — three skipped reasons

**Files:**
- Modify: `cli/internal/provmon/checker_source_hash_test.go`

**Depends on:** Task 15

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Skipped` → fail — table test fails (stub unimplemented)
- `grep -q 'formatdoc_missing\|source_missing\|baseline_empty' cli/internal/provmon/checker_source_hash_test.go` → pass — all three reasons exercised

### Step 1: Append table test

```go
func TestCheckVersion_SourceHash_Skipped(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	paths := map[string]string{"/rules.md": "# content"}
	server, hashes := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	cases := []struct {
		name         string
		setupFormats func(t *testing.T, dir string)
		wantReason   string
	}{
		{
			name: "formatdoc_missing",
			setupFormats: func(t *testing.T, dir string) {
				// Write nothing — FormatDoc absent.
			},
			wantReason: "FormatDoc missing",
		},
		{
			name: "source_missing",
			setupFormats: func(t *testing.T, dir string) {
				// FormatDoc exists but doesn't list this URL.
				writeFormatDocFixture(t, dir, "test-provider", []fixtureSource{
					{URI: server.URL + "/other.md", FetchMethod: "http", ContentHash: "sha256:deadbeef"},
				})
			},
			wantReason: "missing from FormatDoc",
		},
		{
			name: "baseline_empty",
			setupFormats: func(t *testing.T, dir string) {
				writeFormatDocFixture(t, dir, "test-provider", []fixtureSource{
					{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: ""},
				})
			},
			wantReason: "baseline empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			formatsDir := t.TempDir()
			tc.setupFormats(t, formatsDir)

			m := &Manifest{
				Slug: "test-provider",
				ChangeDetection: ChangeDetection{Method: "source-hash"},
				ContentTypes: ContentTypes{
					Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
				},
			}

			drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
			if err != nil {
				t.Fatalf("CheckVersionWithFormats: %v", err)
			}
			if len(drift.Sources) != 1 {
				t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
			}
			if drift.Sources[0].Status != StatusSkipped {
				t.Errorf("status = %q, want skipped (reason: %q)", drift.Sources[0].Status, drift.Sources[0].Reason)
			}
			if drift.Sources[0].Reason == "" {
				t.Error("expected non-empty reason")
			}
			if !strings.Contains(drift.Sources[0].Reason, tc.wantReason) {
				t.Errorf("reason %q should contain %q", drift.Sources[0].Reason, tc.wantReason)
			}
		})
	}
	_ = hashes // avoid unused-warning across skipped cases
}
```

Add `"strings"` to the import block at the top of the file if not already present.

### Step 2: Run — must fail

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_Skipped -v
```

Expected: FAIL on all three subtests.

---

## Task 17: RED test — fetch_failed

**Files:**
- Modify: `cli/internal/provmon/checker_source_hash_test.go`

**Depends on:** Task 16

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_FetchFailed` → fail — test exists and fails

### Step 1: Append test

```go
func TestCheckVersion_SourceHash_FetchFailed(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(server.Close)

	orig := httpClient
	httpClient = server.Client()
	capmon.SetHTTPClientForTest(server.Client())
	t.Cleanup(func() {
		httpClient = orig
		capmon.SetHTTPClientForTest(nil)
	})

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: "sha256:baseline"},
	})

	m := &Manifest{
		Slug: "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	if drift.Sources[0].Status != StatusFetchFailed {
		t.Errorf("status = %q, want fetch_failed (reason: %q)", drift.Sources[0].Status, drift.Sources[0].Reason)
	}
	if drift.Sources[0].Reason == "" {
		t.Error("expected non-empty fetch_failed reason")
	}
	if drift.Drifted {
		t.Error("fetch_failed must not count as drift")
	}
}
```

### Step 2: Run — must fail

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_FetchFailed
```

Expected: FAIL (stub still panics / unimplemented).

---

## Task 18: RED test — content_invalid

**Files:**
- Modify: `cli/internal/provmon/checker_source_hash_test.go`

**Depends on:** Task 17

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_ContentInvalid` → fail — test exists and fails

### Step 1: Append test

```go
func TestCheckVersion_SourceHash_ContentInvalid(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server returns a 200 login wall — ValidateContentResponse must reject.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><form id="login">Sign in to continue</form></body></html>`))
	}))
	t.Cleanup(server.Close)

	orig := httpClient
	httpClient = server.Client()
	capmon.SetHTTPClientForTest(server.Client())
	t.Cleanup(func() {
		httpClient = orig
		capmon.SetHTTPClientForTest(nil)
	})

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: "sha256:baseline"},
	})

	m := &Manifest{
		Slug: "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	if drift.Sources[0].Status != StatusContentInvalid {
		t.Errorf("status = %q, want content_invalid (reason: %q)", drift.Sources[0].Status, drift.Sources[0].Reason)
	}
}
```

### Step 2: Run — must fail

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash_ContentInvalid
```

Expected: FAIL.

---

## Task 19: Implement `checkSourceHash` — all RED tests go GREEN

**Files:**
- Create: `cli/internal/provmon/checker_source_hash.go`
- Modify: `cli/internal/provmon/checker.go` — replace stub `CheckVersionWithFormats` + route `source-hash` case

**Depends on:** Tasks 14, 15, 16, 17, 18

### Success Criteria
- `cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash` → pass — all 6 source-hash subtests GREEN (stable, drifted, 3×skipped, fetch_failed, content_invalid)
- `cd cli && go vet ./internal/provmon/...` → pass
- `grep -q 'capmon\.LoadFormatDoc' cli/internal/provmon/checker_source_hash.go` → pass — capmon import wired
- `grep -q 'CheckVersionWithFormats not implemented' cli/internal/provmon/checker.go` → fail — stub removed

### Step 1: Create `checker_source_hash.go`

Full file contents:

```go
package provmon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// checkSourceHash compares each manifest source URL's current content hash
// against the baseline recorded in capmon's FormatDoc. It is the implementation
// for m.ChangeDetection.Method == "source-hash".
//
// formatsDir is typically "docs/provider-formats" (relative to the repo root).
// Pass an explicit path to support test fixtures.
func checkSourceHash(ctx context.Context, m *Manifest, formatsDir string) (*VersionDrift, error) {
	drift := &VersionDrift{Method: "source-hash"}

	formatDocPath := capmon.FormatDocPath(formatsDir, m.Slug)
	var doc *capmon.FormatDoc
	if _, statErr := os.Stat(formatDocPath); statErr == nil {
		loaded, err := capmon.LoadFormatDoc(formatDocPath)
		if err != nil {
			// Treat unparseable FormatDoc as "missing" from the caller's perspective,
			// but carry the parse error in the reason so it's visible.
			for _, src := range manifestSources(m) {
				drift.Sources = append(drift.Sources, SourceDrift{
					ContentType: src.contentType,
					URL:         src.entry.URL,
					Status:      StatusSkipped,
					Reason:      fmt.Sprintf("FormatDoc parse error: %v", err),
				})
			}
			return drift, nil
		}
		doc = loaded
	}

	for _, src := range manifestSources(m) {
		result := checkOneSource(ctx, m.Slug, src.contentType, src.entry, doc)
		if result.Status == StatusDrifted {
			drift.Drifted = true
		}
		drift.Sources = append(drift.Sources, result)
	}

	return drift, nil
}

// manifestSources flattens every SourceEntry across all content types into a
// single list with its content-type label attached. Preserves manifest order.
type taggedSource struct {
	contentType string
	entry       SourceEntry
}

func manifestSources(m *Manifest) []taggedSource {
	var out []taggedSource
	pairs := []struct {
		name string
		ct   ContentType
	}{
		{"rules", m.ContentTypes.Rules},
		{"hooks", m.ContentTypes.Hooks},
		{"mcp", m.ContentTypes.MCP},
		{"skills", m.ContentTypes.Skills},
		{"agents", m.ContentTypes.Agents},
		{"commands", m.ContentTypes.Commands},
	}
	for _, p := range pairs {
		for _, s := range p.ct.Sources {
			out = append(out, taggedSource{contentType: p.name, entry: s})
		}
	}
	return out
}

// checkOneSource performs the fetch-hash-compare dance for a single source URL.
// It never returns an error — every outcome is expressed as a SourceDrift status.
func checkOneSource(ctx context.Context, slug, contentType string, entry SourceEntry, doc *capmon.FormatDoc) SourceDrift {
	result := SourceDrift{ContentType: contentType, URL: entry.URL}

	if doc == nil {
		result.Status = StatusSkipped
		result.Reason = fmt.Sprintf("FormatDoc missing: run 'syllago capmon run --stage fetch-extract --provider=%s'", slug)
		return result
	}

	ref, ok := findSourceRef(doc, entry.URL)
	if !ok {
		result.Status = StatusSkipped
		result.Reason = fmt.Sprintf("source %s missing from FormatDoc: run 'syllago capmon run --stage fetch-extract --provider=%s'", entry.URL, slug)
		return result
	}

	if ref.ContentHash == "" {
		result.Status = StatusSkipped
		result.Reason = fmt.Sprintf("baseline empty for %s: run 'syllago capmon run --stage fetch-extract --provider=%s'", entry.URL, slug)
		return result
	}

	// NOTE: Do NOT call capmon.ValidateSourceURL here. ValidateSourceURL enforces
	// https-only and performs DNS resolution — both of which break unit tests that use
	// http://127.0.0.1 test servers. Manifest URLs are validated at manifest-add time;
	// provmon drift detection is a read-only consumer and does not need to re-validate.

	// fetch_method comes from the FormatDoc SourceRef (ref), not the provmon
	// SourceEntry (entry) — manifests don't record fetch method; FormatDocs do.
	body, err := fetchSourceBytes(ctx, ref.FetchMethod, slug, entry.URL)
	if err != nil {
		result.Status = StatusFetchFailed
		result.Reason = err.Error()
		return result
	}

	if valErr := capmon.ValidateContentResponse(body, "", entry.URL, entry.URL); valErr != nil {
		result.Status = StatusContentInvalid
		result.Reason = valErr.Error()
		return result
	}

	result.Baseline = ref.ContentHash
	result.Current = capmon.SHA256Hex(body)
	if result.Current == result.Baseline {
		result.Status = StatusStable
	} else {
		result.Status = StatusDrifted
	}
	return result
}

// findSourceRef returns the FormatDoc SourceRef whose URI matches the given URL.
// Match is exact-string.
func findSourceRef(doc *capmon.FormatDoc, url string) (capmon.SourceRef, bool) {
	for _, ct := range doc.ContentTypes {
		for _, src := range ct.Sources {
			if src.URI == url {
				return src, true
			}
		}
	}
	return capmon.SourceRef{}, false
}

// fetchSourceBytes dispatches between capmon.FetchSource (HTTP) and capmon.FetchChromedp
// based on the FormatDoc's recorded fetch_method for this source. Uses a provmon-owned
// temp cache root so we don't collide with capmon's real cache.
func fetchSourceBytes(ctx context.Context, fetchMethod, slug, url string) ([]byte, error) {
	cacheRoot := provmonCacheRoot()
	sourceID := sanitizeSourceID(url)

	var entry *capmon.CacheEntry
	var err error
	if fetchMethod == "chromedp" {
		entry, err = capmon.FetchChromedp(ctx, cacheRoot, slug, sourceID, url)
	} else {
		entry, err = capmon.FetchSource(ctx, cacheRoot, slug, sourceID, url)
	}
	if err != nil {
		return nil, err
	}
	return entry.Raw, nil
}

var provmonCacheRoot = func() string {
	return filepath.Join(os.TempDir(), "syllago-provmon-cache")
}

func sanitizeSourceID(url string) string {
	// Match capmon's source-ID conventions closely enough for the cache path
	// to be unique per URL. Replace path separators and scheme punctuation.
	id := url
	for _, ch := range []string{"://", "/", "?", "&", "=", ":"} {
		id = strings.ReplaceAll(id, ch, "_")
	}
	return id
}
```

### Step 2: Replace stub and route `source-hash` in `checker.go`

Delete the stub `CheckVersionWithFormats` that Task 14 added. Replace it with:

```go
// CheckVersionWithFormats is CheckVersion with an explicit FormatDoc directory.
// Tests pass a temp dir; the default CLI path is docs/provider-formats relative to the repo root.
func CheckVersionWithFormats(ctx context.Context, m *Manifest, formatsDir string) (*VersionDrift, error) {
	switch m.ChangeDetection.Method {
	case "github-releases":
		return checkGithubReleases(ctx, m)
	case "source-hash":
		return checkSourceHash(ctx, m, formatsDir)
	default:
		return nil, nil
	}
}
```

Extract the existing github-releases body into a helper `checkGithubReleases(ctx, m) (*VersionDrift, error)` — keep lines 127–161 logic intact, move into the helper, set `Method: "github-releases"` in the return.

Keep the existing `CheckVersion` as a thin wrapper:

```go
// CheckVersion is the default entry used by RunCheck; resolves formatsDir from the current working directory.
func CheckVersion(ctx context.Context, m *Manifest) (*VersionDrift, error) {
	return CheckVersionWithFormats(ctx, m, defaultFormatsDir())
}

func defaultFormatsDir() string {
	// Resolve repo root relative to the provmon package's calling binary.
	// Primary: current working directory + docs/provider-formats.
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "docs", "provider-formats")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	// Fallback: parent of cwd (handles running from cli/).
	candidate = filepath.Join(cwd, "..", "docs", "provider-formats")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return filepath.Join("docs", "provider-formats")
}
```

Add `"os"`, `"path/filepath"` to the `checker.go` import block if not already present (they are — only add if you split files differently).

### Step 3: Run tests — all GREEN

```bash
cd cli && go test ./internal/provmon/ -run TestCheckVersion_SourceHash -v
```

Expected: all of `TestCheckVersion_SourceHash_Stable`, `_Drifted`, `_Skipped/formatdoc_missing`, `_Skipped/source_missing`, `_Skipped/baseline_empty`, `_FetchFailed`, `_ContentInvalid` PASS.

### Step 4: Verify existing tests still pass

```bash
cd cli && go test ./internal/provmon/ -v
```

Expected: every test in the package PASSES, including `TestCheckVersion_GitHubReleases`, `TestCheckVersion_NoDrift`, `TestRunCheck`, `TestCheckVersion_UnknownMethod`, and the still-present `TestCheckVersion_UnimplementedMethods` (deleted in Task 20).

---

## Task 20: Delete sentinel + obsolete test

**Files:**
- Modify: `cli/internal/provmon/checker.go` (lines 14–22, the `ErrUnimplementedDetectionMethod` sentinel)
- Modify: `cli/internal/provmon/checker_test.go` (lines 145–181, `TestCheckVersion_UnimplementedMethods`)

**Depends on:** Task 19

### Success Criteria
- `grep -q 'ErrUnimplementedDetectionMethod' cli/internal/provmon/checker.go` → fail — sentinel removed
- `grep -q 'TestCheckVersion_UnimplementedMethods' cli/internal/provmon/checker_test.go` → fail — obsolete test removed
- `cd cli && go test ./internal/provmon/...` → pass — remaining suite green
- `cd cli && go build ./...` → pass — no dangling references elsewhere

### Step 1: Delete the sentinel

Remove lines 14–22 from `checker.go`:

```go
var ErrUnimplementedDetectionMethod = errors.New("change detection method not implemented")
```

Remove the `errors` import if no longer used (`go vet` will catch it).

### Step 2: Delete the test

Remove lines 145–181 from `checker_test.go` — the entire `TestCheckVersion_UnimplementedMethods` function and its preceding comment.

Remove the `errors` import from `checker_test.go` if no longer used.

### Step 3: Verify

```bash
cd cli && go vet ./...
cd cli && go test ./internal/provmon/ -v
```

Expected: no vet errors; all provmon tests pass; sentinel is gone.

---

## Task 21: `RunCheck` surfaces per-source results

**Files:**
- Modify: `cli/internal/provmon/checker.go` (RunCheck, lines 163–193)

**Depends on:** Task 19, Task 20

### Success Criteria
- `grep -A2 'func RunCheck' cli/internal/provmon/checker.go | grep -q 'CheckVersion'` → pass — RunCheck still calls CheckVersion
- `cd cli && go test ./internal/provmon/ -run TestRunCheck` → pass — existing RunCheck test still green
- `grep -q '// Version check failures are non-fatal' cli/internal/provmon/checker.go` → fail — old comment gone (replaced with structured-status comment)

### Step 1: Update the RunCheck body

Replace the silent `if err == nil { ... }` handling (lines 186–190) with:

```go
	drift, err := CheckVersion(ctx, m)
	report.VersionDrift = drift // always populate — drift may hold per-source results even if err is non-nil
	if err != nil {
		// Setup-level error (e.g., github-releases API 500). Surface it as a
		// field on the report; non-fatal to URL health results.
		report.CheckVersionError = err.Error()
	}
```

### Step 2: Add field to `CheckReport`

In the `CheckReport` struct (lines 39–51), add:

```go
	CheckVersionError string // empty on success; set when CheckVersion returned an error
```

### Step 3: Verify

```bash
cd cli && go test ./internal/provmon/ -run TestRunCheck -v
```

Expected: PASS.

---

## Task 22: RED test — `--fail-on` default = drifted only

**Files:**
- Create: `cli/cmd/provider-monitor/main_test.go`

**Depends on:** Task 21

### Success Criteria
- `cd cli && go test ./cmd/provider-monitor/ -run TestExitCode_DefaultFailOn_DriftedOnly` → compile error — `computeExitCode` undefined (pre-Task 24). This is the RED state: test file exists but the function it calls doesn't yet.
- `grep -q 'func TestExitCode_DefaultFailOn_DriftedOnly' cli/cmd/provider-monitor/main_test.go` → pass

### Step 1: Create the test file

```go
package main

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provmon"
)

// TestExitCode_DefaultFailOn_DriftedOnly verifies that the default --fail-on=drifted
// policy exits 0 when only fetch_failed / content_invalid / skipped statuses appear,
// and exits non-zero only when drifted appears.
func TestExitCode_DefaultFailOn_DriftedOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reports  []*provmon.CheckReport
		wantExit int
	}{
		{
			name: "all stable",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusStable},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "fetch_failed but default fail_on",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusFetchFailed, Reason: "500"},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "drifted triggers exit",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method:  "source-hash",
					Drifted: true,
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusDrifted},
					},
				},
			}},
			wantExit: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := computeExitCode(tc.reports, []string{"drifted"})
			if got != tc.wantExit {
				t.Errorf("computeExitCode = %d, want %d", got, tc.wantExit)
			}
		})
	}
}
```

### Step 2: Run — must compile-fail

```bash
cd cli && go test ./cmd/provider-monitor/ -run TestExitCode_DefaultFailOn_DriftedOnly
```

Expected: compile error — `computeExitCode` not defined. That's the RED state for this TDD step.

---

## Task 23: RED test — wider `--fail-on`

**Files:**
- Modify: `cli/cmd/provider-monitor/main_test.go`

**Depends on:** Task 22

### Success Criteria
- `cd cli && go test ./cmd/provider-monitor/ -run TestExitCode_WideFailOn_FetchFailed` → compile error — `computeExitCode` undefined (pre-Task 24). RED state.

### Step 1: Append test

```go
func TestExitCode_WideFailOn_FetchFailed(t *testing.T) {
	t.Parallel()

	reports := []*provmon.CheckReport{{
		VersionDrift: &provmon.VersionDrift{
			Method: "source-hash",
			Sources: []provmon.SourceDrift{
				{Status: provmon.StatusFetchFailed, Reason: "500"},
				{Status: provmon.StatusStable},
			},
		},
	}}

	// Default fail_on — exit 0.
	if got := computeExitCode(reports, []string{"drifted"}); got != 0 {
		t.Errorf("default fail_on: got exit %d, want 0", got)
	}

	// Opt-in to fetch_failed — exit non-zero.
	if got := computeExitCode(reports, []string{"drifted", "fetch_failed"}); got == 0 {
		t.Error("widened fail_on should exit non-zero on fetch_failed")
	}
}
```

### Step 2: Run — must compile-fail

```bash
cd cli && go test ./cmd/provider-monitor/ -run TestExitCode_WideFailOn_FetchFailed
```

Expected: compile error — `computeExitCode` undefined. That's the RED state (test exists but function it calls is still absent).

---

## Task 24: Implement `--fail-on` flag + `computeExitCode`

**Files:**
- Modify: `cli/cmd/provider-monitor/main.go`

**Depends on:** Task 22, Task 23

### Success Criteria
- `cd cli && go test ./cmd/provider-monitor/ -run TestExitCode` → pass — both exit-code tests GREEN
- `cd cli && go run ./cmd/provider-monitor --help 2>&1 | grep -q -- '-fail-on'` → pass — flag is documented
- `cd cli && go build ./cmd/provider-monitor` → pass

### Step 1: Add `--fail-on` flag

In `main()`, after the existing flags (line 40 area):

```go
var failOnRaw string
flag.StringVar(&failOnRaw, "fail-on", "drifted",
	"comma-separated statuses that cause non-zero exit: drifted,fetch_failed,content_invalid,skipped")
```

### Step 2: Replace current exit logic (lines 82–90)

Delete:
```go
for _, r := range reports {
	if r.FailedURLs > 0 {
		os.Exit(1)
	}
	if r.VersionDrift != nil && r.VersionDrift.Drifted {
		os.Exit(1)
	}
}
```

Replace with:
```go
failOn := strings.Split(failOnRaw, ",")
for i := range failOn {
	failOn[i] = strings.TrimSpace(failOn[i])
}
os.Exit(computeExitCode(reports, failOn))
```

Add `"strings"` to the imports.

### Step 3: Add `computeExitCode`

Place at the bottom of the file:

```go
// computeExitCode returns 1 if any report has:
//   - a failed URL (always — URL health is always a blocking signal), OR
//   - VersionDrift.Drifted && "drifted" in failOn, OR
//   - any SourceDrift.Status present in failOn (for source-hash providers).
// Returns 0 otherwise.
func computeExitCode(reports []*provmon.CheckReport, failOn []string) int {
	failSet := make(map[string]bool, len(failOn))
	for _, f := range failOn {
		if f != "" {
			failSet[f] = true
		}
	}

	for _, r := range reports {
		if r.FailedURLs > 0 {
			return 1
		}
		if r.VersionDrift == nil {
			continue
		}
		if r.VersionDrift.Drifted && failSet["drifted"] {
			return 1
		}
		for _, s := range r.VersionDrift.Sources {
			if failSet[string(s.Status)] {
				return 1
			}
		}
	}
	return 0
}
```

### Step 4: Run tests — both GREEN

```bash
cd cli && go test ./cmd/provider-monitor/ -v
```

Expected: `TestExitCode_DefaultFailOn_DriftedOnly` and `TestExitCode_WideFailOn_FetchFailed` both PASS.

---

## Task 25: Per-source DRIFT output in `printReports`

**Files:**
- Modify: `cli/cmd/provider-monitor/main.go` (lines 93–123, `printReports` function)

**Depends on:** Task 24

### Success Criteria
- `cd cli && go build ./cmd/provider-monitor` → pass
- `grep -q 'DRIFT.*%s.*%s' cli/cmd/provider-monitor/main.go` → pass — new per-source line format added
- `cd cli && go run ./cmd/provider-monitor --provider=windsurf 2>&1 | head -20` → returns non-error output mentioning `source-hash` or per-source lines when run from repo root (manual smoke — requires network)

### Step 1: Update `printReports`

Replace the existing VersionDrift block (lines 114–117) with:

```go
	// Version drift — formatted per method.
	if r.VersionDrift != nil {
		switch r.VersionDrift.Method {
		case "github-releases":
			if r.VersionDrift.Drifted {
				fmt.Printf("  DRIFT   baseline=%s  latest=%s\n",
					r.VersionDrift.Baseline, r.VersionDrift.LatestVersion)
			}
		case "source-hash":
			for _, s := range r.VersionDrift.Sources {
				switch s.Status {
				case provmon.StatusStable:
					// no output — keep report terse for stable sources
				case provmon.StatusDrifted:
					fmt.Printf("  DRIFT   %-8s %s\n    baseline=%s\n    current =%s\n",
						s.ContentType, s.URL, s.Baseline, s.Current)
				default:
					fmt.Printf("  %-7s %-8s %s  (%s)\n",
						strings.ToUpper(string(s.Status)), s.ContentType, s.URL, s.Reason)
				}
			}
		}
	}

	// Surface CheckVersion setup errors (e.g., GH API rate limit).
	if r.CheckVersionError != "" {
		fmt.Printf("  ERROR   change-detection: %s\n", r.CheckVersionError)
	}
```

### Step 2: Update the summary line (lines 125–135)

Replace with:

```go
	var totalURLs, failedURLs int
	var drifted, fetchFailed, contentInvalid, skipped int
	for _, r := range reports {
		totalURLs += r.TotalURLs
		failedURLs += r.FailedURLs
		if r.VersionDrift == nil {
			continue
		}
		if r.VersionDrift.Drifted {
			drifted++
		}
		for _, s := range r.VersionDrift.Sources {
			switch s.Status {
			case provmon.StatusFetchFailed:
				fetchFailed++
			case provmon.StatusContentInvalid:
				contentInvalid++
			case provmon.StatusSkipped:
				skipped++
			}
		}
	}
	fmt.Printf("\n%d providers, %d URLs checked, %d broken, %d drifted, %d fetch_failed, %d content_invalid, %d skipped\n",
		len(reports), totalURLs, failedURLs, drifted, fetchFailed, contentInvalid, skipped)
```

### Step 3: Build + verify

```bash
cd cli && go build ./cmd/provider-monitor
```

Expected: builds.

---

## Task 26: Add `capmon extract` sequencing note to docs

**Files:**
- Modify: `docs/adding-a-provider.md`

**Depends on:** none (parallel-safe)

### Success Criteria
- `test -f docs/adding-a-provider.md` → pass — file exists (if it doesn't, create it per existing docs/ conventions — check first with `ls docs/adding-a-provider.md || ls docs/ | grep -i adding`)
- `grep -q 'fetch-extract' docs/adding-a-provider.md` → pass — note added
- `grep -q 'content_hash baseline' docs/adding-a-provider.md` → pass — explains what the command does

### Step 1: Verify file exists

```bash
ls docs/adding-a-provider.md 2>&1 || find docs -name "adding*provider*"
```

The file `docs/adding-a-provider.md` exists (verified — see `ls docs/`). Edit it directly.

### Step 2: Append section

Append after the existing "Adding a source URL" section (or at end of file):

```markdown
## After editing a provider source manifest

After adding a provider or a new source URL to `docs/provider-sources/<slug>.yaml`, run:

```
cd cli && go run ./cmd/syllago capmon run --stage fetch-extract --provider=<slug>
```

This populates `docs/provider-formats/<slug>.yaml` with per-source `content_hash` baselines. Commit both files in the same change. Without this step, `provider-monitor` will report `skipped: baseline empty` for any new sources on its next run.

For chromedp-rendered sources (fetch_method: chromedp), set `CHROMEDP_URL=ws://localhost:9222/devtools/browser/<id>` to use a remote headless-shell sidecar — faster and more reliable than launching a local Chrome.
```

### Step 3: Verify

```bash
grep -A3 'fetch-extract' docs/adding-a-provider.md
```

Expected: shows the command.

---

## Task 27: End-to-end smoke run (opt-in, network-dependent)

**Files:**
- None. Manual verification only.

**Depends on:** Task 25

### Success Criteria
- `cd cli && SYLLAGO_TEST_NETWORK=1 go run ./cmd/provider-monitor --provider=kiro` → pass (exits 0) — real smoke test against kiro (fully seeded FormatDoc) produces no drift
- `cd cli && SYLLAGO_TEST_NETWORK=1 go run ./cmd/provider-monitor` → pass or controlled fail — full repo run returns structured output; any non-zero exit is caused by real upstream drift, not code errors

### Step 1: Kiro-only smoke

```bash
cd cli && SYLLAGO_TEST_NETWORK=1 go run ./cmd/provider-monitor --provider=kiro
```

Expected: exit 0. No DRIFT lines. A summary like "1 providers, N URLs checked, 0 broken, 0 drifted, 0 fetch_failed, 0 content_invalid, 0 skipped".

### Step 2: Full run

```bash
cd cli && SYLLAGO_TEST_NETWORK=1 go run ./cmd/provider-monitor
```

Expected: structured output for all 15 providers. Exit code may be non-zero only if there's real drift today — read any DRIFT lines carefully. Any `fetch_failed` entries are tolerated by default (`--fail-on=drifted`).

If this surfaces unexpected drift, that's a real signal — triage separately (may require another `syllago capmon run --stage fetch-extract` for the affected provider).

### Step 3: Document the outcome

Record the run's output in the bead's notes:

```bash
bd update syllago-5gthn --notes="End-to-end smoke run summary: <paste summary line here>"
```

---

## Task 28: Full suite + commit commit 2

**Files:**
- None new.

**Depends on:** Tasks 9–27

### Success Criteria
- `cd cli && make test` → pass — full Go test suite green
- `cd cli && make fmt` → pass — no formatting diffs
- `cd cli && make vet` → pass
- `cd cli && make build` → pass — binary builds
- `git log -1 --format=%s` → outputs subject line matching `feat(provmon): implement source-hash detection` — commit 2 present

### Step 1: Full verification

```bash
cd cli && make fmt && make vet && make test && make build
```

Expected: every step passes. Any failure: diagnose before committing.

### Step 2: Stage + commit

Note: the FormatDoc backfill files under `docs/provider-formats/` (windsurf, amp, cursor, copilot-cli) were committed separately in Tasks 9–12 during this commit-group. Do NOT re-stage them here — `git status` should show them as already committed.

```bash
git add \
  cli/internal/provmon/checker.go \
  cli/internal/provmon/checker_source_hash.go \
  cli/internal/provmon/checker_source_hash_test.go \
  cli/internal/provmon/checker_test.go \
  cli/cmd/provider-monitor/main.go \
  cli/cmd/provider-monitor/main_test.go \
  docs/adding-a-provider.md
git commit -m "$(cat <<'EOF'
feat(provmon): implement source-hash drift detection

Replaces ErrUnimplementedDetectionMethod with working per-source hash
comparison for windsurf, kiro, cursor, amp, and copilot-cli.

- Adds SourceDrift + SourceDriftStatus types (stable | drifted | skipped
  | fetch_failed | content_invalid)
- checker_source_hash.go: loads capmon FormatDoc, loops over each
  manifest source, dispatches http vs chromedp fetch via capmon,
  compares hash to baseline, emits structured status
- Provmon is read-only against FormatDoc — seeding is always
  'syllago capmon run --stage fetch-extract'
- Adds --fail-on flag to provider-monitor (default: drifted); opt-in
  fetch_failed/content_invalid/skipped for stricter CI gates
- Deletes ErrUnimplementedDetectionMethod + the sentinel test
- Documents capmon-extract sequencing in adding-a-provider.md

Closes syllago-5gthn.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

### Step 3: Push (manual, user decision)

Do NOT push automatically. The user decides when to push after reviewing the two commits.

---

# Cross-Commit Notes

## Dependencies between commits

Commit 1 (schema migration) must merge before commit 2 because:
- Commit 2's tests assume `ChangeDetection.Baseline` exists
- Commit 2's YAML-reading tests assume the 16 manifests are migrated
- The `source-hash` method string is added in commit 1

Within commit 2, the backfill tasks (9–12) MUST complete before the detection tests (14–19) because the end-to-end smoke test depends on fully-seeded FormatDocs. Unit tests use their own fixtures so they don't strictly depend on backfill, but the integration smoke in Task 27 does.

## Testing philosophy

Every RED test in commit 2 follows the same shape:
1. Start test server (httptest)
2. Write a temp FormatDoc fixture via `writeFormatDocFixture`
3. Build a manifest pointing at the test server URLs
4. Call `CheckVersionWithFormats(ctx, m, formatsDir)`
5. Assert on `drift.Sources[i].Status` and `drift.Drifted`

The helper functions (`newHashedServer`, `writeFormatDocFixture`, `fixtureSource`) are shared across all source-hash tests — when they're working, every new test is ~30 lines.

## Known risks carried from design

- **Cursor rate-limits** during `syllago capmon run --stage fetch-extract` (Task 11). If this fails, retry with delays; the design treats this as existing capmon territory.
- **Amp + cursor chromedp requirement** (Tasks 10, 11). CHROMEDP_URL sidecar strongly preferred.
- **Copilot-cli URL drift** (Task 12) may reveal manifest-vs-upstream disagreement; fix is one-line.

## Not in scope

- Factoring shared fetch helpers into `cli/internal/fetch` — deferred.
- JSON output from `provider-monitor` — candidate follow-up.
- Changing capmon's own check pipeline.

---

## Ready to Create Beads

Plan saved to `docs/plans/2026-04-21-provmon-drift-detection-implementation.md`.

Ready to create Beads from this plan? Will:
- Create 28 beads (one per task)
- Set up dependency chain (Task N → Task N+1 where applicable)
- Partition at the commit boundary (commit 1 / commit 2 labels)
- Add success criteria to descriptions
