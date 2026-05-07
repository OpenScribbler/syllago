# Design Discussion: capmon-fetch-subcommand

## Summary

**Current state:** `syllago capmon fetch` exists as a stub that validates `--provider` and unconditionally returns a "not yet implemented" error, directing operators to `syllago capmon run --stage fetch-extract` instead (`cli/cmd/syllago/capmon_cmd.go:98–110`).

**Desired state:** `syllago capmon fetch` fully populates `.capmon-cache/` slot directories from live Provider Source Manifest URLs, with per-provider summary output, `--verbose`/`--json`/`--dry-run` flags, exit non-zero on any ultimately failed source, and telemetry.

**End state (narrative):** An operator running `syllago capmon fetch` sees a per-Provider summary (`cursor: 3 fetched, 0 errors`). With `--verbose`, each source line shows `[changed]` or `[cached]`. With `--json`, the same data is emitted as a structured object. `--dry-run` reports what would be fetched without writing to disk. If any source fails after exhausting retries, the command exits non-zero.

## Research questions answered

- Q1: What does the existing `capmonFetchCmd` stub do?
- Q2: How is `runStage1Fetch` invoked, and what fields track per-source success, failure, and cache-hit status?
- Q3: What is the `.capmon-cache/` slot directory layout and what does Stage 1 write?
- Q4: Where is `CacheMeta.Cached` defined and how does it flow through Stage 1?
- Q5: Where does the capmon system maintain the list of known/supported Providers?
- Q6: What output pattern do existing capmon subcommands use?
- Q7: Does any part of the CLI implement retry logic for HTTP requests?
- Q8: What telemetry property keys are registered for capmon-related commands?

## Patterns to Follow

### Pattern: capmon subcommand flag declaration

**Source:** `cli/cmd/syllago/capmon_cmd.go:253–273`

**Snippet:**
```go
capmonSeedCmd.Flags().String("provider", "", "Seed only this provider slug")
capmonSeedCmd.Flags().Bool("force-overwrite-exclusive", false, "...")
capmonSeedCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")
```

**Why it applies here:** All capmon subcommands that need a cache path declare `--cache-root` locally (not on the parent `capmonCmd`), defaulting to `.capmon-cache` in the `RunE` body. `capmon fetch` must follow the same pattern because `capmon run` and the current `capmon fetch` stub lack this flag — adding it on the stub brings parity with `capmon seed`/`capmon verify`/`capmon check`.

---

### Pattern: PipelineOptions + runStage1Fetch invocation

**Source:** `cli/internal/capmon/pipeline.go:145–217`

**Snippet:**
```go
func runStage1Fetch(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error {
    manifests, err := LoadAllSourceManifests(opts.SourceManifestsDir)
    // ...
    for _, m := range manifests {
        if opts.ProviderFilter != "" && m.Slug != opts.ProviderFilter {
            continue
        }
        // ...per-source fetch loop...
        status.SourcesFetched++
        // ...
        manifest.Providers[m.Slug] = status
    }
    return nil
}
```

**Why it applies here:** The `capmon fetch` command must reuse `runStage1Fetch` exactly — it is already the Stage 1 implementation. The command constructs a `PipelineOptions` with `CacheRoot`, `SourceManifestsDir`, `ProviderFilter`, and `DryRun`, builds a `RunManifest`, calls `runStage1Fetch`, then formats the populated `manifest.Providers` map for output. This avoids duplicating the fetch loop and keeps the healing/meta-patching logic in one place.

---

### Pattern: SanitizeSlug validation on --provider

**Source:** `cli/cmd/syllago/capmon_cmd.go:103–106`

**Snippet:**
```go
if provider != "" {
    if _, err := capmon.SanitizeSlug(provider); err != nil {
        return fmt.Errorf("invalid --provider: %w", err)
    }
}
```

