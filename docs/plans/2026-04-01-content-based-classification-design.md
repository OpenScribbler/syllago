# Content-Based Classification — Design Document

**Goal:** Close the detection gap for repos with non-standard AI content layouts by adding content-based classification as a fallback to pattern-based detection, plus user-directed discovery for explicit scanning.

**Decision Date:** 2026-04-01

---

## Problem Statement

The analyzer pipeline gates everything through `MatchPatterns()` — if a file doesn't match a hardcoded glob, it never reaches classification. Repos with custom directory structures (PAI's `Packs/pai-*-skill/`, BMAD's `src/bmm/agents/*.agent.yaml`) return zero detected items even when the content is obviously AI content.

Real-world testing showed 2 of 5 repos returning 0 items despite containing 20+ skills and 9+ agents respectively. The current architecture optimizes for precision (known provider patterns) over recall (finding all AI content).

## Proposed Solution

Three complementary changes:

1. **Content-Signal Detector** — A new fallback detector that inspects unmatched files for AI content signals using weighted scoring.
2. **User-Directed Discovery** — CLI flag + interactive fallback for explicit "scan this directory as type X" workflows.
3. **Quick-Win Patterns + README Exclusion** — Pattern additions to existing detectors and a global false-positive filter.

## Architecture

### Pipeline Change

The analyzer pipeline adds a fallback pass after pattern-based detection:

```
Walk → MatchPatterns (11 existing detectors) → Classify matched items
                                              ↓
                                        unmatched files
                                              ↓
                                  ContentSignalDetector.Classify
                                              ↓
                                      Merge all items
                                              ↓
                                  Dedup → Partition → Result
```

The content-signal detector only sees files that all 11 pattern-based detectors rejected. This avoids double-classifying files and prevents conflicts with existing high-confidence detections.

### Component 1: Content-Signal Detector

A new `ContentSignalDetector` that implements the `ContentDetector` interface but operates differently from existing detectors:

**Pre-filter** (applied before reading file content):
- **Extension filter:** Only inspect `.md`, `.yaml`, `.yml`, `.json`, `.toml` files
- **Directory-name filter:** Parent directory path must contain at least one keyword: `agent`, `skill`, `rule`, `hook`, `command`, `mcp`, `prompt`, `steering`, `pack`, `workflow`
- Files from user-directed scan paths bypass the directory-name filter

**Classification** uses weighted signal scoring. Signals are divided into two categories:

#### Static Fingerprint Signals (hardcoded, high weight)

These are content patterns that are near-unique to a specific content type:

| Signal | Type | Weight | Description |
|--------|------|--------|-------------|
| Filename is `SKILL.md` | Skills | +0.25 | Canonical skill marker |
| Filename is `AGENT.md` | Agents | +0.25 | Canonical agent marker |
| File matches `*.agent.yaml` or `*.agent.md` | Agents | +0.20 | Agent naming convention |
| Frontmatter has `allowed-tools` | Commands | +0.20 | Unique to commands |
| Frontmatter has `argument-hint` | Commands | +0.15 | Unique to commands |
| JSON has top-level `mcpServers` key | MCP | +0.30 | MCP server config fingerprint |
| JSON has `hooks` key with event-name subkeys | Hooks | +0.25 | Hook wiring fingerprint |
| Frontmatter has `alwaysApply` or `globs` | Rules | +0.15 | Rule-specific fields |

**Hook event-name detection:** The detector checks JSON object keys against the known hook event vocabulary across all providers: `PreToolUse`, `PostToolUse`, `SessionStart`, `SessionEnd`, `BeforeTool`, `AfterTool`, `pre_run_command`, `post_run_command`, `pre_read_code`, `pre_write_code`, `BeforeRun`, etc. If ≥2 keys match known event names, it's a hook wiring file.

#### Dynamic Supporting Signals (informational only — from converter registry)

The detector queries `FrontmatterFieldsFor(contentType, slug)` at init time to build a set of known frontmatter fields per content type. When a file's frontmatter contains fields that match a content type's registered fields, the matches are **recorded in the audit trail** but do **NOT** contribute to the confidence score. This prevents injection via malicious converter packages that register fields designed to inflate scores.

