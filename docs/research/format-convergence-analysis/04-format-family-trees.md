# Section 4: Format Family Trees

**Date:** 2026-03-30
**Part of:** Format Convergence Analysis series
**Input data:** `docs/research/provider-agent-naming.md`, `rules-research.md`, `skills-research.md`, `hook-research.md`, `mcp-research.md`

---

## Purpose

This section groups the agent landscape into "format families" — clusters of agents that share file path conventions, format structures, or config patterns. Each family maps directly to a converter module in syllago. Agents in the same family can share import and export logic; agents in different families need translation between canonical and native format.

The key insight: format families are not determined by which LLM a tool uses, or even by form factor (CLI vs IDE vs extension). They are determined by **which files the harness reads and where it looks for them**. An agent that reads `.claude/rules/` is in the Claude Code family regardless of its vendor or architecture.

---

## Family Tree Diagram

```
AI Coding Agent Format Families
════════════════════════════════════════════════════════════════════════

  ┌─────────────────────────────────────────────────────────────┐
  │  AGENTS.md UNIVERSAL LAYER (cross-family compatibility)     │
  │  Almost every agent reads AGENTS.md in the project root     │
  │  This is not a family — it is a shared lowest-common-denom  │
  └─────────────────────────────────────────────────────────────┘
                              │
           ┌──────────────────┼──────────────────┐
           ▼                  ▼                  ▼

  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
  │  CLAUDE CODE     │  │  CURSOR          │  │  VS CODE EXT     │
  │  FAMILY          │  │  FAMILY          │  │  FAMILY          │
  │                  │  │                  │  │                  │
  │ CLAUDE.md        │  │ .cursor/rules/   │  │ .github/         │
  │ .claude/rules/   │  │   *.mdc          │  │   copilot-instr  │
  │ .claude/skills/  │  │ .cursor/skills/  │  │   instructions/  │
  │ settings.json    │  │ mcp.json         │  │   skills/        │
  │  (hooks + MCP)   │  │ hooks v1.7+      │  │ VS Code ext      │
  │                  │  │                  │  │ settings.json    │
  │ Members:         │  │ Members:         │  │                  │
  │ • Claude Code    │  │ • Cursor IDE     │  │ Members:         │
  │ • Codex CLI *    │  │ • Cursor CLI     │  │ • GitHub Copilot │
  │ • Qwen Code      │  │ • Windsurf *     │  │ • Cline          │
  │ • oh-my-pi *     │  │ • Trae *         │  │ • Roo Code       │
  │ • Junie CLI *    │  │                  │  │ • Kilo Code      │
  │ • Pi *           │  │                  │  │ • Continue       │
  │                  │  │                  │  │ • Augment Code   │
  └──────────────────┘  └──────────────────┘  │ • Amazon Q Dev   │
                                               │ • JetBrains/Junie│
                                               └──────────────────┘

  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
  │  AGENT SKILLS    │  │  MCP JSON        │  │  GEMINI CLI      │
  │  FAMILY          │  │  FAMILY          │  │  FAMILY          │
  │                  │  │                  │  │                  │
  │ SKILL.md +       │  │ .mcp.json /      │  │ GEMINI.md        │
  │ YAML frontmatter │  │ mcp.json /       │  │ .gemini/skills/  │
  │ (agentskills.io) │  │ settings.json    │  │ Hierarchical,    │
  │                  │  │ mcpServers block │  │ no frontmatter   │
  │ Universal path:  │  │                  │  │                  │
  │ .agents/skills/  │  │ Nearly all full- │  │ Members:         │
  │                  │  │ support agents   │  │ • Gemini CLI     │
  │ Near-universal;  │  │ share mcpServers │  │ • Gemini Code    │
  │ see detailed     │  │ key + JSON format│  │   Assist         │
  │ breakdown below  │  │                  │  │ • Antigravity *  │
  └──────────────────┘  └──────────────────┘  │ • Qwen Code *    │
                                               └──────────────────┘

* = cross-family reader (reads files from multiple families natively)
```

---

## Family 1: Claude Code Family

**Originator:** Anthropic (Claude Code)
**Core convention:** `CLAUDE.md` + `.claude/` directory + hooks and MCP in `settings.json`

### What Defines This Family

The Claude Code family is defined by the `.claude/` directory structure and the settings.json file as the shared config hub. Rules go in `.claude/rules/`, skills in `.claude/skills/`, and both hooks and MCP servers are configured as blocks inside a JSON settings file. The canonical rules file at project root is `CLAUDE.md`.

### File Paths

```
CLAUDE.md                           # project root instructions
.claude/
  CLAUDE.md                         # alternative location
  rules/
    *.md                            # YAML frontmatter, paths: field
  skills/
    <name>/SKILL.md
  commands/                         # (legacy, migrating to skills)
    *.md
  settings.json                     # hooks: {...}, mcpServers: {...}
  settings.local.json               # local overrides

~/.claude/
  CLAUDE.md                         # user-scope instructions
  rules/*.md
  skills/<name>/SKILL.md
  settings.json
```

