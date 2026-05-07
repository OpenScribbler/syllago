# Structure Outline: capmon-fetch-subcommand

## Current / Desired / End State

**Current:** `syllago capmon fetch` is a stub that validates `--provider` format with `SanitizeSlug` and unconditionally returns "not yet implemented", directing operators to `capmon run --stage fetch-extract` instead.

**Desired:** `syllago capmon fetch` fully populates `.capmon-cache/` slot directories from live Provider Source Manifest URLs, with per-provider summary output, `--verbose`/`--json`/`--dry-run` flags, exit non-zero on any ultimately failed source, and telemetry enrichment.

**End state:** An operator running `syllago capmon fetch` sees a per-provider summary line (`cursor: 3 fetched, 0 errors`). With `--verbose`, each source line shows `hooks.0 [changed] https://...` or `hooks.0 [cached] https://...`. With `--json`, the same data is emitted as `{ "providers": { "<slug>": { "fetched": N, "cached": N, "errors": N } } }`. `--dry-run` reports "would fetch N sources for <slug>" without writing to disk. If any source fails after exhausting retries, the command returns an error and exits non-zero.

## Patterns to Follow

### Pattern: capmon subcommand flag declaration
**Source:** `cli/cmd/syllago/capmon_cmd.go:253–273`
```go
capmonSeedCmd.Flags().String("provider", "", "Seed only this provider slug")
capmonSeedCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")
```
All capmon subcommands that need a cache path declare `--cache-root` locally, defaulting to `.capmon-cache` in the `RunE` body.

### Pattern: PipelineOptions + runStage1Fetch invocation
**Source:** `cli/internal/capmon/pipeline.go:149`
```go
func runStage1Fetch(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error
```
The command constructs `PipelineOptions`, builds a `RunManifest`, calls `runStage1Fetch`, then formats `manifest.Providers` for output.

### Pattern: SanitizeSlug validation on --provider
**Source:** `cli/cmd/syllago/capmon_cmd.go:103–106`
```go
if provider != "" {
    if _, err := capmon.SanitizeSlug(provider); err != nil {
        return fmt.Errorf("invalid --provider: %w", err)
    }
}
```
Two-step check: format validation (`SanitizeSlug`) then existence check against `LoadAllSourceManifests` slugs.

### Pattern: CacheMeta.Cached for verbose output
**Source:** `cli/internal/capmon/fetch.go:87–91`
`ProviderStatus.SourcesCacheHit int` is incremented in the Stage 1 loop when `entry.Meta.Cached == true`, enabling the command to output `[cached]` vs `[changed]` per source in `--verbose` mode.

### Pattern: local --json flag, not output.JSON global
**Source:** `cli/cmd/syllago/capmon_cmd.go:127–157`
Declare `--json` locally on `capmonFetchCmd`; check it in `RunE`. Do NOT set `output.JSON = true`.

## Design Summary

`capmon fetch` wraps `runStage1Fetch` with a new output layer — it does not duplicate the fetch loop. Provider validation is two-step: `SanitizeSlug` for format, then `LoadAllSourceManifests` for existence (which rejects slugs with no Provider Source Manifest). Cache-hit counting requires `SourcesCacheHit int` on `ProviderStatus` (incremented in the Stage 1 loop alongside `SourcesFetched`). In dry-run mode the command skips `runStage1Fetch` entirely and reports source counts from manifest scanning only. Telemetry enriches `provider`, `source_count`, and `fetch_errors` via two new `PropertyDef` entries in `EventCatalog`. The command returns errors normally from `RunE` so telemetry fires via `PersistentPostRun`.

---

## Slices

### Slice 1: Cache-hit counter visible in ProviderStatus

**Observable outcome:** `runStage1Fetch` populates `ProviderStatus.SourcesCacheHit` for each provider. A test can call `runStage1Fetch` against a pre-seeded cache directory, trigger a re-fetch of the same content (same hash), and assert `ProviderStatus.SourcesCacheHit == 1` while `ProviderStatus.SourcesFetched == 1`.

**Interfaces introduced or modified:**

