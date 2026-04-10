# Content Discovery Redesign - Design Document

**Goal:** Enable syllago to discover all AI content in any repository regardless of how it's organized, by splitting the scanner into a dumb manifest reader and an intelligent content analyzer.

**Decision Date:** 2026-03-31

---

## Problem Statement

When a user adds a repository as a registry (via git URL or local path), syllago only discovers content that follows its canonical directory layout (`skills/*/SKILL.md`, `hooks/<provider>/*/hook.json`, etc.). Real-world repos organize content in whatever format their AI tool uses natively -- each provider has its own conventions for where and how agents, hooks, rules, MCP configs, commands, and skills are stored. Claude Code uses `.claude/` with settings.json-wired hooks and flat agent files. Cursor uses `.cursorrules` and `.cursor/rules/`. Windsurf uses `.windsurfrules`. Copilot uses `.github/copilot-instructions.md` and `.copilot/`. And so on -- syllago already has converter adapters that understand many of these formats. But the scanner doesn't leverage that knowledge during discovery.

The deeper issue: the scanner is doing four jobs at once (discovery, classification, metadata extraction, layout interpretation), and its binary classification ("syllago-structured OR native provider repo") fails for hybrid repos -- which is what most community repos actually look like.

---

## Proposed Solution

**Any repo can be a registry.** The `registry.yaml` manifest declares what content exists and where -- the repo's internal organization is irrelevant. The scanner's only job is reading manifests and resolving paths.

When a user adds a repo that doesn't have an author-provided `registry.yaml`, syllago analyzes the repo and generates one. This analysis leverages provider-specific detection knowledge that already exists in the converter adapters -- each provider knows where its content lives and how it's formatted. The generated manifest is stored in syllago's cache (not in the repo), is inspectable as a plain YAML file, and can be corrected by the user.

The architecture has two pieces:

1. **A simplified scanner** that reads manifests and builds catalog entries. No classification logic, no provider awareness, no content inspection. If there's a manifest, read it. That's the job.

2. **A content analyzer** that examines an arbitrary repo and produces a manifest. This is where all the intelligence lives -- provider-specific detectors, file pattern matching, reference resolution, confidence scoring. It runs once at registry-add time. On sync, it diffs files at paths already in the manifest -- if any changed, it re-analyzes those items. Full re-analysis for discovering new content is available via `syllago registry rescan`.

The add-registry wizard offers three paths:
- **Auto-detect** -- syllago analyzes the repo and shows what it found
- **Guided** -- the user points syllago to content locations
- **Manual** -- the user specifies individual items (power-user escape hatch)

All three paths produce the same artifact: a manifest. Everything downstream consumes manifests uniformly.

---

## Architecture

### Component Overview

```
+--------------------------------------------------+
|                  Content Analyzer                 |
|  (runs at add-time and on rescan)                 |
|                                                   |
|  +----------+ +----------+ +----------+           |
|  | CC       | | Cursor   | | Windsurf |  ...      |
|  | Detector | | Detector | | Detector |           |
|  +----------+ +----------+ +----------+           |
|  +----------+ +----------+                        |
|  | Syllago  | | Top-level|                        |
|  | Detector | | Detector |                        |
|  +----------+ +----------+                        |
|                                                   |
|  Reference Resolver -> follows links, scripts     |
|  Conflict Resolver  -> dedupes cross-detector     |
|  Manifest Writer    -> produces registry.yaml     |
+-------------------------+-------------------------+
                          | manifest (YAML)
                          v
+--------------------------------------------------+
|                    Scanner                        |
|  (runs on every TUI launch)                       |
|                                                   |
|  Read manifest -> resolve paths -> build catalog  |
|  That's it. No classification. No file reading.   |
+-------------------------+-------------------------+
                          | []ContentItem
                          v
                     TUI / CLI
```

### The Detector Interface

Each provider contributes a detector. The interface follows the Syft/mise pattern -- each detector declares what file patterns it recognizes and can classify files it claims:

```go
type DetectionPattern struct {
    Glob        string      // e.g., ".claude/agents/*.md", ".cursorrules"
    ContentType ContentType // what this pattern indicates
    Confidence  float64     // base confidence for structural match alone
}

type DetectedItem struct {
    Name         string
    Type         ContentType
    Provider     string
    Path         string            // primary file or directory
    ContentHash  string            // SHA-256 of primary file content
    Confidence   float64           // 0.0-1.0
    Scripts      []string          // referenced script files
    References   []string          // other files needed
    Dependencies []DependencyRef   // other content items this needs
    HookEvent    string            // for hooks: event binding
    HookIndex    int               // for hooks: position in event array
    ConfigSource string            // where wiring was found (e.g., settings.json)
    DisplayName  string
    Description  string
}

type ContentDetector interface {
    // What file patterns does this detector recognize?
    Patterns() []DetectionPattern

    // Given a candidate file/dir, classify it and extract metadata.
    // Returns nil/empty if the detector can't classify this candidate
    // after inspecting its contents (pattern matched but content didn't).
    // Returns a slice because some files contain multiple items
    // (e.g., settings.json with 6 hook entries).
    Classify(path string, repoRoot string) ([]*DetectedItem, error)
}
```

Key design points:

- **Syllago canonical format is just another detector**, not a privileged path. It registers patterns like `skills/*/SKILL.md`, `agents/*/AGENT.md`, `hooks/*/*/hook.json`. Same interface, same confidence scoring. It just happens to have the highest base confidence because the structure is unambiguous.

