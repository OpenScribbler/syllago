# Content Type Distribution Design

**Date:** 2026-04-06
**Status:** Brainstorm — ideas being solidified before planning
**Scope:** Skills, hooks, MCP configs, and rules — distribution, provenance, and injection

---

## Purpose

This doc captures open design problems for how syllago distributes and installs each content type, with provenance attached. We've aligned on the **Agent Ecosystem frontmatter approach** for skills (provenance fields embedded in SKILL.md frontmatter, not a separate sidecar file). This doc extends that decision to hooks, MCP configs, and rules, and records what still needs to be resolved.

Prior provenance research (sidecar model, two-hash design, Sigstore signing) is archived at `docs/spec/skills/archive/provenance.md` and `docs/spec/acp/` for reference — good ideas in there even though the sidecar model is off the table.

Each section records the design problem, options, and candidate direction. These become implementation plans once the direction is settled.

---

## 1. Skills — Frontmatter Provenance

### Decision: Frontmatter, Not Sidecar

Provenance for skills lives **in SKILL.md frontmatter**, aligned with the Agent Ecosystem spec (`docs/spec/skills/archive/skill-frontmatter-spec-agent-ecosystem.md`). No separate `meta.yaml` file.

This means one file travels with the skill. When someone copy-pastes or shares a skill, the provenance is already there. The sidecar model required two files to stay together — a coordination problem the frontmatter approach eliminates.

### Current SKILL.md Frontmatter (Minimal)

```yaml
---
name: code-review
description: Reviews code for quality issues.
---
```

### Target: Agent Ecosystem-Aligned Frontmatter

Based on the Agent Ecosystem spec, the full frontmatter structure:

```yaml
---
metadata:
  skill_metadata_schema_version: 1.0.0
  skill_metadata_schema_definition: https://example.com/schema/v1_0_0

  provenance:
    version: 2.0.0
    source_repo: https://github.com/alice/my-skills
    source_repo_subdirectory: /skills/code-review   # only if multi-skill repo
    authors:
      - "Alice Smith <alice@example.com>"
    license_name: MIT
    license_url: https://github.com/alice/my-skills/blob/main/LICENSE

  expectations:
    software:
      - name: "ripgrep"
        version: ">=14.1.0"
    services:
      - name: "GitHub"
        url: "https://github.com"
    programming_environments:
      - name: "node"
        version: ">=20"
    operating_systems: []   # omit or empty = works on all

  supported_agent_platforms:
    - name: "Claude Code"
      integrations:
        - identifier: "github"
          required: true
---
```

### Open Questions

1. **Semver vs integer versions.** Agent Ecosystem uses semver (`2.0.0`). Prior provenance research chose integers ("what does a breaking change mean for a markdown file?"). Semver is what people expect from a package manager. Candidate: adopt semver, align with Agent Ecosystem.

2. **`metadata` wrapper key vs flat frontmatter.** Agent Ecosystem puts everything under `metadata:`. This is clean for namespacing but verbose. Existing SKILL.md files use flat frontmatter (`name`, `description` at top level). Do we wrap in `metadata:` or keep a mix of flat + provenance section? Candidate: wrap all syllago-managed fields under `metadata:`, keep the flat fields (`name`, `description`) as they are for the LLM-readable portion.

3. **Schema definition URL.** The `skill_metadata_schema_definition` URL needs to point somewhere real. Syllago would need to publish a schema. Where does that live? Candidate: defer this field until the spec is stable enough to publish.

4. **`expectations` enforcement.** Does syllago check `expectations.software` at install time? Or is this advisory metadata for the user to read? Candidate: advisory in v0.1 (surface in TUI/output during install), enforce optionally with `--strict`.

5. **`supported_agent_platforms` and compatibility checking.** If a skill declares `supported_agent_platforms: [{name: "Claude Code"}]`, does `syllago install skill foo --provider cursor` warn or block? Candidate: warn by default.

---

## 2. Hooks — Provenance + Dependency Declarations