| Signal | Purpose | Scoring impact |
|--------|---------|---------------|
| Each frontmatter field matching a registered content-type field | Audit trail context | None (informational) |
| Directory name contains type keyword (`agent`, `skill`, etc.) | Scoring | +0.10 |
| File is in a subdirectory of a keyword directory | Scoring | +0.05 |

**Security constraint:** All classification-critical vocabulary (hook event names, fingerprint patterns) must be hardcoded in the binary, not loaded from converter packages or external registries at runtime. Converters contribute display metadata only.

#### Scoring and Confidence

- **Base score:** 0.40 (starting point for any file that passes pre-filter)
- **Final confidence:** base + sum of matching **static signal** weights, capped at 0.70
- **Minimum threshold:** Items scoring below 0.55 are dropped (skip bucket)
- **Result:** Content-signal items land in the **confirm bucket** (0.55–0.70), always requiring user approval
- **Rationale for 0.55 floor:** Files scoring exactly 0.50 (base 0.40 + one directory keyword 0.10) have zero static fingerprint matches — they're almost certainly not AI content. This eliminates false positives from `.github/workflows/` and similar non-AI directories that happen to match keyword filters.

#### Content Type Priority

When a file matches signals for multiple content types, the highest-scoring type wins. Ties are broken by specificity: Commands > Skills > Agents > Hooks > MCP > Rules (commands have the most unique fields, rules the fewest).

#### Hook Script Resolution

When the content-signal detector identifies a hook wiring file, it follows `command` fields in the hook entries to resolve referenced script files (reusing the existing `resolveHookScript()` logic). Referenced scripts are included as `Scripts` on the `DetectedItem`, matching the existing CC detector behavior.

**Security constraint:** All resolved script paths must be validated to be within the repo root. `resolveHookScript()` must reject paths that traverse outside the repository boundary (e.g., `../../.git/hooks/pre-commit`, `/etc/passwd`). This applies to both the existing CC detector and the new content-signal detector.

### Component 2: User-Directed Discovery

Two surfaces for user-directed scanning:

#### CLI Flag

```bash
syllago manifest generate --scan-as skills:Packs/ --scan-as agents:src/bmm/agents/
syllago add <repo> --scan-as skills:Packs/
```

- Multiple `--scan-as` flags allowed
- Format: `type:path` where type is a content type name and path is relative to repo root
- Paths bypass the content-signal pre-filter's directory-name check
- Type hint is used as the classification result (no signal scoring needed)

#### Interactive Fallback

When the analyzer finds zero or very few items in interactive mode (TUI wizard or interactive CLI):

1. Display the repo's top-level directory tree (filtered: skip Walk-excluded dirs like `node_modules`, `vendor`, `.git`)
2. Let the user select directories to scan
3. Ask what content type each selected directory contains
4. Run the content-signal detector on selected paths with the type hint

#### Confidence Boost

User-directed items receive a confidence boost since the user explicitly identified the directory:

| Source | Confidence Range | Bucket |
|--------|-----------------|--------|
| Content-signal (automatic) | 0.50–0.70 | Confirm |
| User-directed | 0.70–0.85 | Confirm or Auto (depending on signal strength) |

The boost is +0.20 added to whatever the content-signal score produces, capped at 0.85.

### Component 3: Quick-Win Pattern Additions

Added to `TopLevelDetector.Patterns()`:

| Pattern | Content Type | Confidence | Catches |
|---------|-------------|------------|---------|
| `agents/*/*/*.md` | Agents | 0.75 | Categorized agent subdirectories |
| `examples/agents/*.md` | Agents | 0.70 | Example directories |
| `examples/skills/*/SKILL.md` | Skills | 0.70 | Example directories |
| `examples/commands/*.md` | Commands | 0.70 | Example directories |

Lower confidence than direct-path patterns since `examples/` and nested dirs are less certain.

### Component 4: Global README Exclusion

A global exclusion list checked before any detector's `Classify()` is called:

**Excluded filenames:** `README.md`, `CHANGELOG.md`, `LICENSE.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`