- **`Classify` receives `repoRoot`** so detectors can reach beyond the candidate file. The CC detector needs this to read `.claude/settings.json` when classifying hook scripts. The syllago detector needs it to check for `.syllago.yaml` siblings.

- **Detectors return `nil` from `Classify` when a pattern matched but inspection shows it's not actually content.** A `.md` file in `.claude/agents/` that turns out to be a README, for instance.

### Detection Patterns by Detector

Based on empirical research across ~50 community repos (see `docs/research/2026-03-31-community-repo-content-patterns.md`):

**Syllago Canonical Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `skills/*/SKILL.md` | skill | 0.95 |
| `agents/*/AGENT.md` | agent | 0.95 |
| `hooks/*/*/hook.json` | hook | 0.95 |
| `mcp/*/config.json` | mcp | 0.95 |
| `rules/*/*/rule.md` | rule | 0.95 |
| `commands/*/*/command.md` | command | 0.95 |
| `loadouts/*/loadout.yaml` | loadout | 0.95 |

**Claude Code Detector:**

| Pattern | Content Type | Confidence | Notes |
|---------|-------------|------------|-------|
| `.claude/agents/*.md` | agent | 0.90 | |
| `.claude/skills/*/SKILL.md` | skill | 0.90 | |
| `.claude/commands/*.md` | command | 0.90 | |
| `.claude/rules/*.md` | rule | 0.90 | |
| `.claude/hooks/*` | hook-script | 0.70 | Needs settings.json correlation |
| `.claude/settings.json` | hook-wiring | special | Parse for hook entries |
| `.claude/output-styles/*.md` | output-style | 0.85 | |
| `.mcp.json` | mcp | 0.90 | |
| `CLAUDE.md` | rule | 0.80 | |

**Claude Code Plugin Detector:**

| Pattern | Content Type | Confidence | Notes |
|---------|-------------|------------|-------|
| `.claude-plugin/plugin.json` | plugin-manifest | special | Parse for content listing |
| `plugins/*/agents/*.md` | agent | 0.90 | |
| `plugins/*/skills/*/SKILL.md` | skill | 0.90 | |
| `plugins/*/hooks/hooks.json` | hook | 0.90 | |
| `plugins/*/commands/*.md` | command | 0.90 | |

**Top-Level (Provider-Agnostic) Detector:**

| Pattern | Content Type | Confidence | Notes |
|---------|-------------|------------|-------|
| `agents/*.md` | agent | 0.85 | |
| `agents/*/*.md` | agent | 0.80 | Categorized subdirs |
| `commands/*.md` | command | 0.85 | |
| `commands/*/*.md` | command | 0.80 | Categorized subdirs |
| `rules/*.md` | rule | 0.80 | |
| `rules/*.mdc` | rule | 0.80 | |
| `hooks/*.py` | hook-script | 0.60 | No wiring context |
| `hooks/*.js` | hook-script | 0.60 | No wiring context |
| `hooks/*.ts` | hook-script | 0.60 | No wiring context |
| `hooks/*.sh` | hook-script | 0.60 | No wiring context |
| `hooks/hooks.json` | hook-wiring | 0.85 | |
| `hook-scripts/*/*.js` | hook-script | 0.70 | Event-organized |
| `prompts/*.md` | prompt | 0.75 | |

**Cursor Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `.cursorrules` | rule | 0.95 |
| `.cursor/rules/*.mdc` | rule | 0.90 |
| `.cursor/rules/*.md` | rule | 0.85 |
| `.cursor/agents/*.md` | agent | 0.90 |
| `.cursor/skills/*/SKILL.md` | skill | 0.90 |
| `.cursor/commands/*.md` | command | 0.90 |
| `.cursor/hooks.json` | hook-wiring | 0.90 |
| `.cursor/hooks/*` | hook-script | 0.70 |

**Copilot Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `.github/copilot-instructions.md` | rule | 0.95 |
| `.github/instructions/*.instructions.md` | rule | 0.90 |
| `.github/agents/*.md` | agent | 0.90 |

**Windsurf Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `.windsurfrules` | rule | 0.95 |

**Cline Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `.clinerules` | rule | 0.95 |
| `.clinerules/*.md` | rule | 0.90 |

**Roo Code Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `.roo/rules/*.md` | rule | 0.90 |
| `.roomodes` | rule | 0.85 |

**Codex Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `AGENTS.md` | rule | 0.85 |
| `.codex/agents/*.toml` | agent | 0.85 |

**Gemini Detector:**

| Pattern | Content Type | Confidence |
|---------|-------------|------------|
| `GEMINI.md` | rule | 0.85 |
| `.gemini/skills/*/SKILL.md` | skill | 0.85 |

### Content Type Mapping

Detectors use internal classification labels that map to catalog ContentTypes:

| Detector Label | Catalog ContentType | Notes |
|---------------|--------------------|----|
| `skill` | `Skills` | |
| `agent` | `Agents` | |
| `rule` | `Rules` | |
| `command` | `Commands` | |
| `hook` | `Hooks` | Fully wired (event + script) |
| `hook-script` | `Hooks` | Script only, no wiring. Lower confidence. |
| `hook-wiring` | (special) | Not a content item itself. Parsed to produce `hook` items with wiring. |
| `mcp` | `MCP` | |
| `loadout` | `Loadouts` | |
| `output-style` | `Rules` | CC output styles map to rules for portability |
| `plugin-manifest` | (special) | Not a content item. Parsed to discover content declared within. |

