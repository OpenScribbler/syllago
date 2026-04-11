# capmon Information Architecture — Implementation Plan

**Status:** Active — quality review passed, parity validation in progress  
**Created:** 2026-04-11  
**Design doc:** `docs/plans/2026-04-10-capmon-information-architecture.md`  
**Feature state:** `.develop/capmon-information-architecture.json`  
**Bead:** `syllago-jtafb`

---

## Overview

Implements the capmon format doc pipeline: a structured YAML-first system for tracking AI provider
capability formats and detecting drift via SHA-256 content hashing. Replaces the SeederSpec
human-action gate with a deterministic `capmon derive` command that reads format docs and produces
seeder specs automatically.

**Critical path:** Phase 1 → Phase 2/3/4 (parallel) → Phase 5 → Phase 6 → Phase 7 + Phase 8

**Infrastructure to reuse:** `FetchSource`, `SHA256Hex`, `GHRunner`, `SetGHCommandForTest`,
`SetHTTPClientForTest`, `LoadSourceManifest` — all exist in `cli/internal/capmon/`.

### What this replaces

| Aspect | Before | After |
|--------|--------|-------|
| Format doc updates | Optional, no enforcement | Primary output — LLM populates directly |
| Graduation | Ad hoc, no mechanism | Explicit detection → graduation issue → human PR |

Content fetching uses HTTP (not browser/readability) — sources are fetched via `FetchSource` using
Go's standard HTTP client, not a browser automation tool. This keeps CI costs at zero.

### Files Summary

| Path | Purpose |
|------|---------|
| `docs/spec/canonical-keys.yaml` | Authoritative canonical key vocabulary |
| `docs/provider-formats/<slug>.yaml` | YAML format docs (replaces .md files) |
| `cli/internal/capmon/formatdoc.go` | Go types for format doc schema |
| `cli/internal/capmon/derive.go` | Format doc → SeederSpec derivation |
| `cli/cmd/syllago/capmon_check_cmd.go` | `capmon check` command |
| `cli/cmd/syllago/capmon_derive_cmd.go` | `capmon derive` command |
| `cli/cmd/syllago/capmon_onboard_cmd.go` | `capmon onboard` command |
| `docs/workflows/update-format-doc.md` | LLM agent instructions for format doc updates |
| `docs/workflows/graduation-comparison.md` | LLM agent instructions for graduation comparison |

---

## Phase 1: Core Types + Canonical Keys

### Task 1: Create docs/spec/canonical-keys.yaml

**Files:**
- `docs/spec/canonical-keys.yaml` (new)

**Depends on:** nothing

**Description:**  
Create the canonical key vocabulary file for the `skills` content type. This is the machine-readable
source of truth that `capmon validate-format-doc` and `capmon derive` check against. Initial
vocabulary is 13 keys derived from `canonicalKeyFromYAMLKey()` in `recognize.go` and the seeder
specs in `.develop/seeder-specs/`.

Schema: `content_types.skills.<key>: { description: "...", type: string|bool|object }`

Keys to include: `display_name`, `description`, `license`, `compatibility`, `metadata_map`,
`disable_model_invocation`, `user_invocable`, `version`, `project_scope`, `global_scope`,
`shared_scope`, `canonical_filename`, `custom_filename`

**Tests:** No Go tests for this task — the YAML file is validated by Task 3's test suite.

**Success criteria:** File exists with all 13 skills keys. `yq . docs/spec/canonical-keys.yaml`
parses without error.

---

### Task 2: Create cli/internal/capmon/formatdoc.go

**Files:**
- `cli/internal/capmon/formatdoc.go` (new)
- `cli/internal/capmon/formatdoc_test.go` (new)

**Depends on:** Task 1

**Description:**  
Define Go types. `FormatDocPath` computes the canonical path for any purpose. Confidence
(Value / Definition): `unknown` means mapping believed likely no source material.
`ProviderExtension`: named stable id (structural diff), sourced link back to where found,
flagged for graduation candidacy. `LoadFormatDoc`. `content_hash` round-trip.

Types to define:
- `FormatDoc` — top-level struct with `Provider`, `LastFetchedAt`, `LastChangedAt`,
  `GenerationMethod`, `ContentTypes`
- `ContentTypeFormatDoc` — `Status`, `Sources`, `CanonicalMappings`, `ProviderExtensions`,
  `LoadingModel`, `Notes`
- `SourceRef` — `URI`, `Type`, `FetchMethod`, `ContentHash`, `FetchedAt`
- `CanonicalMapping` — `Supported`, `Mechanism`, `Paths`, `Confidence`
- `ProviderExtension` — `ID`, `Name`, `Description`, `SourceRef`, `GraduationCandidate`,
  `GraduationNotes`

