# capmon Implementation Plan

**Status:** Active — implementation in progress  
**Created:** 2026-04-11  
**Design doc:** `docs/plans/2026-04-10-capmon-information-architecture.md`  
**Bead:** `syllago-jtafb`

---

## Phase Overview

| Phase | Scope | Depends On | Parallelizable |
|-------|-------|-----------|----------------|
| 1 | Core types + canonical-keys.yaml | — | — |
| 2 | `capmon validate-format-doc` | 1 | 3, 4 |
| 3 | `capmon validate-sources` | 1 | 2, 4 |
| 4 | `capmon derive` + SeederSpec refactor | 1 | 2, 3 |
| 5 | Format doc migration (14 .md → .yaml) | 1, 2, 4 | — |
| 6 | `capmon check` | 1–5 | — |
| 7 | `capmon onboard` | 3, 6 | 8 |
| 8 | Workflow docs + CI workflow | 5, 6 | 7 |

Critical path: **1 → 2/3/4 (parallel) → 5 → 6 → 7 + 8 (parallel)**

---

## Phase 1: Core Types + Canonical Keys

**Files:**
- `docs/spec/canonical-keys.yaml` (new)
- `cli/internal/capmon/formatdoc.go` (new)

**Tasks:**

1.1 Create `docs/spec/canonical-keys.yaml` — initial `skills` vocabulary only (13 keys):
  - From `canonicalKeyFromYAMLKey()` in `recognize.go`: `display_name`, `description`, `license`, `compatibility`, `metadata_map`, `disable_model_invocation`, `user_invocable`, `version`
  - From existing seeder specs in `.develop/seeder-specs/`: `project_scope`, `global_scope`, `shared_scope`, `canonical_filename`, `custom_filename`
  - Schema: `content_types.skills.<key>: { description: "...", type: string|bool|object }`
  - Do NOT add other content types in this phase

1.2 Create `cli/internal/capmon/formatdoc.go`:
  - Types: `FormatDoc`, `ContentTypeFormatDoc`, `SourceRef`, `CanonicalMapping`, `ProviderExtension`
  - `content_hash` YAML tag must match exactly
  - `LoadFormatDoc(path string) (*FormatDoc, error)`
  - `FormatDocPath(formatsDir, provider string) string`
  - No validation logic here (that's Phase 2's job)

**Tests:** `formatdoc_test.go` — round-trip load/marshal, `content_hash` field survival, zero-value handling (~80% target)

**Done when:** `LoadFormatDoc` round-trips the design doc example YAML. `canonical-keys.yaml` committed with all 13 skills keys.

---

## Phase 2: `capmon validate-format-doc`

*Can run in parallel with Phases 3 and 4 after Phase 1 merges.*

**Files:**
- `cli/internal/capmon/formatdoc_validate.go` (new)
- `cli/cmd/syllago/capmon_validate_format_doc_cmd.go` (new)

**Tasks:**

2.1 `ValidateFormatDoc(formatsDir, canonicalKeysPath, provider string) error`:
  - Required top-level fields: `provider`, `last_fetched_at`, `content_types`
  - Canonical key check: every key in `canonical_mappings` must exist in `canonical-keys.yaml` under the matching content type
  - Extension fields: each entry must have `id`, `name`, `description`, `source_ref`
  - Confidence values: must be `confirmed | inferred | unknown`
  - Informational fields `generation_method` and `notes`: not validated for content
  - Human-readable output (checkmark/cross lines matching design doc example)
  - Package-level override vars: `capmonFormatDocsDirOverride`, `capmonCanonicalKeysDirOverride`

2.2 `capmon validate-format-doc --provider=<slug>` cobra command:
  - Flags: `--provider` (required), `--formats-dir` (default `docs/provider-formats`), `--canonical-keys` (default `docs/spec/canonical-keys.yaml`)
  - Register under `capmonCmd.AddCommand`

**Tests:**
- `formatdoc_validate_test.go`: valid file, unknown key, missing extension field, invalid confidence, missing provider, all happy paths (~85% target)
- `capmon_validate_format_doc_cmd_test.go`: registered, valid file, unknown key, exit codes

**Done when:** `syllago capmon validate-format-doc --provider=amp` exits 0 on valid YAML, exits 1 on invalid with human-readable error.

---

## Phase 3: `capmon validate-sources`