`AGENTS.md` (Codex/Copilot convention) and `GEMINI.md` classification is **provider-dependent** and requires investigation during implementation. These files serve as project-level instructions (like `CLAUDE.md`) in some providers but may define agents in others. The detectors should inspect content structure to determine the correct type per-provider rather than assuming a fixed classification.

### Confidence Thresholds

Confidence scores are internal and never shown to users. Default thresholds for category partitioning:

| Category | Threshold | UI presentation |
|----------|-----------|-----------------|
| Auto-detected | > 0.80 | Shown as fact: "Skills (25) auto-detected" |
| Needs confirmation | 0.50 - 0.80 | Presented with context for user review |
| Skipped | < 0.50 | Not included. Visible via `--verbose` flag. |

**Override: executable content (hooks, MCP) always requires confirmation regardless of score.**

Thresholds are configurable in `~/.syllago/config.yaml`:

```yaml
analyzer:
  auto_threshold: 0.80
  skip_threshold: 0.50
```

### Manifest Schema

The existing `ManifestItem` struct is extended with optional fields for generated manifests. Author-provided manifests use only the base fields; generated manifests populate the additional fields. One schema, one code path.

```go
type ManifestItem struct {
    // Base fields (used by both authored and generated manifests)
    Name      string   `yaml:"name"`
    Type      string   `yaml:"type"`
    Provider  string   `yaml:"provider,omitempty"`
    Path      string   `yaml:"path"`
    HookEvent string   `yaml:"hookEvent,omitempty"`
    HookIndex int      `yaml:"hookIndex,omitempty"`
    Scripts   []string `yaml:"scripts,omitempty"`

    // Extended fields (populated by the analyzer, optional in authored manifests)
    DisplayName  string   `yaml:"displayName,omitempty"`
    Description  string   `yaml:"description,omitempty"`
    ContentHash  string   `yaml:"contentHash,omitempty"`
    References   []string `yaml:"references,omitempty"`
    ConfigSource string   `yaml:"configSource,omitempty"`
    Providers    []string `yaml:"providers,omitempty"`  // all detected providers (including the winner in Provider field)
}
```

**Hash storage:** Content hashes for re-analysis live in the manifest's `contentHash` field (generated manifests only). Install-time hashes for hook integrity checking live in `installed.json` (the local trust store). These are separate concerns: `contentHash` tracks source changes, install hashes track post-install mutation.

### Same-Name Deduplication

Items are uniquely identified by `(registry, type, name)`. A skill named "code-review" and a rule named "code-review" are different items.

**Same type, same name, same content hash:** Deduplicate. Keep the highest-confidence version. Record the other paths as `Providers` aliases.

**Same type, same name, different content hash:** These are genuinely different items. Surface the conflict in the wizard: "Found 2 skills named 'code-review' with different content." Show paths and content preview. User picks which to keep, or keeps both with disambiguated names (e.g., "code-review (from .claude/)" and "code-review (from plugins/)").

**Hook script suppression:** After all detectors run, any item whose `Path` appears in another item's `Scripts` list is suppressed. This prevents the CC detector's fully-wired hook (from settings.json correlation) and the Top-Level detector's orphan script (from `hooks/*.ts` glob) from appearing as duplicates. The wired version always wins.

### The Analysis Flow

**Step 0: Resolve repoRoot.** Before any detection, resolve `repoRoot` via `filepath.EvalSymlinks`. All detectors receive this resolved path. Reject paths that resolve to `/`, `/etc`, `/home`, or other sensitive roots. This is a security requirement -- detectors use `repoRoot` to read files beyond the candidate, so it must be a trusted, canonical path.

**Step 1: Walk the repo.** Collect all file paths (respecting depth/count limits). Build a flat path index. Cheap and provider-agnostic. **Exclusions are applied here**, before glob matching -- excluded directories (`node_modules/`, `vendor/`, `.git/`, etc.) are skipped via `filepath.SkipDir` during the walk. Their contents never enter the path index.

**Step 2: Pattern matching.** Every registered detector's `Patterns()` are evaluated against the path index. Pure glob matching -- no file reading. Produces a candidate list per detector.

**Step 3: Classify candidates.** For each candidate, the claiming detector's `Classify()` method runs. File content gets read -- frontmatter parsed, JSON inspected, settings.json correlated with hook scripts.