### Member Agents

| Agent | Reads CLAUDE.md | .claude/rules/ | .claude/skills/ | settings.json (hooks) | settings.json (MCP) | Notes |
|-------|-----------------|----------------|-----------------|----------------------|---------------------|-------|
| **Claude Code** | Native | Native | Native | Native (21 events) | Native | Originator |
| **Codex CLI** | Via AGENTS.md fallback | Via `.agents/skills/` | Reads `.claude/skills/` | Different format (config.toml + hooks section) | `.agents/` MCP + stdio/streamable-http | Reads Claude paths but config format differs |
| **Qwen Code** | Reads `CLAUDE.md` | `.qwen/` mirrors `.claude/` | `.qwen/skills/` | `.qwen/settings.json` (CC-like hooks) | `.qwen/settings.json` | Closest CC clone — near-identical directory mirror |
| **oh-my-pi** | Yes (one of 8 formats) | Via cross-agent discovery | Via `.agents/skills/` | Own hooks format | Full MCP + OAuth | True cross-family polyglot |
| **Junie CLI** | Reads `.junie/AGENTS.md` | Via `.junie/skills/` | Reads from CC paths | No hooks | `.junie/mcp/mcp.json` | Explicit CC compat; imports from CC dirs |
| **Pi** | `AGENTS.md`/`SYSTEM.md` | Via cross-agent discovery | Via `/skill:name` | Extension lifecycle hooks | No MCP by design | CC-aware but own format |
| **Stagewise** | Yes (`claude.md`) | No | No | No | No | Read-only CC compat for rules |
| **Frontman** | Yes (`claude.md`) | No | No | No | No | Read-only CC compat for rules |
| **Bolt.new** | Yes (`claude.md`) | No | No | No | No | Read-only CC compat for rules |
| **OpenCode** | Fallback after AGENTS.md | `.opencode/rules/` | `.opencode/skills/` + `.claude/skills/` | No hooks | `opencode.json` (mcp block) | CC as secondary compatibility layer |
| **Windsurf** | Reads as cross-tool compat | `.windsurf/rules/` primary | `.windsurf/skills/` + `.claude/skills/` | `.windsurf/settings.json` | `.windsurf/settings.json` | CC paths as compat fallback |

### Converter Implications

The Claude Code family is syllago's canonical format. The import direction (CC → canonical) requires no transformation for most fields — the canonical format was designed around CC. The export direction (canonical → CC) is also a near-identity transform.

**Sub-variant: Qwen Code.** Qwen Code mirrors CC so closely (`.qwen/settings.json`, same hook event names, same MCP block structure) that it can share the CC converter with only a path substitution (`s/.claude/.qwen/g`). A single `cc-family` converter with a `pathRoot` parameter handles both.

**Sub-variant: Read-only consumers.** Stagewise, Frontman, and Bolt.new read `CLAUDE.md` as a rules source but have no other CC-family artifacts. They don't need a full CC converter — they need only the rules canonicalize path.

---

## Family 2: AGENTS.md Universal Layer

**Originator:** OpenAI / Codex CLI (popularized 2025)
**Core convention:** `AGENTS.md` in project root, plain markdown, no frontmatter

### What Defines This Layer

AGENTS.md is not strictly a family — it is a cross-family compatibility layer. Nearly every agent in the landscape reads `AGENTS.md` from the project root as either a primary rules source or a fallback. This makes it the lowest-common-denominator format for rules distribution.

The key technical detail: **AGENTS.md is always plain markdown with no frontmatter support in its original form**. It has no conditional activation, no glob scoping, no multi-file support. It is the simplest possible rules format.

### Agents That Read AGENTS.md as Primary

These agents use AGENTS.md as their primary (or only) rules file. They define the format natively:

| Agent | AGENTS.md Location | Variant Notes |
|-------|-------------------|---------------|
| **Codex CLI** | Project root, `~/.codex/AGENTS.md` | Also `AGENTS.override.md` for local overrides |
| **Goose** | Project root + `goosehints` | ACP server mode; MCP-first |
| **Aider** | Project root (`CONVENTIONS.md` also accepted) | Built-in `/` commands |
| **Warp** | `WARP.md` / `AGENTS.md` | Rich MCP GUI; Oz cloud agents |
| **DeepAgents CLI** | Project root | + `~/.deepagents/skills/` |
| **Jules** | Project root | Async cloud VM |
| **Open SWE** | Project root | Middleware hooks (4 built-in) |
| **Amp** | Project root + subdirs (AGENTS.md with `globs:` frontmatter) | Also reads `AGENT.md`, `CLAUDE.md` as fallbacks |
| **OpenCode** | Primary; falls back to `CLAUDE.md` | Also reads `.opencode/` hierarchy |
| **Factory Droid** | `AGENTS.md` + `~/.factory/` | Cross-IDE extension agent |
| **Composio Orchestrator** | `agent-orchestrator.yaml` reads AGENTS.md | CI/review reaction hooks |