*Can run in parallel with Phases 2 and 4 after Phase 1 merges.*

**Files:**
- `cli/internal/capmon/sourceman_validate.go` (new, or extend `sourceman.go`)
- `cli/cmd/syllago/capmon_validate_sources_cmd.go` (new)

**Tasks:**

3.1 `ValidateSources(sourcesDir, provider string) error`:
  - Load manifest via `LoadSourceManifest`
  - Error if any content type has zero source URIs (and no explicit `supported: false`)
  - May require adding `Supported *bool` to `ContentTypeSource` struct in `sourceman.go`
  - Package-level override var: `capmonSourcesDirOverride`

3.2 `capmon validate-sources --provider=<slug>` cobra command:
  - Flags: `--provider` (required), `--sources-dir` (default `docs/provider-sources`)
  - Register under `capmonCmd.AddCommand`

**Tests:**
- Unit: `TestValidateSources_AllHaveSources`, `TestValidateSources_MissingURIs`, `TestValidateSources_SupportedFalseSkipped`
- Command: registered, missing provider, valid manifest, broken manifest

**Done when:** `syllago capmon validate-sources --provider=amp` exits 0. Broken manifest exits 1 with actionable message.

---

## Phase 4: `capmon derive` + SeederSpec Refactor

*Can run in parallel with Phases 2 and 3 after Phase 1 merges.*

**Files:**
- `cli/internal/capmon/derive.go` (new)
- `cli/cmd/syllago/capmon_derive_cmd.go` (new)
- `cli/internal/capmon/seederspec.go` (modify — remove human_action gate)
- `cli/internal/capmon/seed.go` (modify — remove spec gate)
- `cli/cmd/syllago/capmon_cmd.go` (modify — remove validate-spec command)
- `cli/cmd/syllago/capmon_validate_spec_cmd.go` (delete)
- `cli/cmd/syllago/capmon_validate_spec_cmd_test.go` (delete)

**Tasks:**

4.1 Create `cli/internal/capmon/derive.go`:
  - `DeriveSeederSpec(formatDoc *FormatDoc, canonicalKeysPath string) (*SeederSpec, error)`
  - Deterministic: identical input → identical output. No timestamps, no UUIDs.
  - Validates each canonical key against `canonical-keys.yaml` (exit non-zero on unknown key)
  - Skips content types where `status == "unsupported"`
  - `WriteSeederSpec(spec *SeederSpec, path string) error` — atomic write (temp+rename, same filesystem)

4.2 Modify `cli/internal/capmon/seederspec.go`:
  - Remove `HumanAction` and `ReviewedAt` fields from `SeederSpec`
  - Remove `ValidateSeederSpec` function entirely
  - Keep `LoadSeederSpec`, `SeederSpecPath`, `ProposedMapping`

4.3 Modify `cli/internal/capmon/seed.go`:
  - Remove the seeder spec gate block (human_action validation, lines ~28–50)
  - Remove `SeedOptions.SeederSpecsDir` if it was only used for gate enforcement

4.4 Clean up `capmon_cmd.go`:
  - Remove `capmonValidateSpecCmd` registration
  - Remove `--skip-spec-gate` flag from `capmon seed`
  - Delete `capmon_validate_spec_cmd.go` and its test file

4.5 Create `cli/cmd/syllago/capmon_derive_cmd.go`:
  - `capmon derive --provider=<slug>` command
  - Flags: `--provider` (required), `--formats-dir`, `--output-dir`, `--canonical-keys`
  - Override vars for test redirectability

**Tests:**
- `derive_test.go`: determinism (run twice, `reflect.DeepEqual`), unknown key errors, unsupported type skipped, atomic write pattern
- `capmon_derive_cmd_test.go`: registered, missing provider, valid format doc → seeder spec output
- Update `seederspec_test.go`: remove `ValidateSeederSpec` tests, keep `LoadSeederSpec` tests

**Done when:** `syllago capmon derive --provider=amp` produces valid seeder spec from a fixture format doc. `make test` passes with removed tests replaced by new coverage.

---

## Phase 5: Format Doc Migration (14 .md → .yaml)

*Requires Phases 1, 2, and 4 to be merged. Single PR — all-at-once.*

**Files:**
- `docs/provider-formats/*.yaml` (14 new files)
- `docs/provider-formats/*.md` (14 deleted)
- `docs/provider-formats/cline.md.proposed-additions` (deleted)