**Implementation:** Check in `MatchPatterns()` — if a matched path's basename is in the exclusion list, skip it. Also checked in the content-signal detector's pre-filter.

**Exception:** `CLAUDE.md`, `GEMINI.md`, `AGENTS.md` are NOT excluded — these are legitimate content files.

### Component 5: External Content Sanitization Boundary

A single validation pass applied to all content read from external repos before it reaches display, audit logging, or file path resolution:

- **Strip C0/C1 control characters** (except newline, tab) from all string fields (`Name`, `Description`, script paths)
- **Enforce maximum display lengths** — truncate with `…` at display boundary (Name: 80 chars, Description: 200 chars, Path: 256 chars)
- **Applied at:** The `DetectedItem` construction boundary — before items enter dedup or partition

This is a systemic boundary, not per-feature patching. Every new feature that reads external content inherits the protection.

**Sanitization ordering:** The sequence must be: classify → sanitize → audit write → dedup → partition. Sanitization happens BEFORE audit writes, not after. Audit log structural fields (`file` path, `signals[].signal`) must pass through the same sanitization boundary as content fields — malicious filenames can inject into structured audit entries. This ordering prevents unsanitized values from ever reaching the audit log or downstream SIEM parsers.

### Component 6: --strict Mode for CI

A `--strict` flag (or `SYLLAGO_STRICT=1` env var) that enforces deterministic behavior:

- Content-signal fallback is **disabled** — only pattern-based detectors and explicit `--scan-as` paths
- `--scan-as` path-not-found is **fatal** (exit 1)
- 500-file candidate cap is **fatal** (exit 1)
- Interactive fallback is **disabled** (no prompts)

**Config-file behavior under --strict:** `--strict` honors `.syllago.yaml` `scan-as` entries that pass path containment validation (within repo root). To suppress config-file entries entirely, use `--strict --no-config`. Missing config-file `scan-as` paths are fatal under `--strict` (same as CLI flag paths).

This gives pipeline engineers a mode where classification is fully deterministic and no unspecified paths are scanned.

### Component 7: Scan-As Config Persistence

Interactive fallback selections are proposed as config entries and persisted to `.syllago.yaml` on user confirmation:

```yaml
# .syllago.yaml (project-level, version-controlled)
scan-as:
  - type: skills
    path: Packs/
  - type: agents
    path: src/bmm/agents/
```

- First scan is interactive discovery; subsequent scans are automatic
- Config-file entries are equivalent to `--scan-as` CLI flags
- CI loads config automatically — no flags needed if config is committed

**Layered-config precedence model:**

| Layer | Loaded | Can add paths | Can remove paths from higher layers |
|-------|--------|---------------|-------------------------------------|
| Org-config (`~/.syllago/org-config.yaml`) | First (baseline) | Yes | N/A (top layer) |
| Project-config (`.syllago.yaml`) | Second (additive) | Yes | No — cannot remove org-config entries |
| CLI flags (`--scan-as`) | Last (additive) | Yes | No — cannot remove org or project entries |

**Org-config governance features:**
- `locked: true` per entry — prevents project-config from registering a conflicting type on the same path
- `deny-scan-as` — list of path prefixes the scanner must never enter, regardless of project config or CLI flags (defense-in-depth for compromised repos)

```yaml
# ~/.syllago/org-config.yaml (IT-managed)
scan-as:
  - type: rules
    path: .governance/rules/
    locked: true
deny-scan-as:
  - .git/
  - .ssh/
  - credentials/
```

### Component 8: Audit Signal Traces

Content-signal classifications write full signal breakdowns to the audit log:

```json
{
  "action": "content-signal-classify",
  "file": "Packs/redteam/SKILL.md",
  "type": "skills",
  "confidence": 0.65,
  "signals": {
    "static": [
      {"signal": "filename_SKILL.md", "weight": 0.25},
      {"signal": "directory_keyword_pack", "weight": 0.10}
    ],
    "dynamic_informational": [
      {"field": "name", "provider": "claude-code", "matched": true},
      {"field": "description", "provider": "claude-code", "matched": true}
    ]
  },
  "source": "content-signal",
  "bucket": "confirm"
}
```