- `cli/internal/capmon.ProviderStatus` — add `SourcesCacheHit int json:"sources_cache_hit,omitempty"` — **Deps:** `in-process`
  - **Hides:** The `CacheMeta.Cached` sentinel from `FetchSource` that signals a hash match
  - **Exposes:** An integer count of cache-hit sources per provider; callers see a consistent `ProviderStatus` shape alongside `SourcesFetched` and `Errors`

- `cli/internal/capmon.runStage1Fetch` — increment `status.SourcesCacheHit` when `entry.Meta.Cached == true` at `pipeline.go:211` site — **Deps:** `in-process`
  - **Hides:** The `entry.Meta.Cached` check; callers never inspect individual `CacheEntry` values
  - **Exposes:** Updated `RunManifest.Providers` with `SourcesCacheHit` populated alongside existing `SourcesFetched`

**Files:**

- `cli/internal/capmon/types.go` — add `SourcesCacheHit int` field to `ProviderStatus`
- `cli/internal/capmon/pipeline.go` — add `if entry.Meta.Cached { status.SourcesCacheHit++ }` at the Stage 1 fetch-success site (immediately after `status.SourcesFetched++`)
- `cli/internal/capmon/pipeline_test.go` (existing or new sibling) — unit tests

**Test cases:**

- Unit: `TestRunStage1Fetch_CacheHitCounter` — seed a temp cache with a pre-written `raw.bin`+`meta.json` whose `ContentHash` matches what `FetchSource` would return (use `httptest.NewServer` serving the same content); call `runStage1Fetch`; assert `SourcesCacheHit == 1`, `SourcesFetched == 1`, `len(Errors) == 0`
- Unit: `TestRunStage1Fetch_CacheHitZeroOnChanged` — serve new content with a different hash; assert `SourcesCacheHit == 0`, `SourcesFetched == 1`
- Unit: `TestProviderStatus_JSONRoundtrip_SourcesCacheHit` — marshal a `ProviderStatus` with `SourcesCacheHit: 3` to JSON, unmarshal back, assert field is preserved; also assert `SourcesCacheHit: 0` omits the key (omitempty)

**Checkpoint:** `cd cli && go test ./internal/capmon/... -run TestRunStage1Fetch_CacheHitCounter` passes.

---

### Slice 2: Dry-run source count report

**Observable outcome:** `syllago capmon fetch --dry-run` prints "would fetch N sources for <slug>" for each matched provider without writing any files to `.capmon-cache/`. A test can assert the output string and verify no slot directories were created.

**Interfaces introduced or modified:**

- `cli/cmd/syllago.capmonFetchCmd` — replace stub `RunE` with dry-run path: read flags (`--provider`, `--cache-root`, `--dry-run`, `--json`, `--verbose`), load manifests, count sources, print report, return nil — **Deps:** `local-substitutable`
  - **Hides:** Flag parsing, manifest loading loop, output formatting
  - **Exposes:** `capmonFetchCmd.RunE` cobra contract; callers are cobra's dispatch machinery

- `cli/internal/capmon.LoadAllSourceManifests` — called to count sources per provider in dry-run path — **Deps:** `local-substitutable`
  - **Hides:** YAML parsing, directory scan
  - **Exposes:** `[]*SourceManifest` slice ordered by filesystem

**Files:**

- `cli/cmd/syllago/capmon_cmd.go` — add `--cache-root`, `--dry-run`, `--json`, `--verbose` flags in `init()`; replace stub `RunE` body with dry-run branch (load manifests, count, print, return) and stub live-fetch path (returns early pending Slice 3)
- `cli/cmd/syllago/capmon_fetch_cmd_test.go` (new) — test file

**Test cases:**

