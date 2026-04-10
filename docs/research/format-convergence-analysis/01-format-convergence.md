# Section 1: Format Convergence Analysis

**Date:** 2026-03-30
**Source data:** `docs/research/provider-agent-naming.md`, `docs/research/rules-research.md`,
`docs/research/hook-research.md`, `docs/research/mcp-research.md`,
`docs/research/commands-research.md`, `docs/research/agents-research.md`,
`docs/research/skills-research.md`, `docs/spec/skills/frontmatter_spec_proposal.md`

---

## Overview

This section analyzes format-level convergence and fragmentation across the six syllago content types
(Rules, Skills, Hooks, MCP, Commands, Agents). For each type we map what 40+ agents are actually
doing on disk: exact file paths, format choices, and whether the approach follows an emerging
standard or is proprietary.

**What we are measuring:**
- **Convergence**: Two or more agents independently arriving at the same file format, path
  convention, or schema — whether by deliberate adoption or parallel evolution.
- **Fragmentation**: Each agent inventing its own path or format with no shared surface.

Convergence is not the same as a single agent's format being dominant. Claude Code's `CLAUDE.md`
is widely recognized *as a fallback*, but that is not the same as other agents standardizing on the
`.claude/` directory structure. The analysis distinguishes these cases.

---

## 1. Rules

### 1.1 Actual File Paths and Formats

Rules are the highest-adoption content type (78% of researched agents support them). The landscape
divides into two structural families: **hierarchical single-file** (one file per level, no
per-file scoping) and **multi-file directory** (one `.md` per rule with YAML frontmatter
controlling activation).

