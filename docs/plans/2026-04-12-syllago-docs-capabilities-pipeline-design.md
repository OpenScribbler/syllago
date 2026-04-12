# syllago-docs Capabilities Pipeline Design

**Date:** 2026-04-12
**Status:** Design Complete
**Scope:** Cross-repo feature spanning syllago CLI (Phase 1) and syllago-docs site (Phases 2-5)

## Problem

Provider capability data is stored in `docs/provider-formats/*.yaml` inside the syllago repo, but it is not surfaced to readers of the syllago-docs site in a queryable, transparent way. Readers cannot:

- Look up which providers support a given canonical key
- See when capability data was last verified and from which sources
- Report stale or incorrect capability information
- Browse provider-specific extensions beyond canonical keys

## Solution

A two-step pipeline: the syllago CLI generates a `capabilities.json` release artifact from the provider format YAMLs, and the syllago-docs site fetches that artifact during build to populate Astro data collections used by programmatic documentation components.

## Design Decisions

### Pipeline architecture

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | Generator location | `_gencapabilities` Go command in syllago CLI | Consistent with `_genproviders` pattern already shipping |
| D2 | Release artifact | `capabilities.json` alongside `providers.json` in GitHub release | Docs site already fetches `providers.json` from release; same pattern, same infra |
| D3 | Sync approach | `sync-capabilities.ts` in syllago-docs fetches from latest GitHub release | Independence from local repos; cache-safe; zero cross-repo filesystem deps |
| D4 | Data storage | Astro data collections (type: data) in `src/content/` | First-class Astro querying via `getCollection()`; Zod schema validation at build time |
| D5 | Scheduling | Run `_gencapabilities` in `release.yml` alongside `_genproviders` | Single release event keeps both artifacts in sync |

### Field inventory

**Public fields** (included in `capabilities.json`, visible on docs site):

| Field | Type | Purpose |
|-------|------|---------|
| `status` | string | `supported` or `unsupported` |
| `sources` | array | `{uri, type, fetched_at}` — transparency so readers can verify and report stale data |
| `canonical_mappings` | object | `{key: {supported, mechanism, paths}}` — per-key support detail |
| `provider_extensions` | array | `{id, name, description, source_ref}` — provider-specific features beyond canonical keys |
| `last_changed_at` | string | ISO 8601 — freshness indicator; when the data for this content type last changed |

**Internal-only fields** (read from YAML, never emitted in `capabilities.json`):

| Field | Reason excluded |
|-------|----------------|
| `confidence` | Internal verification signal only — if not in provider source/docs, feature is `supported: false, confidence: confirmed`. No hedging in public output. |
| `graduation_candidate` | Pipeline signal for promoting extensions to canonical keys — not a docs concept |
| `generation_method` | Implementation detail irrelevant to readers |
| `content_hash` | Content-addressed change detection — internal only |

### Confidence policy (critical)

The `confidence` field stays internal. The public rule: **if a feature isn't in provider source or documentation, it's unsupported** — `supported: false, confidence: confirmed`. Absence of documentation is the answer, not ambiguity. No "unknown" or "inferred" values ever appear in published output.

### Transparency model

Sources are public so readers can:
1. See which docs the capability data was derived from
2. Verify the data themselves
3. Report issues when sources have changed

The `MetaBox` component surfaces last_changed_at, source links, provider support count, and a pre-filled GitHub issue link for reporting.

### Components

| Component | Purpose | Placement |
|-----------|---------|-----------|
| `MetaBox.astro` | Top-right infobox for canonical key pages | Inline with page title |
| `CanonicalSupportTable.astro` | Providers × mechanism cross-reference table | Bottom of each canonical key page |
| `ProviderExtensions.astro` | Extension list per provider | Provider content type sections |

**MetaBox fields**: last_changed_at (freshness), source count with links (verification), provider support count, report-issue link (pre-filled GitHub issue URL using Starlight component override).

**Issue reporting**: Starlight component override that generates a pre-filled GitHub issue link. Not an edit-page link — reports go to the syllago repo issue tracker, not direct doc edits.

## Architecture Overview