The `ProviderExtension` struct captures provider-specific capabilities that have no canonical key
yet. Each extension is:
- **Named** — stable `id` field (snake_case, unique within provider+content_type) used for
  structural diff to detect new additions
- **Sourced** — `source_ref` link back to where the capability was found
- **Flagged for graduation candidacy** when appropriate via `graduation_candidate` bool

The `confidence` field on `CanonicalMapping` uses a controlled vocabulary:
- `confirmed` — explicit field definition in source code or unambiguous official documentation
- `inferred` — appears in examples or implied by documentation that does not formally define it
- `unknown` — the mapping is believed likely but no source material clearly confirms or denies it

Functions:
- `LoadFormatDoc(path string) (*FormatDoc, error)` — reads + unmarshals YAML
- `FormatDocPath(formatsDir, provider string) string` — returns `<formatsDir>/<provider>.yaml`

**Step 0 — failing test:**  
`formatdoc_test.go` must include `TestLoadFormatDoc_RoundTrip` that:
1. Creates a fixture YAML matching the design doc example (amp-style)
2. Calls `LoadFormatDoc`
3. Verifies `content_hash` field survives (not lost in round-trip)
4. Marshals back to YAML and confirms no field loss
5. Tests `FormatDocPath` returns correct path

**Step 1 — implementation:**  
Write `formatdoc.go` with types and functions. YAML tags must use snake_case matching the design
doc schema. `content_hash` tag is `yaml:"content_hash"`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestLoadFormatDoc -v`

**Coverage target:** ~80%

---

## Phase 2: validate-format-doc

*Parallel with Phases 3 and 4 after Phase 1 is complete.*

### Task 3: Create cli/internal/capmon/formatdoc_validate.go

**Files:**
- `cli/internal/capmon/formatdoc_validate.go` (new)
- `cli/internal/capmon/formatdoc_validate_test.go` (new)

**Depends on:** Task 1, Task 2

**Description:**  
Implement `ValidateFormatDoc(formatsDir, canonicalKeysPath, provider string) error` with
human-readable checkmark/cross output. Validation rules:

1. Required top-level fields: `provider`, `last_fetched_at`, `content_types` (all non-empty)
2. Each key in `canonical_mappings` must exist in `canonical-keys.yaml` under the matching
   content type
3. Each `provider_extensions` entry must have: `id`, `name`, `description`, `source_ref`
4. `confidence` values must be `confirmed | inferred | unknown`
5. `generation_method` and `notes` fields: NOT validated for content (informational only)

Package-level override vars:
- `var capmonFormatDocsDirOverride string`
- `var capmonCanonicalKeysDirOverride string`

Output format (human-readable):
```
✓ Schema valid
✓ All canonical_mappings keys exist in docs/spec/canonical-keys.yaml
✓ All provider_extensions have required fields (id, name, description, source_ref)
✗ content_types.skills.canonical_mappings.unknown_key: not in canonical-keys.yaml
```

**Step 0 — failing test:**  
`formatdoc_validate_test.go` must include:
- `TestValidateFormatDoc_Valid` — valid file exits clean
- `TestValidateFormatDoc_UnknownKey` — unknown canonical key returns error with key name
- `TestValidateFormatDoc_MissingExtensionField` — provider_extension missing `description`
- `TestValidateFormatDoc_InvalidConfidence` — confidence `"maybe"` returns error
- `TestValidateFormatDoc_MissingProvider` — empty `provider` field returns error
- `TestValidateFormatDoc_InformationalFieldsNotValidated` — `notes: "anything"` passes

**Step 1 — implementation:**  
Write `ValidateFormatDoc`. Load canonical keys YAML first, then load format doc, then check each
rule. Collect all errors (don't short-circuit at first) so output shows complete violation list.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestValidateFormatDoc -v`

**Coverage target:** ~85%

---

### Task 4: Create cli/cmd/syllago/capmon_validate_format_doc_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_validate_format_doc_cmd.go` (new)
- `cli/cmd/syllago/capmon_validate_format_doc_cmd_test.go` (new)

**Depends on:** Task 3

**Description:**  
Cobra command `capmon validate-format-doc --provider=<slug>`.

Flags:
- `--provider` (required)
- `--formats-dir` (default `docs/provider-formats`)
- `--canonical-keys` (default `docs/spec/canonical-keys.yaml`)

Register under `capmonCmd.AddCommand`. Apply telemetry enrichment for `provider`.

**Step 0 — failing test:**  
`capmon_validate_format_doc_cmd_test.go` must include:
- `TestCapmonValidateFormatDocCmd_Registered` — command exists under `capmonCmd`
- `TestCapmonValidateFormatDocCmd_ValidFile` — valid fixture → exit 0
- `TestCapmonValidateFormatDocCmd_UnknownKey` — bad fixture → exit 1 with message
- `TestCapmonValidateFormatDocCmd_MissingProvider` — no `--provider` → exit 1