**Migration source mapping:**
- Skills `canonical_mappings`: source from existing `.develop/seeder-specs/*-skills.yaml` `proposed_mappings` — already has `canonical_key`, `mechanism`, `confidence`
- `sources`: source from `docs/provider-sources/<slug>.yaml` `sources` block
- `content_hash`: initialize to `""` on all source entries (first run will treat all as changed)
- `generation_method`: set to `human-edited`
- `provider_extensions`: start empty — the LLM populates on first pipeline run
- **Two orphaned providers** (`factory-droid`, `pi`): include in migration, they will appear as orphan warnings in `capmon check`

**Migration validation test:**
- `TestValidateAllFormatDocs` in `formatdoc_test.go` (or `capmon_validate_format_doc_cmd_test.go`)
- Loads every `docs/provider-formats/*.yaml` relative to repo root
- Calls `ValidateFormatDoc` on each → asserts no errors
- This test is the automated merge gate for the migration PR

**Done when:** 14 `.yaml` files exist, all pass `validate-format-doc`, `TestValidateAllFormatDocs` passes in CI.

---

## Phase 6: `capmon check`

*Requires all of Phases 1–5.*

**Files:**
- `cli/internal/capmon/check.go` (new)
- `cli/internal/capmon/fetch_validity.go` (new)
- `cli/internal/capmon/check_diff.go` (new)
- `cli/cmd/syllago/capmon_check_cmd.go` (new)
- `cli/internal/capmon/report.go` (extend — issue dedup + create/append)

**Tasks:**

6.1 `fetch_validity.go` — `ValidateContentResponse(body []byte, contentType, originalURL, finalURL string) error`:
  - `len(body) >= 512`
  - `contentType` not matching binary MIME prefixes
  - eTLD+1 domain match between `originalURL` and `finalURL` (use `golang.org/x/net/publicsuffix` if available)
  - Returns typed `ErrContentInvalid` for distinguishable error handling

6.2 `check_diff.go` — `GenerateUnifiedDiff(oldContent, newContent []byte, path string) (string, error)`:
  - Unified diff format between old and new
  - Truncation: `source_code` → 500 lines, all others → 200 lines
  - If truncated: append `[truncated after N lines (~X bytes shown) — full diff at .capmon-cache/<slug>/]`

6.3 Extend `report.go`:
  - `FindOpenCapmonIssue(provider, contentType string) (int, bool, error)` — uses hidden HTML comment anchor
  - `CreateCapmonChangeIssue(...)` — title, labels, body with anchor comment
  - `AppendCapmonChangeEvent(issueNumber int, ...)` — appends to existing issue

6.4 `check.go` — `RunCapmonCheck(ctx context.Context, opts CapmonCheckOptions) error`:
  - Step 0: orphan detection (format docs whose slug not in providers.json → non-blocking warning)
  - Step 1: `ValidateSources` for each provider (blocking)
  - Step 2: `ValidateFormatDoc` for each format doc (blocking)
  - Step 3: for each provider × content_type × source URI:
    - `FetchSource` → `ValidateContentResponse` → SHA-256 hash → compare vs `content_hash` in format doc
    - No format doc = new provider, treat all as changed
    - Fetch/validity error: issue with `capmon-fetch-error` label
    - Changed: `GenerateUnifiedDiff` → `FindOpenCapmonIssue` → create or append issue
  - `--dry-run`: log what would happen, no GitHub writes
  - No hash is written back to format docs — that's the local loop's job

6.5 `capmon_check_cmd.go`:
  - `capmon check --all` or `--provider=<slug>` (mutually exclusive)
  - Flags: `--all`, `--provider`, `--formats-dir`, `--sources-dir`, `--cache-root`, `--providers-json`, `--dry-run`

**Tests:**
- `fetch_validity_test.go`: valid response, body too small, binary content-type, domain mismatch
- `check_diff_test.go`: no truncation, source_code truncation at 500, others at 200, indicator line format
- `check_test.go`: no change (hash match), changed (issue creation), fetch error, content validity failure, orphan detection, dry-run
- Use `SetHTTPClientForTest` + `SetGHCommandForTest` for all network/GH mocking
- `capmon_check_cmd_test.go`: registered, --all and --provider mutual exclusion, dry-run flag