**Why it applies here:** The existing stub already validates `--provider` with `capmon.SanitizeSlug`. The full implementation must retain this check and extend it: after sanitization passes, the validated slug must be compared against the set of slugs discoverable from `LoadAllSourceManifests` — if no Provider Source Manifest exists for the slug, the command returns a hard error with a recovery hint listing known slugs. This two-step check (format → existence) is the correct pattern because `SanitizeSlug` only validates form, not existence.

---

### Pattern: CacheMeta.Cached for [changed]/[cached] per-source verbose output

**Source:** `cli/internal/capmon/cache.go:20` and `cli/internal/capmon/fetch.go:87–91`

**Snippet:**
```go
// cache.go:20
Cached bool `json:"cached,omitempty"`

// fetch.go:87–91
if IsCached(cacheRoot, provider, sourceID) {
    existing, readErr := ReadCacheEntry(cacheRoot, provider, sourceID)
    if readErr == nil && existing.Meta.ContentHash == newHash {
        existing.Meta.Cached = true
        return existing, nil
    }
}
```

**Why it applies here:** `ProviderStatus.SourcesFetched` counts both fresh and cached hits identically (`pipeline.go:211`). To distinguish `[changed]` from `[cached]` in `--verbose` output, the command must collect `entry.Meta.Cached` values directly from the `*CacheEntry` returns. Because `runStage1Fetch` is unexported and does not surface individual entries to callers, the implementation must either (a) make `runStage1Fetch` exported or (b) call `FetchSource`/`FetchChromedp` directly in the command, or (c) add a `SourcesCacheHit int` counter to `ProviderStatus` and increment it in the Stage 1 loop.

---

### Pattern: capmon run telemetry enrichment

**Source:** `cli/cmd/syllago/capmon_cmd.go:135–143`

**Snippet:**
```go
telemetry.Enrich("dry_run", dryRun)
if provider != "" {
    telemetry.Enrich("provider", provider)
}
mode := "full"
if stage != "" {
    mode = stage
}
telemetry.Enrich("mode", mode)
```

**Why it applies here:** `capmon fetch` must enrich telemetry before returning (not via `os.Exit` bypass — unlike `capmon run`). The design concept requires provider slug, source count, and success/failure counts. These require new property keys in `EventCatalog` because `capmon_fetch` is not yet listed in any property's `Commands` array (`catalog.go:47`, `catalog.go:68`).

---

### Pattern: human-readable output via fmt.Printf, not output.JSON global

**Source:** `cli/cmd/syllago/capmon_cmd.go:127–157` (capmon run), `cli/internal/capmon/validate_sources.go` (capmon validate-sources)

**Why it applies here:** No existing capmon subcommand uses the `output.JSON` global or `--json` flag. The `--json` flag requested by the design concept is new to the capmon family. The implementation should declare `--json` locally on `capmonFetchCmd` and check it in `RunE`, consistent with how non-capmon commands like `list` use a local flag rather than a global package-level variable (see `cli/cmd/syllago/list_cmd.go`). It must NOT set `output.JSON = true`; the JSON path is specific to this subcommand.

### Disambiguation: collect cache-hit counts via ProviderStatus counter vs. per-entry collection outside runStage1Fetch

**Chosen:** Add `SourcesCacheHit int` to `ProviderStatus` and increment in the Stage 1 loop (`cli/internal/capmon/types.go:62`)
**Considered:** Call `FetchSource`/`FetchChromedp` directly in the command's `RunE`, duplicating the source-iteration loop (`cli/internal/capmon/pipeline.go:162–213`)

**Why:** `runStage1Fetch` already owns the complete fetch loop including SSRF validation, jitter, chromedp branching, healing, and meta-patching. Duplicating that loop in `capmon_cmd.go` would create two sources of truth for Stage 1 behavior — any future change to fetch mechanics would need to be applied in both places. The right answer is to surface `SourcesCacheHit` as a first-class field on `ProviderStatus`, which is the struct already designed to carry all per-provider Stage 1 outcomes (`types.go:62–78`). The Stage 1 loop increments it at the same site as `SourcesFetched` (`pipeline.go:211`), making both counters consistent.

