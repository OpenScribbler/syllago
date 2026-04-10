# Registry Privacy Gate - Design Document

**Goal:** Prevent content from private registries from being shared to public registries through syllago's tooling (soft gate).

**Decision Date:** 2026-03-21

---

## Problem Statement

Syllago has no concept of registry privacy. Content flows freely between registries regardless of whether the source is private (corporate/internal) or public (open-source/community). A user could accidentally publish proprietary content from a private registry to a public one through normal CLI/TUI workflows.

This is a data leakage prevention gate. It prevents accidental exposure through syllago's tooling, not deliberate circumvention — someone who wants to manually copy files can always do so.

## Proposed Solution

A two-layer privacy gate system ("belt and suspenders"):

1. **Entry tainting:** Content gets tagged with its source registry's visibility when imported into the library. This taint is immutable and persists through the content's lifetime.
2. **Exit verification:** At publish/share time, check both the content's taint AND the source registry's current visibility (live API probe when possible). Block if private content targets a public destination.

## Architecture

### Component 1: Visibility Detection (`cli/internal/registry/visibility.go`)

New file. Platform-aware detector that probes Git hosting APIs to determine repository visibility.

**Detection priority (stricter always wins):**
1. API probe (GitHub/GitLab/Bitbucket) — authoritative when reachable
2. `registry.yaml` `visibility` field — fallback for unknown/self-hosted hosts
3. Default `"unknown"` — treated as private (fail-safe)

**Conflict resolution:** If API and `registry.yaml` disagree, the **stricter value wins**. `private > unknown > public`. A `registry.yaml` declaring `public` on a repo the API reports as `private` resolves to `private`.

**Platform support:**

| Platform | URL pattern | API endpoint | Response field |
|----------|------------|-------------|----------------|
| GitHub | `github.com/{owner}/{repo}` | `GET /repos/{owner}/{repo}` | `"private": bool` |
| GitLab | `gitlab.com/{owner}/{repo}` | `GET /projects/{owner}%2F{repo}` | `"visibility": string` |
| Bitbucket | `bitbucket.org/{owner}/{repo}` | `GET /repositories/{owner}/{repo}` | `"is_private": bool` |
| Unknown host | anything else | No probe | `"unknown"` (= private) |

**Behaviors:**
- No auth needed for public repo checks (public API endpoints)
- Private repos return 404 without auth → inferred as `"private"`
- Network failure → `"unknown"` → treated as private (fail-safe)
- Uses stdlib `net/http` only (no new dependencies)
- Results cached in `Registry.Visibility` in config
- TTL-based caching: re-probe if cached result is older than 1 hour
- Re-probed on `registry sync` and at publish/share time (live check)

### Component 2: Content Tainting (`cli/internal/metadata/metadata.go`)

Two new fields on the `Meta` struct:

```go
SourceRegistry   string `yaml:"sourceRegistry,omitempty"`   // e.g., "acme/internal-rules"
SourceVisibility string `yaml:"sourceVisibility,omitempty"` // "public", "private", "unknown"
```

**When taint is set:**
- `syllago add` — when adding from a provider, check for symlink back to library (see Laundering Defense below)
- TUI import wizard — when importing from a git URL that matches a configured registry
- Any path that calls `add.AddItems()` with registry-sourced content

**Taint is immutable:** Once set, `sourceVisibility` does not change even if the source registry's visibility changes. Users must re-import from the (now-changed) registry to get fresh taint.

### Component 3: Registry Config Extension (`cli/internal/config/config.go`)

New field on `Registry` struct:

```go
type Registry struct {
    Name       string `json:"name"`
    URL        string `json:"url"`
    Ref        string `json:"ref,omitempty"`
    Visibility string `json:"visibility,omitempty"` // "public", "private", "unknown"
}
```

### Component 4: Registry Manifest Extension (`cli/internal/registry/registry.go`)

New field on `Manifest` struct:

```go
type Manifest struct {
    // ... existing fields ...
    Visibility string `yaml:"visibility,omitempty"` // "public", "private"
}
```

### Component 5: Loadout Item References (`cli/internal/loadout/create.go`)

Change item references from `[]string` to `[]ItemRef`:

```go
type ItemRef struct {
    Name string `yaml:"name"`
    ID   string `yaml:"id,omitempty"` // UUID from .syllago.yaml metadata
}
```