### Component 9: Confidence Tiers in Confirm UI

The confirm bucket displays differentiated confidence information:

| Score Range | Label | Visual |
|-------------|-------|--------|
| 0.55–0.60 | Low confidence | Yellow indicator |
| 0.60–0.70 | Medium confidence | Cyan indicator |
| 0.70–0.85 | High confidence | Green indicator |

**User-directed zero-signal label:** When a user-directed item has zero content signals (score = 0.40 base + 0.20 boost = 0.60), the confirm UI must label it **"User-asserted, no content signals"** — not "Medium confidence." The label must reflect signal source, not just numeric score. This distinction also appears in audit log entries.

**Rejection behavior:** Rejected confirm-bucket items are NOT persisted to `.syllago.yaml`. They re-surface on subsequent scans. This is deliberate — persisting rejections in `.syllago.yaml` would expand the config-file attack surface (a compromised repo could pre-populate rejections to suppress detection of malicious content).

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Pipeline position | Fallback only | Avoids conflicts with pattern-based detectors; unmatched files are the only gap |
| Pre-filter strategy | Extension + directory name + user paths | Balances coverage with performance; doesn't read every file in repo |
| Signal architecture | Static fingerprints only for scoring; dynamic converter fields informational | Panel review: dynamic fields in scoring = injection surface. Static fingerprints drive thresholds; dynamic fields appear in audit trail only |
| Content type scope | All 6 types (Skills, Agents, Commands, Hooks, MCP, Rules) | All types have discriminating signals; hooks via wiring JSON, not script content |
| Hook detection | Detect wiring JSON, follow script references | Hooks are embedded in settings files; scripts are indistinguishable from regular scripts |
| Confidence range | 0.50–0.70 (content-signal), 0.70–0.85 (user-directed) | Content-signal always confirms; user direction is a strong signal worth boosting |
| User-directed UX | CLI flag + interactive tree picker + config persistence | CLI flag for automation, interactive fallback for discovery, selections persist to config |
| README handling | Global exclusion list | Prevents false positives across all detectors, not just TopLevel |
| Design goal | Balanced, lean recall | Use confirm bucket as safety net; surface more candidates for user approval |
| Rules detection | Include with low confidence | Directory-name context + `alwaysApply`/`globs` frontmatter provide enough signal |
| Confirm UI | Show confidence tiers (numeric or labeled) | Undifferentiated confirm lists cause approval fatigue; users need signal quality context |
| CI mode | `--strict` flag suppresses content-signal, makes errors fatal | Pipelines need determinism; soft warnings break reproducibility |
| Trust boundary | Sanitize all external content before display/logging | Untrusted repo content can contain terminal escape sequences, control chars |
| Scan-as path errors | Fatal (exit 1), not warnings | Silent degradation in pipelines is worse than failure |
| Audit granularity | Full per-file signal traces | Compliance, security review, and policy audit require field-level breakdown |
| Threshold floor | 0.55 (not 0.50) | Base + one directory keyword (0.50) must NOT clear threshold — eliminates .github/workflows/ false positives |
| Sanitization ordering | Classify → sanitize → audit write | Audit log must never receive unsanitized values, even transiently |
| Rejection persistence | Not persisted (re-surfaces) | Persisting rejections to config expands attack surface; deliberate choice |
| Config precedence | Org baseline → project additive → CLI additive | Project cannot remove org entries; org supports locked + deny-scan-as |
| --strict + config | Honors config with path validation; --no-config suppresses | Prevents compromised config bypass while allowing CI config use |

## Data Flow

### Automatic Discovery (content-signal)

```
Unmatched files from Walk
    → Filter by extension (.md, .yaml, .json, .toml)
    → Filter by directory name keywords
    → For each file:
        → Check filename fingerprints (SKILL.md, *.agent.yaml, etc.)
        → Parse content:
            → JSON? Check for mcpServers key, hooks event keys
            → YAML frontmatter? Check for type-specific fields
            → Match frontmatter fields against converter registry
        → Score = base(0.40) + sum(signal weights), cap at 0.70
        → If score ≥ 0.50, emit DetectedItem
        → If hooks wiring, resolve script references
    → Merge with pattern-detected items
    → Dedup + Partition
```

