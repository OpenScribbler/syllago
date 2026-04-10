# Capability Monitor Pipeline Design

**Date:** 2026-04-08
**Status:** Design Complete (Panel Reviewed, 5-of-5 Consensus)
**Scope:** New `syllago capmon` subcommand + GitHub Actions workflow + seed migration from existing hooks spec

## Problem

The current process for detecting drift in AI coding tool provider capabilities is:
1. A human notices something might have changed (or audit is overdue)
2. Multiple LLM-based subagents read all provider docs + existing spec
3. Each run is non-deterministic — different runs find different things
4. Each run consumes substantial tokens and wall time
5. Output requires manual consolidation and fix application

This approach was used successfully for the hooks capability audit (see `.develop/hooks-audit-tier{1,2,3}.md` and `.develop/cursor-refetch.md`), but it surfaced real issues:
- 9 critical inaccuracies (one would have silently broken generated hooks on gemini-cli)
- 8 minor inaccuracies
- 6 data gaps
- 3 missing providers not tracked at all

The audit caught them, but only because we ran it. We need a deterministic, repeatable, low-cost pipeline that runs automatically and surfaces drift before it becomes a problem.

## Solution

A `syllago capmon` subcommand with a 4-stage pipeline (fetch → extract → diff → review) that runs twice daily via GitHub Actions. Structured YAML data files become the single source of truth for provider capabilities; hooks spec tables (and future spec tables) are generated from this data.

## Design Decisions

### Infrastructure

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | Tool location | `syllago capmon` subcommand | Discoverable via `syllago --help`, ships with main binary, contributors already know the CLI |
| D2 | Scheduling | Twice-daily GitHub Actions cron | ~120-180 min/month, well under 2,000 min free tier for private repos |
| D3 | Local testing | Same binary runs locally and in CI | Zero code duplication; `syllago capmon run --dry-run` for debugging |
| D4 | Implementation language | Go | Consistent with rest of syllago CLI; pure Go path available for all formats we care about |

### Data architecture

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D5 | Data schema organization | Per-provider YAML is source of truth; per-content-type YAML is generated | Best of both worlds — easy per-provider audit, easy per-spec consumption |
| D6 | Attribution model | References table + per-field `refs:` lists | Per-field provenance without verbosity bloat; pipeline auto-updates timestamps |
| D7 | Data directory | `docs/provider-capabilities/` | New top-level docs directory; requires gitignore whitelist |
| D8 | Source URL manifest | Existing `docs/provider-sources/*.yaml` | Already tracked, already has per-content-type source URLs; extended with `selector:` and `fetch_method:` fields |
| D9 | Format reference | Existing `docs/provider-formats/*.md` | Tracked in previous commit; serves as human-readable per-provider format documentation |
| D10 | Spec integration | Hooks spec tables are regenerated from YAML (single source of truth) | Eliminates two-sources-of-truth drift risk |

### Extraction strategy

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D11 | Determinism target | Full determinism, zero LLM in CI | User requirement; simplifies testing; eliminates probabilistic variance |
| D12 | Extraction depth | Section extraction + field extraction + auto YAML patches | Drift gets proposed as a specific YAML patch, not just a diff for human triage |
| D13 | Structural drift detection | Second pass over AST extracting all heading landmarks | Catches new sections that aren't covered by our selectors — flags for human review |
| D14 | LLM helper tool | None | Pure deterministic pipeline; human uses their own tools if LLM help needed |

### Parsers per format

Based on research into current (2026) Go ecosystem:

| # | Format | Tool | CGO? | Rationale |
|---|--------|------|------|-----------|
| D15 | Go source | `go/parser` + `go/ast` (stdlib) | No | Canonical, handles all modern Go including generics |
| D16 | TypeScript source | `odvcencio/gotreesitter` (pure Go reimpl of tree-sitter, 206 grammars) | No | Pure Go tree-sitter is a 2024-2025 development; proper AST queries |
| D17 | Rust source | `odvcencio/gotreesitter` with tree-sitter-rust grammar | No | Same library handles both TypeScript and Rust |
| D18 | JSON Schema | `encoding/json` + struct matching | No | Stdlib |
| D19 | JSON / YAML / TOML | stdlib + `gopkg.in/yaml.v3` / `BurntSushi/toml` | No | Stdlib + standard libraries |
| D20 | HTML | `PuerkitoBio/goquery` | No | jQuery-like CSS selectors, battle-tested |
| D21 | Markdown | `yuin/goldmark` + AST walking | No | CommonMark-compliant AST parser |

### Fetching strategy per provider

Based on empirical testing in this brainstorm session:

| # | Provider | Fetch method | Verified |
|---|----------|--------------|----------|
| D22 | Windsurf | Direct HTTP on `docs.windsurf.com/llms-full.txt` | Confirmed via research report |
| D23 | VS Code Copilot | GitHub API on `microsoft/vscode-docs` repo | Confirmed via research report |
| D24 | Claude Code | Direct HTTP on `docs.anthropic.com/llms-full.txt` | Partially — redirect chain may need handling |
| D25 | Cursor | **chromedp** (headless Chromium) | **Confirmed empirically** — 4 seconds, 34KB content, no bot challenge |
| D26 | Gemini CLI / OpenCode / Cline / Codex / Factory Droid / Roo Code / Zed / Kiro / Crush / Pi / Amp | Direct HTTP (GitHub raw, docs sites) | Per provider-sources.yaml `change_detection.method` |