**Step 1 — implementation:**  
Wire `capmon validate-format-doc`. Use `capmonFormatDocsDirOverride` and
`capmonCanonicalKeysDirOverride` for test redirectability (same pattern as `capmonSpecsDirOverride`).

**Step 2 — verify:**  
`cd cli && go test ./cmd/syllago/ -run TestCapmonValidateFormatDoc -v`

**Coverage target:** ~85%

---

## Phase 3: validate-sources

*Parallel with Phases 2 and 4 after Phase 1 is complete.*

### Task 5: Add Supported field and create sourceman_validate.go

**Files:**
- `cli/internal/capmon/sourceman.go` (modify)
- `cli/internal/capmon/sourceman_validate.go` (new)

**Depends on:** Task 1

**Description:**  
Add `Supported *bool` to `ContentTypeSource` struct in `sourceman.go`. This allows source manifests
to explicitly mark a content type as not supported (vs. just having zero URIs).

Then implement `ValidateSources(sourcesDir, provider string) error`:
- Load manifest via `LoadSourceManifest`
- For each content type: error if zero source URIs AND no explicit `Supported: false`
- Package-level override var: `var capmonSourcesDirOverride string`

**Step 0 — failing test:**  
Add to `sourceman_test.go` (or create `sourceman_validate_test.go`):
- `TestValidateSources_AllHaveSources` — all content types have URIs → no error
- `TestValidateSources_MissingURIs` — content type with empty sources, no supported=false → error
- `TestValidateSources_SupportedFalseSkipped` — explicit `supported: false` → no error for that type

**Step 1 — implementation:**  
Add `Supported *bool \`yaml:"supported,omitempty"\`` to `ContentTypeSource`. Write
`ValidateSources` function in new file `sourceman_validate.go`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestValidateSources -v`

**Coverage target:** ~90%

---

### Task 6: Create cli/cmd/syllago/capmon_validate_sources_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_validate_sources_cmd.go` (new)
- `cli/cmd/syllago/capmon_validate_sources_cmd_test.go` (new)

**Depends on:** Task 5

**Description:**  
Cobra command `capmon validate-sources --provider=<slug>`.

Flags:
- `--provider` (required)
- `--sources-dir` (default `docs/provider-sources`)

Register under `capmonCmd.AddCommand`. Apply telemetry enrichment for `provider`.

**Step 0 — failing test:**  
- `TestCapmonValidateSourcesCmd_Registered`
- `TestCapmonValidateSourcesCmd_MissingProvider`
- `TestCapmonValidateSourcesCmd_ValidManifest`
- `TestCapmonValidateSourcesCmd_BrokenManifest` — missing URIs → exit 1

**Step 1 — implementation:**  
Wire command with `capmonSourcesDirOverride` for test redirectability.

**Step 2 — verify:**  
`cd cli && go test ./cmd/syllago/ -run TestCapmonValidateSources -v`

**Coverage target:** ~85%

---

## Phase 4: derive + SeederSpec Refactor

*Parallel with Phases 2 and 3 after Phase 1 is complete.*

### Task 7: Create cli/internal/capmon/derive.go

**Files:**
- `cli/internal/capmon/derive.go` (new)
- `cli/internal/capmon/derive_test.go` (new)

**Depends on:** Task 1, Task 2

**Description:**  
Implement the deterministic derivation: format doc → seeder spec.

Functions:
- `DeriveSeederSpec(formatDoc *FormatDoc, canonicalKeysPath string) (*SeederSpec, error)`
  - Validates each canonical key against `canonical-keys.yaml`; exits non-zero on unknown key
  - Skips content types where `status == "unsupported"`
  - Deterministic: identical input produces identical output (no timestamps, no UUIDs)
- `WriteSeederSpec(spec *SeederSpec, path string) error`
  - Atomic write: write to temp file in same directory, then `os.Rename`

**Step 0 — failing test:**  
`derive_test.go` must include:
- `TestDeriveSeederSpec_Deterministic` — call twice with same input, `reflect.DeepEqual`
- `TestDeriveSeederSpec_UnknownKey` — canonical_mappings key not in vocab → error
- `TestDeriveSeederSpec_UnsupportedSkipped` — status=unsupported content type is omitted
- `TestWriteSeederSpec_AtomicWrite` — verify temp file pattern (write + rename, not direct create)

