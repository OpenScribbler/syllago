# Research: capmon-fetch-subcommand

## Q1: What does the existing capmonFetchCmd stub do — what file and line is it defined, what flags/parameters does it declare, and what error does it return?

`capmonFetchCmd` is defined at `cli/cmd/syllago/capmon_cmd.go:98`.

- **Use**: `"fetch"`
- **Short**: `"Fetch source URLs and update hash cache"`
- **RunE body** (lines 101–110): reads one flag `--provider` (string), validates it with `capmon.SanitizeSlug` if non-empty, then unconditionally returns:
  ```
  fmt.Errorf("not yet implemented — use 'syllago capmon run --stage fetch-extract'")
  ```
- **Flag declared** in `init()` at `cli/cmd/syllago/capmon_cmd.go:259`:
  - `--provider string` — "Fetch only this provider slug"
- No `--cache-root`, `--dry-run`, `--json`, or `--all` flags are declared on `capmonFetchCmd`.
- The comment inside `RunE` reads: `// Full implementation in pipeline.go (Phase 9)` (`capmon_cmd.go:108`).

`capmonExtractCmd` (same file, line 113) follows the same pattern: one `--provider` flag, same not-yet-implemented error message referencing `fetch-extract`.

## Q2: How is runStage1Fetch invoked — what parameters does it accept, what struct(s) does it return, and what fields on those structs track per-source success, failure, and cache-hit status?

`runStage1Fetch` is defined at `cli/internal/capmon/pipeline.go:149`. Its signature:

```go
func runStage1Fetch(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error
```

It is called at `pipeline.go:75`:

```go
if err := runStage1Fetch(ctx, opts, &manifest); err != nil {
```

**Parameters:**
- `ctx context.Context` — for cancellation and deadline propagation
- `opts PipelineOptions` — holds `CacheRoot`, `SourceManifestsDir`, `ProviderFilter`, `DryRun`, `RepoRoot` (`pipeline.go:18–38`)
- `manifest *RunManifest` — written in-place; Stage 1 populates `manifest.Providers[slug]` entries

**Return:** `error` only; no `CacheEntry` or status struct is returned to the caller. Per-source results accumulate into `manifest.Providers[slug]` (type `ProviderStatus`).

**Fields on `ProviderStatus` tracking per-source outcomes** (`cli/internal/capmon/types.go:62–78`):

| Field | Type | What it tracks |
|---|---|---|
| `SourcesFetched` | `int` | Incremented at `pipeline.go:211` for each successful fetch |
| `Errors` | `[]string` | Append-only; records SSRF-rejected URLs and fetch failures (`pipeline.go:165`, `pipeline.go:189`) |
| `HealEvents` | `[]HealEvent` | One entry per healing attempt triggered by a fetch failure (`pipeline.go:197`) |
| `FetchStatus` | `string` | String field on `ProviderStatus`; not set by Stage 1 loop directly (set elsewhere) |

Cache-hit status is tracked on **`CacheMeta.Cached bool`** (`cli/internal/capmon/cache.go:20`), a field on the `CacheEntry` returned by `FetchSource`/`FetchChromedp`. `ProviderStatus` has no `SourcesCacheHit` field; cache-hit entries are not separately counted in the Stage 1 loop — `status.SourcesFetched` is only incremented when `fetchErr == nil`, covering both fresh fetches and cache hits equally (`pipeline.go:211`).

## Q3: What is the .capmon-cache/ slot directory layout — what paths does Stage 1 fetch write to, and what files appear in a populated slot directory?

**Directory layout:**

```
.capmon-cache/
  <provider-slug>/
    <sourceID>/
      raw.bin       ← fetched content (HTML, JSON, etc.)
      meta.json     ← CacheMeta struct (fetched_at, content_hash, fetch_status, fetch_method, cached, format, source_url)
    <sourceID>/
      raw.bin
      meta.json
      extracted.json  ← added by Stage 2 (ExtractedSource struct)
  last-run.json     ← RunManifest written at pipeline end
```

**sourceID format:** `"<contentType>.<index>"` e.g. `"hooks.0"`, `"skills.2"` — constructed at `pipeline.go:163` as `fmt.Sprintf("%s.%d", ctName, i)`.

**Path construction:** `cacheEntryDir(cacheRoot, provider, sourceID)` returns `filepath.Join(cacheRoot, provider, sourceID)` (`cache.go:52–54`). `WriteCacheEntry` creates the directory with `os.MkdirAll` at mode `0755` and writes both files at mode `0644` (`cache.go:57–73`).

**Observed cache on disk:** Live inspection of `.capmon-cache/claude-code/` shows both old hyphen-style directories (`hooks-0`, `agents-0`) and current dot-style directories (`hooks.0`, `agents.0`). Only the dot-style directories contain `extracted.json`; hyphen-style contain only `raw.bin` and `meta.json`.

**Stage 1 writes:** `raw.bin` and `meta.json` via `WriteCacheEntry` (`fetch.go:107`). After fetch success, `WriteCacheMeta` is called at `pipeline.go:208` to patch `meta.json` with `Format` and `SourceURL` fields.