### Cursor-specific findings (why chromedp is the only option)

All of the following were empirically tested during this brainstorm:

- `cursor.com/llms-full.txt` → **404** (file does not exist, even through a real browser)
- `cursor.com/llms.txt` → **429** Vercel bot wall (plain HTTP)
- `docs.cursor.com/llms*.txt` → **308 redirect** to `cursor.com/docs` (file doesn't exist at this path either)
- `llmstxt-cli install cursor` → **429** (the CLI is a plain HTTP fetcher; also, it's a different product — it's a package manager for llms.txt-distributed AI tooling, not a tool for fetching vendor docs)
- `llmstxthub.com` → catalog only, links to the same broken URLs
- Cursor GitHub org has no docs repo
- `cursor.com/sitemap.xml` has **zero** `/docs/` URLs
- **chromedp on `cursor.com/docs/agent/hooks`** → **4 seconds, 34KB of body text, proper title, no bot challenge** ✓

chromedp is the only approach that works for Cursor. It's isolated to one code path activated by `fetch_method: chromedp` in `provider-sources/cursor.yaml`. GitHub Actions uses the `chromedp/headless-shell` Docker image. Pure Go dependency, no CGO.

### Drift handling

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D27 | Tracked field drift | Auto-PR with YAML patch | Human reviews YAML diff (compact) and merges |
| D28 | Structural drift (new sections) | GitHub issue for triage | Human decides whether to add new tracked sections |
| D29 | Extraction failures | Silent retry with exponential backoff; issue after 3 consecutive failures | Tolerate transient failures, surface persistent ones |

### Testing

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D30 | Primary tests | Fixture-based integration tests | Deterministic, no network, fast |
| D31 | Live tests | Available locally via `SYLLAGO_TEST_NETWORK=1` | Consistent with existing project pattern; catches real-world drift in our parsers |

## Command Surface

Six subcommands under `syllago capmon`:

| Command | Purpose |
|---------|---------|
| `syllago capmon fetch [--provider <slug>]` | Fetches source URLs, updates hash cache. No extraction. |
| `syllago capmon extract [--provider <slug>]` | Runs extraction on cached sources. Writes to `provider-capabilities/`. |
| `syllago capmon run [--provider <slug>] [--dry-run]` | Full pipeline: fetch → extract → diff → report. This is what GitHub Actions calls. |
| `syllago capmon diff [--provider <slug>] [--since <ref>]` | Shows what changed in `provider-capabilities/` since a git ref. Local inspection. |
| `syllago capmon generate` | Regenerates per-content-type views from per-provider source-of-truth files. |
| `syllago capmon verify` | Validates YAML against JSON Schema, validates refs exist, validates generated views in sync. CI check. |

Plus bootstrap and utility commands:

| Command | Purpose |
|---------|---------|
| `syllago capmon seed [--provider <slug>] [--force-overwrite-exclusive]` | Idempotent bootstrap and re-seeding command. Reads existing `provider-capabilities/<slug>.yaml`, preserves `provider_exclusive:` entries unconditionally (additive merge), produces a parse log to stdout showing per-provider: fields extracted, fields skipped with reason, counts vs expected. The `--force-overwrite-exclusive` flag is required to remove any `provider_exclusive` entries; using it prints a warning listing each overwritten entry. |
| `syllago capmon test-fixtures [--update --provider <slug>]` | Without flags: reports fixture age in days from git log per provider. With `--update --provider <slug>`: re-fetches live source via Stage 1 logic and replaces fixture files only (not `expected/` golden outputs). Refuses bulk all-provider invocations to preserve per-provider audit trail. |

## YAML Schema

### Per-provider source of truth

```yaml
# docs/provider-capabilities/claude-code.yaml
schema_version: "1"
slug: claude-code
display_name: Claude Code
last_verified: 2026-04-08
provider_version: v2.1.86
source_manifest: docs/provider-sources/claude-code.yaml
format_reference: docs/provider-formats/claude-code.md

# References table — auto-maintained by pipeline
references:
  cc_hooks_docs:
    url: https://code.claude.com/docs/en/hooks
    fetch_method: http
    verified_at: 2026-04-08
    last_content_hash: "sha256:abc123..."
  cc_hooks_types:
    url: https://raw.githubusercontent.com/.../types.ts
    fetch_method: http
    verified_at: 2026-04-08
    last_content_hash: "sha256:def456..."

content_types:
  rules:
    supported: true
    confidence: high
    features:
      frontmatter: true
      frontmatter_fields: [description, alwaysApply, globs]
      scoping: per_file
      hierarchy: [enterprise, organization, user, project, local]
    refs: [cc_rules_docs]

  hooks:
    supported: true
    confidence: high
    events:
      before_tool_execute:
        native_name: PreToolUse
        blocking: prevent
        refs: [cc_hooks_docs, cc_hooks_types]
      after_tool_execute:
        native_name: PostToolUse
        blocking: observe
        refs: [cc_hooks_docs]
      # ... all events
    capabilities:
      structured_output:
        supported: true
        mechanism: "hookSpecificOutput with rich fields"
        refs: [cc_hooks_docs]
      input_rewrite:
        supported: true
        mechanism: "hookSpecificOutput.updatedInput"
        refs: [cc_hooks_docs]
      # ... all capabilities
    tools:
      shell:
        native: Bash
        refs: [cc_hooks_docs]
      file_read:
        native: Read
        refs: [cc_hooks_docs]
      # ... all tool vocabulary mappings

  mcp:
    supported: true
    confidence: high
    transports: [stdio, sse, http]
    auth: [env_vars, headers]
    config_location: ~/.claude/mcp.json
    refs: [cc_mcp_docs]

  skills:
    supported: true
    confidence: high
    features:
      directory_based: true
      frontmatter_fields: [name, description, allowed-tools, model]
      resource_loading: true
    refs: [cc_skills_docs]

  commands:
    supported: true
    # ... full structure
    refs: [cc_commands_docs]

  agents:
    supported: true
    # ... full structure
    refs: [cc_agents_docs]

# Provider-exclusive events/features without canonical equivalents
provider_exclusive:
  events:
    - native_name: InstructionsLoaded
      description: Fires when CLAUDE.md and rule files are loaded
      refs: [cc_hooks_docs]
    - native_name: TaskCreated
      description: Fires when the agent creates a new task
      refs: [cc_hooks_docs]
    # ...
```

### Structured selector schema

Source entries in `docs/provider-sources/*.yaml` use a structured `selector:` object (not a bare string). This shape was fixed at design time because adding structured subfields to a flat `selector: string` later would require a schema migration across all provider-sources files.

```yaml
hooks:
  sources:
    - url: https://code.claude.com/docs/en/hooks
      type: docs
      format: html
      selector:
        primary: "main h2#events ~ table"
        fallback: "main table:contains('Event')"
        expected_contains: "Event Name"  # structural anchor, NOT extracted provider text
        min_results: 6                    # hard floor; fewer → soft failure, not diff input
        updated_at: 2026-04-08            # human-maintained, not auto-updated
      extracts: [event_names, hook_config_fields]
```

`SelectorConfig` lives in its own `cli/internal/capmon/selector.go` to prevent circular imports between fetch and extract stages.

Selector failure behavior:
- If `expected_contains` is specified and absent from the extracted bytes: hard extraction failure — not a retry. Exit class 3 (infrastructure), not exit class 2 (partial). Written to `extracted.json` with `reason: anchor_missing`.
- If `min_results` is specified and extracted count is below the floor: soft failure — opens an issue listing missing fields, does NOT produce a YAML patch.
- `expected_contains` must be a structural anchor (heading text, type name, schema key), NEVER extracted provider text — security requirement to prevent a weaponized provider doc from suppressing failure detection.

### Schema Evolution Policy

The `provider-capabilities/*.yaml` schema follows a three-rule evolution policy:

1. **Additive changes** (new fields, new content types) do NOT require a version bump. All implementations treat unknown fields as informational.
2. **Breaking changes** (field removal, field rename, type change) bump `schema_version` and require a `MIGRATIONS.md` entry before any auto-PR can land.
3. **`syllago capmon verify` accepts current OR current-minus-one** during an explicit `--migration-window` flag. After the window closes, current-minus-one is rejected.

`MIGRATIONS.md` format: migration ID, affected version range, migration steps (copy-paste commands), verification command. Example: `001-rename-supported-to-status.md`.

**Hard deadline:** Cluster I (schema version policy) must land before any external consumer reads `provider-capabilities/*.yaml`. Currently, the only consumer is the spec regeneration step — so this policy must be in place before spec regeneration code ships.

### Per-content-type generated views

Generated from per-provider files by `syllago capmon generate`:

```yaml
# docs/provider-capabilities/by-content-type/hooks.yaml
# THIS FILE IS GENERATED. Do not edit directly.
# Source: docs/provider-capabilities/*.yaml
# Generated at: 2026-04-08T22:15:00Z

schema_version: "1"
content_type: hooks

providers:
  claude-code:
    supported: true
    events:
      before_tool_execute: { native_name: PreToolUse, blocking: prevent }
      # ...
  gemini-cli:
    supported: true
    events:
      # ...
  # ... all providers
```

## Pipeline Flow (Detailed)

### Stage 1: Fetch

```
Input:  docs/provider-sources/*.yaml (source URL manifests)
Output: .capmon-cache/<provider-slug>/<source-id>/ directories with:
        - raw.bin (fetched content)
        - meta.json (timestamp, hash, status)
```

At startup, `syllago capmon run` performs age-based eviction of `.capmon-cache/` entries older than 30 days via a single `fs.WalkDir`. No separate `capmon clean` subcommand is needed.

Per source:
1. Read `url` and `fetch_method` from provider-sources YAML
2. Call `validateSourceURL` on every source URL at run start — NOT cached. Failure is a hard job-level error.
3. If `fetch_method: chromedp`, connect to the headless-shell sidecar via `CHROMEDP_URL` (CI) or local Chrome (dev); otherwise use `net/http`
4. Compute SHA-256 of fetched content
5. Compare against `.capmon-cache/.../meta.json` previous hash
6. If unchanged, mark cached. If changed, write new content + hash.
7. Apply retry with backoff on network failures. Tolerate 1-2 transient failures before marking source as failed.

### Stage 2: Extract

```
Input:  .capmon-cache/<provider-slug>/<source-id>/raw.bin (only for changed sources)
Output: .capmon-cache/<provider-slug>/<source-id>/extracted.json (structured data)
```

#### Extractor interface

```go
// cli/internal/capmon/extractor.go

type Extractor interface {
    Extract(ctx context.Context, raw []byte, cfg SelectorConfig) (*ExtractedSource, error)
}

// Stage 2 dispatch
var extractors = map[string]Extractor{} // populated via init() from per-format packages

func Extract(ctx context.Context, format string, raw []byte, cfg SelectorConfig) (*ExtractedSource, error) {
    ext, ok := extractors[format]
    if !ok {
        return nil, fmt.Errorf("no extractor for format %q", format)
    }
    return ext.Extract(ctx, raw, cfg)
}
```

The Extractor interface decouples Stage 2 dispatch from specific parser libraries. `gotreesitter` is the provisional choice for TypeScript and Rust extraction. **The intended long-term target is `microsoft/typescript-go`** once its API stabilizes. Swapping parser implementations requires replacing one file per format — the dispatch loop never changes.

#### ExtractedSource envelope (internal artifact)

```go
// cli/internal/capmon/extractor.go

type ExtractedSource struct {
    ExtractorVersion string                 `json:"extractor_version"`
    Provider         string                 `json:"provider"`
    SourceID         string                 `json:"source_id"`
    Format           string                 `json:"format"`
    ExtractedAt      time.Time              `json:"extracted_at"`
    Partial          bool                   `json:"partial"`         // true if below min_results
    Fields           map[string]FieldValue  `json:"fields"`
    Landmarks        []string               `json:"landmarks"`        // all headings/landmarks for structural drift
}

type FieldValue struct {
    Value     string `json:"value"`
    ValueHash string `json:"value_hash"` // SHA-256 of value for fingerprint divergence detection
}
// capmon: pipeline-internal volatile state, no schema_version
```

`ExtractedSource` is a throwaway internal cache artifact, not a versioned public schema. It has no `schema_version` field. If its shape changes, invalidate the cache and regenerate. A Go build comment on the type enforces this: `// capmon: pipeline-internal volatile state, no schema_version`.

Per changed source:
1. Read `format` from provider-sources YAML
2. Dispatch via `Extract()` to the format-keyed `map[string]Extractor` (each format registers via `init()`):
   - `json-schema` → parse + walk definitions
   - `json` → native parse
   - `yaml` → native parse
   - `toml` → native parse
   - `typescript` → gotreesitter with tree-sitter-typescript grammar + S-expression queries
   - `rust` → gotreesitter with tree-sitter-rust grammar
   - `go` → `go/parser` + `go/ast`
   - `markdown` → goldmark AST walking with configured heading path
   - `html` → goquery with configured CSS selector
3. Apply `SelectorConfig` (primary, fallback, `expected_contains`, `min_results`) to extract the relevant section. Selector failure → exit class 3, not class 2.
4. Within the section, extract specific fields (events, capability names, etc.)
5. Call `sanitizeExtractedString` on every string value before writing to `extracted.json` (see Security Controls — H2)
6. Stage 2 output must use `gopkg.in/yaml.v3` with explicit `Tag: "!!str"` on all extracted string fields (see Security Controls — H3)
7. Second pass: extract all top-level headings/landmarks for structural drift detection
8. Write extracted data to `extracted.json`

**D13 limitation acknowledgment:** Structural drift detection catches new or removed headings but CANNOT detect semantic drift within a stable heading structure. If a provider reorganizes content under an unchanged heading while the `expected_contains` anchor remains present, the change is invisible to the deterministic pipeline. This is an accepted limitation; no deterministic extraction scheme can close it without LLM-based semantic comparison, which is explicitly out of scope (D14).

### Stage 3: Diff

```
Input:  .capmon-cache/<provider-slug>/<source-id>/extracted.json (new)
        docs/provider-capabilities/<provider-slug>.yaml (current)
Output: Change report (JSON or markdown) + proposed YAML patch
```

1. For each tracked field extracted in Stage 2, compare against current YAML value
2. Produce a structured diff: additions, removals, modifications
3. For structural drift (new headings): produce a separate "untracked section" report
4. Generate a proposed YAML patch (in-place update of current provider file)

### Stage 4: Review

```
Input:  Diff report + proposed YAML patch
Output: GitHub PR (for field changes) or GitHub Issue (for structural changes)
```

**Step 0: Deduplication check.** Before creating a new branch, query `gh pr list --label capmon --head capmon/drift-<slug>`. If an open PR exists, push the new patch as a commit to the existing branch (if different) or record `action_taken: deduped` in the run manifest (if identical). Two competing YAML patches for the same provider are never allowed to coexist.

1. If field drift detected: validate provider slug via `sanitizeSlug`, create a branch (`capmon/drift-<slug>`), apply the YAML patch, run `syllago capmon verify` to gate the patch before committing, open a PR using `buildPRBody` (see Security Controls — H6)
2. If structural drift detected: open an issue with the structural diff
3. If extraction failures: retry with backoff; open an issue after 3+ consecutive failures
4. Auto-label PRs/issues with `capmon` and the affected provider slug

**PR body template file:** `.github/capmon-pr-body.tmpl` is owned in the repo and change-controlled via normal PR review. It contains ONLY the fixed header and footer prose — extracted content is inserted programmatically (see Security Controls — H6). The PR body includes the `run_id` from the run manifest so reviewers can correlate the PR to the exact run artifact without searching the Actions tab by timestamp.

**Stage 4 execution order (numbered, implementers must not reorder):**
1. Dedup check via GitHub API
2. `syllago capmon verify` gates the patch before `create-pull-request` fires
3. `peter-evans/create-pull-request` invocation with `buildPRBody` output

### Run Manifest

The run manifest is write-only observability output — never a pipeline input. A Go build comment on the type enforces this: `// capmon: never-read-as-input`.

```go
type RunManifest struct {
    RunID                         string              `json:"run_id"`
    StartedAt                     time.Time           `json:"started_at"`
    FinishedAt                    time.Time           `json:"finished_at"`
    ExitClass                     int                 `json:"exit_class"` // 0-5, see Exit Classes section
    SourcesAllCached              bool                `json:"sources_all_cached"`
    Providers                     map[string]ProviderStatus `json:"providers"`
    Warnings                      []string            `json:"warnings"`
    FingerprintDivergenceWarnings []string            `json:"fingerprint_divergence_warnings"`
}

type ProviderStatus struct {
    FetchStatus     string  `json:"fetch_status"`
    ExtractStatus   string  `json:"extract_status"`
    DiffStatus      string  `json:"diff_status"`
    ActionTaken     string  `json:"action_taken"`
    FixtureAgeDays  *int    `json:"fixture_age_days"` // nullable: nil = no fixtures for new provider
}
```

The manifest is populated incrementally as each stage runs. Stage 1 sets `FetchStatus` per provider. Stage 2 sets `ExtractStatus`. Stage 3 sets `DiffStatus`. Stage 4 sets `ActionTaken`. Written to `.capmon-cache/last-run.json` locally and uploaded via `actions/upload-artifact` with `if: always()` at step level (not job level — panics bypass job-level conditions) and `retention-days: 90`.

### Exit Classes

| Code | Class | Meaning |
|------|-------|---------|
| 0 | clean | All providers extracted, no drift detected |
| 1 | drifted | Drift detected, PR/issue opened |
| 2 | partial_failure | Some providers failed, others succeeded; partial manifest uploaded |
| 3 | infrastructure_failure | Chromedp down, network unreachable, selector broken |
| 4 | fatal | Config corrupt, schema validation failed, unrecoverable |
| 5 | paused | `.capmon-pause` sentinel present; Stage 4 skipped |

`.capmon-pause` sentinel: a file at repo root. When present, Stages 1–3 still run, but Stage 4 is skipped and the manifest records `exit_class: 5`. Used during manual audits to prevent auto-PR conflicts with in-flight human changes. Must be documented in `CONTRIBUTING.md`.

The GitHub Actions job summary displays a badge keyed on exit class — a passing job with exit class 2 (partial failure) must not appear green to the operator.

## Security Controls

### H1. Two-job isolation

Fetch and extraction run in `fetch-extract` with `permissions: {}` — no GitHub token is ever injected into the job that touches untrusted provider content. Write operations are isolated to the `report` job that never re-fetches or re-parses provider docs. See the GitHub Actions Workflow section for full details.

### H2. Extracted string sanitization

All 8 extractors call `sanitizeExtractedString` on every string value before writing `extracted.json`:

```go
// cli/internal/capmon/sanitize.go

// yamlStructuralChars is the set of characters that have structural meaning
// in YAML when they appear at the start of a scalar.
const yamlStructuralChars = "{}[]:#&*!|>@%`"

func sanitizeExtractedString(s string) string {
    // 1. Strip trailing newlines
    s = strings.TrimRight(s, "\n\r")
    // 2. Cap at 512 bytes, append "[truncated]" if exceeded
    if len(s) > 512 {
        s = s[:500] + " [truncated]"
    }
    // 3. Percent-encode YAML structural chars in first non-whitespace position
    trimmed := strings.TrimLeft(s, " \t")
    if len(trimmed) > 0 && strings.ContainsRune(yamlStructuralChars, rune(trimmed[0])) {
        // leading indentation + percent-encoded first char + rest
        indent := s[:len(s)-len(trimmed)]
        s = indent + fmt.Sprintf("%%%02X", trimmed[0]) + trimmed[1:]
    }
    return s
}
```

### H3. YAML type coercion prevention

Stage 2 output must use `gopkg.in/yaml.v3` with explicit `Tag: "!!str"` on all extracted string fields. Without this, a provider event named `"true"` parses as a bool in YAML, breaking downstream tooling. Implementation: use `yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}` for every extracted string.

### H4. SSRF prevention via URL allowlist

```go
// cli/internal/capmon/fetch.go

func validateSourceURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("parse URL: %w", err)
    }
    if u.Scheme != "https" {
        return fmt.Errorf("only https scheme allowed, got %q", u.Scheme)
    }
    host := u.Hostname()
    if net.ParseIP(host) != nil {
        return fmt.Errorf("raw IP literal not allowed: %q", host)
    }
    ips, err := net.LookupHost(host)
    if err != nil {
        return fmt.Errorf("resolve %q: %w", host, err)
    }
    for _, ip := range ips {
        parsed := net.ParseIP(ip)
        if isReservedIP(parsed) {
            return fmt.Errorf("hostname %q resolves to reserved IP %q", host, ip)
        }
    }
    return nil
}

func isReservedIP(ip net.IP) bool {
    // Blocks: 127.0.0.0/8 (loopback), 169.254.0.0/16 (link-local/IMDS),
    // 100.64.0.0/10 (CGNAT/Alibaba IMDS), ::1 (IPv6 loopback), fe80::/10 (IPv6 link-local)
    // ...
}
```

`validateSourceURL` must be called at every `capmon run` start for every source URL — NOT cached. Failure is a hard job-level error.

### H5. `sanitizeSlug` for branch names

```go
// cli/internal/capmon/report.go

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func sanitizeSlug(slug string) (string, error) {
    if !slugRegex.MatchString(slug) {
        return "", fmt.Errorf("invalid slug: %q", slug)
    }
    return slug, nil
}
```

`sanitizeSlug` is applied to **both** PR body construction AND branch name construction in Stage 4. Without this, a crafted provider slug could escape the `capmon/drift-` branch prefix.

### H6. PR body template — extracted content never enters template engine

PR body is built by writing extracted fields directly to an `io.Writer`, NOT by passing them into a `text/template` data map. Extracted strings are triple-backtick fenced inline. Template engines that see extracted content are defeated by attacker-controlled template directives.

```go
// cli/internal/capmon/report.go

