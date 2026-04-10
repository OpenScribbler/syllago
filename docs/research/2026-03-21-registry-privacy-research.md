# Registry Privacy: Research Findings

**Date:** 2026-03-21
**Goal:** Ensure content from private registries cannot be shared to public registries through syllago's tooling. Design a data leakage prevention gate.

## Design Decisions (from clarifying questions)

| Decision | Choice |
|----------|--------|
| How to detect private vs public | Both: registry config (primary) + Git remote visibility (fallback) |
| What operations to gate | All cross-boundary operations |
| Override flag | No override for now (backlog: syllago-lpxa) |
| Audit trail | Separate feature (backlog: syllago-78m4) |

---

## Current State: No Privacy Concept Exists

Syllago has **zero** privacy/visibility controls on registries or content. Registries are identified by name and URL only. Content items track source provider and hash, but not whether they came from a private or public registry.

---

## Registry Data Model (Current)

### Registry Struct (`cli/internal/config/config.go`)

```go
type Registry struct {
    Name string `json:"name"`
    URL  string `json:"url"`
    Ref  string `json:"ref,omitempty"` // branch/tag/commit
}
```

### Registry Manifest (`registry.yaml`)

```go
type Manifest struct {
    Name              string
    Description       string
    Maintainers       []string
    Version           string
    MinSyllagoVersion string
    Items             []ManifestItem // indexed content locations
}
```

Both structures are display-only. No visibility, access control, or privacy fields.

### Registry Storage

- **Config:** `.syllago/config.json` (project) and `~/.syllago/config.json` (global)
- **Clones:** `~/.syllago/registries/<name>/` (git clones)
- **Discovery:** `syllago registry add <url>` — teams share via project config

### Existing Access Controls

| Control | Description | Location |
|---------|-------------|----------|
| AllowedRegistries | URL-based allowlist (team/org policy) | `config.go` |
| Sandbox restrictions | Domain/env/port allowlisting for hooks/MCP | `sandbox/` |
| Content precedence | Project > Registry > Global library | `catalog/scanner.go` |
| Git auth delegation | SSH/token handling via system git | `registry/registry.go` |

---

## Content Origin Tracking (Current)

### Metadata System (`cli/internal/metadata/metadata.go`)

Every library item gets a `.syllago.yaml` with:

```yaml
sourceProvider: claude-code     # provider slug
sourceFormat: md                # original file extension
sourceType: provider            # "provider", "git", etc.
sourceHash: sha256...           # hash of source content
sourceURL: ""                   # not yet populated
hasSource: true                 # whether .source/ exists
addedAt: 2026-03-21T...
addedBy: syllago v0.1.0
```

### Content Item Catalog (`cli/internal/catalog/types.go`)

```go
type ContentItem struct {
    Provider string  // which provider (for provider-specific types)
    Registry string  // non-empty if from a registry
    Source   string  // "project", "global", "library", or registry name
    Library  bool    // in global content library
    Meta     *metadata.Meta
}
```

### Installed State (`~/.syllago/installed.json`)

Tracks installed hooks, MCP configs, and symlinks with `source` field (e.g., `"loadout:my-loadout"` or `"export"`). **Does not track registry origin.**

### Key Gaps

- **No `SourceRegistry` field** in metadata — content loses registry association after import
- **No visibility field** anywhere — no concept of private vs public
- **Registry URL not persisted** in installed state — only the locally-derived name
- **Loadout manifests** reference items by name only — no provenance chain

---

## Content Leakage Vectors

### Path 1: Export to Provider (`syllago export`)

**Risk: LOW**

- **Trigger:** `syllago export --to <provider> [--source local|shared|registry|builtin]`
- **Destination:** Local provider directories (`~/.cursor/rules/`, `~/.claude/rules/`)
- **Flow:** Read item -> canonicalize -> render to target format -> write locally
- **Leakage risk:** Content stays on user's machine. Not a registry-to-registry vector.
- **Gate needed:** No (local operation)