**Step 1 — implementation:**  
Write `derive.go`. Atomic write: use `os.CreateTemp` in same directory as target, write, sync,
close, then `os.Rename`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestDerive -v`

**Coverage target:** ~85%

---

### Task 8: Refactor cli/internal/capmon/seederspec.go

**Files:**
- `cli/internal/capmon/seederspec.go` (modify)
- `cli/internal/capmon/seederspec_test.go` (modify)

**Depends on:** Task 7

**Description:**  
Remove the human-action gate from `SeederSpec`. The gate is replaced by `capmon derive` which
runs deterministically without human approval.

Removals:
- `HumanAction string` field from `SeederSpec`
- `ReviewedAt string` field from `SeederSpec`
- `ValidateSeederSpec(spec *SeederSpec) error` function entirely

Keep: `LoadSeederSpec`, `SeederSpecPath`, `ProposedMapping`

Also update `seederspec_test.go`: remove all `ValidateSeederSpec` test cases. Keep `LoadSeederSpec`
tests.

**Coverage target:** ~80% (removal task — target applies to remaining `LoadSeederSpec` coverage)

**Step 0 — pre-removal verification:**  
Before removing fields, confirm existing tests reference `HumanAction`/`ValidateSeederSpec`:
- Grep `seederspec_test.go` for `ValidateSeederSpec` and `HumanAction`
- Those tests must be removed, not just commented out

**Step 1 — implementation:**  
Remove fields and function from `seederspec.go`. Update `seederspec_test.go` to remove removed
test coverage. Verify `derive.go` does not reference removed fields.

**Step 2 — verify:**  
`cd cli && make build && go test ./internal/capmon/ -run TestSeederSpec -v`

---

### Task 9: Refactor cli/internal/capmon/seed.go

**Files:**
- `cli/internal/capmon/seed.go` (modify)
- `cli/internal/capmon/seed_test.go` (verify, no new tests needed)

**Depends on:** Task 8

**Description:**  
Remove the seeder spec gate block from `SeedProviderCapabilities`. The block (lines ~28–50)
validates `SeederSpecsDir`, calls `LoadSeederSpec`, calls `ValidateSeederSpec`, and checks
`HumanAction`. All of this is now replaced by the derive pipeline.

Also remove `SeederSpecsDir string` from `SeedOptions` if it was only used for gate enforcement.
(It was — `seed.go` only uses it in the gate block.)

**Coverage target:** ~80% (removal task — existing seed tests should continue to pass)

**Step 0 — pre-removal verification:**  
Verify existing `seed_test.go` tests don't depend on `SeederSpecsDir`. If they do, update them
to omit the field. The removal should not break any seed functionality — just simplify.

**Step 1 — implementation:**  
Delete the gate block (lines 27–50 in current `seed.go`). Remove `SeederSpecsDir` from
`SeedOptions`. Verify `capmon_cmd.go` also stops setting `SeederSpecsDir`.

**Step 2 — verify:**  
`cd cli && make build && go test ./internal/capmon/ -run TestSeed -v`

---

### Task 10: Clean up cli/cmd/syllago/capmon_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_cmd.go` (modify)
- `cli/cmd/syllago/capmon_validate_spec_cmd.go` (delete)
- `cli/cmd/syllago/capmon_validate_spec_cmd_test.go` (delete)

**Depends on:** Task 8, Task 9

**Description:**  
The reason for this change: the spec-gate path has been replaced by the deterministic derive
pipeline. Remove the old `validate-spec` command and its associated state from `capmon_cmd.go`.
Also remove the `--skip-spec-gate` flag from `capmon seed`.

Removals from `capmon_cmd.go`:
- `capmonSpecsDirOverride` package var
- `capmonValidateSpecCmd` variable and definition
- `capmonCmd.AddCommand(capmonValidateSpecCmd)` registration
- `capmonSeedCmd` flags: `--skip-spec-gate`
- `capmonSeedCmd` RunE: the `skipSpecGate` / `seederSpecsDir` block
- `SeedOptions.SeederSpecsDir` assignment

Delete `capmon_validate_spec_cmd.go` and `capmon_validate_spec_cmd_test.go` entirely.

**Step 0 — pre-deletion verification:**  
`TestCapmonValidateSpecCmd` tests will all fail after deletion — this is expected. Verify the
test file `capmon_validate_spec_cmd_test.go` is fully removed.

**Step 1 — implementation:**  
Delete two files. Edit `capmon_cmd.go` to remove all references. Ensure `make build` passes.

**Step 2 — verify:**  
`cd cli && make build && go test ./cmd/syllago/ -v 2>&1 | grep -v "PASS\|SKIP"`

**Coverage target:** N/A (deletion task — no new coverage, existing coverage removed)

---

### Task 11: Create cli/cmd/syllago/capmon_derive_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_derive_cmd.go` (new)
- `cli/cmd/syllago/capmon_derive_cmd_test.go` (new)

**Depends on:** Task 7, Task 10

**Description:**  
Cobra command `capmon derive --provider=<slug>`.

Flags:
- `--provider` (required)
- `--formats-dir` (default `docs/provider-formats`)
- `--output-dir` (default `.develop/seeder-specs`)
- `--canonical-keys` (default `docs/spec/canonical-keys.yaml`)