func buildPRBody(w io.Writer, diff CapabilityDiff) error {
    // Fixed header — prose only
    fmt.Fprintf(w, "# capmon drift: %s\n\n", diff.Provider) // slug already sanitized
    fmt.Fprintf(w, "Run ID: %s\n", diff.RunID)
    fmt.Fprintf(w, "Changed fields: %d\n\n", len(diff.Changes))

    // Per-field — extracted values always in fenced blocks
    for _, change := range diff.Changes {
        fmt.Fprintf(w, "## %s\n\n", change.FieldPath) // our own YAML key structure, safe
        fmt.Fprintln(w, "Old value:")
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, change.OldValue) // raw, fenced — NOT interpolated into prose
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, "New value:")
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, change.NewValue)
        fmt.Fprintln(w, "```")
    }

    // Fixed footer — non-ground-truth disclaimer
    fmt.Fprintln(w, "\n---")
    fmt.Fprintln(w, "**Pipeline output is not ground truth.** Verify each changed value against the linked source URL independently before approving.")
    return nil
}
```

**Implementation sequencing requirement:** All three H-cluster controls (H2 `sanitizeExtractedString` in Stage 2, H4 `validateSourceURL` at startup, H5/H6 `sanitizeSlug` + `buildPRBody` in Stage 4) land together in one PR. An intermediate state where `validateSourceURL` is deployed without `sanitizeExtractedString` is a real exposure window.

## Spec Integration

After the pipeline exists and initial YAML is seeded:

### Hooks spec regeneration

The hooks spec files become partially generated:

- **`events.md` §4 (Event Name Mapping table):** Generated from `provider-capabilities/*.yaml` events entries
- **`blocking-matrix.md` §2 (Matrix table):** Generated from events `blocking:` fields
- **`capabilities.md` support matrices:** Generated from `capabilities.*.supported` fields
- **`tools.md` §1 (Canonical Tool Names table):** Generated from `tools.*.native` fields

Each generated section gets a banner:

```markdown
<!-- GENERATED FROM provider-capabilities/*.yaml — do not edit directly.
     Regenerate with: syllago capmon generate -->
```

Normative spec text (definitions, introductions, conformance levels) stays hand-maintained.

A CI check (`syllago capmon verify`) fails the build if the generated markdown is out of sync with the YAML source.

### Future spec integration

When rules, commands, MCP, agents, skills specs are created (per the rubric from earlier in this session), they follow the same pattern: normative text hand-maintained, support matrices generated.

## Gitignore Changes Required

Add to `.gitignore` whitelist:
```
!docs/provider-capabilities/
```

Add to `.gitignore` ignore list:
```
.capmon-cache/
```

## Testing Strategy

### Fixture-based integration tests (CI)

Location: `cli/internal/capmon/testdata/`

Structure:
```
testdata/
├── fixtures/
│   ├── claude-code/
│   │   ├── hooks-docs.html          # snapshot of claude-code hooks docs
│   │   └── hooks-types.ts           # snapshot of types.ts
│   ├── gemini-cli/
│   │   └── types.ts                 # snapshot
│   └── windsurf/
│       └── llms-full.txt            # snapshot
├── expected/
│   ├── claude-code.yaml             # expected extraction output
│   └── ...
└── extract_test.go
```

Tests point at fixture files via `file://` URLs. Tests verify:
- Parser correctly extracts expected fields
- Structural drift detection fires when fixture changes
- Extraction failures are handled gracefully

### Live network tests (local only)

Gated by `SYLLAGO_TEST_NETWORK=1`:
- Fetch real source URLs
- Verify selectors still work against current provider docs
- Catch parser regressions against real-world drift

```bash
# CI runs only fixture tests
cd cli && make test

# Local dev can run live tests
SYLLAGO_TEST_NETWORK=1 cd cli && go test ./internal/capmon/...
```

## GitHub Actions Workflow

`.github/workflows/capmon.yml` — a single file containing all three jobs. Three separate workflow files would require three independent Dependabot configurations and three SHA pin sets for the same action versions; a single file prevents this class of drift.

All GitHub Actions are SHA-pinned to full commit SHAs (not tags). Docker image SHA pinning uses Renovate's `docker` datasource configuration in `.github/renovate.json` — GitHub's Dependabot does not pin Docker image SHAs in workflow files. Both Dependabot (Actions) and Renovate (Docker) are mandatory. Add `.github/dependabot.yml` with `package-ecosystem: github-actions`.

```yaml
name: Capability Monitor

on:
  schedule:
    - cron: '0 14 * * *'  # 14:00 UTC daily
    - cron: '0 2 * * *'   # 02:00 UTC daily (twice daily)
  workflow_dispatch:      # manual trigger

jobs:
  fetch-extract:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions: {}           # no token — fetch/parse run without write capability
    services:
      headless-shell:
        image: chromedp/headless-shell@<full-sha>   # pinned via Renovate docker datasource
        ports: ['9222:9222']
        options: --cap-drop=ALL --no-new-privileges --read-only --tmpfs /tmp
    env:
      CHROMEDP_URL: ws://localhost:9222
    steps:
      - uses: actions/checkout@<full-commit-sha>    # pinned via Dependabot
      - uses: actions/setup-go@<full-commit-sha>    # pinned via Dependabot
        with:
          go-version: '1.23'
      - name: Build syllago
        run: cd cli && make build
      - name: chromedp health check
        run: curl -sf http://localhost:9222/json/version || (echo 'chromedp health check failed' && exit 1)
      - name: Run fetch + extract stages
        run: syllago capmon run --stage fetch-extract
      - name: Upload cache artifact
        if: always()    # if: always() at step level — panics bypass job-level conditions
        uses: actions/upload-artifact@<full-commit-sha>    # pinned via Dependabot
        with:
          name: capmon-cache
          path: .capmon-cache/
          retention-days: 90
      - name: Compute and publish artifact SHA-256
        if: always()
        run: |
          hash=$(tar -c .capmon-cache/ | sha256sum | awk '{print $1}')
          echo "::notice::artifact-sha256=$hash"

  report:
    runs-on: ubuntu-latest
    needs: fetch-extract
    continue-on-error: false    # peter-evans/create-pull-request silently succeeds on failures; this forces the job to fail visibly
    permissions:
      contents: write
      pull-requests: write
      issues: write
    steps:
      - uses: actions/checkout@<full-commit-sha>    # checkout BEFORE download-artifact — peter-evans needs a git working tree
      - uses: actions/setup-go@<full-commit-sha>
        with:
          go-version: '1.23'
      - uses: actions/download-artifact@<full-commit-sha>    # pinned via Dependabot
        with:
          name: capmon-cache
          path: .capmon-cache/
      - name: Verify artifact SHA-256
        run: |
          expected=$(gh run view ${{ github.run_id }} --json jobSummaries \
            | jq -r '.jobSummaries[] | select(.name=="fetch-extract") | .summary' \
            | grep 'artifact-sha256=' | cut -d= -f2)
          actual=$(tar -c .capmon-cache/ | sha256sum | awk '{print $1}')
          if [ "$expected" != "$actual" ]; then
            echo "SHA-256 mismatch: expected=$expected actual=$actual"
            exit 1
          fi
      - name: Build syllago
        run: cd cli && make build
      - name: Run report stage
        run: syllago capmon run --stage report
      - name: Create PR for field drift
        uses: peter-evans/create-pull-request@<full-commit-sha>    # pinned via Dependabot
        with:
          title: 'capmon: detected provider capability drift'
          body-path: .github/capmon-pr-body.tmpl
          branch: capmon/drift-${{ github.run_id }}
          labels: capmon

  staleness-check:
    runs-on: ubuntu-latest
    # Separate daily cron, offset from main jobs to avoid GHA queue interference
    # Triggered only by its own schedule entry (add to on.schedule above with offset cron)
    # cron: '0 8 * * *'   # 08:00 UTC daily, offset from main 02:00 and 14:00 runs
    permissions:
      issues: write
    steps:
      - uses: actions/checkout@<full-commit-sha>
      - name: Download most recent manifest artifact
        uses: actions/download-artifact@<full-commit-sha>
        with:
          name: capmon-cache
          path: .capmon-cache/
        continue-on-error: true    # if artifact is missing, the next step opens an issue
      - name: Check staleness
        run: |
          # Open a GH issue labeled capmon,staleness if artifact is missing
          # OR if run_at in last-run.json is more than 36 hours old
          syllago capmon verify --staleness-check --threshold-hours 36
```

## Scope

### In scope for this cycle

1. `syllago capmon` subcommand skeleton with all 6 commands + `seed`
2. Fetch stage with direct HTTP + chromedp for Cursor
3. All 8 format extractors (json-schema, json, yaml, toml, typescript, rust, go, markdown, html)
4. Hash cache infrastructure
5. Diff stage with field-level and structural drift detection
6. YAML schema + JSON Schema validation
7. Seed command to migrate existing hooks spec data into YAML
8. Generate command for per-content-type views
9. Spec regeneration for hooks (events.md, blocking-matrix.md, capabilities.md, tools.md)
10. GitHub Actions workflow
11. Fixture tests for all extractors
12. Live network tests gated by env var
13. Documentation (README in docs/provider-capabilities/, CONTRIBUTING update)

### Out of scope (deferred)

1. Spec regeneration for rules, commands, MCP, agents, skills specs (those specs don't exist yet)
2. Docs site integration (`syllago-pdyxv` bead remains a separate future project)
3. LLM-based extraction fallback
4. Selector configuration UI / interactive helper
5. Historical drift reports / provider version tracking beyond `last_verified`

## Bootstrap Sequence

The order of operations matters because of the chicken-and-egg bootstrap:

1. **Build infrastructure** — subcommand skeleton, fetch cache, extractors, YAML schema, diff, generate, verify

1a. **gotreesitter dependency review** — a named owner must be assigned before implementation begins. Checklist:
   - Module path verified (`github.com/odvcencio/gotreesitter` or current equivalent)
   - No CGO in any transitive dep (verify with `go mod graph | grep -i cgo`)
   - No `.wasm`, `.so`, `.a`, or other native code in bundled grammar modules
   - S-expression query support for TypeScript and Rust confirmed
   - Binary size impact benchmarked (upper bound: 20MB)
   - Owner signs off in writing on the capmon epic bead

2. **Build seed command** — reads existing hooks spec + provider-formats, produces initial `provider-capabilities/*.yaml`
3. **Run seed manually** — human reviews output AND the parse log. The review is successful only if every skipped field is explained in the parse log.
4. **Build spec regeneration** — add banners to hooks spec tables, wire up `generate` to produce them
5. **First live pipeline run (manual)** — verify everything is in sync
6. **Wire GitHub Actions** — add workflow file, let it run on schedule
7. **Monitor first several automated runs** — verify PRs and issues are opened correctly

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Chrome/headless-shell fails to start in GitHub Actions | Docker image is purpose-built for CI; well-documented by chromedp project |
| gotreesitter has bugs with specific TypeScript/Rust constructs | Fixture tests catch parser regressions; fall back to external `ast-grep` binary if blocker |
| Selector configuration breaks when providers reorganize docs | Pipeline reports "extraction failed" and opens an issue; human updates selector |
| Many providers ship changes simultaneously → PR flood | Auto-PRs are per-provider; human reviews at their own pace; `capmon` label makes them filterable |
| Cursor first-party docs unreachable by plain HTTP | Use chromedp via Cluster A workflow. No automated fallback if chromedp breaks; pipeline opens an issue after 3 consecutive failures. Community mirrors are NOT substituted automatically — a human must evaluate them manually before updating YAML. |
| Hash cache grows unbounded | Age-based eviction of entries older than 30 days at startup via `fs.WalkDir`. No separate `capmon clean` command needed. |
| First run on a provider takes long (all sources new) | Acceptable — twice-daily cadence means steady-state is fast |

## Panel Review Record

### Initial design validation

This design was developed collaboratively in the brainstorm session following the hooks capability audit. Key decisions were validated against empirical testing:

- Cursor fetch strategy: llms.txt confirmed non-existent; chromedp confirmed working in 4 seconds with 34KB content
- Go parser ecosystem: `gotreesitter` confirmed as pure-Go tree-sitter reimplementation (2024-2025 development)
- llms.txt coverage: confirmed working for Windsurf, VS Code Copilot; redirect issue for Anthropic
- GitHub Actions cost: negligible at twice-daily cadence (~120-180 min/month vs 2,000 min free tier)

### Formal panel review

A 5-round multi-perspective panel review was conducted with 5 panelists: operator (on-call/reliability lens), implementation engineer (implementability lens), long-horizon maintainer (3-5 year maintenance lens), security reviewer (threat model lens), and remy (convergence facilitator). The review ran rounds 0–5.

**Result: 5-of-5 consensus.** Final positions:
- Remy (facilitator): AGREE — "I support implementation proceeding, with the converged design applied in full."
- Engineer: AGREE WITH RESERVATIONS — "The design is now implementable."
- Operator: AGREE WITH RESERVATIONS — "My operator concerns are resolved."
- Maintainer: AGREE WITH RESERVATIONS — "The design is sound for long-term maintenance."
- Security: AGREE WITH RESERVATIONS — "The design is defensible."

The reservations across all four AGREE WITH RESERVATIONS panelists were addressed by the design changes applied in this doc update. None were design problems requiring redesign — all were specific doc corrections and implementation constraints now explicit in the text.

**Nine convergence clusters produced by the review:**
- **Cluster A:** Two-job workflow split + chromedp wiring fix (unanimous, highest-stakes)
- **Cluster B:** Run manifest as GH Actions artifact
- **Cluster C:** Structured selector schema (`expected_contains`, `min_results`, `fallback`)
- **Cluster D:** Parser-agnostic Extractor interface
- **Cluster E:** Seed idempotency + parse log
- **Cluster F:** PR dedup + body template + fenced content (nice-to-have)
- **Cluster G:** Exit classes + staleness sentinel + `.capmon-pause` (nice-to-have)
- **Cluster H:** Security controls (SSRF, sanitization, slug validation, PR body escaping)
- **Cluster I:** Schema version evolution policy (nice-to-have, hard deadline before external consumers)

Full panel bus: `.develop/capmon-panel-bus.jsonl` (26 messages, rounds 0–5).

## Template Value

This pipeline design applies directly to future content type specs. The same YAML schema + extraction + generation pattern works for rules, commands, MCP, agents, and skills without modification. Only the extractor selectors and YAML `content_types` entries need to be added for each new spec.