**Consequences:** `ProviderStatus` gains one field. The JSON serialization of `last-run.json` gains `"sources_cache_hit"` in provider entries (omitempty, so existing logs that predate this change are unaffected). The `capmon fetch` command reads this counter for `--verbose` output; `runStage1Fetch` becomes the single authoritative Stage 1 implementation as intended.

---

### Disambiguation: --provider validation against LoadAllSourceManifests vs. provider.AllProviders

**Chosen:** Validate against slugs derived from `LoadAllSourceManifests` (`cli/internal/capmon/sourceman.go:127`)
**Considered:** Validate against `provider.AllProviders` (`cli/internal/provider/provider.go:86`)

**Why:** The capmon pipeline (Stage 1) discovers providers solely from `docs/provider-sources/*.yaml` (`pipeline.go:150`). Only 9 of the 15 entries in `provider.AllProviders` have Provider Source Manifests. If the command validated against `AllProviders` and the user requested a slug with no manifest (e.g., `amp`, `codex`), the validation would pass but the fetch loop would produce zero output — a silent no-op that is harder to diagnose than a hard error. Validating against the set of manifest slugs produces an informative error message that lists fetchable providers.

**Consequences:** The error message for an unknown slug must be constructed at runtime from `LoadAllSourceManifests`, not from a static string. This means `LoadAllSourceManifests` is called once for validation and once in the fetch loop; a small redundancy but acceptable because manifest loading is a cheap filesystem scan of 9 YAML files.

## Design Questions

1. **Should `capmon fetch` return normally (allowing telemetry to fire via `PersistentPostRun`) or call `os.Exit(exitClass)` as `capmon run` does?**
   - A) Return error normally from `RunE` — telemetry fires, exit code is 1 on any failure (cobra wraps RunE errors as exit 1)
   - B) Call `os.Exit(exitClass)` with the same exit classes as the full pipeline (0=clean, 2=partial failure, 3=infra failure, 4=fatal)
   - C) Free-form slot — user fills in
   - **Recommended:** A — Returning normally allows telemetry to fire and avoids the `os.Exit` pattern that the existing `capmon run` uses (which the research notes as a telemetry bypass). `capmon fetch` is a targeted interactive command, not a CI pipeline runner; binary exit codes (0 = all succeeded, 1 = any failure) are sufficient for scripting use.

2. **For `--verbose` output, should `[changed]`/`[cached]` labels appear per-source or per-provider?**
   - A) Per-source line, e.g. `  hooks.0 [changed] https://example.com/...`
   - B) Per-provider summary augmented with changed/cached counts, e.g. `cursor: 3 fetched (2 changed, 1 cached), 0 errors`
   - **Recommended:** A — Per-source lines surface the specific URL that changed, which is the information an operator needs when diagnosing a fetch run. Per-provider summaries (option B) are already covered by the default (non-verbose) output line.

3. **Should the `--json` output include heal event details per provider, or only fetch/error counts?**
   - A) Fetch counts only: `{ "providers": { "<slug>": { "fetched": N, "cached": N, "errors": N } } }`
   - B) Full `ProviderStatus` fields including `heal_events`, `errors` strings, and `fetch_status`
   - C) Free-form slot — user fills in
   - **Recommended:** A — The design concept specifies "structured object" for `--json` output with fetch/error counts. Including full `ProviderStatus` (option B) exposes internal heal event details that are not part of the defined output contract and would complicate any consumer scripting against the output. If heal event details are needed, they are already written to `.capmon-cache/last-run.json`.

## Decisions made (not questions)