## Q4: Where is the cache-hit / Meta.Cached field defined and what type carries it through the Stage 1 pipeline?

**Definition:** `CacheMeta.Cached bool` is defined at `cli/internal/capmon/cache.go:20` with JSON tag `json:"cached,omitempty"`.

**Type chain:**

- `CacheMeta` struct — `cache.go:15–25` — holds `Cached bool` alongside `FetchedAt`, `ContentHash`, `FetchStatus`, `FetchMethod`, `Format`, `SourceURL`
- `CacheEntry` struct — `cache.go:38–44` — embeds `Meta CacheMeta` as a field; also carries `Provider string`, `SourceID string`, `Raw []byte`

**Where `Cached` is set to `true`:**

1. `FetchSource` at `fetch.go:89–91`: when `IsCached(...)` returns true and the stored `ContentHash` equals the newly fetched hash, `existing.Meta.Cached = true` is set on the returned entry.
2. `FetchChromedp` at `fetch_chromedp.go:77–79`: identical pattern.

**How it flows through Stage 1:** `FetchSource` / `FetchChromedp` return `*CacheEntry`. The Stage 1 loop at `pipeline.go:183–200` receives this entry but does not inspect `entry.Meta.Cached` — the field is carried in the `CacheEntry` but not counted separately in `ProviderStatus`. `WriteCacheMeta` is called on the returned entry (`pipeline.go:208`) which re-serializes the whole `CacheMeta` including the `Cached` field.

## Q5: Where does the capmon system maintain the list of known/supported providers — static slice, config file, or derived from discovered manifests?

Two distinct registries exist for different purposes:

**1. Go static slice — `provider.AllProviders`**
Defined at `cli/internal/provider/provider.go:86–102`. A hardcoded `[]Provider` containing 15 entries: `ClaudeCode, GeminiCLI, Cursor, Windsurf, Codex, CopilotCLI, Zed, Cline, RooCode, OpenCode, Kiro, Amp, FactoryDroid, Pi, Crush`. This is the authoritative list for install/uninstall/detect operations.

**2. Discovered from YAML manifests — `LoadAllSourceManifests`**
`cli/internal/capmon/sourceman.go:127`. Reads all `*.yaml` files from `docs/provider-sources/` (skipping `_template.yaml`). The pipeline uses this for Stage 1 fetch. Files present: `claude-code.yaml`, `cline.yaml`, `copilot-cli.yaml`, `cursor.yaml`, `gemini-cli.yaml`, `kiro.yaml`, `opencode.yaml`, `windsurf.yaml`, `zed.yaml` — 9 files total (no file for amp, codex, crush, factory-droid, pi, roo-code).

**3. Coverage check cross-reference**
`cli/internal/provider/coverage.go:108` — `CheckCoverage` iterates `AllProviders` and cross-checks against `docs/provider-sources/*.yaml` and `docs/provider-formats/*.yaml` to detect drift between the three registries.

**4. `providers.json` (repo root)**
Referenced by `capmon check` (`capmon_check_cmd.go:23`) and `RunCapmonCheck` (`check.go:134`) for orphan detection — format docs not listed in `providers.json` get a stderr warning.

The capmon pipeline (Stage 1–4) does NOT consult `provider.AllProviders`; it discovers providers solely from `docs/provider-sources/*.yaml`.

## Q6: What pattern do existing capmon subcommands (capmon run, capmon extract, etc.) use for outputting results — how are human-readable vs. --json outputs structured?

No existing capmon subcommand declares a `--json` flag or checks `output.JSON`. All capmon commands use direct `fmt.Printf` / `fmt.Fprintf` to `os.Stdout` / `os.Stderr` or `output.Writer` / `output.ErrWriter` for human-readable text only.

Specific patterns observed:

- **`capmon run`** (`capmon_cmd.go:127`): no output to stdout on success; the pipeline writes `last-run.json` to `.capmon-cache/` and calls `os.Exit(exitClass)` (`capmon_cmd.go:154`). No `--json` flag.
- **`capmon validate-sources`** (`capmon_validate_sources_cmd.go:35`): `fmt.Printf("✓ Source manifest valid for provider %q\n", provider)` — plain text, no JSON mode.
- **`capmon validate-format-doc`** (`capmon_validate_format_doc_cmd.go:52`): `fmt.Fprintf(output.Writer, "✓ Schema valid\n...")` for success; warnings go to `output.ErrWriter`. No JSON mode.
- **`capmon derive`** (`capmon_derive_cmd.go:63`): `fmt.Printf("✓ Derived seeder spec for %q (%s) → %s\n", ...)` — plain text only.
- **`capmon check`** (`capmon_check_cmd.go`): no stdout output; side effects only (GitHub issues). Dry-run output goes to `os.Stderr` inside `flushProviderBatch` (`check.go:113`).
- **`capmon seed`** (`capmon_cmd.go:186`): no explicit output; delegates to `capmon.SeedProviderCapabilities`.

The `output.JSON` global and `--json` flag are used by non-capmon commands (e.g., `install`, `list`) but none of the capmon subcommands have adopted this pattern.