### Agents That Read AGENTS.md as Secondary/Compat

These agents have their own primary format but explicitly fall back to AGENTS.md:

| Agent | Primary Format | AGENTS.md Role |
|-------|---------------|----------------|
| Windsurf | `.windsurf/rules/*.md` | Fallback at root (always-on) |
| Kiro | `.kiro/steering/*.md` | Recognized (no inclusion mode) |
| GitHub Copilot | `.github/copilot-instructions.md` | Cross-tool compat |
| Cline | `.clinerules/` | Cross-tool compat |
| Zed | `.rules` (first match) | Fallback position #7 in priority list |
| Replit | `replit.md` | Cross-tool compat |
| Lovable | Custom instructions | Cross-tool compat |
| v0 | `AGENTS.md` (Agent Skills primary) | Used as rules context |
| Antigravity | `GEMINI.md`/`AGENTS.md` | Both recognized |
| Kilo Code | Primary rules format | Uses AGENTS.md natively |

### Converter Implications

AGENTS.md as a format is trivial to import (no frontmatter to parse) and trivial to export (strip frontmatter, concatenate). The real challenge is that it is **always-apply with no conditional activation**. Any rule with `paths:` frontmatter that is exported to an AGENTS.md target loses its conditional activation semantics and becomes always-on prose.

The AGENTS.md converter is shared across all agents in this layer. It has one import path (plain markdown strip) and one export path (concatenate with prose scope notes if the source had conditional activation).

---

## Family 3: Cursor Family

**Originator:** Cursor (Anysphere)
**Core convention:** `.cursor/rules/*.mdc` + `.cursor/skills/` + `mcp.json` at project root

### What Defines This Family

The Cursor family uses `.mdc` (Markdown + YAML frontmatter) files for rules — a Cursor-specific extension. The frontmatter has a distinctive four-field vocabulary (`alwaysApply`, `globs`, `description`, plus implicit type detection). MCP config lives in a standalone `mcp.json` file, not inside a shared settings file.

### File Paths

```
.cursor/
  rules/
    *.mdc                           # YAML: alwaysApply, globs, description
  skills/
    <name>/SKILL.md                 # Also reads .agents/skills/, .claude/skills/
mcp.json                            # project MCP config (top-level mcpServers key)
~/.cursor/rules/                    # global rules
~/.cursor/mcp.json                  # global MCP
```

### Member Agents

| Agent | .cursor/rules/ | .cursor/skills/ | mcp.json | Hooks | Notes |
|-------|----------------|-----------------|----------|-------|-------|
| **Cursor IDE** | Native (.mdc) | Native | Native | Via hooks v1.7+ | Originator; also reads `.agents/`, `.claude/` for skills |
| **Cursor CLI** | Native (.mdc) | Native | Native | Same as IDE | Full 6/6 support |
| **Windsurf** | Compat: reads `.cursorrules` | `.windsurf/skills/` primary | Own `settings.json` | Own hook format | Uses `.mdc` compat only for legacy `.cursorrules` |
| **Trae** | Reads `.cursor/rules/` | `.trae/rules/` primary | `.mcp.json` | No hooks | Close Cursor fork; shares rule path |
| **Void** | No rules; MCP only | No | `mcp.json` | No | VS Code fork; project paused |

**Note on `.cursorrules`:** This is the legacy single-file format (pre-`.mdc`). Several agents read it as a compatibility fallback (Cline, Zed, Windsurf) but it is not the active Cursor format. The family diagram counts only the `.cursor/rules/*.mdc` modern format.

### Converter Implications

The Cursor family's distinguishing format challenge is the `.mdc` extension and the four-rule-type system (Always, Auto-Attach, Agent, Manual). The MDC frontmatter is Cursor-specific; no other provider uses it.