Override vars for test redirectability (same pattern as other capmon commands).
Apply telemetry enrichment for `provider`.

**Step 0 — failing test:**  
- `TestCapmonDeriveCmd_Registered`
- `TestCapmonDeriveCmd_MissingProvider`
- `TestCapmonDeriveCmd_ValidFormatDoc` — fixture format doc → seeder spec written to temp dir

**Step 1 — implementation:**  
Wire command. Call `LoadFormatDoc` then `DeriveSeederSpec` then `WriteSeederSpec`.

**Step 2 — verify:**  
`cd cli && go test ./cmd/syllago/ -run TestCapmonDerive -v`

**Coverage target:** ~85%

---

## Phase 5: Format Doc Migration

*Requires Phases 1, 2, and 4 complete. All-at-once in single PR.*

### Task 12: Migrate 14 .md files to .yaml format docs

**Files:**
- `docs/provider-formats/*.yaml` (14 new files)
- `docs/provider-formats/*.md` (14 deleted)
- `docs/provider-formats/cline.md.proposed-additions` (deleted)
- `cli/internal/capmon/formatdoc_test.go` (extend — add `TestValidateAllFormatDocs`)

**Depends on:** Task 3, Task 7 (ValidateFormatDoc + DeriveSeederSpec must exist)

**Description:**  
Graduation issue detection replaces ad hoc. Before: no enforcement. After: format docs primary
output, explicit detection pipeline. Convert all 14 existing `.md` format docs to `.yaml`. Providers:
`amp`, `claude-code`, `cline`, `codex`, `copilot-cli`, `cursor`, `factory-droid`, `gemini-cli`,
`kiro`, `opencode`, `pi`, `roo-code`, `windsurf`, `zed`.

**Source mapping per YAML file:**
- `canonical_mappings`: source from `.develop/seeder-specs/<provider>-skills.yaml`
  `proposed_mappings` — fields: `canonical_key`, `mechanism`, `confidence`
- `sources`: source from `docs/provider-sources/<slug>.yaml` `sources` block
- `content_hash`: initialize to `""` (first CI run treats all as changed)
- `generation_method`: set to `human-edited`
- `provider_extensions`: start empty (LLM populates on first pipeline run)
- `factory-droid` and `pi` are orphaned (not in providers.json) — include in migration,
  they will appear as orphan warnings in `capmon check`

**Migration validation test (`TestValidateAllFormatDocs`):**  
This is the automated merge gate. Test:
1. Glob all `docs/provider-formats/*.yaml` relative to repo root
2. Call `ValidateFormatDoc` on each
3. Assert no errors
If any file fails, the migration PR is blocked.

**Step 0 — failing test:**  
Add `TestValidateAllFormatDocs` to `formatdoc_test.go`. It will fail until all YAML files exist
and are valid.

**Step 1 — implementation:**  
Write all 14 YAML files. Run `TestValidateAllFormatDocs` after each batch. Fix any issues.
Delete all 14 `.md` files and `cline.md.proposed-additions`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestValidateAllFormatDocs -v`

---

## Phase 6: capmon check

*Requires all of Phases 1–5.*

### Task 13: Create cli/internal/capmon/fetch_validity.go

**Files:**
- `cli/internal/capmon/fetch_validity.go` (new)
- `cli/internal/capmon/fetch_validity_test.go` (new)

**Depends on:** Task 12

**Description:**  
Validates HTML and text content readability via HTTP. Mozilla Readability is not used — Go's
standard HTTP client plus Content-Type header checks suffice. Implement the content validity
predicate:

`ValidateContentResponse(body []byte, contentType, originalURL, finalURL string) error`

Three checks:
1. `len(body) >= 512` — minimum meaningful content
2. `contentType` does not match binary MIME prefixes (`image/`, `video/`, `audio/`, `application/octet-stream`, `application/zip`, etc.)
3. eTLD+1 domain match between `originalURL` and `finalURL` — detect redirect-to-login domain hijacking

Returns typed `ErrContentInvalid` (sentinel value) for distinguishable error handling downstream.

For eTLD+1: use `golang.org/x/net/publicsuffix` if already in go.mod, otherwise extract domain
with simple host comparison (strip subdomains).

**Step 0 — failing test:**  
- `TestValidateContentResponse_Valid` — 1000-byte text/html response, same domain
- `TestValidateContentResponse_TooSmall` — 400-byte body → error
- `TestValidateContentResponse_BinaryContentType` — `image/png` → error
- `TestValidateContentResponse_DomainMismatch` — redirected to `login.example.com` from `docs.example.com` — same eTLD+1, should pass; but `otherdomain.com` should fail

**Step 1 — implementation:**  
Write `fetch_validity.go`. Define `ErrContentInvalid` as a sentinel type. Keep domain check simple
for v1 — exact host comparison is acceptable if `publicsuffix` is not available.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestValidateContentResponse -v`