```
syllago repo                          syllago-docs repo
────────────────────────────────      ────────────────────────────────────────
docs/provider-formats/*.yaml          GitHub Release (capabilities.json)
        ↓                                  ↓
_gencapabilities (Go command)         sync-capabilities.ts
        ↓                             (also reads docs/spec/canonical-keys.yaml)
capabilities.json                          ↓
                                      src/content/capabilities/*.json
                                      src/content/canonical-keys/*.json
        ↓                                  ↓
release.yml → GitHub Release          src/content/config.ts (Zod schemas)
                                           ↓
                                      Astro components (MetaBox, CanonicalSupportTable, ProviderExtensions)
                                           ↓
                                      Generated pages (canonical key pages, enriched provider pages)
```

## Phases

### Phase 1 — `_gencapabilities` Go command (syllago repo) ← THIS FEATURE

Reads `docs/provider-formats/*.yaml`. Outputs a `capabilities.json` with the public field inventory. Integrated into `release.yml` alongside `_genproviders`.

**Note on `canonical-keys.yaml`:** Phase 1 does NOT read `docs/spec/canonical-keys.yaml`. Canonical key names are taken directly from the `canonical_mappings` map keys in each provider format YAML (which are already validated by `capmon validate-format-doc`). The `canonical-keys.yaml` file is Phase 2 scope — `sync-capabilities.ts` in syllago-docs uses it to generate per-key cross-provider data files (`src/content/canonical-keys/*.json`).

**Input schema** (subset of provider format YAML to read):
```yaml
provider: <slug>
last_changed_at: <ISO 8601>
content_types:
  <type>:
    status: supported | unsupported
    sources:
      - uri: <url>
        type: documentation | source_code | changelog
        fetched_at: <ISO 8601>
    canonical_mappings:
      <key>:
        supported: true | false
        mechanism: <string>
        paths: [<string>]      # optional
        # confidence: omitted from output
    provider_extensions:
      - id: <string>
        name: <string>
        description: <string>
        source_ref: <url>      # optional
```

**Output schema** (`capabilities.json`):
```json
{
  "version": "1",
  "generated_at": "<ISO 8601>",
  "providers": {
    "<slug>": {
      "<content_type>": {
        "status": "supported",
        "last_changed_at": "<ISO 8601>",
        "sources": [{ "uri": "...", "type": "...", "fetched_at": "..." }],
        "canonical_mappings": {
          "<key>": { "supported": true, "mechanism": "...", "paths": [] }
        },
        "provider_extensions": [{ "id": "...", "name": "...", "description": "..." }]
      }
    }
  }
}
```

**Release integration**: `release.yml` already runs `_genproviders` to produce `providers.json`. The `_gencapabilities` command runs in the same step, outputting `capabilities.json` to the same directory, uploaded as the same release artifact.

### Phase 2 — sync-capabilities.ts (syllago-docs repo)

Fetches `capabilities.json` from the latest GitHub release (same URL pattern as `sync-providers.ts`). Writes:
- `src/content/capabilities/<provider>-<content-type>.json` (one file per provider+type combination)
- `src/content/canonical-keys/<key>.json` (cross-provider view per canonical key)

### Phase 3 — src/content/config.ts (syllago-docs repo)

New file (doesn't exist yet). Defines Astro data collections with Zod schemas for `capabilities` and `canonical-keys` collections.

### Phase 4 — Components (syllago-docs repo)

`MetaBox.astro`, `CanonicalSupportTable.astro`, `ProviderExtensions.astro` in `src/components/`.

### Phase 5 — Page generation (syllago-docs repo)

Auto-generate canonical key pages. Enrich provider pages with capability tables and extension lists. Update the capability matrix page.

## Providers

14 providers tracked:
`amp`, `claude-code`, `cline`, `codex`, `copilot-cli`, `cursor`, `factory-droid`, `gemini-cli`, `kiro`, `opencode`, `pi`, `roo-code`, `windsurf`, `zed`

## Notes

- The syllago-docs work (Phases 2-5) is tracked as a separate `/develop` feature in the syllago-docs repo
- Existing `sync-providers.ts` in syllago-docs is the reference pattern for `sync-capabilities.ts`
- Starlight version in syllago-docs: 5.6
- `src/content/config.ts` does not yet exist in syllago-docs
- 14 content type keys currently defined in `docs/spec/canonical-keys.yaml` (skills only)
- The `_genproviders` pattern in `release.yml` is the reference for release integration