- Unit: `TestCapmonFetchCmd_DryRun_PrintsSourceCounts` — create temp `docs/provider-sources/` with one minimal YAML (2 sources), call `RunE` with `--dry-run`, assert stdout contains "would fetch 2 sources for <slug>", assert no files created under a temp cache root
- Unit: `TestCapmonFetchCmd_DryRun_ProviderFilter` — two source manifests in temp dir; pass `--provider <slug1> --dry-run`; assert only slug1 appears in output, not slug2
- Unit: `TestCapmonFetchCmd_DryRun_UnknownProvider` — pass `--provider nosuchslug --dry-run`; assert error returned containing the slug name and a recovery hint with valid slugs
- Unit: `TestCapmonFetchCmd_DryRun_InvalidSlugFormat` — pass `--provider "bad slug!" --dry-run`; assert `SanitizeSlug` error is surfaced
- Unit: `TestCapmonFetchCmd_DryRun_JSON` — pass `--dry-run --json`; parse stdout as JSON; assert `providers.<slug>.fetched` key is present with correct source count

**Checkpoint:** `cd cli && go test ./cmd/syllago/... -run TestCapmonFetchCmd_DryRun` passes; `syllago capmon fetch --dry-run` exits 0 and prints source counts instead of "not yet implemented".

---

### Slice 3: Live fetch with per-provider summary output

**Observable outcome:** `syllago capmon fetch` with no `--dry-run` runs `runStage1Fetch` and prints a per-provider summary line (`cursor: 3 fetched, 0 errors`). With `--verbose`, each source line shows `hooks.0 [changed] https://...` or `hooks.0 [cached] https://...`. The command exits non-zero when any provider has errors.

**Interfaces introduced or modified:**

- `cli/cmd/syllago.capmonFetchCmd.RunE` (live path) — build `PipelineOptions{CacheRoot, SourceManifestsDir, ProviderFilter, DryRun: false}`, allocate `RunManifest`, call `runStage1Fetch`, format output from `manifest.Providers` — **Deps:** `local-substitutable`
  - **Hides:** `PipelineOptions` construction, `RunManifest` allocation, source manifest directory defaulting
  - **Exposes:** Human-readable summary per provider on stdout; error return when any `ProviderStatus.Errors` is non-empty

- `cli/internal/capmon.runStage1Fetch` — unchanged signature; now used by both `capmon run` and `capmon fetch` — **Deps:** `in-process`
  - **Hides:** Fetch loop, healing, SSRF validation, jitter
  - **Exposes:** Populated `RunManifest.Providers` map

**Files:**

- `cli/cmd/syllago/capmon_cmd.go` — implement live fetch path in `capmonFetchCmd.RunE`: after dry-run guard, construct opts, allocate manifest, call `runStage1Fetch`, iterate `manifest.Providers`, print summary or verbose lines, collect total errors, return non-nil error if `totalErrors > 0`
- `cli/cmd/syllago/capmon_fetch_cmd_test.go` — additional test cases

**Test cases:**

- Unit: `TestCapmonFetchCmd_LiveFetch_SummaryOutput` — serve two source URLs via `httptest.NewServer` (one unchanged-hash, one new); write a minimal source manifest pointing at these URLs; call `RunE` without `--dry-run`; assert stdout contains "2 fetched" for the provider slug
- Unit: `TestCapmonFetchCmd_LiveFetch_VerboseOutput` — same setup; add `--verbose`; assert stdout contains one `[cached]` line and one `[changed]` line with the source ID prefix (e.g., `skills.0`)
- Unit: `TestCapmonFetchCmd_LiveFetch_ExitNonZeroOnError` — serve a URL that returns 500 after retries; assert `RunE` returns a non-nil error
- Unit: `TestCapmonFetchCmd_LiveFetch_JSONOutput` — pass `--json`; parse stdout; assert `providers.<slug>.fetched == 2`, `providers.<slug>.cached == 1`, `providers.<slug>.errors == 0`
- Unit: `TestCapmonFetchCmd_LiveFetch_AllCached_ExitZero` — all sources unchanged; assert return is nil (exit 0)

**Checkpoint:** `cd cli && go test ./cmd/syllago/... -run TestCapmonFetchCmd_LiveFetch` passes; `syllago capmon fetch --provider cursor` (against a real or httptest source manifest) prints a summary line and exits 0 on success.

---

### Slice 4: Telemetry registration for capmon_fetch