**Coverage target:** ~85%

---

### Task 14: Create cli/internal/capmon/check_diff.go

**Files:**
- `cli/internal/capmon/check_diff.go` (new)
- `cli/internal/capmon/check_diff_test.go` (new)

**Depends on:** Task 12

**Description:**  
Implement unified diff generation with truncation:

`GenerateUnifiedDiff(oldContent, newContent []byte, path string) (string, error)`

Truncation rules:
- `source_code` type: truncate at 500 lines
- All other types: truncate at 200 lines
- If truncated: append indicator line:
  `[truncated after N lines (~X bytes shown) — full diff at .capmon-cache/<slug>/]`

**Step 0 — failing test:**  
- `TestGenerateUnifiedDiff_NoTruncation` — 100-line diff, no truncation
- `TestGenerateUnifiedDiff_SourceCodeTruncation` — 600-line source_code diff, truncated at 500
- `TestGenerateUnifiedDiff_OtherTruncation` — 300-line documentation diff, truncated at 200
- `TestGenerateUnifiedDiff_TruncationIndicator` — indicator line format correct

**Step 1 — implementation:**  
Use `github.com/sergi/go-diff` if available, otherwise use the existing `diff.go` package in
`cli/internal/capmon/`. Check what `diff.go` already provides — it may be reusable.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestGenerateUnifiedDiff -v`

**Coverage target:** ~85%

---

### Task 15: Extend cli/internal/capmon/report.go

**Files:**
- `cli/internal/capmon/report.go` (modify)
- `cli/internal/capmon/report_test.go` (modify)

**Depends on:** Task 12

**Description:**  
Add three functions to `report.go` for capmon issue deduplication and management:

1. `FindOpenCapmonIssue(provider, contentType string) (int, bool, error)`  
   Uses `gh issue list --label=capmon-change --label=provider:<slug>` and filters by hidden
   HTML comment anchor `<!-- capmon-check: <slug>/<content_type> -->`.

2. `CreateCapmonChangeIssue(ctx context.Context, provider, contentType, title, body string) (int, error)`  
   Creates issue with labels `capmon-change`, `provider:<slug>`. Includes anchor comment in body.

3. `AppendCapmonChangeEvent(ctx context.Context, issueNumber int, body string) error`  
   Appends a new comment to an existing issue.

All three use `ghRunner` (already exists) — fully mockable via `SetGHCommandForTest`.

**Step 0 — failing test:**  
Add to `report_test.go`:
- `TestFindOpenCapmonIssue_Found` — stub gh returning matching issue JSON
- `TestFindOpenCapmonIssue_NotFound` — stub gh returning empty list
- `TestCreateCapmonChangeIssue_Success` — verify anchor comment in body
- `TestAppendCapmonChangeEvent_Success` — verify comment appended

**Step 1 — implementation:**  
Add three functions. Parse issue number from `gh issue create --json number` output.
Anchor comment format: `<!-- capmon-check: <provider>/<content_type> -->`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestFindOpenCapmon\|TestCreateCapmon\|TestAppendCapmon -v`

**Coverage target:** ~85%

---

### Task 16: Create cli/internal/capmon/check.go

**Files:**
- `cli/internal/capmon/check.go` (new)
- `cli/internal/capmon/check_test.go` (new)

**Depends on:** Task 13, Task 14, Task 15

**Description:**  
Implement `RunCapmonCheck(ctx context.Context, opts CapmonCheckOptions) error`.

`CapmonCheckOptions`:
- `ProvidersJSON string` (default `providers.json`)
- `FormatsDir string` (default `docs/provider-formats`)
- `SourcesDir string` (default `docs/provider-sources`)
- `CacheRoot string` (default `.capmon-cache`)
- `CanonicalKeysPath string` (default `docs/spec/canonical-keys.yaml`)
- `ProviderFilter string` (single provider slug, empty = all)
- `DryRun bool`

Pipeline steps:
- **Step 0:** Orphan detection — format docs whose slug is not in `providers.json` → non-blocking
  warning logged to stderr
- **Step 1:** `ValidateSources` for each provider — blocking, exit 1 on failure
- **Step 2:** `ValidateFormatDoc` for each format doc — blocking, exit 1 on failure
- **Step 3:** For each provider × content_type × source URI:
  - `FetchSource` → `ValidateContentResponse` → SHA-256 → compare vs `content_hash` in format doc
  - No format doc = new provider, treat all sources as changed
  - Fetch/validity error → issue with label `capmon-fetch-error`
  - Changed → `GenerateUnifiedDiff` → `FindOpenCapmonIssue` → create or append issue
- **`--dry-run`:** Log actions, no GitHub writes