### Path 2: Publish to Shared Directory (`syllago publish <name>`)

**Risk: HIGH**

- **Trigger:** `syllago publish <name>`
- **Destination:** Creates branch in the current git repo, pushes, opens PR
- **Flow:** Validate -> create branch `syllago/promote/{type}/{name}` -> copy content -> git add/commit/push -> PR
- **Leakage risk:** If current repo is public and content came from a private registry, private content gets pushed to a public repo
- **Gate needed:** YES — check if content origin is private, check if target repo is public

### Path 3: Publish to Registry (`syllago publish <name> --registry <name>`)

**Risk: HIGHEST**

- **Trigger:** `syllago publish <name> --registry <registryName>`
- **Destination:** Copies content into registry clone dir, creates branch, pushes, opens PR
- **Flow:** Resolve registry clone -> determine destination path -> copy content -> git add/commit/push in registry clone -> PR
- **Leakage risk:** Direct content transfer from library (which may contain private registry content) to a potentially public registry
- **Gate needed:** YES — check content origin visibility vs target registry visibility

### Path 4: Loadout Creation (`syllago loadout create`)

**Risk: MEDIUM**

- **Trigger:** `syllago loadout create` (CLI or TUI wizard)
- **Destination:** `{contentRoot}/content/loadouts/{provider}/{name}/loadout.yaml`
- **Flow:** User selects items -> build manifest -> write YAML
- **Leakage risk:** Loadout references items by name. If the loadout lives in a public registry and references private content, the content will be bundled on install.
- **Gate needed:** YES — at creation time, warn if mixing private and public content; at publish time, block if target is public

### Path 5: TUI Import Wizard (Indirect)

**Risk: LOW-MEDIUM**

- **Trigger:** TUI import wizard with "Git URL" source
- **Destination:** Library at `~/.syllago/content/`
- **Leakage risk:** Not a direct leakage vector, but this is where private content enters the library and loses its registry association. If origin isn't tagged, downstream operations can't enforce gates.
- **Gate needed:** Tag origin visibility at import time

---

## Git Remote Visibility Detection

### Current Git Infrastructure

- **No go-git or GitHub API client** in dependencies — all git via `os/exec`
- **Only HTTP client:** `net/http` in `cli/internal/updater/updater.go` for GitHub releases API
- **URL parsing:** `registry.NameFromURL()` handles HTTPS and SSH formats
- **Auth model:** Fully delegated to system git (SSH keys, credential manager)

### Detection Approaches

| Method | Public repos | Private repos | Auth needed | Network needed |
|--------|-------------|---------------|-------------|----------------|
| GitHub REST API `GET /repos/{owner}/{repo}` | Returns `"private": false` | Returns `"private": true` (with auth) or 404 (without) | For private details | Yes |
| GitLab REST API `GET /projects/:id` | Returns `"visibility": "public"` | Returns visibility with auth | For private details | Yes |
| `git ls-remote` probe | Succeeds | Fails without auth | For private repos | Yes |
| `registry.yaml` declaration | Manual | Manual | No | No |

### Recommended Approach

1. **Primary:** `registry.yaml` declares `visibility: private|public` (registry owner sets this)
2. **Fallback:** On `registry add`, probe the GitHub/GitLab API to detect visibility
3. **Cache:** Store detected visibility in the `Registry` struct in config
4. **Unknown:** Default to "unknown" if detection fails — treat as private (safe default)

### GitHub API Detection (No Auth Needed for Public)

```
GET https://api.github.com/repos/{owner}/{repo}
```

Response includes `"private": true/false`. No auth token needed to check public repos. For private repos, returns 404 without auth (which itself signals "not public").

### Platform Support

| Platform | API endpoint | Visibility field |
|----------|-------------|-----------------|
| GitHub | `/repos/{owner}/{repo}` | `"private": bool` |
| GitLab | `/projects/{owner}%2F{repo}` | `"visibility": "public\|internal\|private"` |
| Bitbucket | `/repositories/{owner}/{repo}` | `"is_private": bool` |
| Self-hosted | Varies | Unknown — default to private |