**Done when:** `syllago capmon check --provider=amp --dry-run` runs against a mocked HTTP source without errors.

---

## Phase 7: `capmon onboard`

*Requires Phases 3 and 6. Can run in parallel with Phase 8.*

**Files:**
- `cli/cmd/syllago/capmon_onboard_cmd.go` (new)

**Tasks:**

7.1 `capmon onboard --provider=<slug>`:
  - Step 1: `ValidateSources` — exit 1 if fails
  - Step 2: fetch all sources via `FetchSource`
  - Step 3: `CreateCapmonChangeIssue` for all sources (new provider = all changed)
  - Override var: `capmonOnboardSourcesDirOverride`

**Tests:**
- `capmon_onboard_cmd_test.go`: registered, missing provider, validate-sources called first, issue created

**Done when:** `syllago capmon onboard --provider=newprovider` creates an issue and exits 0 (with `SetGHCommandForTest` mock).

---

## Phase 8: Workflow Docs + CI Workflow

*Requires Phases 5 and 6. Can run in parallel with Phase 7.*

**Files:**
- `docs/workflows/update-format-doc.md` (new — extract verbatim from design doc)
- `docs/workflows/graduation-comparison.md` (new — extract verbatim from design doc)
- `.github/workflows/capmon-check.yml` (new)
- `docs/workflows/inspect-provider-skills.md` (deprecation notice added)

**Tasks:**

8.1 Extract `docs/workflows/update-format-doc.md` verbatim from the design doc section `### docs/workflows/update-format-doc.md (target file)`

8.2 Extract `docs/workflows/graduation-comparison.md` verbatim from the design doc section `### docs/workflows/graduation-comparison.md (target file)`

8.3 Create `.github/workflows/capmon-check.yml`:
  - Schedule: `0 */12 * * 1-5` (Mon–Fri every 12h), `0 6 * * 0,6` (Sat–Sun once)
  - Also `workflow_dispatch` for manual runs
  - Permissions: `issues: write`, `contents: read`
  - Single job: checkout, setup-go, build, `./cli/syllago capmon check --all`
  - SHA-pin all action versions (match pattern in existing workflows)
  - No Chromedp container, no artifact upload — lean HTTP-only check

8.4 Add deprecation notice to `docs/workflows/inspect-provider-skills.md` pointing to `update-format-doc.md`

**Done when:** Both workflow docs exist. `capmon-check.yml` is syntactically valid (passes `actionlint` if available).

---

## Test Coverage Summary

| Phase | Primary test files | Target coverage |
|-------|-------------------|-----------------|
| 1 | `formatdoc_test.go` | ~80% |
| 2 | `formatdoc_validate_test.go`, `capmon_validate_format_doc_cmd_test.go` | ~85% |
| 3 | `sourceman_test.go` additions, `capmon_validate_sources_cmd_test.go` | ~90% |
| 4 | `derive_test.go`, `capmon_derive_cmd_test.go` | ~85% |
| 5 | `TestValidateAllFormatDocs` (integration) | migration gate |
| 6 | `fetch_validity_test.go`, `check_diff_test.go`, `check_test.go`, `capmon_check_cmd_test.go` | ~80% |
| 7 | `capmon_onboard_cmd_test.go` | ~80% |
| 8 | N/A (markdown/YAML docs) | — |

**Regression guards:**
- `seederspec_test.go`: remove `ValidateSeederSpec` tests, keep `LoadSeederSpec`
- Delete `capmon_validate_spec_cmd_test.go` entirely in Phase 4
- `seed_test.go`: verify gate removal doesn't break existing seed tests

---

## Notes

- **Existing infrastructure to reuse:** `FetchSource`, `WriteCacheEntry`, `SHA256Hex`, `GHRunner`, `SetGHCommandForTest`, `SetHTTPClientForTest`, `LoadSourceManifest` — all exist already
- **`.capmon-cache/` is already gitignored** — verify `.gitignore` has this entry before Phase 6
- **`capmon.yml` (old CI)** stays in place — not removed in this plan. The old `capmon run`/`capmon verify`/`capmon seed` pipeline keeps working until explicitly decommissioned
- **`docs/provider-formats/*.md` files exist for 14 providers** — confirm exact count and filenames before Phase 5
- **Provider rename known limitation** — documented in design doc, no special handling needed in this plan