This prevents name-swap attacks where a private item is replaced with a same-named public one. At publish time, verify that referenced items still have the same IDs as when the loadout was created.

### Component 6: Gate Enforcement

Four gates at content exit points:

| Gate | Command | Check | Action |
|------|---------|-------|--------|
| G1 | `syllago publish --registry <name>` | Content taint + live probe of target registry | Block if private→public |
| G2 | `syllago share <name>` | Content taint + live probe of current repo | Block if private→public |
| G3 | `syllago loadout create` | Check items for private taint | Warn (not block) |
| G4 | `syllago publish --registry` (loadout) | Loadout contains private items + target is public | Block |

**`syllago export` is NOT gated** — exports go to local provider directories. Not a cross-boundary operation. However, a warning is printed if the exported content has private taint.

**Resolution logic:**
```
content is private IF:
  content.sourceVisibility == "private"
  OR sourceRegistry's current visibility == "private"
  (either layer triggers the gate)

target is public IF:
  target registry/repo visibility == "public"

private content + public target = BLOCKED
```

### Component 7: Laundering Defense

Prevents content from being "laundered" through a provider round-trip (import from private registry → install to provider → `add --from provider` → taint stripped).

**Two-layer defense in `syllago add --from <provider>`:**

1. **Symlink tracing (default case):** When discovering a file at a provider path, check if it's a symlink pointing into `~/.syllago/content/`. If so, follow the symlink to the original library item, read its `.syllago.yaml`, and propagate the taint to the new item.

2. **Hash-match fallback (copy-installed case):** If the file is not a symlink, compute its hash and compare against all library items with private taint (`sourceVisibility == "private"`). If a match is found, propagate the taint.

