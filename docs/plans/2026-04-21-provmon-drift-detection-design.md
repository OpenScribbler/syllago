# provmon drift detection — Design Document

**Goal:** Implement working content-hash and github-commits drift detection in `cli/internal/provmon`, replacing the `ErrUnimplementedDetectionMethod` sentinel with real comparisons for 5 production providers.

**Bead:** syllago-5gthn (follow-up to audit bead syllago-p0phh)

**Decision Date:** 2026-04-21

---

## Problem Statement

`cli/internal/provmon/CheckVersion` currently only implements the `github-releases` detection method. Two other methods declared in manifests and documented in `manifest.go:49` — `content-hash` (used by windsurf, kiro, cursor) and `github-commits` (used by amp, copilot-cli) — return `ErrUnimplementedDetectionMethod` and produce no drift signal. **Five of the sixteen tracked providers therefore have no active drift detection today.**

Audit bead syllago-p0phh made this gap explicit by adding the sentinel error so callers could distinguish "no drift" from "we never tried." This bead is the feature follow-up that replaces the sentinel with real detection.

---

## Proposed Solution

Three-layer change, split into two commits:

1. **Schema migration (commit 1):** Rename and relocate the drift-comparison reference from `Manifest.ProviderVersion` (top-level) to `ChangeDetection.Baseline` (nested alongside the method that interprets it). Migrate all 16 existing manifests and the JSON schema. No behavior change.
2. **Detection implementation (commit 2):** Add `content-hash` and `github-commits` branches to `CheckVersion`, delete the sentinel-assertion tests, add real happy-path drift tests using `httptest` servers, seed baselines into the 5 affected manifests.

---

## Key Decisions

| # | Decision | Choice | Reasoning (short) |
|---|----------|--------|-------------------|
| 1 | Baseline schema location | Dedicated `Baseline` field on `ChangeDetection`; full migration from `ProviderVersion`. | One nullable slot for one concept, lives next to the method that interprets it, stable as new methods are added. No existing users means migration cost is low. |
| 2 | content-hash semantics | _(pending Decision 2)_ | |
| 3 | github-commits semantics | _(pending Decision 3)_ | |
| 4 | Error propagation | _(pending Decision 4)_ | |
| 5 | Baseline seeding | _(pending Decision 5)_ | |

---

## Decision 1 — Baseline schema location

### Choice: Option A-migrate — Dedicated `Baseline` field on `ChangeDetection`, full migration

### Rejected alternatives

- **Reuse `ProviderVersion`**: Field name lies when it holds a sha256 hex or commit SHA. Downstream tooling, audit logs, and git-diff output all surface the lie. No way to enforce per-format validation.
- **Method-specific fields (`CommitSHA`, `ContentHash`)**: Schema grows with every detection method. Mirrors the CheckVersion `switch` into the YAML. Allows field-method mismatch footguns (method=content-hash with commit_sha populated). YAGNI — no current provider needs dual-signal tracking.
- **Option A with fallback (`A-fallback`)**: `CheckVersion` reads `Baseline`, falls back to `ProviderVersion` when empty. Avoids manifest churn but bakes dual-read logic in for the life of the code. Every future contributor has to learn the fallback rule.

### Rationale

Option A-migrate pays a one-time migration cost (16 YAMLs, ~10 Go LOC plus renames) and leaves the codebase with a single honestly-named field. Given zero users, no external registries, and no downstream consumers of the `provider-monitor` JSON output, the blast radius is fully contained inside the repo.

### Schema change

```go
// manifest.go
type Manifest struct {
    SchemaVersion   string          `yaml:"schema_version"`
    LastVerified    string          `yaml:"last_verified"`
    // ProviderVersion removed — migrated to ChangeDetection.Baseline
    Slug            string          `yaml:"slug"`
    // ... other fields unchanged ...
    ChangeDetection ChangeDetection `yaml:"change_detection"`
    // ... other fields unchanged ...
}

type ChangeDetection struct {
    Method   string `yaml:"method"`
    Endpoint string `yaml:"endpoint"`
    Baseline string `yaml:"baseline,omitempty"` // version tag, commit SHA, or sha256 hex — opaque comparison reference
}
```