---

## Architectural Recommendations

### 1. Extend Registry Struct

```go
type Registry struct {
    Name       string `json:"name"`
    URL        string `json:"url"`
    Ref        string `json:"ref,omitempty"`
    Visibility string `json:"visibility,omitempty"` // "public", "private", "unknown"
}
```

### 2. Extend Registry Manifest

```yaml
# registry.yaml
name: my-company-rules
description: Internal engineering standards
visibility: private            # NEW: declared by registry owner
```

### 3. Extend Content Metadata

```yaml
# .syllago.yaml (per-item metadata)
sourceProvider: claude-code
sourceRegistry: my-company/internal-rules   # NEW: which registry this came from
sourceVisibility: private                    # NEW: visibility at time of import
```

### 4. Gate Enforcement Points

| Operation | Check | Action |
|-----------|-------|--------|
| `publish <name>` | Is content from private registry? Is target repo public? | Block with error |
| `publish --registry <name>` | Is content from private registry? Is target registry public? | Block with error |
| `loadout create` | Does loadout mix private and public content? | Warn at creation |
| `loadout create` + `publish` | Is loadout in a public registry referencing private content? | Block at publish |
| `registry add` | Detect and cache visibility | Inform user |

### 5. Safe Defaults

- **Unknown visibility = private** — if we can't determine visibility, assume private (fail-safe)
- **No override flag** — users must manually recreate content to share it (intentional friction)
- **Soft gate** — CLI blocks the operation with a clear error message explaining why

### 6. Error Message Design

```
Error: Cannot publish "my-rule" to registry "public-community/rules"

  Content origin:  my-company/internal-rules (private)
  Target registry: public-community/rules (public)

  Private content cannot be published to public registries.
  If you want to share this content publicly, recreate it
  in your library without the private registry association.
```

---

## Key Files for Implementation

| Area | File | What to change |
|------|------|----------------|
| Registry struct | `cli/internal/config/config.go` | Add `Visibility` field |
| Registry manifest | `cli/internal/registry/registry.go` | Add `Visibility` to `Manifest` |
| Content metadata | `cli/internal/metadata/metadata.go` | Add `SourceRegistry`, `SourceVisibility` |
| Publish gate | `cli/internal/promote/promote.go` | Check visibility before publish |
| Registry publish gate | `cli/internal/promote/registry_promote.go` | Check visibility before registry publish |
| Loadout creation gate | `cli/internal/loadout/create.go` | Warn on mixed visibility |
| Visibility detection | `cli/internal/registry/visibility.go` (NEW) | GitHub/GitLab API probing |
| Add/import tagging | `cli/internal/add/add.go` | Tag content with registry visibility |
| Catalog scanner | `cli/internal/catalog/scanner.go` | Propagate visibility to ContentItem |

---

## Open Questions

1. **Transitive tagging:** If content is imported from a private registry, modified locally, then published — should the private taint persist? (Recommendation: yes, it persists until the user explicitly removes it by recreating the item.)

2. **Mixed loadouts:** Should a loadout be allowed to contain both private and public items, as long as it's only used locally? (Recommendation: yes, warn but allow for local use; block on publish to public.)

3. **Self-hosted Git:** How do we handle self-hosted Git servers (Gitea, Gogs, etc.) where we can't probe visibility? (Recommendation: default to private, require `registry.yaml` declaration.)

4. **Registry visibility changes:** If a previously-private repo goes public, should we update cached visibility? (Recommendation: re-probe on `registry sync`, update cache.)

5. **Content in project shared dir:** The `content/` directory within a project repo — should this be gated too? (Recommendation: the project repo's own visibility determines this, detected via git remote.)

---

## Related Beads

- **syllago-lpxa:** Explore override flag for private->public content sharing (P4 backlog)
- **syllago-78m4:** Audit trail / provenance tracking for content operations (P4 backlog)