Hook correlation (the hardest case):
1. If a settings.json or hooks.json is found, parse it to extract hook entries (event, command, matcher)
2. For each hook entry, resolve the `command` field to a script path. Command fields are shell strings (e.g., `"bun hooks/lint.sh $FILE"`). Extract the script path by splitting on whitespace and resolving the first segment that looks like a file path (contains `/` or `\`, or ends in a known script extension). Inline commands without a script file (e.g., `"echo done"`) produce a hook item with no `Scripts` entry.
3. Match resolved scripts against hook-script candidates already in the candidate list
4. Produce one manifest entry per hook that bundles the event binding + the script
5. Hook scripts that can't be correlated with a wiring file stay at their base confidence (0.60-0.70) and get flagged for user review

Reference resolution (skills and agents):
1. For each skill/agent, scan markdown content for relative links `[text](../path)` and backtick paths
2. Resolve against the repo filesystem -- do the referenced files actually exist?
3. Add existing references to the manifest entry's `references` list
4. For skills with `references/`, `scripts/`, `helpers/`, `assets/` subdirectories, include the entire subtree

Cross-provider deduplication:
1. If the same content appears in `.claude/skills/go-patterns/` AND `.cursor/skills/go-patterns/` AND `skills/go-patterns/`, detect the duplication
2. Keep the highest-confidence version (syllago canonical > provider-specific)
3. Note the other providers in the manifest entry so syllago knows it can install to multiple providers

**Step 4: Conflict resolution and confidence partitioning.**

Confidence scores are internal to the analyzer -- never surfaced as numbers in the UI. Instead, items are partitioned into human-readable categories:

- **Auto-detected** (high confidence) -> included in manifest, shown as fact: "Found 25 skills"
- **Needs confirmation** (medium confidence) -> flagged for user review with context: "Found 2 files that look like hooks"
- **Skipped** (low confidence) -> not included, visible via `--verbose`

**Security invariant: executable content (hooks, MCP configs) always requires explicit user confirmation regardless of confidence.** Rules, skills, agents, commands, and prompts are inert markdown -- auto-include is safe. Hooks and MCP configs execute code -- the user must always review and confirm these.

When two detectors claim the same file:
1. Syllago canonical detector always wins (if someone put it in `skills/*/SKILL.md`, they meant it)
2. Otherwise, highest confidence wins
3. If tied, prefer the more specific detector (`.claude/agents/` over `agents/`)

**Step 5: Write manifest.** Generated manifest goes to `~/.syllago/registries/<name>/manifest.yaml`. Same schema as author-provided `registry.yaml`. Scanner doesn't know or care whether it was authored or generated.

### Re-Analysis Algorithm

On sync (git pull), the analyzer determines what changed using path + content hash comparison:

1. For each item in the existing manifest, compute SHA-256 of the file(s) at the manifest path
2. Compare against the hash stored in the manifest from the previous analysis
3. **Changed hash** -> re-classify that specific path, update manifest entry. User-edited fields (DisplayName, Description) are preserved by skipping overwrite of non-empty values that differ from what the analyzer would generate. If both path AND content hash changed, treat as a new item (clear all user metadata).
4. **Missing path** -> remove entry from manifest with a warning
5. **New files matching detector patterns** -> NOT auto-detected on sync (requires `syllago registry rescan` for full re-analysis)

This means sync is cheap (hash comparison only) and predictable (only re-analyzes what changed). New content discovery is an explicit user action.

### Exclusion List

The analyzer skips known non-content directories and files during the filesystem walk:

**Default exclusions:** `node_modules/`, `vendor/`, `.git/`, `dist/`, `build/`, `__pycache__/`, `.venv/`, binary files (detected by extension or null-byte sniffing).

**Note:** Exclusions only apply to the discovery walk (Step 1). The registry.yaml cross-validation pass (Decision #40) reads declared paths directly, bypassing the exclusion list. This ensures an author can't hide malicious content in an excluded directory and have it skip type verification.

**Configurable per-registry:**

```yaml
registries:
  my-monorepo:
    exclude:
      - "packages/legacy/"
      - "*.generated.md"
```

### Content Parsing Limits

All file reads during analysis are gated by size limits to prevent resource exhaustion from malicious or oversized repos:

| Limit | Default | Purpose |
|-------|---------|---------|
| Per-file (markdown) | 1 MB | Prevents memory exhaustion from huge markdown files |
| Per-file (JSON) | 256 KB | Settings files and hook configs are small |
| Per-repo total reads | 50 MB | Bounds total analysis cost |

Files exceeding limits are skipped with a warning.

### Registry.yaml Trust Model

Author-provided `registry.yaml` is trusted for **display metadata** (names, descriptions, tags, grouping) but is **cross-validated for security-relevant fields**:

- **Path validation:** Every path in the manifest must resolve to a real file within the repo boundary
- **Content type cross-check:** The analyzer independently classifies each declared item. If `registry.yaml` declares a file as a "rule" but the analyzer detects it as a hook script, the disagreement is surfaced to the user
- **Executable content:** Items declared as hooks or MCP configs in `registry.yaml` still require explicit user confirmation at install time -- the manifest cannot suppress the review prompt
- **Schema validation:** Unknown fields are rejected. The manifest schema is strict to prevent injection of unexpected keys

This means `registry.yaml` provides author intent (useful for display and organization) while the analyzer provides security verification (useful for trust).

### Hook Script Integrity

Hook scripts are hashed (SHA-256) at install time. On registry sync:

1. Compare current script hash against the hash stored at install time
2. If changed, surface the change to the user: "Hook script `enforce-wrappers.ts` was modified since you installed it. Review changes? [y/n]"
3. User must re-confirm before the updated script is active

This prevents post-install script mutation from bypassing the initial review.

### Reference Resolution Boundaries

When following markdown links and resolving referenced file paths:

- **Clone repos:** References must resolve within the clone boundary (same enforcement as symlinks)
- **Local repos:** References follow the symlink policy (ask/follow/skip)
- **Circular references:** Track visited paths, cap at 3 levels of reference following
- **Backtick path extraction:** Restricted to paths matching known content file patterns (`.md`, `.json`, `.yaml`, `.ts`, `.py`, `.sh`) within the same directory tree. Arbitrary backtick content is not treated as a path.

### Symlink Handling

The analyzer follows symlinks during the file walk but tracks both the symlink path and the resolved real path.

**Trust contexts:**

| Context | Symlink policy |
|---------|---------------|
| Local directory (user-provided path) | Configurable: `ask` (default), `follow`, or `skip` |
| Registry clone (git URL) | Enforce boundary -- symlinks must resolve within the clone |

**User confirmation:** When `symlink_policy: ask` (the default), the wizard surfaces symlinks pointing outside the directory before following them:

```
Detected 3 symlinks pointing outside this directory:
  skills/ -> /home/user/.config/pai/skills/
  hooks/lint.sh -> /home/user/shared-hooks/lint.sh
  .mcp.json -> /home/user/.config/mcp/default.json

  [f] Follow all symlinks    [r] Review each    [s] Skip symlinked content
```

**Configuration:**

```yaml
# Global default in ~/.syllago/config.yaml
symlink_policy: ask    # ask | follow | skip

# Per-registry override
registries:
  my-local-setup:
    symlink_policy: follow    # I trust this, don't ask
```

**Deduplication signal:** A directory full of symlinks pointing into another content directory (e.g., `.cursor/agents/` -> `agents/`) indicates the target is canonical and the symlink directory is an installation artifact. The analyzer prefers the canonical path as the item's location.

### Add-Registry Wizard UX

Three paths, presented as a funnel:

**Path A: Auto-detect (default).** Analyzer runs, user sees results, confirms or adjusts. Confidence scores are never shown -- items are presented as facts or questions.

```
Analyzed repository: github.com/aemccormick/ai-tools

Found 36 items:

  Skills (25)         auto-detected
  Agents (8)          auto-detected

  Items requiring confirmation:
    [x] hooks/enforce-wrappers.ts  ->  Hook: PreToolUse (validates shell wrappers)
    [x] hooks/mcp-research.ts      ->  Hook: PreToolUse (research workflow)
    [x] .mcp.json                  ->  MCP Server: Readability (npx)

  [space] Toggle    [enter] View contents    [right] Continue
```

The confirmation section uses a `checkboxList` component (consistent with the existing add wizard). Items are pre-selected. User toggles with Space, previews with Enter, advances with Right-arrow. This provides undo (toggle back) and eliminates cursor ambiguity.

Hooks and MCP configs always appear in the "confirm" section regardless of detection confidence -- this is a security invariant, not a UX choice. Zero-count types are omitted (no "Commands (0) none found" noise).

**Path B: Guided.** User specifies content roots. "My agents are in `agents/`, hooks in `hooks/`." Analyzer scans those locations specifically.

**Path C: Manual.** User declares individual items, types, paths. Syllago validates entries. Power-user escape hatch.

These aren't separate wizard modes -- they're a funnel. Start with auto-detection (A). Show results. User can accept, or drill into specific items and adjust (sliding toward B/C).

---

## Data Flow

```
User adds registry (URL or local path)
    |
    +- git clone (if URL)
    |
    v
Check for registry.yaml in repo
    |
    +- YES -> Scanner reads it directly -> Catalog
    |
    +- NO -> Content Analyzer runs:
    |       1. Walk filesystem, build path index
    |       2. Check symlinks -> present to user if policy=ask
    |       3. Pattern match (all detectors' globs vs path index)
    |       4. Classify candidates (read files, parse content)
    |       5. Correlate hooks (settings.json <-> scripts)
    |       6. Resolve references (markdown links, script paths)
    |       7. Deduplicate cross-provider items
    |       8. Partition by confidence
    |       9. Present results in wizard
    |      10. Write manifest to cache
    |           |
    |           v
    |       Scanner reads generated manifest -> Catalog
    |
    v
TUI / CLI displays catalog
```

---

## Key Decisions

| # | Decision | Choice | Reasoning |
|---|----------|--------|-----------|
| 1 | Scanner architecture | Split into "dumb scanner" + "content analyzer" | Scanner reads manifests only. Analyzer does the intelligent work at add/sync time. Clean separation of concerns. |
| 2 | Manifest as single source of truth | Yes -- scanner exclusively reads manifests | If no manifest exists (author-provided or generated), analyzer generates one before scanning. No parallel discovery paths. |
| 3 | No manifest at add time | Auto-generate via content analyzer | The analyzer runs automatically when a registry is added. No user action required. |
| 4 | Re-analysis trigger | Manifest-path diffing | On sync, diff files at paths already in the manifest. Re-analyze only changed items. Full re-scan via `syllago registry rescan`. |
| 5 | Provider support at launch | All providers with known patterns | CC, Cursor, Copilot, Windsurf, Cline, Roo Code, Codex, Gemini, plus top-level provider-agnostic and syllago canonical. Research showed clear patterns for all of these. |
| 6 | Confidence presentation | Human-readable categories, never numeric | "Auto-detected" / "Needs confirmation" / skipped. Scores are internal to analyzer only. |
| 7 | Analyzer implementation | New module, reuse converter adapter knowledge | `ScanNativeContent` has the right idea but wrong architecture (early-return bug, different output types). Build fresh with the detector interface, informed by existing provider knowledge. |
| 8 | Generated manifest storage | Syllago cache: `~/.syllago/registries/<name>/manifest.yaml` | Repo stays untouched. Same schema as author-provided `registry.yaml`. Scanner doesn't know the difference. |
| 9 | Generated vs authored manifest | Same file format, different location | Author-provided `registry.yaml` in repo always wins. Generated manifests live in cache. No confusion about which is which. |
| 10 | User edits to generated manifest | Preserved across re-analysis for items that still exist | Re-analysis diffs against existing manifest. Changed items updated, removed items flagged, user overrides kept. `registry regenerate` discards all edits. |
| 11 | Metadata extraction | Analyzer does it, manifest carries it | Display name, description, hook events, scripts, references all resolved at analysis time and stored in the manifest. Scanner just reads manifest fields. |
| 12 | Directory-walk fallback | Deprecated legacy path | Existing directory-walk scanner stays for backward compatibility during transition but is not the primary path. All new registries go through manifest generation. |
| 13 | Wizard UX | Three paths funneling to one artifact | Auto-detect (default) -> Guided -> Manual. All produce a manifest. User can accept auto-detection or drill in to adjust. |
| 14 | Manifest format versioning | Yes, include version field from start | Future-proofing. `version: 1` in generated manifests. Schema can evolve without breaking existing manifests. |
| 15 | Minimum viable generator | All providers with known patterns, CC deepest | CC gets full hook correlation + settings.json parsing. Other providers get path-based detection. Expand depth over time. |
| 16 | Symlink handling | Trust-context dependent with user confirmation | Local paths: follow with user confirmation in wizard (default `ask` policy). Registry clones: enforce boundary. Configurable per-registry via `symlink_policy`. |
| 17 | Reference resolution | Markdown links + backtick paths + known subdirs | Skills: include `references/`, `scripts/`, `helpers/`, `assets/` subtrees. Hooks: resolve `command` field to script paths. Agents/skills: parse markdown for relative links to real files. |
| 18 | Cross-provider deduplication | Keep highest-confidence, note aliases | Same content in `.claude/skills/` and `skills/` produces one manifest entry with the canonical path and provider aliases. |
| 19 | Syllago canonical detector priority | Always wins ties | If content matches syllago's canonical layout, that classification takes priority. The author deliberately organized it that way. |
| 20 | Plugin manifest support | Parse `.claude-plugin/plugin.json` | Growing adoption in the ecosystem. The plugin manifest is essentially an author-provided content declaration -- parse it like a lightweight `registry.yaml`. |
| 21 | Skipped items | Drop entirely, diagnostics via `--verbose` | Manifest is a declaration of content, not an analysis log. Persisting low-confidence items creates schema bloat and a security surface. |
| 22 | User metadata keying | Key by `(registry, type, item-name)` not path | Path-keyed metadata breaks on directory renames. Name-keyed metadata survives reorganizations. Type in key allows same name across different content types. |
| 23 | `syllago init` | Include in v1 | Serializes analyzer output to `registry.yaml`. Low cost, high ecosystem signal. Show diff + `--force` for overwrites. No interactive wizard. |
| 24 | Executable content confirmation | Always require explicit confirmation for hooks and MCP | Security invariant: confidence scores never bypass review for content types that execute code. |
| 25 | Registry.yaml trust model | Cross-validate, don't blindly trust | registry.yaml controls display metadata. Security-relevant fields (types, paths) are verified against analyzer. Disagreements surfaced to user. |
| 26 | Content parsing limits | Per-file (1MB md, 256KB JSON) and per-repo (50MB) size gates | Prevents resource exhaustion from malicious or oversized repos. |
| 27 | Hook script integrity | Hash at install, check on sync | Post-install script mutation surfaced for re-review. Prevents bypassing initial confirmation. |
| 28 | Exclusion list | Default excludes (node_modules, vendor, .git, etc.) + per-registry config | Reduces noise in large repos. Configurable for unusual layouts. |
| 29 | Re-analysis algorithm | Path + content hash (SHA-256) comparison | Changed hashes re-classify. Missing paths remove. New content requires explicit `rescan`. User edits preserved. |
| 30 | Classify return type | Returns slice, not single item | settings.json with 6 hooks produces 6 DetectedItems from one Classify call. |
| 31 | Reference resolution boundaries | Same enforcement as symlinks + depth cap | Prevents path traversal via markdown references. Backtick extraction restricted to known content patterns. |
| 32 | repoRoot resolution | Must be `filepath.EvalSymlinks`-resolved before passing to detectors | Security: detectors read files relative to repoRoot. Unresolved symlinks could expose the entire filesystem. |
| 33 | Same-name dedup | Content hash determines identity | Same type + same name + same hash = dedup. Different hash = surface conflict for user to resolve. |
| 34 | Hook script suppression | Post-classification pass suppresses consumed paths | Items whose Path appears in another item's Scripts list are suppressed. Wired hooks always win over orphan scripts. |
| 35 | Manifest schema | Extend ManifestItem with optional fields | One schema for both authored and generated manifests. Generated manifests populate extended fields (hash, references, display metadata). |
| 36 | Confidence thresholds | >0.80 auto, 0.50-0.80 confirm, <0.50 skip (configurable) | Concrete defaults with config override. Executable content ignores thresholds (always confirm). |
| 37 | Exclusion order | Walk-time, before glob matching | Excluded directories never enter the path index. Detectors never see them. |
| 38 | YAML serialization safety | `yaml.v3 Marshal` only, never templates | Prevents injection via user-controlled strings in generated registry.yaml. |
| 39 | AGENTS.md / GEMINI.md classification | Provider-dependent, investigate during implementation | These files may be rules or agents depending on provider. Detectors should inspect content structure. |
| 40 | registry.yaml cross-validation scope | Type verification, not full analysis | Lightweight check (~50-100 lines) that verifies declared content types against file content. Not the full detector pipeline. Bypasses exclusion list to prevent hiding content in excluded dirs. |
| 41 | Empty registry.yaml | Respect it (zero items = author says nothing here) | An existing registry.yaml with no items is an explicit author choice, not a stub. To re-detect, delete the file and re-run `syllago init`. |
| 42 | Nested registry.yaml | Ignore (only root manifest matters) | Subdirectories with their own registry.yaml are not treated as nested registries. Only the top-level manifest is read. |
| 43 | Detector ordering | All run independently, conflict resolution in Step 4 | No detector depends on another's output. Side effects (settings.json reads) are per-detector. Ordering doesn't affect results. |
| 44 | Content hash scope for dedup | Primary content file only, not directory/sidecar | Hash for dedup is computed on the main content file (SKILL.md, agent .md, hook .json). Sidecar files (.syllago.yaml) and supporting directories don't affect the hash. |
| 45 | Static content requirement | Registries must contain committed, static files | Generated/templated content (build artifacts) is not detected unless committed. Repos requiring build steps before content exists will produce "0 items found." |

---

## Error Handling

| Failure | Behavior |
|---------|----------|
| Analyzer finds zero items | "No AI content detected. Try `syllago add --guided` to point to content locations, or ask the repo author to add a registry.yaml." |
| Analyzer finds too many items (>100 with low avg confidence) | Auto-switch to guided mode: "Found many potential matches but results are noisy. Let's narrow the search." |
| Settings.json is malformed | Warning on affected hooks. Other content types unaffected. |
| Symlink resolves outside boundary (untrusted clone) | Skip with warning. |
| File exceeds size limit | Skip with warning: "Skipped large-file.md (exceeds 1MB limit)." |
| File read fails during classification | Skip item with warning. Don't fail the entire analysis. |
| Multiple detectors claim same file at same confidence | Prefer: syllago canonical > provider-specific > top-level agnostic. If still tied, include with note for user review. |
| Generated manifest becomes stale | Path + content hash comparison on sync. Changed hashes re-classify. Missing paths remove with warning. |
| registry.yaml type disagrees with analyzer | Surface to user: "registry.yaml declares X as a rule, but it looks like a hook script. Which is correct?" |
| Clone fails (private repo, bad URL, network) | Actionable error: "Could not clone: repository not found. Check the URL and ensure you have access." |
| Hook script modified after install | Surface change on sync: "Hook script was modified since install. Review changes before it becomes active." |
| Partial detection (user expected more) | Post-add hint: "Installed N items. If you expected more, run `syllago scan --verbose` to see what wasn't detected." |

---

## Success Criteria

1. Point syllago at `github.com/aemccormick/ai-tools` -> finds all 25 skills, 8 agents, 2 hooks (with event bindings), and 1 MCP server
2. Point syllago at a Cursor-only repo -> finds all `.cursorrules` and `.cursor/rules/*.mdc` files
3. Point syllago at a multi-provider repo -> deduplicates cross-provider content, shows one entry per item
4. Point syllago at a repo with `registry.yaml` -> skips analysis, uses manifest directly
5. Scanner launch time is unchanged (reads manifests, no file content inspection)
6. User can inspect and correct generated manifests via CLI or TUI

---

## Resolved Questions

Questions raised during design, resolved via persona panel review (4 personas, 3 rounds each).

### Q1: Should the generated manifest include skipped items?

**Resolution: No -- drop them entirely.**

The manifest is a declaration of content, not a log of analysis. Diagnostics belong in tooling output, not persisted artifacts.

- `syllago scan --verbose` prints all candidates with confidence scores to stderr (ephemeral, informational)
- Future `syllago scan --explain <path>` for targeted "why didn't it find X?" debugging
- Persisting low-confidence items was rejected on three grounds:
  - **Schema bloat** -- `hidden` or `confidence` fields can never be removed once added
  - **File clutter** -- separate `skipped-items.yaml` gets committed accidentally, creates confusion
  - **Security surface** -- persisted low-confidence items in `registry inspect` output could be used for social engineering in untrusted registries

### Q2: How do we handle repo reorganizations?

**Resolution: Re-key user metadata by `(registry, type, item-name)` instead of path.**

The panel identified that rename detection (content hashing, user prompts) solves a symptom. The real issue is path-keyed metadata being coupled to a third-party identifier. Switching the key to the item's `(type, name)` (from `.syllago.yaml` or frontmatter) makes directory renames invisible:

- Item moves from `agents/reviewer.md` to `.claude/agents/reviewer.md` -- same name and type, metadata persists automatically
- Item is actually renamed (new `name` field) -- treated as "old item removed, new item created," which is semantically correct
- A skill named "code-review" and a rule named "code-review" are different items (type is part of the key)
- No detection heuristics, no merge logic, no prompts. Less code, not more.

**Implementation:** In `installed.json` and user metadata storage, change keys from relative path to `(registry-url, type, item-name)`. Enforce name uniqueness per type within a registry during validation.

### Q3: Should `syllago init` generate a registry.yaml for repo authors?

**Resolution: Yes, include in v1.**

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Timing | v1 | Low cost (analyzer already exists), high ecosystem signal |
| Command name | `syllago init` | Universal developer muscle memory |
| Manifest richness | Whatever the analyzer already produces | No new schema or generation logic. `.syllago.yaml` per-item metadata comes along for free since the analyzer already reads it |
| Re-run behavior | Show diff + require `--force` to overwrite | Protects hand-tuned manifests without merge complexity |
| Scope boundary | No interactive wizard, no template selection, no merge logic | Run analyzer, serialize, write. Three steps. |
| Generated file header | `# Generated by syllago init -- edit freely, re-run with --force to regenerate` | Sets correct mental model: this is the author's file, not a lockfile |
| Serialization safety | Must use `yaml.v3 Marshal`, never string templates | Auto-quotes strings with YAML-special characters. Prevents injection via user-controlled content (frontmatter, filenames). |

---

## Design Review

### Round 1

Design reviewed by a 4-persona panel (skeptical technical, pragmatic implementer, security engineer, ecosystem/UX). Key changes incorporated:

**Security hardening:**
- Executable content (hooks, MCP) always requires explicit confirmation regardless of confidence (security invariant)
- registry.yaml cross-validated against analyzer -- can't spoof content types
- File size limits on all content parsing (1MB markdown, 256KB JSON, 50MB per-repo)
- Hook scripts hashed at install, changes surfaced on sync for re-review
- Reference resolution bounded by same rules as symlinks + depth cap
- Backtick path extraction restricted to known content file patterns

**Architecture refinements:**
- Classify returns `[]*DetectedItem` (slice) to handle settings.json with multiple hooks
- Re-analysis algorithm specified: path + content hash (SHA-256) comparison
- Default exclusion list (node_modules, vendor, .git, etc.) + per-registry config
- Confidence scores are purely internal -- UI shows "auto-detected" vs "needs confirmation"

**UX improvements:**
- Wizard shows what items ARE, not what the analyzer is uncertain about
- Hooks and MCP always in "confirm" section with descriptions and `[v] View contents`
- Zero-results error gives actionable next steps (guided mode, registry.yaml)
- High-noise detection auto-switches to guided mode
- Post-add hint for partial detection

**Deferred to future work (noted but not blocking):**
- `syllago init --preview` for author incentive (show how their repo looks to consumers)
- Conflict resolution when two registries provide same-named content
- Update notifications for changed registries
- `syllago search` for discovery across GitHub

### Round 2

Full re-review by 4-persona panel (architect, security, UX, implementer) after Round 1 changes. Verified all Round 1 fixes landed correctly. Additional changes:

**Architecture clarifications:**
- Manifest schema defined explicitly (extended ManifestItem with optional fields)
- Content type mapping table: detector-internal labels -> catalog ContentTypes
- Confidence thresholds specified with defaults (>0.80 / 0.50-0.80 / <0.50), configurable
- AGENTS.md/GEMINI.md classification flagged as provider-dependent, needs implementation investigation
- registry.yaml cross-validation clarified as lightweight type verification (~50-100 lines), not full analysis

**Security hardening (second pass):**
- repoRoot must be `filepath.EvalSymlinks`-resolved before passing to detectors
- Cross-type dedup prohibited (same name, different type = different items)
- YAML serialization for `syllago init` must use `yaml.v3 Marshal`, never templates
- On re-analysis, if both path AND content hash changed, treat as new item
- Install-time hashes live in `installed.json`, not the manifest (separate trust store)
- Cross-validation of registry.yaml must classify declared paths even if in excluded directories

**UX refinements:**
- Wizard confirmation uses `checkboxList` component (consistent with existing add wizard)
- Items pre-selected, toggle with Space, preview with Enter, advance with Right-arrow
- Zero-count types omitted from results display
- Same-name conflicts surfaced with content preview for user resolution

**Dedup model:**
- Items uniquely identified by `(registry, type, name)`
- Same type + same name + same content hash = dedup (keep highest confidence)
- Same type + same name + different hash = conflict (user resolves)
- Hook script suppression: items consumed by wired hooks are suppressed post-classification

### Round 3

Final review by 3-persona panel (completeness, devil's advocate, implementation readiness). Verified Round 1+2 fixes, stress-tested core assumptions, assessed planning readiness. Result: **ready for implementation planning.**

**Clarifications added:**
- Exclusion list bypass for cross-validation explained (discovery exclusions don't apply to registry.yaml type checks)
- Hook command field parsing specified (split on whitespace, resolve path-like segments)
- User-edited metadata preservation mechanism defined (skip overwrite of non-empty values)
- Provider vs Providers field semantics clarified
- Empty registry.yaml = author says nothing here (respected, not overridden)
- Nested registry.yaml = ignored (root manifest only)
- Detector ordering = independent, no cross-dependencies
- Content hash for dedup = primary file only, not sidecars
- Static content requirement = repos with generated content need build before indexing

**Confirmed assumptions (devil's advocate tried to break, couldn't):**
- "Any repo can be a registry" holds for all tested scenarios (monorepos, private repos, empty repos)
- "Scanner reads manifests only" holds -- missing paths handled gracefully
- Cache loss is recoverable (re-clone + re-analyze, no persistent state lost)

**Deferred to future (noted, not blocking):**
- Manifest staleness detection (`syllago validate` or `syllago init --check`)
- Re-generation story for existing registry.yaml (show diff before overwrite)

---

## Research

- Community repo content patterns: `docs/research/2026-03-31-community-repo-content-patterns.md` (~50 repos analyzed)
- Multi-format scanner architecture patterns (Trivy, Syft, KICS, Checkov, ScanCode, FOSSA, mise, IntelliJ): researched during brainstorm phase

---

## Next Steps

Ready for implementation planning with Plan skill.