### YAML shape

```yaml
# Before (current)
provider_version: "v2.1.86"
change_detection:
  method: github-releases
  endpoint: https://api.github.com/repos/anthropics/claude-code/releases/latest

# After (migrated)
change_detection:
  method: github-releases
  endpoint: https://api.github.com/repos/anthropics/claude-code/releases/latest
  baseline: "v2.1.86"
```

### Related rename (in scope)

`VersionDrift.ManifestVersion` → `VersionDrift.Baseline` (or `Before`). The struct field name otherwise lies when drift is reported for content-hash or github-commits providers.

### Impact audit

**Go code (4 files, all in `cli/internal/provmon/`):**
- `manifest.go`: drop `ProviderVersion` from `Manifest`, add `Baseline` to `ChangeDetection`.
- `checker.go`: update `CheckVersion` github-releases path to read `m.ChangeDetection.Baseline`; update `RunCheck` to populate `CheckReport` from the new field; rename `VersionDrift.ManifestVersion` → `.Baseline`.
- `checker_test.go`: migrate 3 existing test fixtures (lines 88, 127, 233) to use `m.ChangeDetection.Baseline`; update field-read assertions (lines 104-105).
- `manifest_test.go`: update inline YAML (line 15) to use `change_detection.baseline` instead of top-level `provider_version`.

**Single binary consumer — `cli/cmd/provider-monitor/main.go`:**
- Line 115-116 printf string `"DRIFT   manifest=%s  latest=%s"` — rename variable reference; update label (`manifest=` → `baseline=`) so output is honest when value is a hash.

**JSON schema — `docs/provider-sources/manifest.schema.json`:**
- Remove top-level `provider_version` property.
- Add optional `baseline` property to `change_detection`.

**All 16 YAML manifests in `docs/provider-sources/`** (including `_template.yaml`):
- Move `provider_version: "X"` at top level → `change_detection.baseline: "X"`.
- Empty-string cases (`""`) become `baseline: ""` or omit the field.

### Explicitly NOT affected

- `cli/internal/capmon/capyaml/types.go:9` — has its own `ProviderVersion` field, but on a **different schema** (`docs/provider-capabilities/*.yaml`). Separate pipeline, separate concern. Do not touch.
- `docs/provider-capabilities/schema.json` — same rationale.
- Historical plan docs under `docs/plans/` — frozen records.

### Commit strategy

Two commits:
1. **Schema migration** — rename field, update json-schema, migrate 16 YAMLs, update existing consumers and tests. Build + full suite pass with no behavior change.
2. **Detection implementation** — content-hash and github-commits branches, new tests, baseline seeding. Behavior change.

If Decisions 2-5 produce churn, commit 1 is already merged and locked.

---

## Decision 2 — content-hash semantics

_(pending — next in the brainstorm queue)_

---

## Decision 3 — github-commits semantics

_(pending)_

---

## Decision 4 — Error propagation for unreachable endpoints

_(pending)_

---

## Decision 5 — Baseline seeding for the 5 affected manifests

_(pending)_

---

## Architecture

_(to be completed once Decisions 2-4 are resolved)_

## Data Flow

_(to be completed once Decisions 2-3 are resolved)_

## Error Handling

_(to be completed once Decision 4 is resolved)_

## Success Criteria

_(to be completed once all decisions are resolved)_

## Resolved During Design

| Question | Decision | Reasoning |
|----------|----------|-----------|
| Should the drift-comparison baseline live in a new field, reuse `ProviderVersion`, or split per-method? | New `Baseline` field on `ChangeDetection`; full migration from `ProviderVersion`. | One concept, one slot, lives with the method. No users means low blast radius. |
| Should `VersionDrift.ManifestVersion` be renamed as part of this bead? | Yes — rename to `.Baseline` (or `.Before`) so the struct field name stays honest for all methods. | Done in the same bead to avoid leaving a field name that's true only for `github-releases`. |
| Should the change ship as one commit or two? | Two — schema migration first (no behavior change), detection implementation second. | Isolates bikeshedding risk on Decisions 2-5 from the mechanical rename. |

---

## Next Steps

Continue brainstorm through Decisions 2-5 before handing off to the Plan skill.