**Observable outcome:** `capmon_fetch` appears in the `provider` and `dry_run` property `Commands` arrays in `EventCatalog`. Two new properties (`source_count`, `fetch_errors`) are registered. The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` passes. The telemetry doc `telemetry.json` is regenerated without stale entries.

**Interfaces introduced or modified:**

- `cli/internal/telemetry.EventCatalog` — add `"capmon_fetch"` to `provider.Commands` and `dry_run.Commands`; add two new `PropertyDef` entries `source_count` (int) and `fetch_errors` (int) each with `Commands: []string{"capmon_fetch"}` — **Deps:** `in-process`
  - **Hides:** Full catalog iteration; callers call `EventCatalog()` and iterate
  - **Exposes:** Updated `[]EventDef` slice; drift-detection test validates it against live `Enrich` calls in `capmon_cmd.go`

- `cli/cmd/syllago.capmonFetchCmd.RunE` — add three `telemetry.Enrich` calls before return: `telemetry.Enrich("provider", provider)` (when set), `telemetry.Enrich("source_count", totalSources)`, `telemetry.Enrich("fetch_errors", totalErrors)` — **Deps:** `in-process`
  - **Hides:** Telemetry client plumbing
  - **Exposes:** Three enriched properties visible in `command_executed` events for `capmon_fetch`

**Files:**

- `cli/internal/telemetry/catalog.go` — add `"capmon_fetch"` to `provider` and `dry_run` `Commands` slices; add `source_count` and `fetch_errors` `PropertyDef` entries to `command_executed`
- `cli/cmd/syllago/capmon_cmd.go` — add `telemetry.Enrich` calls at end of `capmonFetchCmd.RunE`
- `cli/internal/telemetry/gentelemetry_test.go` (existing) — drift-detection test must pass without modification (it auto-discovers `Enrich` calls)
- Run `cd cli && make gendocs` to regenerate `telemetry.json`

**Test cases:**

- Unit: `TestEventCatalog_CapmonFetchProperties` — call `EventCatalog()`, find `command_executed`, assert `source_count` and `fetch_errors` properties exist; assert `provider.Commands` contains `"capmon_fetch"`; assert `dry_run.Commands` contains `"capmon_fetch"`
- Integration: `TestGentelemetry_CatalogMatchesEnrichCalls` (existing test, must stay green) — validates that every `telemetry.Enrich("key", ...)` call in `capmon_cmd.go` has a matching `PropertyDef` in the catalog

**Checkpoint:** `cd cli && go test ./internal/telemetry/... -run TestEventCatalog_CapmonFetchProperties` passes; `cd cli && go test ./internal/telemetry/... -run TestGentelemetry_CatalogMatchesEnrichCalls` passes; `telemetry.json` is up to date (pre-push hook does not block).

---

## Acceptance

- `syllago capmon fetch` exits 0 and prints per-provider summary when all sources succeed
- `syllago capmon fetch --verbose` prints one `[changed]` or `[cached]` line per source, prefixed with the source ID (e.g., `hooks.0`)
- `syllago capmon fetch --json` emits `{ "providers": { "<slug>": { "fetched": N, "cached": N, "errors": N } } }` with no human-readable lines
- `syllago capmon fetch --dry-run` prints "would fetch N sources for <slug>" without creating or modifying any files under `.capmon-cache/`
- `syllago capmon fetch --provider nosuchslug` returns an error listing valid slugs (derived from `LoadAllSourceManifests`)
- `syllago capmon fetch` exits non-zero and returns an error when any source ultimately fails after retries
- `syllago capmon fetch` does NOT write `last-run.json`
- `TestGentelemetry_CatalogMatchesEnrichCalls` passes after adding `source_count` and `fetch_errors` `Enrich` calls
- `ProviderStatus.SourcesCacheHit` is `omitempty` — existing `last-run.json` files without the field deserialize without error
- All tests pass: `cd cli && go test ./...`
- Binary builds cleanly: `make build`

## Out of Scope

- Scheduling or cron integration
- Diff/changelog output comparing before/after slot content
- Authenticated sources
- Any capmon stage beyond Stage 1 fetch
- Making retry count or backoff configurable
- Fixing the `capmon run` telemetry bypass (`os.Exit` pattern)
- Backfilling telemetry catalog entries for other capmon commands
- Removing stale hyphen-style slot directories (e.g., `hooks-0`)