**Step 0 — failing test:**  
- `TestRunCapmonCheck_NoChange` — content hash matches, no issues created
- `TestRunCapmonCheck_Changed` — hash mismatch, issue creation called
- `TestRunCapmonCheck_FetchError` — fetch fails, fetch-error issue created
- `TestRunCapmonCheck_ContentValidityFailure` — body too small → fetch-error issue
- `TestRunCapmonCheck_OrphanDetection` — format doc with no providers.json entry → warning only
- `TestRunCapmonCheck_DryRun` — log output only, no gh calls

Use `SetHTTPClientForTest` and `SetGHCommandForTest` for all mocking.

**Step 1 — implementation:**  
Write `check.go`. Load providers.json for orphan check (simple JSON file with array of slugs).
Build per-provider loop. Dry-run: gate all `ghRunner` calls behind `!opts.DryRun`.

**Step 2 — verify:**  
`cd cli && go test ./internal/capmon/ -run TestRunCapmonCheck -v`

**Coverage target:** ~80%

---

### Task 17: Create cli/cmd/syllago/capmon_check_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_check_cmd.go` (new)
- `cli/cmd/syllago/capmon_check_cmd_test.go` (new)

**Depends on:** Task 16

**Description:**  
Cobra command `capmon check --all | --provider=<slug>` (mutually exclusive).

Flags:
- `--all` (bool)
- `--provider` (string)
- `--formats-dir` (default `docs/provider-formats`)
- `--sources-dir` (default `docs/provider-sources`)
- `--cache-root` (default `.capmon-cache`)
- `--providers-json` (default `providers.json`)
- `--canonical-keys` (default `docs/spec/canonical-keys.yaml`)
- `--dry-run` (bool)

Mutual exclusion: `--all` and `--provider` cannot both be set.
Apply telemetry enrichment for `dry_run` and `provider`.

**Step 0 — failing test:**  
- `TestCapmonCheckCmd_Registered`
- `TestCapmonCheckCmd_MutualExclusion` — both --all and --provider → exit 1
- `TestCapmonCheckCmd_DryRunFlag` — `--dry-run` flag accepted

**Step 1 — implementation:**  
Wire `capmon check`. Validate mutual exclusion in `RunE` before calling `RunCapmonCheck`.

**Step 2 — verify:**  
`cd cli && go test ./cmd/syllago/ -run TestCapmonCheck -v`

**Coverage target:** ~80%

---

## Phase 7: capmon onboard

*Requires Phases 3 and 6. Parallel with Phase 8.*

### Task 18: Create cli/cmd/syllago/capmon_onboard_cmd.go

**Files:**
- `cli/cmd/syllago/capmon_onboard_cmd.go` (new)
- `cli/cmd/syllago/capmon_onboard_cmd_test.go` (new)

**Depends on:** Task 6, Task 17

**Description:**  
Cobra command `capmon onboard --provider=<slug>`.

This command handles the **new provider case**, where no cached baseline exists. Unlike a normal
incremental update (where only changed sources are flagged), onboarding treats all sources as
changed because there is no prior `content_hash` baseline to compare against. Review priority is
higher for a new provider — it is the first complete picture of that provider's capabilities.

Workflow:
1. `ValidateSources` — exit 1 if fails
2. Fetch all sources via `FetchSource`
3. `CreateCapmonChangeIssue` for all sources (new provider = all changed)

Flags:
- `--provider` (required)
- `--sources-dir` (default `docs/provider-sources`)
- `--formats-dir` (default `docs/provider-formats`)

Override var: `var capmonOnboardSourcesDirOverride string`

**Step 0 — failing test:**  
- `TestCapmonOnboardCmd_Registered`
- `TestCapmonOnboardCmd_MissingProvider`
- `TestCapmonOnboardCmd_ValidateSources_CalledFirst` — validate-sources error short-circuits
- `TestCapmonOnboardCmd_IssueCreated` — sources fetched, issue created (with `SetGHCommandForTest`)

**Step 1 — implementation:**  
Wire command. Call `ValidateSources` first, then iterate sources, then create issue.

**Step 2 — verify:**  
`cd cli && go test ./cmd/syllago/ -run TestCapmonOnboard -v`

**Coverage target:** ~80%

---

## Phase 8: Workflow Docs + CI Workflow

*Requires Phases 5 and 6. Parallel with Phase 7.*

### Task 19: Extract workflow docs from design doc

**Files:**
- `docs/workflows/update-format-doc.md` (new)
- `docs/workflows/graduation-comparison.md` (new)

**Depends on:** Task 12

**Description:**  
update-format-doc: inferred = appears in examples; unknown = you believe capability exists, no
source; graduation_candidate false, positive evidence; extension: clear name, description what it
does and why it matters. graduation-comparison: concept(s) overlap, which providers have it, what
names, suggested canonical key name and definition.