### User-Directed Discovery

```
User provides --scan-as type:path
    → Bypass directory-name pre-filter for specified paths
    → Run content-signal detector with type hint
    → Apply +0.20 confidence boost (cap 0.85)
    → Merge with other detected items
```

### Interactive Fallback

```
Analyzer returns ≤ 5 items in interactive mode
    → Display filtered directory tree
    → User selects directories + assigns content types
    → Each selection becomes a --scan-as equivalent
    → Run content-signal detector with boost
    → Re-present results with new items included
```

## Error Handling

| Scenario | Handling |
|----------|----------|
| File read error during content inspection | Skip file, continue (same as existing detectors) |
| Invalid JSON/YAML | Skip file, no error propagated |
| `--scan-as` path doesn't exist | Fatal error (exit 1) — non-existent paths must not silently degrade |
| `--scan-as` invalid type name | Error with list of valid types |
| `--scan-as` conflicting type hints on same path | Fatal error — `--scan-as skills:X --scan-as agents:X` is rejected at parse time |
| Pre-filter produces too many candidates (>500) | Cap at 500, warn user (JSON-parseable in `--json` mode per I8), suggest `--scan-as`. Fatal in `--strict` mode |
| Config-file `scan-as` path not found | Fatal in `--strict` mode (same as CLI flag paths). Warning in normal mode |
| Config-file `scan-as` conflicts with org-config locked entry | Fatal error — project cannot override locked org entries |
| Content-signal + pattern detector both match same file | Can't happen — content-signal only sees unmatched files |

## Testing Strategy

- **Unit tests:** Signal scoring per content type with fixture files
- **Integration tests:** Full pipeline with repos containing non-standard layouts (PAI-like, BMAD-like structures)
- **False positive tests:** Ensure regular docs, config files, READMEs are not classified as AI content
- **Regression tests:** Existing pattern-detected items must not change scores or classification
- **User-directed tests:** `--scan-as` flag parsing, confidence boost, interactive fallback flow
- **Security tests:** Path traversal in `resolveHookScript()` with adversarial inputs (`../../`, absolute paths)
- **Sanitization tests:** Control characters, terminal escapes, and oversized strings in ALL DetectedItem string fields (not just frontmatter)
- **Symlink tests:** Symlinks inside repo pointing outside must be resolved and rejected by deny-scan-as
- **Strict mode tests:** Content-signal disabled, fatal errors on path-not-found and cap breach
- **Audit tests:** Signal trace output contains per-field breakdown with correct weights
- **Config persistence tests:** Interactive selections saved to `.syllago.yaml`, loaded on repeat scan

## Success Criteria

1. PAI repo (`Packs/pai-*-skill/`) detects ≥15 skills (currently: 0)
2. BMAD repo (`src/bmm/agents/*.agent.yaml`) detects ≥5 agents (currently: 0)
3. Zero new false positives on repos that currently work (ai-tools, kitchen-sink, phyllotaxis)
4. All content-signal items land in confirm bucket (never auto-include)
5. User-directed `--scan-as` produces items with elevated confidence
6. README.md no longer classified as agent/command/etc.
7. `resolveHookScript()` rejects paths outside repo root
8. All external content sanitized before display/logging (no control chars)
9. `--strict` mode produces deterministic output with no fallback scanning
10. Audit log contains full signal traces for content-signal classifications
11. Confirm UI shows confidence tiers, not undifferentiated list

## Panel Review Findings (2026-04-01)

A four-persona panel review (individual developer, security engineer, DevOps engineer, IT administrator) identified issues across security, automation, governance, and UX. Five rounds of cross-panel discussion produced consensus recommendations.

### Blocking Requirements (incorporated into design above)