| Agent | File Path(s) | Format | Scoping Model | Standard or Proprietary |
|---|---|---|---|---|
| Claude Code | `CLAUDE.md`, `.claude/CLAUDE.md`, `.claude/rules/*.md` | MD + YAML frontmatter (`paths:` only) | Path-glob via `paths:` array | Proprietary (but widely recognized) |
| Cursor | `.cursor/rules/*.mdc` | MDC (Markdown + YAML; `.mdc` extension) | `alwaysApply`, `globs`, `description` — 4 distinct types | Proprietary |
| Windsurf | `.windsurf/rules/*.md` | MD + YAML frontmatter (`trigger:` enum) | `always_on`, `glob`, `model_decision`, `manual` | Proprietary |
| Kiro | `.kiro/steering/*.md` | MD + YAML frontmatter (`inclusion:` enum) | `auto`, `fileMatch`, `manual` | Proprietary |
| Codex CLI | `AGENTS.md`, `AGENTS.override.md` | Plain MD (no frontmatter) | Hierarchical walk; no conditional activation | Convergent (AGENTS.md cluster) |
| Gemini CLI | `GEMINI.md` (hierarchical) | Plain MD (no frontmatter) | Hierarchical walk; no conditional activation | Proprietary filename |
| Copilot CLI | `.github/copilot-instructions.md`, `AGENTS.md`, `.github/instructions/*.instructions.md` | MD + YAML frontmatter (`applyTo:` string) | `applyTo` comma-separated globs | Partially convergent |
| VS Code Copilot | `.github/instructions/*.instructions.md`, `AGENTS.md`, `CLAUDE.md` | MD + YAML frontmatter (`applyTo:`, `name:`, `description:`, `excludeAgent:`) | `applyTo` globs; reads `.claude/rules/` natively | Convergent (reads multiple formats) |
| Cline | `.clinerules/*.md` | MD + YAML frontmatter (`paths:` array) | `paths:` array globs (same field as Claude Code) | Proprietary dir; Claude-compatible field name |
| Roo Code | `.roo/rules/*.md`, `.roo/rules-{mode}/*.md` | Plain MD (no frontmatter) | Mode-based directory scoping | Proprietary (mode system unique) |
| OpenCode | `AGENTS.md`, `CLAUDE.md` (fallback) | Plain MD (no frontmatter) | Hierarchical; no conditional activation | Convergent (AGENTS.md cluster) |
| Zed | `.rules` (primary); also reads `.cursorrules`, `.windsurfrules`, `.clinerules`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` | Plain text (no frontmatter) | Always-on; single file only | Most cross-compatible reader in ecosystem |
| Amp | `AGENTS.md` (primary), `AGENT.md`, `CLAUDE.md` (fallbacks) | MD + YAML frontmatter (`globs:` array) | Hierarchical walk + `globs:` frontmatter | AGENTS.md cluster + proprietary frontmatter |
| Trae | `.trae/rules/` | MD | Unknown frontmatter support | Proprietary |
| Antigravity | `GEMINI.md`, `AGENTS.md` | Plain MD | Hierarchical (Google's model) | Convergent (Gemini family) |
| Crush | `CRUSH.md`, `AGENTS.md`, `CLAUDE.md` | Plain MD | Hierarchical walk; reads multiple filenames | Multi-name convergent |
| Goose | `AGENTS.md`, `goosehints` | Plain MD | Hierarchical | AGENTS.md cluster |
| Aider | `CONVENTIONS.md`, `AGENTS.md` | Plain MD | Hierarchical | Proprietary convention file |
| Junie CLI | `.junie/AGENTS.md` | Plain MD | Inside `.junie/` scoped directory | AGENTS.md variant |
| Warp | `WARP.md`, `AGENTS.md` | Plain MD | Hierarchical | Proprietary primary + AGENTS.md fallback |
| DeepAgents CLI | `AGENTS.md` | Plain MD | Hierarchical | Convergent (AGENTS.md cluster) |
| Qwen Code | `.qwen/settings.json` (refs rules), `AGENTS.md` | MD / JSON refs | Claude Code-like | Convergent (Claude Code family) |
| Amazon Q Developer | `.amazonq/rules/*.md` | Plain MD | Directory per rule | Proprietary dir |
| GitHub Copilot | `.github/copilot-instructions.md`, `AGENTS.md` | MD + frontmatter | `applyTo` | Copilot cluster |
| Augment Code | `.augment/rules/` | MD | Unknown frontmatter | Proprietary |
| Tabnine | `.tabnine/guidelines/` | MD | Unknown | Proprietary |
| Continue | `.continue/rules/` | MD | Unknown | Proprietary |
| Pi | `AGENTS.md`, `SYSTEM.md` | MD | Hierarchical | AGENTS.md cluster |
| oh-my-pi | Multi-format discovery (8+ agent formats) | MD | Reads all major formats | Meta-convergent reader |
| OpenHands | `.openhands/microagents/` (rules embedded in microagent MD) | MD + YAML frontmatter | Per-microagent | Proprietary (microagents model) |

### 1.2 Convergence Clusters

**Cluster 1 — AGENTS.md (Plain Markdown, Hierarchical)**

The largest convergence cluster in the ecosystem. At least 15 agents read an `AGENTS.md` file
from the project root as an always-on instruction file with no frontmatter requirements.
Agents in this cluster: Codex CLI, OpenCode, Amp, Goose, DeepAgents CLI, Warp, Pi, oh-my-pi,
Crush, Antigravity, Aider, Jules, Stagewise, Frontman, Bolt.new, Lovable, Replit.

This emerged organically from Codex CLI's initial `AGENTS.md` convention and propagated through
cross-tool compatibility layers. No spec governs it — it is purely adoption-by-imitation.

**Cluster 2 — YAML Frontmatter + Glob Scoping (Active Conditional Rules)**

Six agents converged on the model of YAML frontmatter controlling per-rule glob-based activation.
The field names differ but the underlying model is identical:

| Agent | Glob Field | Type |
|---|---|---|
| Claude Code | `paths:` | YAML array |
| Cline | `paths:` | YAML array |
| Cursor | `globs:` | Comma-separated string |
| Windsurf | `globs:` + `trigger:` | String + enum |
| Copilot CLI / VS Code Copilot | `applyTo:` | Comma-separated string |
| Amp | `globs:` | YAML array |

Claude Code and Cline use identical field names and types (`paths:` as a YAML array). Cursor,
Windsurf, and Copilot share the concept but renamed it. Amp matches neither family — it uses
`globs:` as an array (matching canonical shape but not Cursor's comma-string).

**Cluster 3 — AI-Decided Activation**

Only Cursor (`description:` Agent type) and Windsurf (`trigger: model_decision`) implement
this. It is a two-agent cluster with no adoption outside these two.

**Cluster 4 — Gemini Hierarchical (No Frontmatter, Named File)**

Gemini CLI, Antigravity, and several Google-adjacent tools use the `GEMINI.md` filename with
hierarchical concatenation. No frontmatter, no conditional activation. A small proprietary cluster.

### 1.3 Fragmentation Hotspots

**Directory naming is fully fragmented.** Every agent with a dedicated directory invents its own:
`.claude/rules/`, `.cursor/rules/`, `.windsurf/rules/`, `.kiro/steering/`, `.clinerules/`,
`.roo/rules/`, `.augment/rules/`, `.continue/rules/`, `.tabnine/guidelines/`, `.junie/`,
`.trae/rules/`, `.amazonq/rules/`. No two non-related agents share a rules directory name.

**Scoping vocabulary is fragmented despite shared semantics.** Every provider with conditional
activation uses a different field name and type for the same concept (glob-based activation).

**File extension fragmentation.** Cursor's `.mdc` extension is unique in the ecosystem. All other
agents use `.md` or plain text for rule files.

**AGENTS.md scoping split.** The AGENTS.md cluster is unified on the filename but split on
placement: root-level `AGENTS.md` (Codex, OpenCode, Amp, most), `.junie/AGENTS.md` (Junie),
`~/.codex/AGENTS.md` (Codex global). No shared convention on where the global file lives.

### 1.4 Conversion Leverage Points

The lowest-common-denominator rule is: **plain markdown, project root, always-on**. This is
expressible in 100% of agents. Any conversion that targets the AGENTS.md cluster or plain CLAUDE.md
files is near-lossless for basic content.

The highest-conversion-complexity path is: **Roo Code mode-scoped rules -> any other agent**.
Mode is a concept unique to Roo Code. No target has an equivalent; mode-specific rules must become
always-on or be dropped.

---

## 2. Skills

### 2.1 Actual File Paths and Formats

Skills are the fastest-growing content type (37% adoption overall, but accelerating since the
Agent Skills spec was published in December 2025). The format has converged more
than any other content type.

| Agent | File Path | Format | Frontmatter Fields | Standard or Proprietary |
|---|---|---|---|---|
| Claude Code | `.claude/skills/<name>/SKILL.md` | MD + YAML frontmatter | 12 fields (superset of spec) | Agent Skills spec + extensions |
| Cursor | `.cursor/skills/<name>/SKILL.md` | MD + YAML frontmatter | 6 fields (base + `disable-model-invocation`) | Agent Skills spec |
| Gemini CLI | `.gemini/skills/<name>/SKILL.md` | MD + YAML frontmatter | 2 fields (base only: `name`, `description`) | Agent Skills spec (base) |
| Copilot CLI | `.github/skills/<name>/SKILL.md` | MD + YAML frontmatter | 6 fields (near-CC) | Agent Skills spec |
| VS Code Copilot | `.github/skills/<name>/SKILL.md` | MD + YAML frontmatter | 6 fields (near-CC) | Agent Skills spec |
| Windsurf | `.windsurf/skills/<name>/SKILL.md` | MD + YAML frontmatter | 2 fields (base only) | Agent Skills spec (base) |
| Kiro | `.kiro/skills/<name>/SKILL.md` | MD + YAML frontmatter | 5 fields (base + `compatibility`) | Agent Skills spec |
| Codex CLI | `.agents/skills/<name>/SKILL.md` | MD + YAML frontmatter + `openai.yaml` | 2 + openai.yaml | Agent Skills spec (base) |
| Cline | `.cline/skills/<name>/SKILL.md` | MD + YAML frontmatter | 2 fields (base only) | Agent Skills spec (base) |
| OpenCode | `.opencode/skills/<name>/SKILL.md` | MD + YAML frontmatter | 5 fields (base + `compatibility`) | Agent Skills spec |
| Roo Code | `.roo/skills/<name>/SKILL.md` | MD + YAML frontmatter | 2 fields (base only) | Agent Skills spec (base) |
| Amp | `.agents/skills/<name>/SKILL.md` | MD + YAML frontmatter | 2 fields (base only) | Agent Skills spec (base) |
| Manus | Cloud upload | MD + YAML frontmatter | 2 fields (base only) | Agent Skills spec (base) |
| Crush | `.crush/skills/<name>/SKILL.md` (or AGENTS.md discovery) | MD + YAML frontmatter | Agent Skills standard | Agent Skills spec |
| Junie CLI | `.junie/skills/<name>/SKILL.md` | MD + YAML frontmatter | Agent Skills standard | Agent Skills spec |
| DeepAgents CLI | `~/.deepagents/skills/<name>/SKILL.md` | MD + YAML frontmatter | Agent Skills standard | Agent Skills spec |
| Mistral Vibe | `~/.vibe/prompts/<name>/SKILL.md` | MD + YAML frontmatter | Agent Skills standard | Agent Skills spec |
| v0 | Via Skills.sh | MD + YAML frontmatter | Agent Skills standard | Agent Skills spec |

### 2.2 The Agent Skills Standard: The Strongest Convergence in the Ecosystem

Skills represent the most successful format convergence across any content type in the coding
agent ecosystem. The Agent Skills specification (agentskills.io, published December 2025) defines:

```yaml
---
name: skill-name
description: "What this skill does and when to use it"
license: MIT
compatibility: "Requires ripgrep >= 14"
metadata:
  key: value
allowed-tools: Read Grep Glob
---
```

The `SKILL.md` filename and the `name` + `description` frontmatter fields are the convergence
nucleus — adopted by 33+ agents within three months of the spec's publication.

The community frontmatter proposal (spec v1.0.0, from `frontmatter_spec_proposal.md`) extends this
further with a `metadata` block containing:
- `skill_metadata_schema_version` — schema version for the frontmatter itself
- `provenance` — `version`, `source_repo`, `source_repo_subdirectory`, `authors`, `license_url`
- `expectations` — `software`, `services`, `programming_environments`, `operating_systems`
- `supported_agent_platforms` — array of agent names + required integrations

This proposal uses the term "supported agent platforms" which the provider-agent-naming.md
research recommends replacing with "supported agents" — but the field concept is sound and
represents the only cross-provider metadata standard in the skills space.

### 2.3 Fragmentation Hotspots

**Install directory fragmentation.** Despite format convergence, every agent uses its own directory:

| Directory Pattern | Agents |
|---|---|
| `.claude/skills/` | Claude Code |
| `.cursor/skills/` | Cursor |
| `.gemini/skills/` | Gemini CLI |
| `.github/skills/` | Copilot CLI, VS Code Copilot |
| `.windsurf/skills/` | Windsurf |
| `.kiro/skills/` | Kiro |
| `.cline/skills/` | Cline |
| `.opencode/skills/` | OpenCode |
| `.roo/skills/` | Roo Code |
| `.agents/skills/` | Amp, Codex CLI |
| `.crush/skills/` | Crush |
| `.junie/skills/` | Junie CLI |
| `~/.deepagents/skills/` | DeepAgents CLI |
| `~/.vibe/prompts/` | Mistral Vibe (proprietary path) |

All directories follow the pattern `.<agent>/skills/<name>/SKILL.md` except for the `.agents/`
consolidation (Amp, Codex CLI) and Mistral Vibe's `prompts/` deviation.

**Frontmatter field support varies by tier.** The base `name` + `description` pair is universal.
Beyond that, adoption fragments:

| Feature Tier | Fields | Adopters |
|---|---|---|
| Base spec | `name`, `description` | All 33+ agents |
| Extended spec | `compatibility`, `metadata`, `allowed-tools`, `license` | Kiro, OpenCode, Claude Code, Copilot |
| Claude Code extensions | `context`, `agent`, `model`, `effort`, `disable-model-invocation`, `user-invocable`, `argument-hint`, `hooks` | Claude Code, partially Cursor/Copilot |
| Agent Ecosystem spec metadata | `skill_metadata_schema_version`, `provenance.*`, `expectations.*`, `supported_agent_platforms` | Spec proposal only (not yet in any agent) |

**Cross-agent discovery.** Many agents discover skills from other agents' directories as a
compatibility layer: Cursor reads `.agents/`, `.claude/`, `.codex/`; Copilot reads `.claude/`;
Cline reads `.agents/`, `.claude/`. This creates an informal multi-path resolution layer that
partially mitigates directory fragmentation but creates load-order ambiguity.

---

## 3. Hooks

### 3.1 Actual File Paths and Formats

Hooks are the lowest-adoption content type (26% support) and the most fragmented in terms of
format, event naming, and configuration location.

| Agent | Config Location | Format | Event Naming Style | Can Block | Handler Types |
|---|---|---|---|---|---|
| Claude Code | `.claude/settings.json` (hooks key), `~/.claude/settings.json` | JSON (`hooks: { EventName: [{ matcher, hooks: [...] }] }`) | PascalCase: `PreToolUse`, `PostToolUse`, `SessionStart` | Yes (exit 2) | `command`, `http`, `prompt`, `agent` |
| Gemini CLI | `.gemini/settings.json` (hooks key), `~/.gemini/settings.json` | JSON (`hooks: { EventName: [{ command, args, timeout }] }`) | CamelCase: `BeforeToolExecution`, `AfterToolExecution`, `BeforeModel` | Yes (exit 2) | `command` only |
| Cursor | `.cursor/hooks.json`, `~/.cursor/hooks.json` | JSON (`{ hookName: { command, workingDir } }`) | camelCase: `beforeShellExecution`, `afterShellExecution`, `beforeReadFile` | Yes (3 of 6) | `command` only |
| Windsurf | `.windsurf/hooks.json`, enterprise `/etc/windsurf/hooks.json` | JSON (`{ hookType: [{ command, timeout }] }`) | snake_case: `pre_run_command`, `post_run_command`, `pre_write_code` | Yes (JSON response) | `command` only |
| VS Code Copilot | `.vscode/hooks.json` | JSON (`{ eventName: [{ command }] }`) | camelCase: `preToolUse`, `postToolUse`, `preUserMessage` | Yes | `command` only |
| Copilot CLI | `.github/hooks/<event>/` (directory-based) | Script files per event (executable scripts) | camelCase: `preToolUse`, `postToolUse`, `preUserMessage` | Yes | `command` only |
| Cline | `.cline/hooks/pre-tool-use/`, `.cline/hooks/post-tool-use/`, `.cline/hooks/pre-task/`, `.cline/hooks/post-task/` | Executable scripts in directories (no config file) | Directory names: `pre-tool-use`, `post-tool-use`, `pre-task`, `post-task` | Yes (throw) | Executable scripts |
| Kiro | `.kiro.hook` (YAML) / JSON | YAML or JSON with `event:`, `matcher:`, `command:` | PascalCase-ish: `PreToolUse`, `PostToolUse`, `AgentInit`, `AgentStop` | Yes (CLI mode) | `askAgent` + `command` |
| OpenCode | `.opencode/plugins/*.ts`, `~/.config/opencode/plugins/` | TypeScript/JavaScript plugin files (Bun runtime) | camelCase: `tool.execute.before`, `tool.execute.after`, `session.start` | Yes (throw) | TS/JS modules |
| Pi Agent | TypeScript extensions (QuickJS embedded) | TypeScript | camelCase lifecycle methods | Yes | TS via QuickJS |
| Qwen Code | `.qwen/settings.json` (hooks, experimental) | JSON (Claude Code-like) | PascalCase (Claude Code-like) | Unknown | `command` (experimental) |
| Amazon Q Developer | `.amazonq/hooks.json` (inferred) | JSON with matchers | camelCase: `agentSpawn`, `preToolUse` | Yes | `command` |

### 3.2 The Core Fragmentation: Event Naming

The same pre-tool event has a different name in every agent that implements it:

| Canonical Concept | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Cline |
|---|---|---|---|---|---|---|
| Before any tool runs | `PreToolUse` | `BeforeToolExecution` | `beforeShellExecution` | `pre_run_command` | `preToolUse` | `pre-tool-use/` (dir) |
| After any tool runs | `PostToolUse` | `AfterToolExecution` | `afterShellExecution` | `post_run_command` | `postToolUse` | `post-tool-use/` (dir) |
| Session starts | `SessionStart` | `SessionStart` | N/A | N/A | N/A | `pre-task/` |
| Session ends | `SessionEnd` | `SessionEnd` | N/A | N/A | N/A | `post-task/` |

The naming divergence follows case convention patterns:
- **PascalCase:** Claude Code, Kiro, Qwen Code
- **camelCase:** VS Code Copilot, OpenCode, Pi, Amazon Q
- **snake_case:** Windsurf, Cline (directory names)
- **Cursor-camelCase:** Cursor (lowercase-first compound words with "before"/"after" prefix)
- **Gemini-CamelCase:** Gemini CLI (Before/After prefix, unique vocabulary)

### 3.3 Response Shape Fragmentation

The hook response format — how a hook tells the agent to block or allow — is also fully fragmented:

| Agent | Block Decision | Input Modification |
|---|---|---|
| Claude Code | exit 2 + `{"permissionDecision": "deny"}` | `{"updatedInput": {...}}` in stdout |
| Gemini CLI | exit 2 + `{"decision": "block"}` | `{"rewrite": {...}}` in stdout |
| Cursor | `{"action": "block", "reason": "..."}` in stdout | N/A (not supported) |
| Windsurf | JSON response | N/A (not supported) |
| VS Code Copilot | exit 2 + JSON | N/A |
| Cline | throw exception | Context injection into subsequent prompt |
| OpenCode | JavaScript `throw` | Return modified value from handler |

### 3.4 Config File Format Fragmentation

| Approach | Agents |
|---|---|
| JSON key in settings.json | Claude Code, Gemini CLI, Qwen Code, Amazon Q |
| Separate hooks.json | Cursor, Windsurf, VS Code Copilot |
| Directory-based scripts | Cline, Copilot CLI |
| TypeScript plugin modules | OpenCode, Pi |
| YAML/JSON hybrid | Kiro |

### 3.5 Convergence Signals

Despite the fragmentation, two convergence signals are emerging:

**Signal 1 — VS Code Copilot adopted Claude Code naming.** When VS Code Copilot launched hooks,
it chose `preToolUse`/`postToolUse` (camelCase variant of Claude Code's PascalCase) rather than
Cursor's `beforeShellExecution` vocabulary. This suggests Claude Code's naming is winning the
conceptual vocabulary race, even if exact casing differs.

**Signal 2 — Exit code 2 as block signal.** Claude Code, Gemini CLI, and VS Code
Copilot use exit code 2 as the "block this action" signal. Cursor uses JSON output instead.
Windsurf uses JSON responses for blocking, not exit codes.
Exit 2 is the emerging standard among command-type handlers, though not yet universal.

**The gap:** There is no published cross-platform hooks specification. The sondera-ai/sondera-coding-agent-hooks
project (Cedar policy-based, 2 GitHub stars) is the only public attempt at normalization.

---

## 4. MCP

### 4.1 Actual File Paths and Formats

MCP is the second-highest adoption content type (70%) and has the most surface area for
per-field divergence despite a shared underlying protocol.

| Agent | Config File | Top-Level Key | Notable Deviations |
|---|---|---|---|
| Claude Code | `.mcp.json` (project), `~/.claude.json` (user) | `mcpServers` | `${VAR:-default}` env syntax; `type: "http"` for streamable HTTP |
| Cursor | `.cursor/mcp.json`, `~/.cursor/mcp.json` | `mcpServers` | `${env:VAR}` syntax; `type: "streamable-http"` |
| Gemini CLI | `.gemini/settings.json`, `~/.gemini/settings.json` | `mcpServers` | `httpUrl` (not `url`); `trust: bool`; `includeTools`/`excludeTools` arrays |
| Copilot CLI | `.copilot/mcp-config.json`, `~/.copilot/mcp-config.json` | `mcpServers` | `type: "http"` (matches Claude Code) |
| VS Code Copilot | `.vscode/mcp.json`, user profile `mcp.json` | `servers` (NOT `mcpServers`) | `inputs` system; `envFile`; `sandboxEnabled`; most structurally different |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` (global only) | `mcpServers` | `serverUrl` for streamable-http; `url` for SSE; `${env:VAR}` syntax |
| Kiro | `.kiro/settings/mcp.json`, `~/.kiro/settings/mcp.json` | `mcpServers` | `autoApprove: ["*"]`; `disabledTools` array |
| Codex CLI | `~/.codex/config.toml`, `.codex/config.toml` | `[mcp_servers.*]` TOML tables | Only TOML in ecosystem; `env_vars` (forward by name); `bearer_token_env_var` |
| Cline | Extension storage `cline_mcp_settings.json` | `mcpServers` | `alwaysAllow` (not `autoApprove`); global only |
| OpenCode | `opencode.json`, `opencode.jsonc` | `mcp` (NOT `mcpServers`) | `environment` (not `env`); `command` as array; `enabled` (not `disabled`); `type: "local"`/`"remote"` |
| Roo Code | `.roo/mcp.json`, extension storage `mcp_settings.json` | `mcpServers` | `alwaysAllow`; `disabledTools`; `watchPaths`; `timeout` in seconds |
| Zed | `.zed/settings.json`, `~/.config/zed/settings.json` | `context_servers` (NOT `mcpServers`) | Embedded in general settings file; `url: "custom"` sentinel |
| Amp | `.amp/settings.json`, `~/.config/amp/settings.json` | `amp.mcpServers` (namespaced) | `includeTools` with glob patterns |
| JetBrains Junie | `.junie/mcp/mcp.json` | `mcpServers` | Standard format inside Junie directory |
| Kilo Code | `.kilocode/mcp.json` | `mcpServers` | Standard format |
| Tabnine | `.tabnine/mcp_servers.json` | `mcpServers` | Standard format |

### 4.2 Convergence Clusters

**Cluster 1 — `mcpServers` + JSON (the majority cluster)**

10 of 16 agents use `mcpServers` as the top-level key in a JSON file. This is the strongest
single-field convergence in the MCP space and reflects the early adoption pattern from Claude
Code and Cursor.

**Cluster 2 — stdio + `command`/`args` (universal base)**

Every agent supports stdio transport with `command` (string) + `args` (string array). This is
the only truly universal MCP feature across all 16 agents.

**Cluster 3 — Claude Code transport naming (`type: "http"`)**

Claude Code, Copilot CLI, and VS Code Copilot use `type: "http"` for streamable HTTP. Cursor
and Roo Code use `type: "streamable-http"`. This is a direct naming collision for the same
transport — a conversion must normalize it.

**Cluster 4 — Auto-approval naming split**

The field for auto-approving tool calls splits into two camps:
- `autoApprove`: Claude Code, Cursor, Kiro
- `alwaysAllow`: Cline, Roo Code

Same concept, different names. A canonical format must pick one and map the other.

### 4.3 Fragmentation Hotspots

**Top-level key is fragmented despite being the entry point:**

| Key | Agents |
|---|---|
| `mcpServers` | Claude Code, Cursor, Gemini CLI, Copilot CLI, Windsurf, Kiro, Cline, Roo Code, JetBrains Junie, Kilo Code, Tabnine |
| `servers` | VS Code Copilot |
| `mcp` | OpenCode |
| `context_servers` | Zed |
| `amp.mcpServers` | Amp |
| TOML `[mcp_servers.*]` | Codex CLI |

**URL field naming is fragmented for HTTP transports:**

| Field | Agents |
|---|---|
| `url` | Claude Code, Cursor, Copilot CLI, VS Code Copilot, Kiro, Cline, Roo Code, Zed, OpenCode, Amp |
| `httpUrl` | Gemini CLI (uses this for ALL HTTP, not `url`) |
| `serverUrl` | Windsurf (uses this for streamable-http, `url` for SSE) |

**Format is fragmented at the container level:**

| Format | Agents |
|---|---|
| JSON standalone file | Claude Code (`.mcp.json`), Cursor, Windsurf, Kiro, Roo Code, Kilo Code, Tabnine |
| JSON embedded in settings | Gemini CLI, Zed (inside bigger settings.json) |
| JSON in custom paths | Cline (extension storage), OpenCode (opencode.json), VS Code Copilot (.vscode/) |
| JSONC | OpenCode (`opencode.jsonc` variant) |
| TOML | Codex CLI only |

**Tool filtering has 5 competing approaches:**
- `autoApprove` (allowlist, string[]): Claude Code, Cursor, Kiro
- `alwaysAllow` (allowlist, string[]): Cline, Roo Code
- `includeTools` / `excludeTools` (allowlist/denylist, string[]): Gemini CLI
- `includeTools` (glob patterns, not strings): Amp
- `disabledTools` (denylist): Kiro, Windsurf, Roo Code
- `enabled_tools` / `disabled_tools`: Codex CLI (TOML)

---

## 5. Commands

### 5.1 Actual File Paths and Formats

Commands (slash commands, custom commands, workflows) have 44% adoption. The category is in
active transition: Claude Code, Codex CLI, and Amp have merged commands into skills.

| Agent | File Path | Format | Arg Placeholder | Standard or Proprietary |
|---|---|---|---|---|
| Claude Code | `.claude/commands/<name>.md` or `.claude/skills/<name>/SKILL.md` | MD + YAML frontmatter | `$ARGUMENTS`, `$1`..`$N` | Proprietary; merging into skills |
| Cursor | `.cursor/commands/<name>.md`, `~/.cursor/commands/` | Plain MD (no frontmatter) | `$1`, `$2` | Proprietary |
| Gemini CLI | `.gemini/commands/<name>.md` | MD with TOML-like frontmatter (`title`, `description`, `prompt` fields) | `{{args}}` | Proprietary |
| Copilot CLI | `.github/commands/<name>.md` | MD + YAML frontmatter (`$ARGUMENTS`) | `$ARGUMENTS` | Convergent with Claude Code |
| VS Code Copilot | `.github/prompts/<name>.prompt.md` | MD + YAML frontmatter (`mode:`, `tools:`, `description:`, `${input:varName}`) | `${input:varName}` | Proprietary (.prompt.md extension) |
| Windsurf | `.windsurf/workflows/<name>.md` | Plain MD (structured sections: Description, Instructions, Steps) | None (embedded) | Proprietary (workflow model) |
| Kiro | Indirect via hooks/steering | Frontmatter-based activation | N/A | Not a standalone content type |
| Codex CLI | `~/.codex/prompts/<name>.md` (global only) | Plain MD + optional frontmatter | `$ARGUMENTS`, `$1`..`$9` | Proprietary; merging into skills |
| Cline | `.cline/workflows/<name>.md` | Plain MD | N/A | Proprietary |
| OpenCode | `.opencode/commands/<name>.md` | MD + optional YAML frontmatter | `$ARGUMENTS`, `$1`..`$N` | Convergent with Claude Code |
| Roo Code | `.roo/commands/<name>.md` | MD + YAML frontmatter | `$ARGUMENTS` | Convergent with Claude Code |
| Zed | Extension-based (WASM/extension.toml) | Extension manifest | Varies | Proprietary (extension model) |
| Amp | Deprecated (use skills instead) | Formerly plain MD | N/A | Deprecated |
| Tabnine | `.tabnine/agent/commands/` | MD | Unknown | Proprietary |

### 5.2 Convergence Clusters

**Cluster 1 — `$ARGUMENTS` Markdown + YAML Frontmatter**

Claude Code, Copilot CLI, OpenCode, and Roo Code all use `.md` files with YAML frontmatter
and `$ARGUMENTS` as the placeholder for user-provided arguments. This is the most convergent
commands cluster. The directory names differ (`.claude/commands/`, `.github/commands/`,
`.opencode/commands/`, `.roo/commands/`) but the file format and arg syntax are identical.

**Cluster 2 — Skills as Commands**

Claude Code, Codex CLI, Amp, and increasingly Cursor are converging commands onto the Agent
Skills format. A skill with `user-invocable: true` is functionally a command. This means
the skills convergence cluster absorbs the commands space over time — commands as a distinct
format are a transitional state.

### 5.3 Fragmentation Hotspots

**Gemini CLI's TOML format is a standalone outlier.** No other agent uses TOML for commands.
The `{{args}}` placeholder is also unique.

**VS Code Copilot's `.prompt.md` extension is unique.** The `${input:varName}` interpolation
system (with typed prompts) has no equivalent elsewhere.

**Windsurf's workflow model is architecturally distinct.** Workflows are structured step
definitions rather than prompt templates. No arg placeholder; behavior is embedded in steps.

**Invocation syntax is split:**
- `/name` (most agents)
- `/prompts:name` (Codex CLI legacy)
- `/skills:name` (emerging in skills-merged systems)
- `/filename.md` (Cline literal filename reference)

---

## 6. Agents

### 6.1 Actual File Paths and Formats

Agents (sub-agents, custom agents, custom modes) have 63% adoption. The content type
has the most architectural divergence — three distinct models coexist.

| Agent | File Path | Format | Key Fields | Model |
|---|---|---|---|---|
| Claude Code | `.claude/agents/<name>.md`, `~/.claude/agents/` | MD + YAML frontmatter | `name`, `description`, `tools`, `disallowedTools`, `model`, `permissionMode`, `maxTurns`, `skills`, `mcpServers`, `hooks`, `memory`, `background`, `isolation`, `effort`, `color` | Markdown + YAML (most fields) |
| Cursor | `.cursor/agents/<name>.md`, `~/.cursor/agents/` | MD + YAML frontmatter | `name`, `description`, `tools`, `model` | Markdown + YAML |
| Gemini CLI | `.gemini/agents/<name>.md`, `~/.gemini/agents/` | MD + YAML frontmatter | `name`, `description`, `tools`, `model` | Markdown + YAML |
| Copilot CLI | `.github/agents/<name>.md` | MD + YAML frontmatter | `name`, `description`, `tools`, `model` | Markdown + YAML |
| VS Code Copilot | `.github/agents/<name>.md` | MD + YAML frontmatter | `name`, `description`, `tools`, `model`, `instructions` | Markdown + YAML |
| Kiro (IDE) | `.kiro/agents/<name>.md` | MD + YAML frontmatter | `name`, `description`, `tools`, `model` | Markdown + YAML |
| Kiro (CLI) | `.kiro/agents/<name>.json` | JSON | Full agent config | JSON only |
| Codex CLI | `.codex/agents/<name>.toml` | TOML | `name`, `description`, `model`, `tools`, `system_prompt` | TOML only |
| OpenCode | `.opencode/agents/<name>.md` | MD + YAML frontmatter | `name`, `description`, `tools`, `model` | Markdown + YAML |
| Roo Code | `.roo/modes/` or `.roomodes` | YAML/JSON (`customModes` array) | `slug`, `name`, `roleDefinition`, `groups`, `customInstructions` | YAML/JSON mode definitions |
| Cline | N/A (Plan/Act built-in modes) | N/A | N/A | No user-defined agents |
| Zed | `settings.json` (`assistant.agent_profiles`) | JSON embedded in settings | Profile with tool enable/disable | Settings-embedded JSON |
| Amp | `.agents/` (AGENTS.md / SKILL.md) | MD | Via skills delegation | No standalone agent files |
| OpenHands | `.openhands/microagents/<name>.md` | MD + YAML frontmatter | `name`, `type`, `agent`, `triggers`, `instructions` | Microagent model |
| Warp | Agent profiles (JSON) | JSON | Cloud + local profiles | Proprietary cloud |
| Devin | Child sessions (API) | API-based | Per-task configuration | No files |

### 6.2 Convergence Clusters

**Cluster 1 — Markdown + YAML Frontmatter (the dominant cluster)**

The strongest convergence in the agents space: Claude Code, Cursor, Gemini CLI, Copilot CLI,
VS Code Copilot, Kiro (IDE), and OpenCode all use `.md` files with YAML frontmatter in
agent-specific directories. The directory pattern is `.<agent>/agents/<name>.md`.

Core fields shared across this cluster:

| Field | Agents |
|---|---|
| `name` | Claude Code, Cursor, Gemini CLI, Copilot CLI, VS Code Copilot, Kiro, OpenCode |
| `description` | All of the above |
| `tools` (allowlist) | Claude Code, Cursor, Gemini CLI, Kiro, OpenCode |
| `model` | Claude Code, Cursor, Gemini CLI, Copilot CLI, VS Code Copilot, Kiro, OpenCode |

The directory path follows a consistent template even though the agent prefix changes:
```
.claude/agents/     (Claude Code)
.cursor/agents/     (Cursor)
.gemini/agents/     (Gemini CLI)
.github/agents/     (Copilot CLI, VS Code Copilot)
.kiro/agents/       (Kiro IDE)
.opencode/agents/   (OpenCode)
```

**Cluster 2 — JSON/YAML Mode Definitions**

Roo Code (`.roomodes` / `.roo/modes/`) and Kiro CLI (`.kiro/agents/*.json`) use structured
data files rather than markdown bodies for agent definitions. These are architecturally different
— the "system prompt" is a field value, not the document body.

### 6.3 Fragmentation Hotspots

**Codex CLI's TOML agent format is isolated.** The `.codex/agents/<name>.toml` approach with
a `system_prompt` field has no equivalent in any other agent.

**Roo Code's mode model is architecturally distinct.** Modes define `groups` of tools
(read-only, write, etc.) and use a `roleDefinition` rather than a markdown body. No other agent
has this model.

**Claude Code has 10 agent-specific fields with no cross-agent equivalents.** Fields like
`permissionMode`, `maxTurns`, `memory`, `background`, `isolation`, `effort`, `color`, and
`skills` reference Claude Code-specific runtime concepts. Conversion to other agents requires
dropping these with warnings.

**Kiro's dual format (MD for IDE, JSON for CLI) means Kiro agents are not self-consistent.**
An agent defined for the Kiro IDE cannot be directly loaded by the Kiro CLI without conversion.

**OpenHands' microagent model is semantically different.** A microagent has `triggers` (what
causes the agent to activate) and `type` (`repo` or `knowledge`), which has no equivalent in
the MD+YAML cluster.

---

## 7. Summary: Convergence Map

### Convergence Scores by Content Type

| Content Type | Format Convergence | Path Convergence | Overall |
|---|---|---|---|
| Skills | **High** — SKILL.md + YAML frontmatter adopted by 33+ agents | Low — every agent uses `.<agent>/skills/` | Medium-High |
| Agents | **Medium** — MD + YAML dominant (7 agents), but 3 structural outliers | Low — every agent uses `.<agent>/agents/` | Medium |
| MCP | **Low** — `mcpServers` key widely used but format details diverge | Low — each agent has its own path | Low-Medium |
| Rules | **Low** — AGENTS.md cluster for plain files; 6 agents with similar YAML scoping | Very Low — no shared directory name | Low |
| Commands | **Low** — `$ARGUMENTS` + MD cluster for 4 agents; rest fragmented | Very Low — all different | Low |
| Hooks | **Very Low** — no shared event naming, response format, or config location | Very Low — all different | Very Low |

### The Three Convergence Patterns

**Pattern A — Spec-driven convergence (Skills):** A published specification drives rapid
adoption. The Agent Skills spec achieved 33+ adopters in 3 months. The format nucleus
(`SKILL.md` + `name`/`description`) is stable; the extension layer is still fragmented.
This is the healthiest convergence pattern.

**Pattern B — Imitation convergence (AGENTS.md, MCP `mcpServers`):** One agent establishes
a convention (Codex CLI's `AGENTS.md`, Claude Code's `mcpServers`), and other agents copy it
for compatibility. The convention is stable at the surface but does not standardize the full
feature set. Many agents read `AGENTS.md` but none add the same frontmatter features to it.

**Pattern C — Parallel evolution (rules scoping, hook events):** Multiple agents independently
invent the same capability with different names. The concept converges (glob-based rule
activation, pre-tool hooks) but the vocabulary does not. This is the hardest pattern for
syllago to convert — every pair requires a bespoke field mapping.

### Conversion Complexity by Content Type

| Content Type | Conversion Notes |
|---|---|
| Skills | Near-lossless between base-spec adopters; lossy when Claude Code extensions used |
| MCP | Structurally convertible; many field renames; transport type values collide |
| Agents | MD+YAML cluster converts well internally; Codex TOML, Roo modes, OpenHands require special cases |
| Rules | AGENTS.md to/from always-on: lossless. Frontmatter-based conditional rules: medium-high loss to plain-MD targets |
| Commands | `$ARGUMENTS` cluster: lossless between 4 agents. Gemini TOML: requires field mapping. Windsurf workflows: architecturally incompatible |
| Hooks | Any conversion: lossy. Event names must be looked up per-pair. Response shapes incompatible. Claude Code `prompt`/`agent`/`http` handler types have no cross-agent equivalents |

### Fragmentation Hotspots Requiring Immediate Spec Work

1. **Hook event naming** — No published canonical vocabulary. The community has proposed
   snake_case (`before_tool_execute`, `after_tool_execute`) but no spec adopts it yet. This
   is the highest-priority fragmentation problem in the ecosystem.

2. **MCP transport type value collision** — `"http"` vs `"streamable-http"` for the same
   transport. Silent data loss in conversion. Needs a normalization decision.

3. **MCP auto-approval field name split** — `autoApprove` vs `alwaysAllow`. Simple rename,
   but needs to be in a canonical schema to prevent bugs.

4. **Agent frontmatter field proliferation** — Claude Code's 15 agent fields are mostly
   proprietary. The community cluster (7 agents) shares only 4. A minimal shared schema
   needs to be defined before this diverges further.

5. **Skills `supported_agents` field** — The frontmatter spec proposal uses `supported_agent_platforms`
   (string names only). The provider-agent-naming.md research recommends `supported_agents` as
   the term. This needs to be resolved in the spec before any agent adopts the metadata block.

---

## Appendix: Adoption Matrix Reference

The following matrix is derived from the Content Type Support Matrix in
`provider-agent-naming.md`. Agents are listed with their supported content types for
quick cross-reference.

**Full support (6/6 content types):**
Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands

**5/6 support:**
Claude Code (reference), Codex CLI, Gemini CLI, Copilot CLI, VS Code Copilot (hooks preview),
Amazon Q Developer, OpenCode

**4/6 support:**
Windsurf, Kiro, Antigravity, Trae, v0, Pi, Qodo, Factory Droid

**3/6 support:**
Aider, Warp, JetBrains Junie, Augment Code, Gemini Code Assist, DeepAgents CLI,
Manus, Open SWE, Devin

**1-2/6 support:**
Goose, Herm, Mistral Vibe, Open Interpreter, GitHub Copilot, Kilo Code, Continue,
Tabnine, Zencoder, CodeGPT, Replit, Jules, agenticSeek

**0/6 or specialized:**
Supermaven, Bolt.new, Lovable, v0 (pre-2026), CodeGeeX, AutoCodeRover, SWE-agent,
all code review specialists (CodeRabbit, Greptile, etc.)