## Q7: Does any part of the CLI codebase implement retry logic for HTTP requests — if so, what package and function, and what is the retry approach?

Yes. Retry logic exists in `cli/internal/capmon/fetch.go`, function `FetchSource` (line 58).

**Approach:** Exponential backoff with a fixed delay sequence.

```go
delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
for attempt := 0; attempt <= len(delays); attempt++ {
    if attempt > 0 {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(delays[attempt-1]):
        }
    }
    raw, err = doHTTPFetch(ctx, rawURL)
    if err == nil {
        break
    }
    lastErr = err
}
```

(`fetch.go:65–79`)

- Maximum **4 total attempts** (1 initial + 3 retries)
- Delays: 1s, 2s, 4s between attempts
- Context cancellation is honored between retries
- Retries only on `doHTTPFetch` error (includes server errors `>= 500` and non-2xx responses per `fetch.go:124–128`)
- On exhaustion, returns `fmt.Errorf("fetch %s after retries: %w", rawURL, lastErr)` (`fetch.go:81`)

**`FetchChromedp`** (`fetch_chromedp.go`) has no retry logic — a single chromedp run either succeeds or returns the error directly.

No retry logic exists in any other package. The registry client (`cli/internal/registry/`), installer (`cli/internal/installer/`), and other HTTP callers do not implement retries.

## Q8: What telemetry property keys are registered in cli/internal/telemetry/catalog.go for capmon-related commands?

From `cli/internal/telemetry/catalog.go`, the `command_executed` event's `Properties` list is the single event definition. Capmon commands appear only in two `Commands` arrays:

**`provider` property** (`catalog.go:46–48`):
- Commands that send `provider`: `"capmon_validate_spec"`, `"capmon_validate_format_doc"`, `"capmon_validate_sources"`, `"capmon_derive"`, `"capmon_check"`, `"capmon_onboard"` (among non-capmon commands)

**`content_type` property** (`catalog.go:53–55`):
- Commands that send `content_type`: `"capmon_validate_spec"` (among non-capmon commands)

The following capmon command names do NOT appear in the telemetry catalog at all:
- `capmon_run` (the `capmon run` command uses `telemetry.Enrich("dry_run", ...)`, `telemetry.Enrich("provider", ...)`, `telemetry.Enrich("mode", ...)` at `capmon_cmd.go:135–143`, but `"mode"` property in the catalog lists only `"loadout_apply"` and `"add"` as its commands — `capmon_run` is absent)
- `capmon_seed` (uses `telemetry.Enrich("provider", ...)` at `capmon_cmd.go:202` but is not listed in `provider` property's commands)
- `capmon_fetch`, `capmon_extract`, `capmon_diff`, `capmon_generate`, `capmon_verify`, `capmon_test_fixtures`

**`dry_run` property** (`catalog.go:68`): lists `"install"`, `"add"`, `"uninstall"`, `"remove"`, `"sync-and-export"` — `capmon_run`, `capmon_check`, `capmon_onboard` all call `telemetry.Enrich("dry_run", ...)` but none appear in this property's `Commands` list.

**`mode` property** (`catalog.go:120–125`): lists `"loadout_apply"` and `"add"` — `capmon_run` is absent despite calling `telemetry.Enrich("mode", mode)`.

In summary: the only capmon commands registered in the telemetry catalog's `Commands` arrays are `capmon_validate_spec`, `capmon_validate_format_doc`, `capmon_validate_sources`, `capmon_derive`, `capmon_check`, and `capmon_onboard` — all under the `provider` property. The `content_type` property lists only `capmon_validate_spec`.

## Cross-cutting observations

- The `capmon run` command calls `os.Exit(exitClass)` directly (`capmon_cmd.go:154`) rather than returning an error code — this bypasses cobra's `PersistentPostRun` telemetry flush. The `capmon fetch` stub returns an error normally, so telemetry would fire on completion.

- `ProviderStatus` has both `SourcesFetched int` and separate `FetchStatus string` / `ExtractStatus string` / `DiffStatus string` fields (`types.go:72–75`). The per-stage string fields are not set by the current Stage 1 implementation; they remain zero-value empty strings.

- The slot directory naming convention changed from hyphen-separated (`hooks-0`) to dot-separated (`hooks.0`). Both formats coexist in the live `.capmon-cache/`. The current `sourceID` construction in `pipeline.go:163` uses `fmt.Sprintf("%s.%d", ctName, i)` (dot-style). The hyphen-style directories have no `extracted.json`.

- `FetchSource` writes `meta.json` initially without `Format` or `SourceURL`; then `pipeline.go:206–210` patches those fields via `WriteCacheMeta`. Two writes to `meta.json` occur per new (non-cached) source.

- `IsCached` (`cache.go:99–103`) checks only for the existence of `meta.json`, not `raw.bin`. A partial write (meta but no raw) would return `true` from `IsCached`.

- The `--cache-root` flag is present on `capmon verify`, `capmon seed`, and `capmon check` but NOT on `capmon fetch` or `capmon extract` stubs. `capmon run` also lacks a `--cache-root` flag; it uses the `PipelineOptions.CacheRoot` default of `".capmon-cache"`.