- `runStage1Fetch` is reused directly, not duplicated — the command wraps it with a new output layer, not a new fetch loop. Research confirmed this is the only implementation of Stage 1 behavior.
- Provider discovery for validation uses `LoadAllSourceManifests`, not `provider.AllProviders` — see Disambiguation above.
- `SourcesCacheHit int` is added to `ProviderStatus` in `types.go` — see Disambiguation above. This is the minimal struct change needed; it does not alter `RunManifest` shape.
- Retry behavior is inherited from `FetchSource`'s existing 1+3 exponential backoff (`fetch.go:65–79`). No additional retry layer is needed in the command. The design concept's "up to 3 retries" maps directly to the existing 4-attempt implementation.
- The `--cache-root` flag is added to `capmonFetchCmd.init()`, defaulting to `.capmon-cache`, consistent with `capmonSeedCmd` and `capmonVerifyCmd`.
- Exit behavior: return error normally from `RunE` (see Design Question 1 recommendation). The command returns a non-nil error wrapping a summary when any source ultimately failed.
- Telemetry: three new `Enrich` calls — `provider` (if flag set), `source_count` (total sources attempted), `fetch_errors` (count of ultimately-failed sources). The `capmon_fetch` command name must be added to the `provider` and `dry_run` property `Commands` arrays in `EventCatalog`. Two new property keys (`source_count`, `fetch_errors`) must be registered as `PropertyDef` entries.
- `--dry-run` passes through to `PipelineOptions.DryRun = true`. In dry-run mode `runStage1Fetch` is called normally but heal side-effects are suppressed (this is existing `tryHealSource` behavior at `pipeline.go:244–248`); the implementation must additionally skip the `WriteCacheEntry`/`WriteCacheMeta` calls. This requires either a `DryRun` check inside `FetchSource` (not desirable — it knows nothing about the command) or a post-call skip: call `runStage1Fetch` but pass `DryRun: true` so healing is suppressed, then note that cache writes still occur. **Resolution:** For dry-run the command should NOT call `runStage1Fetch`; it should call `LoadAllSourceManifests`, iterate source manifests to count sources, and emit the "would fetch N sources" report without invoking any fetch or write operation. This is the correct dry-run semantics.
- The `capmon fetch` command does not write `last-run.json` — that is the `capmon run` pipeline output. The command only writes slot directories under `.capmon-cache/<slug>/`.

## Out of Scope

- Scheduling or cron integration — `capmon fetch` is a one-shot interactive command.
- Diff/changelog output comparing before/after slot content — Stage 3 (`runStage3Diff`) handles this; it is not triggered by `capmon fetch`.
- Authenticated sources — all sources must be accessible via unauthenticated HTTPS; SSRF validation at `ValidateSourceURL` (`fetch.go:16`) enforces this.
- Any capmon stage beyond Stage 1 fetch — `capmon extract`, `capmon diff`, `capmon seed`, and `capmon run` are separate subcommands.
- Making retry count or backoff configurable — `FetchSource` hardcodes delays `[1s, 2s, 4s]` (`fetch.go:65`); this is intentional and not exposed as a flag.
- Fixing the `capmon run` telemetry bypass (`os.Exit` pattern at `capmon_cmd.go:154`) — that is a separate concern outside this feature's scope, confirmed by the design concept.
- Backfilling telemetry catalog entries for other capmon commands (`capmon_run`, `capmon_seed`, `capmon_extract`) that currently call `telemetry.Enrich` for unregistered keys — only `capmon_fetch` is in scope.
- Removing stale hyphen-style slot directories (e.g., `hooks-0`) that coexist with current dot-style directories in `.capmon-cache/` — this is a separate cleanup concern.

## Interfaces affected (preview)

- `cli/internal/capmon/types.go` — add `SourcesCacheHit int` field to `ProviderStatus`
- `cli/internal/capmon/pipeline.go` — increment `status.SourcesCacheHit` in `runStage1Fetch` loop when `entry.Meta.Cached == true`
- `cli/cmd/syllago/capmon_cmd.go` — implement `capmonFetchCmd.RunE`; add `--cache-root`, `--dry-run`, `--json`, `--verbose` flags in `init()`
- `cli/internal/telemetry/catalog.go` — add `capmon_fetch` to `provider` and `dry_run` property `Commands` arrays; add `source_count` and `fetch_errors` `PropertyDef` entries
- `cli/cmd/syllago/capmon_fetch_cmd_test.go` (new) — unit and integration tests for the command