**Accepted limitation:** If content was modified after export/install, the hash won't match and the content enters the library without taint. This is acceptable because (a) modified content is arguably a derivative work, (b) this is an edge case of an edge case (copy-install + modification + re-add), and (c) this is a soft gate, not DRM.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Detection priority | API probe > registry.yaml > default | Prevents registry.yaml spoofing (Exploit #6) |
| Conflict resolution | Stricter value wins | private > unknown > public — fail-safe |
| Taint model | Both entry + exit (belt & suspenders) | Maximum protection against leakage |
| Override flag | None (for now) | Intentional friction; backlog bead syllago-lpxa |
| HMAC integrity hash | Dropped | Security theater — key is on same filesystem as data |
| Migration | None needed | Pre-release, no external users |
| Modified content | Accepted gap | Soft gate, not DRM; modified = derivative work |
| Platforms | GitHub + GitLab + Bitbucket + unknown | "All major platforms" per user decision |
| Cache TTL | 1 hour | Balance between freshness and API rate limits |
| Loadout references | ItemRef{Name, ID} | Prevents name-swap attacks (Exploit #10) |

## Data Flow

### Happy Path: Private Content Stays Private

```
1. syllago registry add https://github.com/acme/internal-rules
   → parse URL → detect GitHub → probe API → 404 → "private"
   → clone repo → save Registry{Visibility: "private"} to config

2. User browses registry in TUI (visibility badge shown)

3. User imports "acme-auth-policy" to library
   → .syllago.yaml written with:
     sourceRegistry: acme/internal-rules
     sourceVisibility: private

4. User tries: syllago publish acme-auth-policy --registry community/public-rules
   → Load item metadata: sourceVisibility = "private"
   → Live-probe target registry: community/public-rules = "public"
   → BLOCKED

   Error: Cannot publish "acme-auth-policy" to registry "community/public-rules"

     Content origin:  acme/internal-rules (private)
     Target registry: community/public-rules (public)

     Private content cannot be published to public registries.
     Remove the private taint by recreating the content in your
     library without the private registry association.
```

### Happy Path: Public Content Flows Freely

```
1. syllago registry add https://github.com/community/shared-rules
   → probe API → 200, private=false → "public"

2. User imports "community-linting-rule" to library
   → sourceVisibility: public

3. syllago publish community-linting-rule --registry another/public-registry
   → sourceVisibility = "public", target = "public"
   → ALLOWED
```

### Laundering Attempt (Blocked)

```
1. Import "acme-secret" from private registry (tainted)
2. syllago install acme-secret --to claude-code (symlinked)
3. syllago add --from claude-code
   → discovers file at ~/.claude/rules/acme-secret
   → file is symlink → follows to ~/.syllago/content/rules/acme-secret/
   → reads .syllago.yaml → sourceVisibility: private
   → propagates taint to new item
   → TAINT PRESERVED
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| API probe fails (network error) | Visibility = "unknown" = private (fail-safe) |
| API rate limited (429) | Visibility = "unknown" = private (fail-safe) |
| Registry URL not recognized | No probe, visibility = "unknown" = private |
| Content has no taint fields (legacy) | N/A — pre-release, no legacy content |
| Registry removed from config | Content taint persists (belt still works) |
| Registry visibility changes | Re-probed on sync and at publish time |
| Loadout item ID mismatch | Warn at publish: "Item X has changed since loadout was created" |

## Security Analysis

### Addressed Exploits (P0-P2)

| # | Exploit | Severity | Fix |
|---|---------|----------|-----|
| 11 | `publish --registry` has no check | Critical | G1 gate in `PromoteToRegistry()` |
| 1 | Provider round-trip laundering | Critical | Symlink tracing + hash-match |
| 3 | TUI import bypasses tainting | High | Set taint in `doImport()` |
| 4 | `share` command has no gate | High | G2 gate in `Promote()` |
| 13 | CLI `add` no registry awareness | High | Registry fields in `AddOptions` |
| 6 | registry.yaml spoofing | Medium | API overrides registry.yaml |
| 10 | Loadout name-swap | Medium | ItemRef with UUID |
| 5 | Export of private content | Medium | Warning before export |
| 7 | Config cache manipulation | Low | Live re-probe at publish time |
| 9 | Copy-install loses traceability | Medium | Hash-match fallback |

### Accepted Risks (P3)

| # | Risk | Why Accepted |
|---|------|-------------|
| 2 | Direct .syllago.yaml editing | Soft gate; filesystem access = game over anyway |
| 8 | Race condition (repo goes public) | Conservative-safe direction; taint from import time |
| 12 | Direct git operations in clone dirs | Outside syllago's control surface |
| — | Modified content after export | Edge case of edge case; derivative work argument |

## Success Criteria

1. Content from a private registry cannot be published to a public registry through any syllago command
2. Content from a private registry cannot be shared to a public team repo through `syllago share`
3. Loadouts containing private content warn at creation and block at publish-to-public
4. Registry visibility is detected automatically for GitHub, GitLab, and Bitbucket
5. Unknown hosts default to private (fail-safe)
6. Provider round-trip laundering is caught via symlink tracing and hash matching
7. All gates produce clear, actionable error messages

## Files to Create/Modify

| File | Change | New/Modified |
|------|--------|-------------|
| `cli/internal/registry/visibility.go` | Platform-aware visibility detector | **New** |
| `cli/internal/registry/visibility_test.go` | Tests for visibility detection | **New** |
| `cli/internal/config/config.go` | Add `Visibility` to `Registry` struct | Modified |
| `cli/internal/registry/registry.go` | Add `Visibility` to `Manifest`; probe on clone/sync | Modified |
| `cli/internal/metadata/metadata.go` | Add `SourceRegistry`, `SourceVisibility` to `Meta` | Modified |
| `cli/internal/add/add.go` | Registry fields in `AddOptions`; symlink tracing; hash-match | Modified |
| `cli/internal/tui/import.go` | Set taint fields in `doImport()` | Modified |
| `cli/internal/promote/promote.go` | G2 gate: check taint + repo visibility before share | Modified |
| `cli/internal/promote/registry_promote.go` | G1 gate: check taint + registry visibility before publish | Modified |
| `cli/internal/loadout/create.go` | G3: warn on private items; change to `ItemRef` | Modified |
| `cli/cmd/syllago/sync_and_export.go` | Warn on export of private-tainted content | Modified |

## Open Questions

None remaining — all questions resolved during design.

## Related Work

- **syllago-lpxa:** Explore override flag for private->public content sharing (P4 backlog)
- **syllago-78m4:** Audit trail / provenance tracking for content operations (P4 backlog)
- **Research doc:** `docs/research/2026-03-21-registry-privacy-research.md`

---

## Next Steps

Ready for implementation planning with the Plan skill.