### Current State

Hooks in syllago's library are stored as:
```
content/hooks/claude-code/before-tool-execute-shell/
  hook.json
```

`hook.json` is pure JSON — the canonical hook format from `docs/spec/hooks/hooks.md`. No metadata layer exists today.

### The Gap

Hooks need two things skills already have (or will have):
1. **Provenance** — who made this, where it came from, what version
2. **Dependency declarations** — which skills this hook requires to make sense

A hook written specifically to work with a skill (e.g., validates a skill's output) currently has no way to express that relationship. If someone installs the hook without the skill, it breaks silently.

### Idea: Hook Manifest Wrapper (YAML)

Wrap hooks in a YAML manifest that includes provenance inline (same approach as skills — no separate sidecar), dependency declarations, and the hook payload either inline or as a file reference.

**Draft structure:**

```yaml
# hook-manifest.yaml
metadata:
  provenance:
    version: 1.0.0
    source_repo: https://github.com/alice/hooks
    authors:
      - "Alice Smith <alice@example.com>"
    license_name: MIT
    license_url: https://github.com/alice/hooks/blob/main/LICENSE

name: validate-brainstorm-output
type: hook
description: Validates that brainstorm output follows the expected structure.

requires:
  skills:
    - brainstorm         # install warns if this skill is missing
  providers:
    - claude-code        # which providers this hook is compatible with

recommends:
  skills:
    - develop            # install suggests this, but doesn't block

# Inline hook definition (canonical format from docs/spec/hooks/hooks.md)
hook:
  events:
    - after_tool_execute
  matchers:
    - tool: file_write
  handler:
    type: command
    command: bun run ./validate-brainstorm.ts
```

### Open Questions

1. **Inline vs file reference?** Inline hook payload means one file to share. File reference (`hook_file: hook.json`) preserves existing `hook.json` files in the library. Candidate: support both; inline is preferred for distribution, file reference for library storage.

2. **Library storage vs distribution format.** The canonical library could still store `hook.json` internally, and `hook-manifest.yaml` is only the distribution/export format. This avoids migrating existing content. Candidate: keep `hook.json` in library, generate manifest on export.

3. **Provenance structure alignment.** Should hook manifests use the same `metadata.provenance` structure as skills? Candidate: yes — one provenance schema for all content types.

### Dependency Install Behavior

When a hook declares `requires.skills: [brainstorm]`:
- `syllago install hook validate-brainstorm` checks if `brainstorm` is installed
- If missing: **warn** (not block, by default)
- `--strict` flag: block if any `requires` dependency is missing
- `recommends` dependencies: suggestion only in install output

---

## 3. MCP Configs — Provenance + Injection

### Current State

MCP configs are stored similarly to hooks: JSON files that merge into a provider's settings. The distribution problem is nearly identical to hooks.

### Differences from Hooks

- MCP configs are server definitions (one JSON blob per server), not scripts
- Dependency-on-skills concept doesn't apply in the same way
- MCP configs can have provider-specific formats (Claude Code's `mcpServers` structure differs from other providers)

### Candidate Direction

Same YAML manifest wrapper as hooks, with inline provenance:

```yaml
# mcp-manifest.yaml
metadata:
  provenance:
    version: 1.0.0
    source_repo: https://github.com/alice/mcp-configs
    authors:
      - "Alice Smith <alice@example.com>"
    license_name: MIT
    license_url: https://github.com/alice/mcp-configs/blob/main/LICENSE

name: github-mcp
type: mcp
description: GitHub API access via MCP server.

requires:
  providers:
    - claude-code
    - cursor

config:
  server:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"
```

---

## 4. Rules — The Hard Problem

Rules are the most under-designed content type. Three distinct problems stack on each other.

### Problem 1: What Is a Rule Unit?

Current syllago structure:
```
content/rules/claude-code/CLAUDE/
  rule.md              ← the entire content of one CLAUDE.md file
```

Rules can be either:
- **File rules**: A full `.claude/rules/go-patterns.md` — one file, one topic, file-level granularity
- **Monolith fragments**: Sections pulled from CLAUDE.md or similar catch-all files — no natural file-level boundaries

A user's CLAUDE.md may contain 30 different rules mixed together. They might want to share just the "don't add error handling for impossible cases" section, not the whole file. That section has no identity as a standalone unit.

**Candidate approaches:**

A. **File rules only** — syllago only deals in file-level rules (`.claude/rules/*.md` style). Monolith rule files are explicitly out of scope. Users who want shareable rules must refactor their CLAUDE.md into separate rule files.

B. **Snippet rules** — introduce a snippet content type. A rule has a `name`, `content`, and `scope`. Stored as structured data, not raw Markdown.

C. **File rules with extraction tooling** — syllago operates on file rules, but provides a `syllago rules extract` command that scans CLAUDE.md and suggests splitting it into separate files. Users migrate on their own terms.

**Candidate direction:** Option A with Option C as a helper. File-level granularity is the right distribution unit. Extraction is a quality-of-life tool, not core to the spec.

### Problem 2: Injection Without Overwriting

Installing a rule is fundamentally different from installing a skill. A skill goes to a new directory. A rule might need to go:

- Into an existing config file (CLAUDE.md) as appended content
- Into a new per-rule file (`.claude/rules/my-rule.md`) — already clean
- Into a provider's equivalent location that may not support rule files at all

**Monolith injection problem**: if a provider only supports a single rule file, syllago must append/insert into it rather than create a new file.

**Candidate injection models:**

A. **Append model**: `syllago install rule foo` appends content to the target file with a `<!-- syllago: foo -->` marker. Uninstall finds and removes the marked section.

B. **Include model**: Target file loads separate rule files via provider-specific include syntax. Syllago creates the rule file and adds the include line.

C. **Overlay model**: Syllago maintains its own rules directory; patches the provider config to load from it. User's native config is never modified directly.

D. **File-first model**: Only install rules that can go into a dedicated file (`.claude/rules/*.md`). Monolith providers get a "not supported" warning.

**Candidate direction:** Option D short-term, Option A as fallback for providers that need it.

### Problem 3: Provider-Specific Rule Formats

Rules differ substantially across providers:
- **Claude Code**: `.claude/rules/*.md` (Markdown, arbitrary content) or CLAUDE.md
- **Cursor**: `.cursor/rules/*.mdc` (MDC format — `alwaysApply`, `globs`, `description` frontmatter)
- **Windsurf**: `.windsurf/rules/*.md` (similar to Claude Code but different path)
- Others: unknown or not yet mapped

### The Hub-and-Spoke Question for Rules

Does syllago define a **canonical rule format** (like it does for hooks) and convert to/from provider formats? Or does it treat each provider's format as native?

**Option A: Canonical rule format** — convert on install/export. Consistent with hub-and-spoke architecture.

**Option B: Native format, provider-aware storage** — no conversion. Limits cross-provider portability.

**Candidate direction:** Option A long-term. But canonical rule format needs its own spec before implementation.

### Proposed Canonical Rule Format (Draft Sketch)

Provenance lives in frontmatter, same model as skills:

```yaml
---
metadata:
  provenance:
    version: 1.0.0
    source_repo: https://github.com/alice/rules
    authors:
      - "Alice Smith <alice@example.com>"
    license_name: MIT
    license_url: https://github.com/alice/rules/blob/main/LICENSE

name: no-impossible-error-handling
type: rule
description: Don't add error handling for conditions that cannot occur.
scope: global         # global | project | local | always
applies_to: "**"      # glob — what files this rule targets (empty = all)
---

Don't add error handling, fallbacks, or validation for scenarios that
can't happen. Trust internal code and framework guarantees. Only validate
at system boundaries (user input, external APIs).
```

Provider-specific mappings:
- `scope: global` → Claude Code global CLAUDE.md or `~/.claude/rules/`
- `scope: project` → project `.claude/rules/`
- `scope: always` → Cursor MDC `alwaysApply: true`
- `applies_to` → Cursor MDC `globs` field

Content stays as Markdown below the frontmatter. Providers that can't read the frontmatter just see the rule text.

---

## 5. Cross-Cutting: Dependency Graph

Once hooks can declare dependencies on skills, and loadouts bundle multiple content types, syllago needs a **dependency resolution model**:

- Install hook `validate-brainstorm` → requires skill `brainstorm`
- Install loadout `ai-writing-setup` → includes skill `brainstorm` + hook `validate-brainstorm`
- If skill is already installed: skip, don't duplicate
- If skill is a different version: warn, ask user to resolve

### Open Questions

- Does syllago do version pinning for dependencies? (e.g., `requires skill brainstorm >= 2.0.0`)
- What happens when two installed hooks require different versions of the same skill?
- Does syllago have a "lockfile" equivalent?

**Candidate direction:** Advisory only for v0.1 — warn on missing dependencies, no version pinning. Full dependency resolution (conflict detection, lockfile) is a later feature.

---

## 6. Summary of Open Decisions

| # | Content Type | Decision Needed | Options | Candidate |
|---|---|---|---|---|
| S1 | Skills | Semver vs integer versions | Semver / Integer | Semver (Agent Ecosystem alignment) |
| S2 | Skills | `metadata` wrapper vs flat frontmatter | Wrap all / Mixed | Wrap syllago fields under `metadata:` |
| S3 | Skills | Schema definition URL | Publish now / Defer | Defer until spec is stable |
| S4 | Skills | `expectations` enforcement | Advisory / Strict | Advisory in v0.1 |
| S5 | Skills | Platform compatibility enforcement | Warn / Block | Warn by default |
| R1 | Rules | What is a "rule unit"? | File-only / Snippet / Extraction tool | File-only + extraction tool |
| R2 | Rules | Injection model | Append / Include / Overlay / File-first | File-first + append fallback |
| R3 | Rules | Canonical format? | Yes (canonical YAML) / No (native per provider) | Yes — needs spec |
| H1 | Hooks | Distribution format | hook.json / hook-manifest.yaml | Manifest wrapper |
| H2 | Hooks | Dependency enforcement | Advisory / Strict | Advisory default, `--strict` opt-in |
| H3 | Hooks | Library storage change? | Migrate to YAML / Keep JSON | Keep JSON, generate manifest on export |
| D1 | All types | Dependency resolution | Advisory / Version-pinned / Lockfile | Advisory for v0.1 |

---

## 7. Work This Generates

Each section above maps to a future plan or spec:

1. **Rules canonical format spec** — the biggest gap. Define `rule.yaml` canonical format and provider conversion maps. Own spec alongside `docs/spec/hooks/hooks.md`.
2. **Hook manifest spec** — add `hook-manifest.yaml` wrapper format to hooks spec. Define `requires` + `recommends` dependency syntax.
3. **Rules injection spec** — define install behavior for file-first and append-with-markers models.
4. **Dependency resolution design** — advisory model for v0.1, spec the format and CLI output.
5. **SKILL.md frontmatter spec update** — align with Agent Ecosystem spec. Define which fields are required vs optional, version constraints, `expectations` structure.
6. **Rules extraction tool design** — `syllago rules extract` scans CLAUDE.md, suggests splits, handles migration.

---

## Prior Work and Context

- `docs/spec/skills/archive/skill-frontmatter-spec-agent-ecosystem.md` — Agent Ecosystem frontmatter spec (the direction we're aligning with)
- `docs/spec/skills/archive/provenance.md` — prior provenance design (sidecar model, archived)
- `docs/spec/hooks/hooks.md` — canonical hook interchange format (hook payload, no provenance/dependencies yet)
- `docs/spec/hooks/policy-interface.md` — enterprise policy enforcement interface for hooks