| # | Requirement | Origin |
|---|-------------|--------|
| B1 | Path traversal prevention in `resolveHookScript()` — validate paths within repo root | Security |
| B2 | External content sanitization boundary — strip control chars, enforce display lengths on all untrusted content before display/logging | Security + IT |
| B3 | Dynamic converter signals informational-only — excluded from threshold scoring | Developer + Security |
| B4 | Classification vocabulary hardcoded in binary — hook event names, fingerprint patterns not loaded from converters | Security |

### Important Requirements (included in implementation plan scope)

| # | Requirement | Origin |
|---|-------------|--------|
| I1 | Confidence tiers in confirm UI — numeric score or labeled tiers, not undifferentiated list | Developer |
| I2 | Interactive fallback selections persist to `.syllago.yaml` config — repeat scans automatic | Developer + IT |
| I3 | Org-config support for `--scan-as` mappings — version-controlled, team-shared | IT + DevOps |
| I4 | `--strict` mode for CI — content-signal disabled, explicit paths required, cap breach fatal | DevOps |
| I5 | Full signal traces in audit log — per-file breakdown of which signals fired and weights | IT + DevOps |
| I6 | Interactive fallback threshold raised from ≤2 to ≤5 | Developer |
| I7 | Conflicting `--scan-as` type hints on same path = hard error | DevOps |
| I8 | JSON output for 500-file cap warning (must respect `--json` mode) | DevOps (promoted from N5) |
| I9 | `--debug-skips` flag — shows per-file disposition across four cases | DevOps (promoted from follow-up in third panel) |
| I10 | Structured four-case failure JSON for exit-code-1 errors (`pre_filter_excluded`, `below_threshold`, `locked_conflict`, `walk_skipped`) | IT + DevOps |

### Nice-to-Have (deferred to roadmap beads)

| # | Requirement | Origin |
|---|-------------|--------|
| N1 | Cap dynamic signal contribution in audit display | Security |
| N2 | Converter field manifest for admin review | IT |
| N3 | `--dry-run` mode for manifest generate | IT |
| N4 | User identity in confirm audit log entries | IT |

### Second Panel Review (2026-04-01)

Re-reviewed the updated design with the same four personas. Most first-panel issues were fully resolved. Residual and new findings:

**Design changes applied:**
- D1: Threshold floor raised from 0.50 to 0.55 (eliminates `.github/workflows/` false positives)
- D2: Sanitization ordering made explicit (classify → sanitize → audit write)
- D3: User-directed zero-signal label ("User-asserted, no content signals")
- D4: Rejection non-persistence documented explicitly
- D5: `--strict` + config behavior specified (`--no-config` to suppress)
- D6: Layered-config precedence model with `locked` and `deny-scan-as`
- D7: JSON cap warning promoted from N5 to I8
- D8: Open questions updated

**Implementation notes for plan:**
- `resolveHookScript` path containment = day-one task (B1 still unimplemented in code)
- Correction workflow = `syllago manifest edit --remove <path>` (CLI path for wrong approvals)
- Audit log retention = operator note, not v1 scope
- Test coverage for audit field sanitization (malicious filenames in structured fields)
- Symlink resolution: `filepath.EvalSymlinks` per-file before deny-scan-as check during walk
- Exhaustive sanitization field list: enumerate every `DetectedItem` string field by name in plan, not representative sample
- Org-config deployment out of scope for v1 — operators provision via existing MDM/dotfiles toolchain
- User/org config boundary: `locked` mappings = org-config, `ignore` entries = user-config (`.syllago.yaml`). Cannot conflict by design

**Resolved/accepted:**
- `workflow` keyword stays in pre-filter (threshold handles false positives)
- Rejected items re-surface by design (safe over persistent)
- Audit retention out of scope for v1

## Open Questions

All design questions resolved through gap analysis and two panel reviews. The following decisions were made during panel review and are documented above:

1. **Threshold floor** — 0.55, not 0.50. Resolved in second panel (D1).
2. **Sanitization ordering** — classify → sanitize → audit write. Resolved in second panel (D2).
3. **Config precedence** — org baseline → project additive → CLI additive. Resolved in second panel (D6).
4. **--strict config behavior** — honors config with path validation; `--no-config` suppresses. Resolved in second panel (D5).

---

## Next Steps

Ready for implementation planning with the Plan skill.
