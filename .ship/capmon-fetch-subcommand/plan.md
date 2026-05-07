# Plan: capmon-fetch-subcommand

## Execution order

1. **Slice 1: Cache-hit counter visible in ProviderStatus** — test bead + impl bead (TDD)
   - Test: `cli/internal/capmon/pipeline_test.go` — asserts `SourcesCacheHit` is incremented when `entry.Meta.Cached == true` and stays 0 on new content
   - Impl: `cli/internal/capmon/types.go` + `cli/internal/capmon/pipeline.go` — add `SourcesCacheHit int` field; increment in Stage 1 loop
   - Checkpoint: `cd cli && go test ./internal/capmon/... -run TestRunStage1Fetch_CacheHitCounter` passes

2. **Slice 2: Dry-run source count report** — test bead + impl bead (TDD)
   - Test: `cli/cmd/syllago/capmon_fetch_cmd_test.go` (new) — asserts dry-run prints "would fetch N sources", no files created, unknown provider returns error with valid slug list
   - Impl: `cli/cmd/syllago/capmon_cmd.go` — add flags (`--cache-root`, `--dry-run`, `--json`, `--verbose`) in `init()`; replace stub `RunE` with dry-run path (load manifests, count sources, print, return)
   - Checkpoint: `cd cli && go test ./cmd/syllago/... -run TestCapmonFetchCmd_DryRun` passes; `syllago capmon fetch --dry-run` exits 0 with source counts instead of "not yet implemented"

3. **Slice 3: Live fetch with per-provider summary output** — test bead + impl bead (TDD)
   - Test: `cli/cmd/syllago/capmon_fetch_cmd_test.go` — asserts per-provider summary, `[changed]`/`[cached]` verbose lines, JSON shape, exit non-zero on fetch failure
   - Impl: `cli/cmd/syllago/capmon_cmd.go` — implement live fetch path: construct `PipelineOptions`, call `runStage1Fetch`, format output from `manifest.Providers`, return non-nil error if any `ProviderStatus.Errors` non-empty
   - Checkpoint: `cd cli && go test ./cmd/syllago/... -run TestCapmonFetchCmd_LiveFetch` passes

4. **Slice 4: Telemetry registration for capmon_fetch** — test bead + impl bead (TDD)
   - Test: `cli/internal/telemetry/catalog_test.go` — asserts `capmon_fetch` in `provider`/`dry_run` Commands arrays; asserts `source_count` and `fetch_errors` PropertyDef entries exist; `TestGentelemetry_CatalogMatchesEnrichCalls` must stay green
   - Impl: `cli/internal/telemetry/catalog.go` + `cli/cmd/syllago/capmon_cmd.go` — add catalog entries; add `telemetry.Enrich` calls in `capmonFetchCmd.RunE`; run `cd cli && make gendocs` to regenerate `telemetry.json`
   - Checkpoint: `cd cli && go test ./internal/telemetry/... -run TestEventCatalog_CapmonFetchProperties` passes; `TestGentelemetry_CatalogMatchesEnrichCalls` passes; pre-push hook does not block on stale `telemetry.json`

## Gate

Before moving from one slice to the next: the checkpoint for the current slice must pass. If it fails, stop and involve the user — never skip ahead.

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

## Non-TDD exemptions

None