**Sub-variant: Trae.** Trae reads `.cursor/rules/` natively but uses `.trae/rules/` as its own directory. It can share the Cursor rules converter with a path override. MCP uses `.mcp.json` (note: dot prefix, vs Cursor's unprefixed `mcp.json`).

**Sub-variant: Windsurf.** Windsurf is best treated as its own family member rather than a Cursor sub-variant because its trigger vocabulary (`always_on`, `glob`, `model_decision`, `manual`) is semantically richer than Cursor's four types and requires its own converter. However, its `.cursorrules` legacy support means the Cursor canonicalize path works for importing old Windsurf rule files.

---

## Family 4: VS Code Extension Family

**Originator:** Microsoft (VS Code + GitHub Copilot)
**Core convention:** `.github/copilot-instructions.md` + `.github/instructions/*.instructions.md` + VS Code `settings.json`

### What Defines This Family

The VS Code Extension family is defined by living inside VS Code's extension host. These agents share the VS Code workspace abstraction, `settings.json` for configuration, the `.github/` directory convention for instructions, and increasingly the `SKILL.md` format via `.github/skills/`.

The family has a spectrum: some agents (GitHub Copilot, Cline, Roo Code) have rich content type support; others (Tabnine, CodeGeeX) have minimal extensibility. The unifying trait is the VS Code host environment.

### File Paths

```
.github/
  copilot-instructions.md           # GitHub Copilot primary
  instructions/
    *.instructions.md               # YAML: applyTo field (glob pattern)
  skills/
    <name>/SKILL.md                 # Copilot + VS Code Copilot
.clinerules/
  *.md                              # Cline rules (paths: frontmatter)
.roo/
  rules/
    *.md                            # Roo Code rules (no frontmatter)
  rules-{mode}/
    *.md                            # Roo Code mode-scoped rules
.kilocode/
  mcp.json                          # Kilo Code MCP
.tabnine/
  guidelines/                       # Tabnine rules
  mcp_servers.json                  # Tabnine MCP
  agent/commands/                   # Tabnine commands
.augment/
  rules/
  skills/
  hooks/
.continue/
  rules/                            # Continue rules
.amazonq/
  rules/*.md                        # Amazon Q rules
VS Code settings.json               # Extension config + MCP for several
```

### Member Agents

| Agent | Rules Format | Skills | Hooks | MCP Config | Notes |
|-------|-------------|--------|-------|------------|-------|
| **GitHub Copilot** | `.github/copilot-instructions.md` + `AGENTS.md` | `.github/skills/` | JetBrains preview only | VS Code settings or JSON | Agent YAML profiles |
| **Cline** | `.clinerules/*.md` (paths: frontmatter) | `.cline/skills/` | No | VS Code settings | Also reads `.cursorrules`, `AGENTS.md` |
| **Roo Code** | `.roo/rules/` + `.roo/rules-{mode}/` | `.roo/skills/` | No | VS Code settings | Mode-scoped rules unique in landscape |
| **Kilo Code** | `AGENTS.md` | No | No | `.kilocode/mcp.json` | Orchestrator mode; Agent Manager |
| **Continue** | `.continue/rules/` | No | No | VS Code settings; auto-imports from Claude/Cursor configs | Custom prompts as slash commands |
| **Tabnine** | `.tabnine/guidelines/` | No | No | `.tabnine/mcp_servers.json` | Enterprise context engine |
| **Augment Code** | `.augment/rules/` | `.augment/skills/` | `.augment/hooks/` | VS Code + JSON import | Agent modes; `context_groups` |
| **Amazon Q Developer** | `.amazonq/rules/*.md` | No | Yes (agentSpawn, preToolUse, etc.) | Per-agent MCP | Hooks with matchers, unique among ext family |
| **Gemini Code Assist** | Context files | Via Gemini CLI hooks | Gemini CLI hooks | VS Code settings | Defers to Gemini CLI for format |
| **JetBrains AI / Junie** | `AGENTS.md` | `.junie/skills/` | No | `.junie/mcp/mcp.json` | JetBrains IDEs primary; AGENTS.md native |
| **Qodo** | Agent TOML/YAML | `qodo-skills` | Webhook hooks | `mcp.json` | Full marketplace; `qodo.ai` platform |
| **Traycer** | Delegates to Cursor/CC | None | None | Partial | Thin orchestration layer |
| **Zencoder** | Via Zen Agents JSON | None | No | MCP Library (100+ servers) | Agent marketplace; `zencoder.ai` |
| **Factory Droid** | `AGENTS.md` + `~/.factory/` | Factory skills | Hooks with global toggle | `~/.factory/mcp/` | Multi-IDE; VS Code + JetBrains + Zed |

### Converter Implications

The VS Code Extension family has the most internal fragmentation of any family. The right architecture here is not a single "VS Code extension converter" but rather **per-agent converters that share VS Code-specific utility functions**:

1. **Copilot converter** — `.github/instructions/*.instructions.md` with `applyTo:` frontmatter, `.github/skills/` skills, VS Code `settings.json` MCP
2. **Cline converter** — `.clinerules/` rules with `paths:` frontmatter (CC-compatible), `.cline/skills/`
3. **Roo Code converter** — `.roo/rules/` (no frontmatter, mode-scoped), `.roo/skills/`
4. **Continue converter** — `.continue/rules/`, MCP auto-import from Claude/Cursor configs
5. **Amazon Q converter** — `.amazonq/rules/*.md`, hooks with matchers
6. **Augment converter** — `.augment/` directory with rules/skills/hooks sub-dirs

The shared utility is VS Code `settings.json` MCP block handling, which uses the `servers` key (vs most agents' `mcpServers`).

---

## Family 5: Agent Skills Family

**Originator:** Anthropic (Claude Code) — published as open spec at agentskills.io, December 2025
**Core convention:** `SKILL.md` + YAML frontmatter (`name`, `description`) + directory-per-skill structure

### What Defines This Family

The Agent Skills family is defined by the `SKILL.md` format — a file named exactly `SKILL.md` in a per-skill directory, with YAML frontmatter containing at minimum `name` and `description`. This is an open specification (`agentskills.io`) with Claude Code as the reference implementation.

Unlike other families where one agent is the originator and others clone it, the Agent Skills family is structured as a shared community spec with tiered field support. Agents support different subsets of the 12 fields Claude Code recognizes.

### File Paths

```
# Universal cross-agent path (widest compatibility)
.agents/skills/
  <skill-name>/
    SKILL.md                        # YAML frontmatter + markdown body

# Provider-specific paths
.claude/skills/<name>/SKILL.md      # Claude Code / Copilot / Windsurf compat
.cursor/skills/<name>/SKILL.md      # Cursor
.gemini/skills/<name>/SKILL.md      # Gemini CLI
.windsurf/skills/<name>/SKILL.md    # Windsurf
.kiro/skills/<name>/SKILL.md        # Kiro
.cline/skills/<name>/SKILL.md       # Cline
.opencode/skills/<name>/SKILL.md    # OpenCode
.roo/skills/<name>/SKILL.md         # Roo Code
.github/skills/<name>/SKILL.md      # GitHub Copilot / VS Code Copilot
.junie/skills/<name>/SKILL.md       # Junie CLI
.crush/skills/<name>/SKILL.md       # Crush
```

### Agent Skills Field Tier Breakdown

```
Tier 1 — Base spec (name + description only)
  Gemini CLI, Windsurf, Cline, Roo Code, Amp, Manus

Tier 2 — Extended spec (+ license, compatibility, metadata)
  Cursor, Kiro, OpenCode

Tier 3 — CC near-parity (+ disable-model-invocation, user-invocable, argument-hint)
  GitHub Copilot, VS Code Copilot, Codex CLI*

Tier 4 — Full CC superset (+ allowed-tools, disallowed-tools, context, agent, model, effort, hooks)
  Claude Code only

  * Codex uses openai.yaml sidecar instead of SKILL.md frontmatter
    for invocation control (allow_implicit_invocation: false)
```

### Member Agents (All SKILL.md Readers)

| Agent | Tier | Native Path | Cross-Agent Paths Scanned | Notes |
|-------|------|------------|--------------------------|-------|
| Claude Code | 4 | `.claude/skills/` | `.agents/skills/`, `--add-dir` | Reference impl; 12 FM fields |
| Cursor | 2+ | `.cursor/skills/` | `.agents/`, `.claude/`, `.codex/` | `disable-model-invocation` supported |
| Gemini CLI | 1 | `.gemini/skills/` | `.agents/` | Unique consent model per invocation |
| GitHub Copilot | 3 | `.github/skills/` | `.claude/`, `.agents/` | Shares impl with VS Code Copilot |
| VS Code Copilot | 3 | `.github/skills/` | `.claude/`, `.agents/` | Stricter validator than CLI |
| Windsurf | 1 | `.windsurf/skills/` | `.agents/`, `.claude/` | `@mention` invocation (unique) |
| Kiro | 2 | `.kiro/skills/` | N/A | Separate Steering system coexists |
| Codex CLI | 3* | `.agents/skills/` | `.claude/skills/` | `openai.yaml` sidecar unique |
| Cline | 1 | `.cline/skills/` | `.agents/`, `.claude/` | `use_skill` tool; experimental gate |
| OpenCode | 2 | `.opencode/skills/` | `.claude/`, `.agents/` | Semantic similarity nudging (unique) |
| Roo Code | 1 | `.roo/skills/` | `.agents/` | Mode-scoped directories (unique) |
| Amp | 1 | `.agents/skills/` | `.claude/` | MCP lazy-load via skills (unique) |
| Manus | 1 | Cloud upload | N/A | Workflow capture (unique) |
| Crush | 1 | (AGENTS.md + Agent Skills) | N/A | `CRUSH.md`/`CLAUDE.md` also read |
| gptme | 1 | `.gptme/skills/` | N/A | Also has lessons + prompt templates |
| Junie CLI | 1 | `.junie/skills/` | Imports from CC/Cursor/other dirs | Explicit cross-agent import |
| Mistral Vibe | 1 | `~/.vibe/prompts/` | N/A | TOML agent profiles alongside |
| DeepAgents CLI | 1 | `~/.deepagents/skills/` | N/A | AGENTS.md primary; skills secondary |
| v0 | 1 | Via Skills.sh | N/A | Agent Skills + ContextKit commands |
| Antigravity | 1 | AgentKit 2.0 (40+ built-in) | N/A | Skills first-class; Workflows as commands |
| Trae | 1 | `.trae/rules/` | `.mcp.json` | SKILL.md via Agent Skills standard |
| OpenHands | 1 | `.openhands/microagents/` | N/A | md + YAML frontmatter micro-agents |

### Converter Implications

The Agent Skills family is the most important family for syllago's long-term coverage. It is a genuine open standard, not a proprietary clone, which means skills can flow through syllago without lossy round-trips for the base spec fields.

The converter architecture for this family:
- **One canonical import path** — parse `SKILL.md` YAML frontmatter using the full 12-field schema (unknown fields preserved passthrough)
- **Per-agent export paths** — each agent's export strips unsupported fields and embeds them as prose warnings
- **Path mapping** is the cheapest transform — the SKILL.md content is identical, only the install directory changes

The real work is handling Claude Code's power features (`hooks`, `context: fork`, `allowed-tools`) when downgrading to base-spec targets.

---

## Family 6: MCP JSON Family

**Originator:** Anthropic (Claude Code) — MCP protocol by Anthropic, December 2024
**Core convention:** JSON config with `mcpServers` top-level key + server objects with `command`/`args`/`env`

### What Defines This Family

Nearly every agent with MCP support converged on the same JSON schema originally established by Claude Code: a `mcpServers` object where each key is a server name and each value contains `command`, `args`, and `env` fields. Despite this convergence, there are meaningful sub-variants in transport types, config file locations, and scoping.

### Sub-Variants

```
mcpServers-block family (JSON, top-level key = mcpServers):
  Claude Code   — .mcp.json (project), ~/.claude.json (user)
  Cursor        — mcp.json (project), ~/.cursor/mcp.json (global)
  Gemini CLI    — settings.json mcpServers block
  Windsurf      — settings.json mcpServers block (global only)
  Kiro          — .kiro/settings.json or .air/mcp.json
  Cline         — VS Code settings mcpServers
  Roo Code      — VS Code settings mcpServers
  GitHub Copilot— VS Code settings mcpServers block
  Kilo Code     — .kilocode/mcp.json
  Zencoder      — MCP Library JSON config
  Superset      — .mcp.json (inherits from installed agents)
  Amp           — project + global JSON (amp.mcpServers key — unique)
  JetBrains Air — .air/mcp.json (ACP-mediated)
  Trae          — .mcp.json (dot-prefixed)
  Codex CLI     — config.toml [[mcp_servers.*]] section (TOML — outlier)
  OpenCode      — opencode.json mcp block (JSONC format — minor variant)
  Zed           — context_servers key (not mcpServers — unique name)
```

### Convergence Analysis

| Field | CC Format | De Facto Standard? | Outliers |
|-------|-----------|--------------------|---------|
| Top-level key | `mcpServers` | Yes (12+ agents) | `amp.mcpServers` (Amp), `context_servers` (Zed), `mcp` (OpenCode), TOML table (Codex) |
| stdio transport | `command`+`args`+`env` | Yes (universal) | None |
| HTTP transport | `type: "http"` | Contested | Cursor uses `"streamable-http"`, CC uses `"http"` — same protocol, different string |
| SSE transport | `type: "sse"` | Legacy | CC deprecated in favor of HTTP |
| Server name key | Arbitrary string | Yes | None |
| `autoApprove` | `string[]` | CC-specific | Cursor calls it `alwaysAllowed`, VS Code uses `allowedTools` |

### Converter Implications

The MCP JSON family has the highest convergence of any family — the core `mcpServers` + `command`/`args`/`env` schema is shared by 12+ agents. A single MCP converter handles ~80% of cases with only field renames for transport type and auto-approve.

**Three actual sub-converters are needed:**

1. **mcpServers-JSON** (default): handles Claude Code, Cursor, Windsurf, Kiro, Cline, Roo Code, Copilot, and most others
2. **TOML sub-converter**: Codex CLI uses `[[mcp_servers.server-name]]` TOML table format
3. **opencode.json sub-converter**: `mcp: { local: {...}, remote: {...} }` JSONC structure

**Transport type normalization:** The canonical MCP transport type names should be `stdio`, `sse`, and `http` (following the MCP spec). The converter must handle both `"http"` (CC) and `"streamable-http"` (Cursor) as the same thing on import, and emit the target's preferred string on export.

---

## Family 7: Gemini CLI Family

**Originator:** Google (Gemini CLI)
**Core convention:** `GEMINI.md` + `.gemini/` directory + hierarchical plain markdown (no frontmatter)

### What Defines This Family

The Gemini CLI family is the simplest: rules are plain markdown files named `GEMINI.md`, discovered hierarchically (global → project root → subdirectories). No frontmatter, no conditional activation, no multi-file support. Skills live in `.gemini/skills/`. Commands are defined in `.gemini/` with TOML format for Gemini CLI specifically.

### File Paths

```
~/.gemini/
  GEMINI.md                         # global instructions
  settings.json                     # MCP + hooks config
  skills/<name>/SKILL.md

GEMINI.md                           # project root
src/
  GEMINI.md                         # subdirectory (auto-scoped)
.gemini/
  settings.json                     # project MCP + hooks
  skills/<name>/SKILL.md
```

### Member Agents

| Agent | GEMINI.md | .gemini/ | Hooks | Notes |
|-------|-----------|----------|-------|-------|
| **Gemini CLI** | Native | Native | Yes (settings.json) | Originator; `@file` import syntax |
| **Gemini Code Assist** | Via Gemini CLI context files | Partial | Via Gemini CLI hooks | Extension defers to CLI format |
| **Antigravity** | `GEMINI.md`/`AGENTS.md` both read | AgentKit structure | No hooks | Google-backed VS Code fork |
| **Qwen Code** | Also reads GEMINI.md | `.qwen/` primary | `.qwen/settings.json` | Multi-family reader; CC architecture primary |

### Converter Implications

The Gemini CLI family is simple to import (strip to plain markdown) but lossy to export (all conditional activation must become prose or be dropped). The `@file.md` import syntax is unique to Gemini (and Claude Code's `@path` variant) — converters must either resolve imports inline or strip them.

The one notable sub-variant is Gemini's commands format (`.gemini/commands/*.md` with TOML-like frontmatter vs Claude Code's plain markdown commands) — this needs its own command converter.

---

## Family 8: Windsurf Family

**Originator:** Codeium / Windsurf
**Core convention:** `.windsurf/rules/*.md` + trigger-based frontmatter (four types) + settings.json

### What Defines This Family

Windsurf is unique enough to deserve its own family despite being a VS Code fork. Its rules system has four activation modes (`always_on`, `glob`, `model_decision`, `manual`) with a richer vocabulary than Cursor's four-type system. The `model_decision` mode (AI decides whether to activate a rule) is a **rules activation mode**, not a hook trigger — but it has no equivalent in any other family's rules system. Its skills system also introduced enterprise MDM deployment paths.

### File Paths

```
.windsurf/
  rules/
    *.md                            # YAML: trigger, globs, description
  skills/<name>/SKILL.md
  settings.json                     # MCP + hooks config

~/.codeium/windsurf/
  memories/global_rules.md          # Global rules (6K char limit)
  skills/<name>/SKILL.md

# Enterprise read-only paths
/Library/Application Support/Windsurf/rules/   (macOS)
/etc/windsurf/rules/                            (Linux)
/etc/windsurf/skills/                          (Linux)
```

### Converter Implications

The `model_decision` rules activation mode is the key challenge. It has no canonical equivalent and cannot be faithfully represented in any other family. The converter must choose between:
- Downgrading to `always_on` (over-includes the rule)
- Downgrading to prose-guided (embedding the description as a note)
- Emitting a compat warning and defaulting to always-on

This is a legitimate lossy conversion. The compat score should reflect the loss when converting `model_decision` rules to Claude Code, Cursor, Kiro, or AGENTS.md targets.

---

## Family 9: Autonomous / Research Agent Subset

**Originator:** Various (academic + cloud-native agents)
**Core convention:** Task-based execution, YAML config files, no persistent user-defined skill content

### What Defines This Subset

Autonomous agents (Devin, Jules, Open SWE, OpenHands, SWE-agent) are not a "family" in the format sense — they are largely out of scope for syllago's converter. They consume tasks rather than persistent skills, and their configuration is service-level (API calls, cloud VM config) rather than filesystem artifacts.

However, a few of these agents have interesting format intersections:

| Agent | Readable Formats | Syllago Relevance |
|-------|-----------------|-------------------|
| **OpenHands** | Full 6/6; `.openhands/microagents/` with YAML frontmatter | High — micro-agent format is worth supporting |
| **Open SWE** | `AGENTS.md` + middleware hooks | Low — hooks are framework-level, not user-config |
| **Jules** | `AGENTS.md` | Minimal — async only, no skills/hooks |
| **Devin** | Playbooks (proprietary) | Out of scope |

OpenHands' micro-agent format (`AGENT.md` + YAML frontmatter in `.openhands/microagents/`) is the most interesting. It is close enough to SKILL.md that a micro-agent converter is feasible. OpenHands is one of only five agents with full 6/6 content type support.

---

## Cross-Family Readers

Several agents are genuine polyglots — they natively read formats from multiple families. These agents require special handling because their import path must walk multiple directory trees.

| Agent | Families Read | Mechanism |
|-------|--------------|-----------|
| **oh-my-pi** | CC, AGENTS.md, Cursor, Gemini, Copilot, Windsurf, Cline, and more | Explicit cross-agent discovery at startup; reads 8+ format directories |
| **Junie CLI** | CC (.claude/), AGENTS.md, own `.junie/` dir | Explicit import from other agents' directories documented |
| **Continue** | Cursor (auto-imports .cursor/mcp.json), Claude Code | MCP auto-import from CC and Cursor config paths |
| **Cline** | CC (CLAUDE.md), Cursor (.cursorrules), Windsurf (.windsurfrules), AGENTS.md | Cross-tool compat on import |
| **Zed** | AGENTS.md, CLAUDE.md, GEMINI.md, .cursorrules, .windsurfrules, .clinerules, .github/ | Priority-ordered 9-file scan |
| **OpenCode** | AGENTS.md (primary), CLAUDE.md (fallback), .cursor/rules/* (via instructions field) | Config-driven multi-source rules |
| **Windsurf** | .windsurf/ (primary), .agents/ (skills), .claude/ (skills), AGENTS.md (root) | Explicit cross-agent skill paths |
| **Kiro** | .kiro/ (primary), AGENTS.md, ~/.kiro/steering | Multiple scopes but single format |
| **GitHub Copilot** | AGENTS.md, CLAUDE.md, GEMINI.md | Cross-tool compat via toggle settings |
| **Amp** | AGENTS.md (primary), AGENT.md, CLAUDE.md (fallbacks) | Explicit fallback chain |

**Implication for syllago:** Cross-family readers do not need a unique converter family — they already consume other formats natively. Syllago's value for these agents is not conversion but **packaging**: a syllago install can write the correct format for each family and the cross-reader will pick it up automatically. This is why syllago's content install strategy (writing to `.agents/skills/` for skills, `AGENTS.md` for rules) reaches the most agents with the least format complexity.

---

## Converter Architecture Summary

Based on the family analysis, syllago needs these converter modules:

| Module | Families Served | Priority | Complexity |
|--------|----------------|----------|------------|
| `claude-code` | CC family (full) | P1 (done) | Medium — canonical format |
| `cursor` | Cursor family, .mdc rules | P1 (done) | Medium — 4-type rule system |
| `windsurf` | Windsurf family | P1 (done) | High — model_decision has no peer |
| `kiro` | Kiro steering + skills | P1 (partial) | Medium — two systems coexist |
| `gemini-cli` | Gemini CLI family | P1 (done) | Low — plain markdown |
| `copilot` | VS Code Ext family (Copilot) | P1 (partial) | Medium — .instructions.md + applyTo |
| `cline` | VS Code Ext family (Cline) | P1 (done) | Low — paths: frontmatter same as CC |
| `roo-code` | VS Code Ext family (Roo) | P1 (done) | Medium — mode-scoped dirs |
| `agents-md` | AGENTS.md layer | P1 (done) | Low — plain markdown |
| `codex` | CC family + TOML MCP | P1 (done) | Medium — TOML MCP sub-converter |
| `opencode` | AGENTS.md layer + own dirs | P1 (partial) | Low — now uses SKILL.md frontmatter |
| `zed` | Cross-reader; minimal own format | P1 (done) | Low — .rules single file |
| `amp` | AGENTS.md + Agent Skills | P2 | Low — base spec only |
| `qwen-code` | CC family sub-variant | P2 | Trivial — path substitution |
| `junie` | AGENTS.md + .junie/ | P2 | Low |
| `crush` | AGENTS.md + Agent Skills | P2 | Low |
| `gptme` | Own format + Agent Skills | P3 | Medium — TOML config |
| `warp` | AGENTS.md layer | P3 | Low |
| `antigravity` | Gemini family + AgentKit | P3 | Medium — AgentKit skills differ |
| `openhands` | Own micro-agent format | P3 | Medium — YAML frontmatter micro-agents |
| `continue` | VS Code Ext + MCP auto-import | P3 | Low for rules; medium for MCP cross-import |
| `amazon-q` | VS Code Ext family | P3 | Medium — hooks with matchers |
| `augment` | VS Code Ext family | P3 | Medium — .augment/ multi-dir |
| `trae` | Cursor sub-variant | P3 | Trivial — path substitution |
| `factory-droid` | AGENTS.md + ~/.factory/ | P3 | Low |
| `goose` | AGENTS.md layer | P3 | Low — ACP server mode |
| `aider` | AGENTS.md layer | P3 | Low |
| `pi` | AGENTS.md + extension hooks | P4 | Medium — lifecycle hooks differ |
| `kilo-code` | VS Code Ext + AGENTS.md | P4 | Low |
| `tabnine` | VS Code Ext family | P4 | Low — .tabnine/ dirs |
| `manus` | Agent Skills (cloud) | P4 | Low — base spec only |
| `v0` | AGENTS.md + Agent Skills | P4 | Low |

**Key architectural decision:** Converters are per-agent, not per-family. However, family membership determines which converter logic can be shared. The CC family shares `cc-rules` import/export functions. The AGENTS.md layer shares the `plain-markdown` import/export. The Agent Skills family shares the `skill-frontmatter` parse/render. The MCP JSON family shares the `mcpservers-json` import/export.

This is the right trade-off: families define the shared code, agents define the converter entry points with agent-specific path mappings and field overrides.

---

## Sources

- `docs/research/provider-agent-naming.md` — Agent landscape tables, content type support matrix
- `docs/research/rules-research.md` — Per-provider rules format deep dive, feature matrix
- `docs/research/skills-research.md` — Agent Skills format tiers, invocation mechanics
- `docs/research/hook-research.md` — Hook system comparison, production hook inventory
- `docs/research/mcp-research.md` — MCP config format comparison, transport type variants
- `docs/spec/hooks.md` — Canonical hook event names and tool maps