Extract verbatim from design doc:
- `update-format-doc.md` from section `### docs/workflows/update-format-doc.md (target file)`
- `graduation-comparison.md` from section `### docs/workflows/graduation-comparison.md (target file)`

**`update-format-doc.md` instructs the agent on:**
- Confidence definitions: `confirmed` (explicit source code or doc statement), `inferred` (appears
  in examples or implied by documentation), `unknown` (believed likely but no source material
  clearly confirms or denies it)
- `graduation_candidate` semantics: default is `false` meaning "not yet evaluated" — set `true`
  only with positive evidence another provider already has the same concept. Do not set
  `graduation_candidate: true` without evidence.
- Provider extensions naming conventions: give each extension a stable `id` (snake_case), a clear
  name, and a `source_ref` pointing to where it was found
- What NOT to do: do not invent canonical keys, do not summarize source content

**`graduation-comparison.md` instructs the agent on:**
- The graduation issue output format: for each candidate, produce a section with which concept(s)
  overlap, which providers have it under what names (format: PROVIDER-SLUG: extension EXTENSION-ID),
  suggested canonical key name and definition
- Provider extensions comparison: read ALL `provider_extensions` entries across ALL provider format
  docs, regardless of `graduation_candidate` flag value
- `graduation_candidate: false` semantics: means "not yet evaluated by the graduation agent" — not
  a gate, do not skip entries based on this flag
- Anti-recurrence check: before creating a graduation issue, query closed graduation issues in
  GitHub; if a closed issue already covers this concept pairing, do NOT re-flag it
- Flag only clear semantic equivalents — do not flag tenuous connections

**Tests:** No Go tests — these are documentation files.

**Success criteria:** Both files exist and match the content in the design doc verbatim.

---

### Task 20: Create .github/workflows/capmon-check.yml

**Files:**
- `.github/workflows/capmon-check.yml` (new)

**Depends on:** Task 17

**Description:**  
GitHub Actions workflow for automated drift detection.

Schedule:
- `0 */12 * * 1-5` — Mon–Fri every 12 hours
- `0 6 * * 0,6` — Sat–Sun once at 6am

Also `workflow_dispatch` for manual runs.

Permissions: `issues: write`, `contents: read`

Single job: `checkout` → `setup-go` → `build` → `./cli/syllago capmon check --all`

SHA-pin all action versions (match pattern in existing `.github/workflows/` files).
No Chromedp container, no artifact upload. Lean HTTP-only check.

**Tests:** Verify YAML is syntactically valid. If `actionlint` is available, run it.

**Success criteria:** File exists and parses as valid YAML. Action versions are SHA-pinned.

---

### Task 21: Add deprecation notice to inspect-provider-skills.md

**Files:**
- `docs/workflows/inspect-provider-skills.md` (modify)

**Depends on:** Task 19

**Description:**  
Add deprecation notice at top of `docs/workflows/inspect-provider-skills.md` pointing to
`docs/workflows/update-format-doc.md` as the replacement workflow.

**Tests:** No Go tests.

**Success criteria:** File has deprecation notice linking to `update-format-doc.md`.

---

## Test Coverage Summary

| Phase | Primary test files | Target |
|-------|-------------------|--------|
| 1 | `formatdoc_test.go` | ~80% |
| 2 | `formatdoc_validate_test.go`, `capmon_validate_format_doc_cmd_test.go` | ~85% |
| 3 | `sourceman_validate_test.go`, `capmon_validate_sources_cmd_test.go` | ~90% |
| 4 | `derive_test.go`, `capmon_derive_cmd_test.go` | ~85% |
| 5 | `TestValidateAllFormatDocs` (integration migration gate) | — |
| 6 | `fetch_validity_test.go`, `check_diff_test.go`, `check_test.go`, `capmon_check_cmd_test.go` | ~80% |
| 7 | `capmon_onboard_cmd_test.go` | ~80% |
| 8 | N/A (docs + YAML) | — |

**Regression guards:**
- `seederspec_test.go`: `ValidateSeederSpec` tests removed (Task 8)
- `capmon_validate_spec_cmd_test.go`: deleted entirely (Task 10)
- `seed_test.go`: verify gate removal doesn't break existing seed tests (Task 9)

---

## Decisions Made During Planning

1. **Task numbering:** Sequential integers (1–21) across all phases, not decimal (1.1, 1.2) — required by validate-parity hook.
2. **Phase 5 all-at-once:** Migration is a single all-or-nothing PR with `TestValidateAllFormatDocs` as the gate.
3. **`publicsuffix` dependency:** Use if already in go.mod; otherwise simple host comparison for v1. Check before implementing Task 13.
4. **`diff.go` reuse:** Check existing `cli/internal/capmon/diff.go` before pulling in `go-diff` for Task 14.
5. **`capmonValidateSpecCmd` deletion:** Entire file + test file deleted in Task 10, not just unregistered.
