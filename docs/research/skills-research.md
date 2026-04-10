# Skills: Cross-Provider Deep Dive

> Research compiled 2026-03-22. Covers skills systems across 14 AI coding tools.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview](#overview)
2. [Per-Provider Deep Dive](#per-provider-deep-dive)
3. [Cross-Platform Normalization Problem](#cross-platform-normalization-problem)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#featurecapability-matrix)
6. [Compat Scoring Implications](#compat-scoring-implications)
7. [Converter Coverage Audit](#converter-coverage-audit)

---

## Overview

Skills are reusable, invocable instruction bundles that extend an AI coding assistant's capabilities. Unlike rules (passive context) or agents (autonomous actors), skills are typically triggered on-demand — either by the user (slash command) or the model (tool-like invocation). They carry metadata about tool access, invocation mode, and execution context.

This is the MOST COMPLEX content type for cross-provider portability. The Agent Skills specification ([agentskills.io](https://agentskills.io/specification)) was published by Anthropic in December 2025 and adopted by 16+ tools by March 2026. However, providers implement different subsets — Claude Code extends the spec with 7+ fields beyond the base standard, while most providers only support the base `name` + `description` fields. The community flags this spec divergence as a major pain point.

### The Agent Skills Standard (agentskills.io)

The open specification defines these frontmatter fields:

| Field | Required | Constraints |
|-------|----------|-------------|
| `name` | Yes | 1-64 chars, lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens, must match directory name |
| `description` | Yes | 1-1024 chars, describes what + when |
| `license` | No | License name or reference to bundled file |
| `compatibility` | No | 1-500 chars, environment requirements |
| `metadata` | No | Arbitrary string-to-string key-value map |
| `allowed-tools` | No | Space-delimited tool list (experimental) |

Claude Code extends this with `disallowed-tools`, `context`, `agent`, `model`, `effort`, `disable-model-invocation`, `user-invocable`, `argument-hint`, and `hooks`. VS Code Copilot and Copilot CLI have adopted several of these extensions. Other providers generally ignore unknown fields.

### Summary Table

| Provider | Has Skills? | Format | FM Fields | Primary Install Path | Invocation | Cross-Agent Paths |
|----------|-------------|--------|-----------|---------------------|------------|-------------------|
| Claude Code | Yes | SKILL.md + YAML FM | 12 (superset) | `.claude/skills/` | Slash + model | N/A (is the reference) |
| Cursor | Yes | SKILL.md + YAML FM | 6 (base + DMI) | `.cursor/skills/` | Slash + model | `.agents/`, `.claude/`, `.codex/` |
| Gemini CLI | Yes | SKILL.md + YAML FM | 2 (base only) | `.gemini/skills/` | `/skills` + model | `.agents/` |
| Copilot CLI | Yes | SKILL.md + YAML FM | 6 (near-CC) | `.github/skills/` | Slash + model | `.claude/` |
| VS Code Copilot | Yes | SKILL.md + YAML FM | 6 (near-CC) | `.github/skills/` | Slash + model | `.claude/`, `.agents/` |
| Windsurf | Yes | SKILL.md + YAML FM | 2 (base only) | `.windsurf/skills/` | `@mention` + model | `.agents/`, `.claude/` |
| Kiro | Yes | SKILL.md + YAML FM | 5 (base + compat) | `.kiro/skills/` | Slash + model | N/A |
| Codex CLI | Yes | SKILL.md + YAML FM | 2 + openai.yaml | `.agents/skills/` | Slash + model | `.claude/` |
| Cline | Yes | SKILL.md + YAML FM | 2 (base only) | `.cline/skills/` | `use_skill` tool | `.agents/`, `.claude/` |
| OpenCode | Yes | SKILL.md + YAML FM | 5 (base + compat) | `.opencode/skills/` | `skill` tool | `.agents/`, `.claude/` |
| Roo Code | Yes | SKILL.md + YAML FM | 2 (base only) | `.roo/skills/` | Model + discovery | `.agents/` |
| Zed | No | N/A | N/A | N/A | N/A | N/A |
| Amp | Yes | SKILL.md + YAML FM | 2 (base only) | `.agents/skills/` | Model + discovery | `.claude/` |
| Manus | Yes | SKILL.md + YAML FM | 2 (base only) | Cloud upload | `/SKILL_NAME` | N/A |

---

## Per-Provider Deep Dive

### Claude Code

**Status:** Full support — reference implementation and superset of Agent Skills standard.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Enterprise: Managed settings (org-wide)
- Personal: `~/.claude/skills/<skill-name>/SKILL.md`
- Project: `.claude/skills/<skill-name>/SKILL.md`
- Plugin: `<plugin>/skills/<skill-name>/SKILL.md`
- Nested: Autodiscovery from subdirectory `.claude/skills/` (monorepo support)
- Additional: `--add-dir` directories with live change detection

**Priority:** Enterprise > Personal > Project. Plugin skills namespaced as `plugin-name:skill-name`.

**Frontmatter fields (12 total):**

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `name` | No (uses dir name) | string | Display name, becomes `/slash-command`. Max 64 chars, lowercase+hyphens. |
| `description` | Recommended | string | Max 1024 chars. Primary trigger mechanism for model discovery. |
| `allowed-tools` | No | string (space/comma-delimited) | Tools Claude can use without asking permission when skill active |
| `disallowed-tools` | No | string (space/comma-delimited) | Tools explicitly forbidden during skill execution |
| `context` | No | string | Set to `fork` to run in isolated subagent context |
| `agent` | No | string | Subagent type when `context: fork` (e.g., `Explore`, `Plan`, custom) |
| `model` | No | string | Model override for skill execution |
| `effort` | No | string | Reasoning effort: `low`, `medium`, `high`, `max` |
| `disable-model-invocation` | No | bool | If true, only user can invoke via `/name`. Removes from model context. |
| `user-invocable` | No | bool | If false, hidden from `/` menu. Model can still invoke. Default: true. |
| `argument-hint` | No | string | Placeholder shown during autocomplete (e.g., `[issue-number]`) |
| `hooks` | No | object | Skill-scoped lifecycle hooks (PreToolUse, PostToolUse, etc.) |

**Invocation control matrix:**

| Frontmatter | User can invoke | Model can invoke | Context loading |
|-------------|----------------|-----------------|-----------------|
| (default) | Yes | Yes | Description always loaded, full skill on invoke |
| `disable-model-invocation: true` | Yes | No | Description NOT in context |
| `user-invocable: false` | No | Yes | Description always loaded |

**String substitutions:** `$ARGUMENTS`, `$ARGUMENTS[N]`/`$N`, `${CLAUDE_SESSION_ID}`, `${CLAUDE_SKILL_DIR}`

**Dynamic context injection:** `` !`<command>` `` syntax runs shell commands as preprocessing, output replaces placeholder before Claude sees it.

**Hooks:** Skill-scoped hooks are UNIQUE to Claude Code. Hooks fire shell commands on lifecycle events scoped to the skill execution only. No other provider supports skill-level hook scoping.

**Built-in tool names:** Read, Write, Edit, Bash, Glob, Grep, WebSearch, WebFetch, TodoRead, TodoWrite, Agent, NotebookEdit

**Source:** [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

---

### Cursor

**Status:** Full SKILL.md support with partial frontmatter. Adopted Agent Skills standard.

**Format:** SKILL.md files with YAML frontmatter + markdown body. Previously used `.cursorrules` and `.mdc` files — now migrating to SKILL.md. Built-in `/migrate-to-skills` skill for conversion.

**Install locations:**
- Project: `.cursor/skills/`, `.agents/skills/`
- Global: `~/.cursor/skills/`
- Legacy/compat: `.claude/skills/`, `.codex/skills/`, `~/.claude/skills/`, `~/.codex/skills/`

**Frontmatter fields (6 recognized):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Must match parent directory name |
| `description` | Yes | Max 1024 chars, drives model discovery |
| `license` | No | License reference |
| `compatibility` | No | Environment requirements |
| `metadata` | No | Arbitrary key-value pairs |
| `disable-model-invocation` | No | Prevents auto-invocation when true |

**Fields NOT supported:** `allowed-tools`, `disallowed-tools`, `context`, `agent`, `model`, `effort`, `user-invocable`, `argument-hint`, `hooks`. Unknown fields are ignored.

**Invocation:** Slash command via `/` menu in Agent chat, or automatic model invocation based on description matching. Progressive disclosure: only name+description loaded at startup.

**Source:** [cursor.com/docs/context/skills](https://cursor.com/docs/context/skills)

---

### Gemini CLI

**Status:** Full SKILL.md support with minimal frontmatter. Unique consent-based activation model.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Workspace: `.gemini/skills/` or `.agents/skills/` (`.agents/` takes precedence)
- User: `~/.gemini/skills/` or `~/.agents/skills/`
- Extension: Bundled within installed Gemini extensions
- Distribution: Installable as `.skill` zip files, Git repos, or local directories

**Frontmatter fields (2 recognized):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Must match directory name |
| `description` | Yes | Drives model discovery and consent prompt text |

Gemini CLI only documents `name` and `description`. No additional fields from the Agent Skills spec (license, compatibility, metadata) are confirmed as parsed — unknown fields are likely ignored.

**Unique: Consent model.** When Gemini decides to use a skill, a confirmation prompt appears in the UI showing the skill's name, purpose, and directory path it will access. User must approve. This is unique among providers — others auto-activate or only require opt-in at install time.

**Unique: `activate_skill` tool.** Internally, Gemini treats skill activation as a tool call. The model calls `activate_skill` when it determines a skill matches the task.

**Session commands:**
- `/skills list`, `/skills link <path>`, `/skills enable|disable <name>`, `/skills reload`

**CLI commands:**
- `gemini skills list|link|install|uninstall|enable|disable`

**Source:** [geminicli.com/docs/cli/skills](https://geminicli.com/docs/cli/skills/)

---

### Copilot CLI (GitHub Copilot CLI)

**Status:** Full SKILL.md support. Adopts several Claude Code extensions beyond the base spec.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Project: `.github/skills/<skill-name>/`, `.claude/skills/<skill-name>/`
- Personal: `~/.copilot/skills/<skill-name>/`, `~/.claude/skills/<skill-name>/`

**Frontmatter fields (6 recognized):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Lowercase, hyphens for spaces |
| `description` | Yes | Max 1024 chars, drives discovery |
| `license` | No | License reference |
| `argument-hint` | No | Shown in chat input on invocation |
| `user-invocable` | No | Controls slash command menu visibility (default: true) |
| `disable-model-invocation` | No | Prevents agent auto-loading |

**Fields NOT confirmed:** `allowed-tools`, `disallowed-tools`, `context`, `agent`, `model`, `effort`, `hooks`. The Copilot CLI changelog notes "suppress unknown field warnings in skill and command frontmatter" — suggesting these fields are tolerated but not acted upon.

**Session commands:**
- `/skills list`, `/skills`, `/skills info`, `/skills add`, `/skills reload`, `/skills remove`

**Source:** [docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills)

---

### VS Code Copilot

**Status:** Full SKILL.md support. Shares implementation with Copilot CLI; supports the same extended fields.

**Format:** SKILL.md files with YAML frontmatter + markdown body. Works in Agent mode.

**Install locations:**
- Project: `.github/skills/`, `.claude/skills/`, `.agents/skills/`
- Personal: `~/.copilot/skills/`, `~/.claude/skills/`

**Frontmatter fields (6 recognized):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Must match parent directory name |
| `description` | Yes | Max 1024 chars |
| `argument-hint` | No | Hint shown in chat input |
| `user-invocable` | No | Controls `/` menu visibility |
| `disable-model-invocation` | No | Controls agent auto-loading |
| `license` | No | License reference |

**Known bug:** The VS Code extension's SKILL.md validator rejects several valid frontmatter fields that work in the CLI, including `allowed-tools`, `context`, `agent`, `model`, `hooks`. The validator only recognizes base Agent Skills spec fields plus a few extensions. Reported as [issue #30611](https://github.com/anthropics/claude-code/issues/30611).

**Source:** [code.visualstudio.com/docs/copilot/customization/agent-skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills)

---

### Windsurf

**Status:** Full SKILL.md support with minimal frontmatter. Enterprise system-level deployment.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Workspace: `.windsurf/skills/<skill-name>/`
- Global: `~/.codeium/windsurf/skills/<skill-name>/`
- System/Enterprise (read-only):
  - macOS: `/Library/Application Support/Windsurf/skills/`
  - Linux/WSL: `/etc/windsurf/skills/`
  - Windows: `C:\ProgramData\Windsurf\skills\`
- Cross-agent: `.agents/skills/`, `~/.agents/skills/`
- Claude compat: `.claude/skills/`, `~/.claude/skills/`

**Frontmatter fields (2 confirmed):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Lowercase, hyphens, numbers |
| `description` | Yes | Drives model discovery |

Windsurf documentation only confirms `name` and `description`. No additional Agent Skills spec fields (license, compatibility, metadata) are documented as parsed.

**Invocation:**
- Automatic: Cascade model determines relevance from description (primary method)
- Manual: `@skill-name` in Cascade input
- UI: Create via Cascade panel customizations menu

**Unique: Enterprise MDM deployment.** System-level skill paths for read-only enterprise distribution via MDM (Mobile Device Management). Unique among providers.

**Added:** March 9, 2026 (relatively recent adoption).

**Source:** [docs.windsurf.com/windsurf/cascade/skills](https://docs.windsurf.com/windsurf/cascade/skills)

---

### Kiro

**Status:** Full SKILL.md support via Agent Skills standard. Also has separate "Steering" system (Kiro-specific).

**Important:** Kiro has TWO systems: Agent Skills (open standard, SKILL.md) and Steering (Kiro-specific, `.kiro/steering/`). The syllago converter currently treats Kiro as plain-markdown-only — this is outdated. Kiro adopted the Agent Skills standard in February 2026.

**Format:** SKILL.md files with YAML frontmatter + markdown body (for Agent Skills). Plain markdown with optional inclusion-mode frontmatter (for Steering).

**Install locations (Agent Skills):**
- Workspace: `.kiro/skills/<skill-name>/SKILL.md`
- Global: `~/.kiro/skills/<skill-name>/SKILL.md`
- Priority: Workspace overrides global on name conflict

**Install locations (Steering — separate system):**
- Workspace: `.kiro/steering/`
- Global: `~/.kiro/steering/`
- Inclusion modes: `always`, `fileMatch`, `auto`, `manual`

**Frontmatter fields (Agent Skills, 5 recognized):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Max 64 chars, lowercase+hyphens, must match folder name |
| `description` | Yes | Max 1024 chars |
| `license` | No | License reference |
| `compatibility` | No | Environment requirements |
| `metadata` | No | Key-value pairs (author, version) |

**Invocation:** Automatic (description matching) or manual via `/` slash commands in chat.

**Source:** [kiro.dev/docs/skills](https://kiro.dev/docs/skills/), [kiro.dev/docs/steering](https://kiro.dev/docs/steering/)

---

### Codex CLI (OpenAI)

**Status:** Full SKILL.md support. Minimal frontmatter with separate `agents/openai.yaml` sidecar for extended metadata.

**Format:** SKILL.md files with YAML frontmatter + markdown body. OpenAI explicitly advises: "Do not include any other fields in YAML frontmatter" beyond name and description.

**Install locations (precedence order):**
1. `$CWD/.agents/skills/` (current directory, walks up to repo root)
2. `$HOME/.agents/skills/` (user)
3. `/etc/codex/skills/` (system/admin)
4. Bundled system skills
5. Also discovers: `.claude/skills/`

**Frontmatter fields (2 in SKILL.md):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill identifier |
| `description` | Yes | Trigger description — "explain exactly when this skill should and should not trigger" |

**Sidecar: `agents/openai.yaml`** — Codex-specific configuration alongside SKILL.md:

```yaml
interface:
  display_name: "User-facing name"
  short_description: "User-facing description"
  icon_small: "./assets/small-logo.svg"
  icon_large: "./assets/large-logo.png"
  brand_color: "#3B82F6"
  default_prompt: "Surrounding prompt context"

policy:
  allow_implicit_invocation: true  # boolean

dependencies:
  tools:
    - type: "mcp"
      value: "tool identifier"
      description: "Tool description"
      transport: "streamable_http"
      url: "endpoint URL"
```

This sidecar is unique to Codex — no other provider uses it. It provides UI metadata (icons, colors), invocation policy, and MCP tool dependencies.

**Config override:** `~/.codex/config.toml` with `[[skills.config]]` entries to enable/disable skills.

**Source:** [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills)

---

### Cline

**Status:** Full SKILL.md support. Experimental feature (must be enabled in settings).

**Format:** SKILL.md files with YAML frontmatter + markdown body. Uses `use_skill` tool internally.

**Install locations:**
- Project: `.cline/skills/`, `.clinerules/skills/`, `.claude/skills/`
- Global: `~/.cline/skills/` (macOS/Linux), `C:\Users\USERNAME\.cline\skills\` (Windows)
- Cross-agent: `.agents/skills/`
- Priority: Global overrides identically-named project skills (opposite of most providers)

**Frontmatter fields (2 confirmed):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Must match directory name, kebab-case |
| `description` | Yes | Max 1024 chars, drives activation |

**Unique: `use_skill` tool.** When request matches a skill description, Cline activates via an internal `use_skill` tool call. This is similar to Gemini's `activate_skill` but without the consent prompt.

**Unique: Toggle UI.** Individual skills can be enabled/disabled via the Cline settings panel without deleting them. No config file editing needed.

**Note:** Cline also has a separate "Custom Modes" system (Plan/Act modes) which is distinct from skills. Skills and modes are orthogonal features.

**Source:** [docs.cline.bot/customization/skills](https://docs.cline.bot/customization/skills)

---

### OpenCode

**Status:** Full SKILL.md support with standard Agent Skills fields.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations (walks up from CWD to git root):**
- Project: `.opencode/skills/<name>/SKILL.md`
- Project compat: `.claude/skills/<name>/`, `.agents/skills/<name>/`
- Global: `~/.config/opencode/skills/<name>/SKILL.md`
- Global compat: `~/.claude/skills/<name>/`, `~/.agents/skills/<name>/`
- Priority: Project overrides global (last wins)

**Frontmatter fields (5 recognized, unknown fields ignored):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | 1-64 chars, pattern `^[a-z0-9]+(-[a-z0-9]+)*$` |
| `description` | Yes | 1-1024 chars |
| `license` | No | License identifier |
| `compatibility` | No | Compatibility marker |
| `metadata` | No | String-to-string key-value pairs |

**Invocation:** Agents invoke via `skill({ name: "skill-name" })` tool call. Skills visible in tool description with name+description pairs.

**Unique: Pattern-based permissions.** `opencode.json` controls skill access with `allow`, `deny`, or `ask` states supporting wildcards (e.g., `internal-*`). Per-agent skill overrides possible.

**Source:** [opencode.ai/docs/skills](https://opencode.ai/docs/skills/)

---

### Roo Code

**Status:** Full SKILL.md support. Fork of Cline with mode-specific skill scoping.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Global (Roo-specific, highest priority): `~/.roo/skills/<skill-name>/`
- Global (cross-agent): `~/.agents/skills/<skill-name>/`
- Project (Roo-specific): `.roo/skills/<skill-name>/`
- Project (cross-agent): `.agents/skills/<skill-name>/`
- Mode-specific variants: `.roo/skills-code/`, `.roo/skills-architect/`, `.agents/skills-{modeSlug}/`
- Priority: Project overrides global; `.roo/` overrides `.agents/` at same level

**Frontmatter fields (2 confirmed):**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | 1-64 chars, lowercase alphanumeric + hyphens, must match directory |
| `description` | Yes | 1-1024 chars |

**Unique: Mode-specific skill directories.** Skills in `.roo/skills-code/` only load when "code" mode is active. Skills in base `.roo/skills/` load in all modes. This is a filesystem-level scoping mechanism unique to Roo Code. Also introduced `modeSlugs` frontmatter array in v3.47.0 (Feb 2026) for multi-mode targeting, but this is not part of the Agent Skills spec.

**Unique: `.roomodes` integration.** Custom modes defined in `.roomodes` (YAML/JSON) can reference skills. The Kastalien-Research/rooskills project converts Agent Skills into Roo Code custom modes.

**Source:** [docs.roocode.com/features/skills](https://docs.roocode.com/features/skills)

---

### Zed

**Status:** No SKILL.md support. Uses tools, MCP servers, and profiles instead.

Zed does not implement the Agent Skills standard. Its AI assistant (Agent Panel) uses a fundamentally different extensibility model:

- **Built-in tools:** Search, file editing, terminal, diagnostics
- **MCP Servers:** External tool providers
- **Slash commands:** `/file`, `/tab`, `/diagnostics`, `/selection`, `/terminal`, `/now`, `/prompt`, `/fetch`, `/workflow`
- **Custom profiles:** Group different tool combinations
- **External agents:** Supports Claude Code, Gemini CLI, Codex CLI, OpenCode via ACP (Agent Client Protocol)

Zed's slash commands are hard-coded or extension-provided — they don't read SKILL.md files. When using external agents through ACP, the agent's own skill system applies, but Zed itself has no skill discovery.

**Converter implication:** No skill conversion target. Skills targeting Zed should degrade to rules/context.

**Source:** [zed.dev/docs/ai/agent-panel](https://zed.dev/docs/ai/agent-panel)

---

### Amp

**Status:** Full SKILL.md support with base Agent Skills fields.

**Format:** SKILL.md files with YAML frontmatter + markdown body.

**Install locations:**
- Workspace: `.agents/skills/`
- User: `~/.config/agents/skills/`
- Compat: `.claude/skills/`, `~/.claude/skills/`

**Frontmatter fields:** Amp follows the base Agent Skills spec (`name`, `description`). No documentation of extended fields.

**Invocation:** Model determines relevance from description (automatic). Skills lazily load instructions on demand.

**Built-in reference skills:** Agent Sandbox, Agent Skill Creator, BigQuery, Tmux, Web Browser.

**Note:** Amp uses `.AGENT.md` files for rules/constraints (similar to CLAUDE.md). Sub-agents handle specialized tasks. Skills and agent files are distinct systems.

**Source:** [ampcode.com/news/agent-skills](https://ampcode.com/news/agent-skills)

---

### Manus

**Status:** Full SKILL.md support. Cloud-based platform with unique installation model.

**Format:** SKILL.md files with YAML frontmatter + markdown body, but accessed through Manus's cloud sandbox rather than local filesystem.

**Installation methods:**
1. Build with Manus: "Package this workflow into a Skill" after a successful task
2. Upload: `.zip`, `.skill` file, or folder
3. Import from GitHub: Paste repository link
4. Official Library: Curated skills from Manus team

**Invocation:** `/SKILL_NAME` command in Manus interface. Also supports automatic triggering via description matching.

**Frontmatter fields:** Base Agent Skills spec (`name`, `description`). Manus runs in isolated sandbox environments (full Ubuntu) that can execute Python/Bash scripts from skill directories.

**Unique: Workflow capture.** Manus can analyze a successful interaction and auto-generate a SKILL.md packaging the workflow. No other provider offers this.

**Note on acquisition:** Manus was acquired by Meta Platforms. Roadmap includes project-level skill integration and team skill libraries for enterprise.

**Source:** [manus.im/blog/manus-skills](https://manus.im/blog/manus-skills)

---

## Cross-Platform Normalization Problem

### What Differs

**1. Frontmatter field support is a spectrum, not a binary.**

The Agent Skills spec defines 6 fields. Claude Code extends to 12. Other providers land somewhere in between. The practical impact:

| Field | Claude Code | Copilot/VS Code | Cursor | Gemini/Windsurf/Cline/Roo/Amp | Codex | OpenCode/Kiro |
|-------|-------------|-----------------|--------|-------------------------------|-------|---------------|
| `name` | Y | Y | Y | Y | Y | Y |
| `description` | Y | Y | Y | Y | Y | Y |
| `license` | Y | Y | Y | - | - | Y |
| `compatibility` | Y | - | Y | - | - | Y |
| `metadata` | Y | - | Y | - | - | Y |
| `allowed-tools` | Y (native) | - | - | - | - | - |
| `disallowed-tools` | Y | - | - | - | - | - |
| `context` | Y | - | - | - | - | - |
| `agent` | Y | - | - | - | - | - |
| `model` | Y | - | - | - | - | - |
| `effort` | Y | - | - | - | - | - |
| `disable-model-invocation` | Y | Y | Y | - | (via openai.yaml) | - |
| `user-invocable` | Y | Y | - | - | - | - |
| `argument-hint` | Y | Y | - | - | - | - |
| `hooks` | Y | - | - | - | - | - |

**2. Install paths diverge significantly.**

Every provider has its own primary path (`.cursor/skills/`, `.gemini/skills/`, `.roo/skills/`, etc.) but most also scan `.agents/skills/` and/or `.claude/skills/` for cross-agent compatibility. The `.agents/skills/` path is becoming the de facto cross-agent standard location.

**3. Invocation mechanisms differ.**

- Claude Code, Cursor, Copilot, Kiro: `/skill-name` slash command
- Windsurf: `@skill-name` mention
- Gemini CLI: `activate_skill` tool with consent prompt
- Cline: `use_skill` tool (no consent)
- OpenCode: `skill({ name })` tool call
- Roo Code: Automatic model discovery
- Manus: `/SKILL_NAME` command

**4. Tool names are provider-specific.**

Claude Code's `Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep` have no standard equivalents. Each provider has different built-in tool names, making `allowed-tools` essentially Claude-Code-specific.

**5. Sidecar configuration files.**

Codex CLI uses `agents/openai.yaml` alongside SKILL.md for UI metadata, invocation policy, and MCP tool dependencies. No other provider uses a sidecar format. This is a different axis of metadata that doesn't map to frontmatter.

### The Asymmetry Problem

Claude Code is simultaneously:
1. **The reference implementation** that other providers build from
2. **The superset** with fields no one else supports
3. **The source of most community skills** (due to earlier adoption)

This creates a one-way compatibility trap: Skills written FOR Claude Code often use `allowed-tools`, `context: fork`, `hooks`, or `model` — all of which silently degrade on other providers. Skills written for the base spec work everywhere but can't leverage Claude Code's power features.

The converter must handle this asymmetry: when converting Claude Code skills to other providers, behavioral metadata (tool restrictions, execution context, effort levels) must either be embedded as prose instructions or generate warnings. When converting from other providers to Claude Code, there's nothing to lose — it's always an upgrade.

### What's Universal

Despite the divergence, a strong common core exists:

1. **File format:** SKILL.md with YAML frontmatter + markdown body (every provider except Zed)
2. **Required fields:** `name` + `description` (universal)
3. **Directory structure:** `SKILL.md` + optional `scripts/`, `references/`, `assets/` subdirectories
4. **Progressive disclosure:** All providers implement the three-level loading model (metadata at startup, instructions on activation, resources on demand)
5. **Name validation:** Lowercase alphanumeric + hyphens, max 64 chars, must match directory name
6. **Description as trigger:** Every provider uses the description field for model-based skill discovery
7. **Cross-agent paths:** `.agents/skills/` is widely recognized as the portable location

---

## Canonical Mapping

Syllago's canonical format (`SkillMeta`) maps to the Agent Skills spec + Claude Code extensions:

| Canonical Field | Agent Skills Spec | Claude Code | Notes |
|----------------|-------------------|-------------|-------|
| `name` | `name` (required) | `name` | Universal |
| `description` | `description` (required) | `description` | Universal |
| `allowed-tools` | `allowed-tools` (experimental) | `allowed-tools` | CC-native, spec experimental |
| `disallowed-tools` | - | `disallowed-tools` | CC-only extension |
| `context` | - | `context` | CC-only (`fork`) |
| `agent` | - | `agent` | CC-only (subagent type) |
| `model` | - | `model` | CC-only |
| `effort` | - | `effort` | CC-only |
| `disable-model-invocation` | - | `disable-model-invocation` | CC + Copilot + Cursor |
| `user-invocable` | - | `user-invocable` | CC + Copilot/VS Code |
| `argument-hint` | - | `argument-hint` | CC + Copilot/VS Code |
| `hooks` | - | `hooks` | CC-only (skill-scoped) |

**Missing from canonical:** `license`, `compatibility`, `metadata` from Agent Skills spec. These are present in the spec and recognized by Cursor, Kiro, and OpenCode. The converter currently does not preserve these fields.

**Codex sidecar not mapped:** `agents/openai.yaml` fields (display_name, icon, brand_color, default_prompt, policy, dependencies) have no canonical equivalent. These are Codex-specific UI/policy metadata.

---

## Feature/Capability Matrix

### Feature Definitions

| Feature | Definition |
|---------|-----------|
| **SKILL.md Format** | Supports YAML frontmatter + markdown body in SKILL.md files |
| **Base FM (name+desc)** | Parses `name` and `description` from frontmatter |
| **Extended FM** | Supports fields beyond name+description from Agent Skills spec |
| **CC Extensions** | Supports Claude Code extension fields (allowed-tools, context, etc.) |
| **Progressive Disclosure** | Three-level loading: metadata -> instructions -> resources |
| **Slash Invocation** | User can invoke via `/skill-name` or similar command |
| **Model Invocation** | Model can auto-invoke based on description matching |
| **Invocation Control** | Can restrict who invokes (user-only, model-only) |
| **Tool Restrictions** | Can specify allowed/disallowed tools per skill |
| **Subagent Context** | Can run skill in isolated/forked context |
| **Skill-Scoped Hooks** | Lifecycle hooks scoped to skill execution |
| **Cross-Agent Paths** | Discovers skills from `.agents/skills/` |
| **String Substitutions** | Supports `$ARGUMENTS` and similar variables |
| **Dynamic Context** | Can inject shell command output into skill content |
| **Enterprise Deployment** | System-level managed skill distribution |
| **Consent Model** | User must approve before skill activates |
| **Mode Scoping** | Skills scoped to specific agent modes |
| **Sidecar Config** | Additional config file alongside SKILL.md |

### Provider Support Matrix

| Feature | CC | Cursor | Gemini | Copilot CLI | VS Code Copilot | Windsurf | Kiro | Codex | Cline | OpenCode | Roo | Amp | Manus |
|---------|------|--------|--------|-------------|-----------------|----------|------|-------|-------|----------|-----|-----|-------|
| SKILL.md Format | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| Base FM | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| Extended FM | Y | Y | - | Y | Y | - | Y | - | - | Y | - | - | - |
| CC Extensions | Y | P | - | P | P | - | - | - | - | - | - | - | - |
| Progressive Disclosure | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| Slash Invocation | Y | Y | Y | Y | Y | - | Y | Y | - | - | - | - | Y |
| Model Invocation | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| Invocation Control | Y | P | - | Y | Y | - | - | P | - | - | - | - | - |
| Tool Restrictions | Y | - | - | - | - | - | - | - | - | - | - | - | - |
| Subagent Context | Y | - | - | - | - | - | - | - | - | - | - | - | - |
| Skill-Scoped Hooks | Y | - | - | - | - | - | - | - | - | - | - | - | - |
| Cross-Agent Paths | - | Y | Y | - | Y | Y | - | Y | Y | Y | Y | Y | - |
| String Substitutions | Y | - | - | - | - | - | - | - | - | - | - | - | - |
| Dynamic Context | Y | - | - | - | - | - | - | - | - | - | - | - | - |
| Enterprise Deployment | Y | - | - | - | - | Y | - | Y | - | - | - | - | - |
| Consent Model | - | - | Y | - | - | - | - | - | - | - | - | - | - |
| Mode Scoping | - | - | - | - | - | - | - | - | - | - | Y | - | - |
| Sidecar Config | - | - | - | - | - | - | - | Y | - | - | - | - | - |

Legend: Y = full support, P = partial support, - = not supported

---

## Compat Scoring Implications

### Conversion Paths — Expected Compat Levels

**High compat (90-100%):** Conversions between providers with similar field support.

| Path | Expected Compat | Rationale |
|------|----------------|-----------|
| Claude Code -> Copilot CLI | ~85% | Copilot supports most CC extensions (DMI, user-invocable, argument-hint). Loses: allowed-tools, context, agent, model, effort, hooks |
| Claude Code -> VS Code Copilot | ~85% | Same as Copilot CLI |
| Copilot CLI -> Claude Code | ~100% | CC is superset, nothing lost |
| Cursor -> Claude Code | ~100% | CC is superset |
| Any base-spec -> Claude Code | ~100% | CC is superset |
| Any base-spec -> Any base-spec | ~95% | Name+description preserved. Install path changes. |

**Medium compat (60-85%):** Conversions where behavioral fields degrade to prose.

| Path | Expected Compat | Rationale |
|------|----------------|-----------|
| Claude Code -> Cursor | ~70% | Loses: allowed-tools, context, agent, model, effort, hooks, user-invocable, argument-hint. Keeps: DMI |
| Claude Code -> Gemini CLI | ~55% | Loses all except name+description. Hooks generate separate warnings (Gemini has global hooks). |
| Claude Code -> Windsurf | ~55% | Same as Gemini |
| Claude Code -> Kiro | ~60% | Kiro supports license/compat/metadata but none of CC extensions |
| Claude Code -> OpenCode | ~60% | Same as Kiro |

**Low compat (30-55%):** Conversions where significant features are lost.

| Path | Expected Compat | Rationale |
|------|----------------|-----------|
| Claude Code -> Cline | ~50% | Only name+description. Experimental feature. Different activation model. |
| Claude Code -> Roo Code | ~50% | Only name+description. Mode scoping can't be expressed in CC skills. |
| Claude Code -> Codex CLI | ~55% | Only name+description in SKILL.md. openai.yaml sidecar cannot be generated from CC fields. |
| Any -> Zed | ~0% | No skill support. Must degrade to rules. |

**Reverse paths (to Claude Code) are always high compat** since CC is the superset.

---

## Converter Coverage Audit

### Current State (`cli/internal/converter/skills.go`)

**Canonical format (SkillMeta):** 12 fields — name, description, allowed-tools (flexStringList), disallowed-tools, context, agent, model, effort, disable-model-invocation, user-invocable (*bool), argument-hint, hooks (any).

**Canonicalize paths:**
- `kiro`, `opencode`: Plain markdown wrap (no frontmatter parsing)
- All others: YAML frontmatter parse

**Render paths:**
- `gemini-cli`: name+description FM only, behavioral notes, hook warnings
- `opencode`: Plain markdown with prose notes
- `kiro`: Plain markdown with hook warnings
- `cursor`: name+description+DMI FM, behavioral notes, hook warnings
- `windsurf`: name+description FM only, behavioral notes, hook warnings
- `claude-code`, `copilot-cli`: Full frontmatter preserved

### Issues Found

**1. Kiro canonicalization is outdated.** Kiro adopted Agent Skills standard (SKILL.md with YAML frontmatter) in February 2026. The converter currently treats Kiro input as plain markdown (`canonicalizeSkillFromMarkdown`), stripping any frontmatter that may be present. Fix: Route Kiro through the standard frontmatter parser, same as Claude Code/Cursor.

**2. Kiro rendering is outdated.** Kiro now supports SKILL.md with name+description+license+compatibility+metadata frontmatter. The converter renders Kiro skills as plain markdown with a prose header. Fix: Render with Gemini-style minimal frontmatter (name+description), or ideally the 5-field subset Kiro supports.

**3. OpenCode canonicalization is outdated.** OpenCode now supports SKILL.md with YAML frontmatter (5 fields). The converter treats OpenCode as plain markdown. Fix: Same as Kiro — route through standard frontmatter parser.

**4. OpenCode rendering is outdated.** OpenCode skills now use SKILL.md with frontmatter. The converter renders as plain markdown. Fix: Render with frontmatter (at minimum name+description, ideally all 5 supported fields).

**5. Missing `license`, `compatibility`, `metadata` from canonical format.** The Agent Skills spec defines these three fields, and they're supported by Cursor, Kiro, OpenCode, and Copilot. The canonical `SkillMeta` struct doesn't include them. These fields would round-trip through the spec-compliant providers.

**6. Copilot CLI render path is too generous.** The converter renders Copilot CLI with full Claude Code frontmatter. Copilot CLI actually supports a subset (name, description, license, argument-hint, user-invocable, disable-model-invocation). Fields like `allowed-tools`, `context`, `agent`, `model`, `effort`, `hooks` are tolerated but not acted upon. The converter should either: (a) render only the supported subset and embed the rest as prose, or (b) accept that unknown fields are ignored and keep the full dump. Option (b) is defensible since Copilot suppresses unknown field warnings.

**7. Missing providers:** Cline, Roo Code, Codex CLI, Amp, Manus — all support SKILL.md but have no converter paths. These should be added with appropriate field support levels.

**8. Codex CLI sidecar not addressed.** The `agents/openai.yaml` sidecar has no canonical equivalent. This is a one-way mapping problem — Codex-specific metadata (icons, brand colors, MCP tool dependencies) can't be generated from other providers' skills.

**9. VS Code Copilot not distinguished from Copilot CLI.** These share a codebase but the VS Code validator is stricter about which fields it accepts. May warrant separate render paths, or at least a note.

**10. `is-mode` field.** Some sources reference an `is-mode` boolean field in Claude Code that makes skills appear in a "Mode Commands" section. This field is not in the official docs, not in the canonical SkillMeta, and not in the converter. If it exists as an undocumented feature, it may need investigation.

### Recommended Priority Fixes

1. **P1: Update Kiro and OpenCode to use frontmatter** — Both now support SKILL.md with YAML frontmatter. Current plain-markdown treatment loses data in both directions.
2. **P2: Add `license`, `compatibility`, `metadata` to canonical** — Three fields from the Agent Skills spec that round-trip through 4+ providers.
3. **P3: Add Cline/Roo/Codex/Amp render paths** — These are all SKILL.md-compatible with base fields. Minimal effort: name+description frontmatter + behavioral notes for CC extensions.
4. **P4: Refine Copilot CLI rendering** — Decide whether to keep full-dump (tolerated) or render supported subset only.

### Is "Claude Code = superset" Still True?

**Yes, with two caveats:**

1. **Codex CLI's `agents/openai.yaml`** introduces metadata (icons, brand_color, MCP dependencies) that Claude Code cannot express. This is a different metadata axis, not a frontmatter superset violation.
2. **Roo Code's `modeSlugs`** introduces mode-scoping that Claude Code has no equivalent for. Skills can be limited to specific Roo modes — there's no Claude Code frontmatter to express this.

Neither of these breaks the "CC is the SKILL.md frontmatter superset" claim, since they're either sidecar files or provider-specific extensions in separate systems. For the core SKILL.md frontmatter, Claude Code remains the superset.

### Skills.sh Registry Fields

The skills.sh registry (and SkillReg) adds fields beyond the Agent Skills spec:

- `tags`: Array of keywords for search/discovery
- `env`: Array of environment variable declarations
- `metadata.author`, `metadata.version`: Used by registry for tracking releases

These are registry-level metadata, not provider-level. The converter doesn't need to handle them directly, but the canonical format could benefit from supporting `metadata` (already in the spec) to preserve author/version through conversions.

---

## Sources

- [Agent Skills Specification](https://agentskills.io/specification)
- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills)
- [Cursor Agent Skills](https://cursor.com/docs/context/skills)
- [Gemini CLI Skills](https://geminicli.com/docs/cli/skills/)
- [Gemini CLI Creating Skills](https://geminicli.com/docs/cli/creating-skills/)
- [GitHub Copilot CLI Create Skills](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills)
- [VS Code Copilot Agent Skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills)
- [Windsurf Cascade Skills](https://docs.windsurf.com/windsurf/cascade/skills)
- [Kiro Agent Skills](https://kiro.dev/docs/skills/)
- [Kiro Steering](https://kiro.dev/docs/steering/)
- [Codex CLI Skills](https://developers.openai.com/codex/skills)
- [Cline Skills](https://docs.cline.bot/customization/skills)
- [OpenCode Skills](https://opencode.ai/docs/skills/)
- [Roo Code Skills](https://docs.roocode.com/features/skills)
- [Amp Agent Skills](https://ampcode.com/news/agent-skills)
- [Manus Skills](https://manus.im/blog/manus-skills)
- [Zed Agent Panel](https://zed.dev/docs/ai/agent-panel)
- [Serenities AI Agent Skills Guide 2026](https://serenitiesai.com/articles/agent-skills-guide-2026)
- [SkillReg SKILL.md Reference](https://skillreg.dev/docs/skill-md-reference)

---

## Appendix: Invocation & Trigger Mechanics Deep Dive

> Supplemental research compiled 2026-03-22. Goes deeper on HOW skills get triggered, discovered, and invoked across providers. Feeds into compat scoring for invocation-sensitive fields (`disable-model-invocation`, `user-invocable`, `argument-hint`).

---

### Per-Provider Invocation Mechanics

#### Claude Code

**Discovery Architecture (3-Level Progressive Disclosure):**

1. **Level 1 — Metadata Discovery:** At startup, Claude Code scans all skill directories (`~/.claude/skills/`, `.claude/skills/`, plugin dirs, `--add-dir` dirs) and parses YAML frontmatter from each SKILL.md. The `name` and `description` from every non-DMI skill get packed into an `<available_skills>` XML block inside the Skill tool's definition. This XML block is part of the system prompt — it is the tool description for the Skill tool itself. The Skill tool uses a dynamic prompt generator that constructs its description at runtime by aggregating all available skill metadata.

2. **Level 2 — Full Instructions:** When Claude determines a skill matches the task (or the user invokes via `/name`), the full SKILL.md body is loaded into context. Claude uses the Skill tool to read and inject the content.

3. **Level 3 — Bundled Resources:** Scripts, templates, reference files in the skill directory are loaded on-demand when the SKILL.md instructions reference them. Claude navigates the skill directory like a filesystem using Read/Bash tools.

**Context Budget:** Skill descriptions share a character budget that scales dynamically at **2% of the context window**, with a fallback floor of **16,000 characters**. If total description text exceeds this, some skills are excluded (lower-priority ones get dropped). Run `/context` to check for excluded skills. Override with `SLASH_COMMAND_TOOL_CHAR_BUDGET` env var. At ~100 tokens per skill metadata entry, this supports roughly 80-160 skills before budget pressure, though the 2% scaling means larger context windows (200K tokens) support more.

**Skill Body Limit:** Recommended under 500 lines per SKILL.md. Exceeding this degrades performance — split into reference files using progressive disclosure.

**Description as Trigger:** The description is the PRIMARY matching mechanism. Claude scans descriptions on every turn to decide relevance. Best practices: write in third person, include both what and when, use trigger keywords/verbs that match natural user phrasing. Descriptions that are vague ("helps with tests") fail to trigger. Descriptions should be specific ("Generates and modifies .spec.ts Playwright test files").

**Invocation Timing:** Claude evaluates available skills against the user's message on EVERY conversation turn. Skills can activate mid-conversation, not just at session start. No rate limits on skill invocation documented.

**User Invocation UX:** Type `/` to see autocomplete menu of all user-invocable skills. Select or type `/skill-name [args]`. Arguments follow the slash command. The `argument-hint` field provides placeholder text in the autocomplete UI (e.g., `[issue-number]`).

**Model Invocation Mechanics:** Claude calls the Skill tool programmatically. This is functionally identical to calling any other tool — the model emits a tool_use block with the skill name and optional arguments. The Skill tool reads the SKILL.md and injects it into context.

**disable-model-invocation Mechanics:** When `true`, the skill's description is **removed entirely from the `<available_skills>` XML block**. Claude literally cannot see the skill exists. It is completely invisible to the model. Only the user's `/` menu shows it. This is not a soft block — it is total context removal.

**user-invocable Mechanics:** When `false`, the skill is **hidden from the user's `/` menu** but its description **remains in the `<available_skills>` block**. The model can still see it and invoke it via the Skill tool. Important: `user-invocable: false` does NOT prevent the Skill tool from being called — it only controls menu visibility. To block programmatic invocation, use `disable-model-invocation: true`.

**Multi-Skill Behavior:** Multiple skills can match simultaneously. Claude decides which to load based on description relevance. There is no documented priority or ordering system beyond the description match — Claude uses judgment. Multiple skills can be active in the same session. Skills do not reference or invoke each other directly (no inter-skill calls), but a skill could instruct Claude to invoke another skill by name.

**Permission-Based Skill Restriction:** Beyond frontmatter, the permission system can allow/deny specific skills: `Skill(name)` for exact match, `Skill(name *)` for prefix match. Denying the `Skill` tool entirely disables all model-invoked skills.

**Known Bug:** Skills with `disable-model-invocation: true` are only visible to the LLM when the slash command is at the start of the user's message (GitHub issue #19729).

---

#### Cursor

**Discovery Architecture:**

At conversation start, Cursor presents available skills to the agent by loading only name + description from frontmatter. The full skill content loads only when the agent determines relevance. Cursor's progressive disclosure mirrors Claude Code's three-tier model.

**Description as Trigger:** Same pattern as Claude Code — the description field drives automatic model discovery. Cursor uses the description to decide when a skill matches the user's request.

**Invocation Modes:**
- **Manual:** Type `/` in Agent chat to see available skills. Select a skill to invoke it with the full content injected.
- **Automatic:** The model reads descriptions and loads matching skills without user intervention.

**disable-model-invocation Support:** Cursor recognizes `disable-model-invocation: true` in frontmatter. When set, the skill behaves like a command — the agent won't auto-load it, requiring explicit `/skill-name` invocation.

**Fields NOT Supported:** `user-invocable`, `argument-hint`, `allowed-tools`, `context`, `agent`, `model`, `effort`, `hooks`. These are silently ignored. This means:
- No way to create model-only skills (no `user-invocable: false` equivalent)
- No argument hint in autocomplete
- No tool restriction per skill
- No subagent execution

**Context Budget:** Not explicitly documented. Cursor uses a similar pattern to Claude Code where skill metadata is kept lightweight, but specific limits are not published.

**Multi-Skill:** Not documented. Cursor presumably loads the most relevant match, similar to Claude Code's judgment-based approach.

---

#### Gemini CLI

**Discovery Architecture:**

Gemini CLI implements a two-phase system: discovery + consent. At startup, Gemini reads name and description from all SKILL.md files. These populate the model's awareness of available capabilities.

**The `activate_skill` Tool:**
- **Type:** Internal tool, model-only. Users cannot call `activate_skill` manually.
- **Parameter:** Single required parameter `name` (enum of available skill names, e.g., `code-reviewer`, `pr-creator`).
- **Trigger:** When the model determines a user request matches a skill description, it calls `activate_skill` with the skill name.
- **Consent Prompt:** After the model calls `activate_skill`, a confirmation prompt appears in the terminal UI showing the skill's name, purpose, and the directory path it will access. The user must approve before activation proceeds. This is **unique among all providers** — no other tool requires explicit per-invocation user consent.
- **Post-Consent:** The full SKILL.md body and folder structure are injected into conversation history. The skill's directory is added to allowed file paths. The model is instructed to prioritize the skill's guidance for the remainder of the session.

**Session Persistence:** Once activated, a skill remains active for the entire session. There is no documented mechanism to deactivate a skill mid-session.

**No Invocation Control Fields:** Gemini CLI does not recognize `disable-model-invocation`, `user-invocable`, or `argument-hint`. The consent prompt serves as the sole invocation gate — every skill activation requires user approval regardless.

**User Invocation:** Users cannot directly invoke skills. The only path is: user makes a request → model decides to activate → consent prompt → activation. However, session commands (`/skills list`, `/skills enable|disable`) allow managing which skills are available. The `/skills` command shows available skills but does not invoke them.

**Multi-Skill:** No documented behavior for simultaneous skill activation or conflict resolution.

**Context Budget:** Not explicitly documented. The lazy-loading architecture (metadata only at startup) is designed to keep context lean, but specific token budgets are not published.

---

#### Copilot CLI (GitHub)

**Discovery Architecture:**

At startup, Copilot CLI scans skill directories (`.github/skills/`, `.claude/skills/`, personal paths) and loads name + description metadata. Progressive disclosure follows the same three-tier pattern as Claude Code.

**Invocation Modes:**
- **Manual:** `/SKILL-NAME` slash command in the CLI. Arguments follow the command name.
- **Automatic:** The model auto-invokes when description matches the user's request. If the description is well-written, users can describe tasks naturally and Copilot picks the right skill.

**Invocation Control Fields (3 supported):**
- `disable-model-invocation: true` — Prevents auto-loading; skill only available via explicit `/skill-name`.
- `user-invocable: false` — Hides from `/` menu while still allowing model auto-loading.
- `argument-hint` — Shown in chat input on invocation (e.g., `[filename]`).

**Plugin System Integration:** Skills can be provided by plugins. Plugin-provided skills automatically become available as slash commands. When the agent invokes a skill tool, it's equivalent to running the skill's slash command with provided arguments.

**Multi-Skill:** When duplicate skill names exist across locations (e.g., both `.github/skills/` and `.claude/skills/`), Copilot discovers both but the resolution behavior is not explicitly documented.

**Context Budget:** Not explicitly documented.

---

#### VS Code Copilot

**Discovery Architecture:**

Shares implementation with Copilot CLI. Skills from `.github/skills/`, `.claude/skills/`, `.agents/skills/`, and personal dirs are discovered at startup. Name + description loaded; full content on invoke.

**Invocation Modes:**
- **Manual:** Type `/` in chat input to see available skills as slash commands. Select and add context.
- **Automatic:** Model decides based on description match.
- **Extension-Contributed:** VS Code extensions can contribute skills that appear in the `/` menu.

**Invocation Control Fields:** Same as Copilot CLI — `disable-model-invocation`, `user-invocable`, `argument-hint` all supported.

**Key Difference from CLI — Validator Bug:** The VS Code extension's SKILL.md validator is stricter than the CLI. It rejects frontmatter fields that work in the CLI (`allowed-tools`, `context`, `agent`, `model`, `hooks`). These fields are valid in the Agent Skills spec but trigger warnings in VS Code.

**February 2026 (v1.110) Updates:**
- Skills usable as slash commands in both interactive and background agent sessions.
- `/create-skill` command to generate skills from conversation.
- Agent plugins: prepackaged bundles of skills, tools, hooks, and MCP servers installable from Extensions view.

**Multi-Skill:** Not explicitly documented beyond the shared Copilot CLI behavior.

---

#### Windsurf

**Discovery Architecture:**

At startup, Windsurf loads skill metadata (name + description) from `.windsurf/skills/`, `.agents/skills/`, and compat paths. Full instructions load only on invocation. Windsurf also uses a RAG-based context engine that builds vector embeddings of the codebase, but this is separate from skill discovery.

**Invocation Modes:**
- **Manual:** `@skill-name` mention in Cascade input. This is **unique** — Windsurf uses `@` mention syntax rather than `/` slash commands.
- **Automatic:** Cascade model determines relevance from description. However, **automatic invocation is documented as unreliable**. Community guidance recommends always using explicit `@skill-name` for guaranteed loading.

**No Invocation Control Fields:** Windsurf does not recognize `disable-model-invocation`, `user-invocable`, or `argument-hint`. All skills are both user-invocable (via `@mention`) and model-invocable (via description match). There is no mechanism to create user-only or model-only skills.

**Single-Context Limitation:** Windsurf has no subagent/context fork capability. Skills that relied on `context: fork` in Claude Code must be adapted for single-context use. Long conversations with many skill activations risk context bloat.

**Flow Awareness:** Windsurf tracks IDE actions (file edits, terminal runs, navigation) and updates Cascade's context in real time. This "Flow" awareness can influence when skills feel relevant, but it does not change the description-based matching mechanism.

**Multi-Skill:** Not documented. Given the unreliable auto-invocation, explicit `@` mentions are the practical pattern.

**Context Budget:** Windsurf shows a visual indicator of context window usage. A summarization system clears older history when the window grows too long, but specific skill budget limits are not published.

---

#### Kiro

**Discovery Architecture:**

Kiro reads name + description from SKILL.md frontmatter at session start. When a request matches, full instructions load. Kiro has TWO separate systems for this:

1. **Agent Skills (SKILL.md):** Standard progressive disclosure. Automatic activation based on description match, or manual via `/` slash commands in IDE chat.
2. **Steering files:** Separate system with `inclusion` modes: `always` (every conversation), `auto` (description-matched, like skills), `fileMatch` (loaded when specific file patterns match), `manual` (via `#` reference in chat). Steering files are NOT skills — they are behavioral constraints similar to rules.

**IDE vs CLI Invocation:**
- **IDE:** Both slash command (`/skill-name`) and automatic activation supported.
- **CLI:** Skills activate automatically only — no slash command support in CLI mode.

**No Invocation Control Fields:** Kiro does not recognize `disable-model-invocation`, `user-invocable`, or `argument-hint` in Agent Skills. All skills are available to both user and model.

**Powers (Dynamic Tool Loading):** Kiro's "Powers" system bundles MCP servers, steering files, and hooks into packages that activate on-demand based on conversation context. Powers load tools lazily when relevant keywords are mentioned. This is a higher-level orchestration than individual skills — a single Power can activate multiple MCP tools and steering files.

**Hook Integration:** Kiro supports pre/post tool use hooks that can intercept agent tool invocations. These hooks can filter by tool category (read, write, shell, web) or specific tool names with wildcards. However, these are global hooks, not skill-scoped.

---

#### Codex CLI (OpenAI)

**Discovery Architecture:**

Codex scans locations in hierarchy: `.agents/skills/` (walking up to repo root), `~/.agents/skills/`, `/etc/codex/skills/`, bundled system skills, `.claude/skills/`. Skill metadata (name, description, file path, optional openai.yaml data) is loaded at startup. Full SKILL.md content loads only when Codex decides to use a skill.

**Invocation Modes:**
- **Explicit:** Type `$` to mention a skill by name, or use `/skills` command. The `$` syntax is unique to Codex — no other provider uses this prefix.
- **Implicit:** Codex auto-selects skills when the task description matches. Depends entirely on description quality — there is no ML ranking layer.

**openai.yaml `allow_implicit_invocation`:**
- Located in `agents/openai.yaml` sidecar alongside SKILL.md.
- Default: `true`. When set to `false`, Codex won't auto-select the skill; explicit `$skill` invocation still works.
- This is the Codex equivalent of `disable-model-invocation`, but stored in a separate file rather than SKILL.md frontmatter.
- **Key difference from Claude Code:** Claude Code's DMI removes the skill from context entirely. Codex's `allow_implicit_invocation: false` likely still makes the skill visible for explicit invocation via `$` — the description may remain in context.

**No user-invocable Equivalent:** Codex has no mechanism to hide a skill from the user while keeping it model-accessible. The `$` explicit invocation is always available for all skills.

**Duplicate Name Handling:** When duplicate skill names exist across locations, both appear in skill selectors. The user chooses explicitly rather than automatic precedence resolution.

**Testing with Evals:** OpenAI recommends writing eval tests for skill triggering:
- Implicit invocation tests: describe the scenario without naming the skill, verify Codex selects it.
- Negative control tests: prompts that should NOT invoke a specific skill.

---

#### Cline

**Discovery Architecture:**

During initialization, the `SkillsTool` scans configured directories (`.cline/skills/`, `.clinerules/skills/`, `.claude/skills/`, `.agents/skills/`, global paths) and parses YAML frontmatter from each SKILL.md. It extracts name and description to build a lightweight skill registry embedded directly into the Skill tool's description — the same `<available_skills>` pattern as Claude Code. This makes skills visible to the LLM without consuming conversation context.

**The `use_skill` Tool:**
- Internal tool registered in the ToolExecutor.
- When the model determines a skill matches the current task, it calls `use_skill` with the skill name.
- The system locates the SKILL.md, reads content, and injects it into conversation context as a tool response.
- Similar to Gemini's `activate_skill` but **without the consent prompt** — activation is automatic once the model decides.

**No Invocation Control Fields:** Cline does not recognize `disable-model-invocation`, `user-invocable`, or `argument-hint`. All skills are available to the model. Users cannot directly invoke skills via slash command — the model is the sole invoker.

**UI Toggle:** Individual skills can be enabled/disabled via the Cline settings panel (`ClineRulesToggleModal`). Toggle state persists as `globalSkillsToggles` / `localSkillsToggles`. Disabled skills are excluded from discovery entirely. This is the only invocation control mechanism.

**Feature Gate:** Skills are experimental in Cline. Must be enabled in Settings → Features → Enable Skills. When disabled, no skill discovery occurs.

**Priority Inversion:** Global skills override identically-named project skills (opposite of most providers where project overrides global).

---

#### OpenCode

**Discovery Architecture:**

OpenCode searches six locations with strict priority ordering (first skill found with a given name wins): `.opencode/skills/`, `.claude/skills/`, `.agents/skills/` (both project-local walking up to git root and global). On session start, a list of all discovered skills wrapped in `<available-skills>` tags is injected into the system prompt — same pattern as Claude Code and Cline.

**Skill Tool:** A dedicated `skill` tool registered in the ToolRegistry. Agents invoke via `skill({ name: "skill-name" })`. The tool reads and loads the skill in one operation, injecting content directly into context.

**Semantic Matching (Unique):** Beyond the initial skill list injection, OpenCode monitors subsequent messages and uses **semantic similarity** to detect when a message relates to an available skill. When matches are found, it injects a prompt encouraging the agent to evaluate and load relevant skills. This happens automatically — the system actively nudges the model toward skill usage, not just passively making skills available.

**Session Compaction Protection:** The skill tool receives special treatment during compaction. It is explicitly protected from output pruning (listed in `PRUNE_PROTECTED_TOOLS`). Even when token count exceeds the 40,000-token prune threshold, skill invocations and their results are preserved. This ensures skill context survives long sessions.

**Permission System:** `opencode.json` controls skill access with `allow`, `deny`, or `ask` states supporting wildcards (e.g., `internal-*`). Per-agent skill overrides possible. This is the closest equivalent to Claude Code's permission-based skill restriction.

**Caching:** Skill descriptions in system prompts participate in a `ProviderTransform` caching layer. For Anthropic and OpenRouter providers, ephemeral cache control directives are applied to reduce repeated token processing.

**No Slash Command:** As of early 2026, there is no `/skills` command for direct invocation. A feature request (issue #7846) proposes adding one. Skills are model-invoked only.

---

#### Roo Code

**Discovery Architecture:**

Three-tier progressive disclosure identical to the standard pattern:
1. Metadata (name + description) parsed from SKILL.md frontmatter.
2. Full SKILL.md loaded via `read_file` when request matches.
3. Bundled resources discovered on-demand when instructions reference them.

**Mode-Specific Scoping (Unique):**
- **Directory-based:** Skills in `.roo/skills-code/` only load in "code" mode. Skills in `.roo/skills-architect/` only in "architect" mode. Base `.roo/skills/` loads in all modes.
- **Priority:** Project > global; `.roo/` > `.agents/` at same level; mode-specific > generic.
- **No frontmatter control:** The `modeSlugs` field mentioned in v3.47.0 release notes enables targeting skills to specific modes via frontmatter, but documentation does not confirm current implementation status. Primary mode scoping remains directory-based.

**Skill Tool:** The `skill` tool loads and injects skill instructions. Parameters: `skill` (required, name), `args` (optional, context). Invoked via XML-style tags internally.

**No User Invocation:** Roo Code has no slash command or direct user invocation for skills. Skills are entirely model-driven — the model decides when to load them based on description matching. A feature request for a skills management UI exists (issue #10513) but no user invocation mechanism is implemented.

**No Invocation Control Fields:** No `disable-model-invocation`, `user-invocable`, or `argument-hint` support. All skills are auto-discoverable by the model. The only control is directory-based mode scoping.

**`.roomodes` Integration:** Custom modes defined in `.roomodes` can reference skills. Third-party tools (rooskills) convert Agent Skills into Roo Code custom modes. This is an indirect form of skill organization but not an invocation control mechanism.

---

#### Amp

**Discovery Architecture:**

Amp reads skills from `.agents/skills/` (workspace), `~/.config/agents/skills/` (user), `.claude/skills/`, `~/.claude/skills/` (compat). Name and description from frontmatter are loaded at startup. Full SKILL.md body loads lazily on demand.

**Invocation Modes:**
- **Agent-Invoked:** The model determines relevance from description and loads the skill. This was the ONLY mode until the user-invokable update.
- **User-Invoked:** Added after initial launch. Access via command palette (Cmd/Alt-Shift-A in editor, Ctrl-O in CLI) → `skill: invoke`. Selecting a skill forces the agent to use it on the next message. Amp removed custom commands entirely in favor of skills — "they were two ways of doing the same thing, except that only users could invoke custom commands and only the agent could invoke skills."

**MCP Tool Lazy Loading (Unique):** Amp has a unique pattern where skills can bundle `mcp.json` files alongside SKILL.md. The `mcp.json` specifies which MCP servers/tools to load via `includeTools` (supports glob patterns). Initially, only the skill description is in context. When the agent invokes the skill, Amp appends matching MCP tool descriptions to the context window. Example: chrome-devtools MCP has 26 tools (17K tokens), but a skill configured with `includeTools` loads only 4 tools (1.5K tokens). This is a skill-as-MCP-tool-loader pattern unique to Amp.

**No Invocation Control Fields:** Amp does not document support for `disable-model-invocation`, `user-invocable`, or `argument-hint` frontmatter fields. All skills are available to both user (via command palette) and agent (via description matching).

**Multi-Skill:** Not explicitly documented. The 15 built-in skills coexist without documented conflict resolution.

---

### Invocation Comparison Matrix

| Aspect | Claude Code | Cursor | Gemini CLI | Copilot CLI | VS Code Copilot | Windsurf | Kiro | Codex CLI | Cline | OpenCode | Roo Code | Amp |
|--------|-------------|--------|------------|-------------|-----------------|----------|------|-----------|-------|----------|----------|-----|
| **Discovery Method** | `<available_skills>` in Skill tool desc | Name+desc list to agent | Name+desc in model context | Name+desc list | Name+desc list | Name+desc metadata | Name+desc from FM | Name+desc+path+yaml metadata | `<available_skills>` in tool desc | `<available-skills>` tags in system prompt | Name+desc from FM | Name+desc from FM |
| **User Invocation Syntax** | `/skill-name [args]` | `/skill-name` | None (model-only) | `/SKILL-NAME` | `/skill-name` | `@skill-name` | `/skill-name` (IDE only) | `$skill-name` | None (model-only) | None (model-only) | None (model-only) | Command palette → `skill: invoke` |
| **Model Invocation Tool** | Skill tool | Agent decision | `activate_skill` | Skill tool | Skill tool | Cascade decision | Agent decision | Agent decision | `use_skill` | `skill()` | `skill` tool (XML) | Agent decision |
| **Consent Required** | No | No | **Yes** (per-invocation) | No | No | No | No | No | No | No | No | No |
| **DMI Support** | Yes (removes from context) | Yes (hides from agent) | No | Yes | Yes | No | No | Via `openai.yaml` | No | No | No | No |
| **user-invocable Support** | Yes (hides from `/` menu) | No | No | Yes | Yes | No | No | No | No | No | No | No |
| **argument-hint Support** | Yes | No | No | Yes | Yes | No | No | No | No | No | No | No |
| **Description Budget** | 2% of context window (16K char fallback) | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published |
| **Skill Body Limit** | 500 lines recommended | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published | Not published |
| **Invocation Timing** | Every turn | Every turn | Every turn | Every turn | Every turn | Every turn | Every turn | Every turn | Every turn | Every turn + semantic nudging | Every turn | Every turn |
| **Multi-Skill Activation** | Yes (multiple concurrent) | Not documented | Not documented | Not documented | Not documented | Not documented | Not documented | Duplicate names shown to user | Not documented | Not documented | Not documented | Not documented |
| **Session Persistence** | Per-invocation (reloads each time) | Per-invocation | Full session (once activated) | Per-invocation | Per-invocation | Per-invocation | Per-invocation | Per-invocation | Per-invocation | Protected from compaction | Per-invocation | Per-invocation |
| **Unique Mechanism** | Dynamic tool desc generator | `/migrate-to-skills` | Consent prompt | Plugin-contributed skills | Extension-contributed skills | `@mention` + unreliable auto | Powers (bundled MCP+steering) | `$` prefix + openai.yaml sidecar | UI toggle enable/disable | Semantic similarity nudging | Mode-scoped directories | MCP tool lazy-loading via skills |

---

### Compat Implications for Invocation

#### 1. `disable-model-invocation` Degradation

**What it does in Claude Code:** Removes the skill entirely from model context. The model cannot see or invoke the skill. Only the user's `/` menu shows it.

**How it degrades:**

| Target Provider | Behavior | Risk Level |
|----------------|----------|------------|
| Cursor | Recognized — skill hidden from agent auto-load | Low |
| Copilot CLI / VS Code | Recognized — prevents auto-loading | Low |
| Codex CLI | Not in SKILL.md — requires `openai.yaml` with `allow_implicit_invocation: false` | Medium (needs sidecar generation) |
| Gemini CLI | Ignored — but consent prompt provides equivalent gate | Low (consent = manual gate) |
| Windsurf | Ignored — skill auto-activates if description matches | **High** (unwanted auto-triggering) |
| Kiro | Ignored — skill auto-activates | **High** |
| Cline | Ignored — model decides freely | **High** |
| OpenCode | Ignored — model decides freely | **High** |
| Roo Code | Ignored — model decides freely | **High** |
| Amp | Ignored — model decides freely | **High** |

**Converter action:** When converting a DMI skill to providers that ignore it, the converter should either:
- Embed a prose warning at the top of SKILL.md: "WARNING: This skill was designed for manual-only invocation. Do not auto-activate."
- Flag it as a compat warning in the conversion report.
- For Codex: generate an `openai.yaml` sidecar with `allow_implicit_invocation: false`.

**Why this matters:** DMI skills often perform dangerous operations (deploy, send-slack-message, database migration). Auto-triggering these on providers without DMI support could cause real damage.

#### 2. `user-invocable: false` Degradation

**What it does in Claude Code:** Hides from `/` menu but keeps description in model context. Model can still invoke. Use case: background knowledge the model should know about but users shouldn't trigger as a command.

**How it degrades:**

| Target Provider | Behavior |
|----------------|----------|
| Copilot CLI / VS Code | Recognized — hidden from menu, model can auto-load |
| All others | Ignored — skill appears in both user menu AND model context |

**Risk:** Low. A skill becoming user-visible when it shouldn't be is an inconvenience, not a danger. The skill content is still valid — it just shows up in a menu where users might invoke it unnecessarily.

**Converter action:** Embed a prose note: "Note: This skill is designed for model use only, not direct user invocation."

#### 3. `argument-hint` Degradation

**What it does in Claude Code:** Shows placeholder text in autocomplete (e.g., `[issue-number]`).

**How it degrades:** Silently dropped on all providers except Copilot CLI/VS Code. No functional impact — the skill still works, users just don't see the hint.

**Risk:** Negligible. No converter action needed.

#### 4. Invocation Syntax Incompatibility

Skills authored with instructions referencing `/skill-name` syntax (e.g., "Run `/deploy` to begin deployment") will confuse users on providers with different syntax:
- Windsurf: `@deploy`
- Codex CLI: `$deploy`
- Cline/OpenCode/Roo Code: No user invocation — model only
- Amp: Command palette → `skill: invoke`

**Converter action:** If skill body contains `/skill-name` references, rewrite to match target provider syntax. This requires body-level transformation, not just frontmatter mapping.

#### 5. Consent Model Gap (Gemini)

Skills converted TO Gemini CLI will gain the consent prompt — every activation requires user approval. This is generally safe (more restrictive is better). Skills converted FROM Gemini CLI lose the consent gate and may auto-activate without user awareness on other providers.

#### 6. Model-Only Providers (Cline, OpenCode, Roo Code)

These providers have no user invocation mechanism. Skills designed for explicit user triggering (DMI skills) have no invocation path at all on these providers — the model cannot see them (DMI removes from context) and the user cannot invoke them (no slash command). The skill effectively becomes dead code.

**Converter action:** For DMI skills targeting model-only providers, the converter should warn that the skill will be non-functional. Options: (a) drop DMI and let the model auto-invoke, (b) convert to a rule/instruction that's always loaded, (c) flag as incompatible.

#### 7. Compat Score Adjustments for Invocation Fields

Current compat scoring should account for these invocation-specific penalties:

| Field | Penalty When Target Lacks Support |
|-------|----------------------------------|
| `disable-model-invocation: true` | -15% (safety risk: unwanted auto-triggering of dangerous skills) |
| `user-invocable: false` | -3% (cosmetic: skill appears in user menu unnecessarily) |
| `argument-hint` | -1% (cosmetic: no autocomplete hint) |
| `context: fork` (invocation-related) | -10% (behavioral: skill runs inline instead of isolated) |
| `allowed-tools` (invocation-related) | -8% (behavioral: no tool restriction during skill execution) |

The `disable-model-invocation` penalty is high because the skill was explicitly designed to NOT auto-trigger, and removing that protection can cause side effects. The `context: fork` penalty is invocation-related because it changes WHERE the skill executes, not just what it contains.

---

### Appendix Sources

- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills)
- [Claude Code Skill Authoring Best Practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)
- [Cursor Agent Skills Docs](https://cursor.com/docs/context/skills)
- [Cursor Rules/Skills Guide](https://theodoroskokosioulis.com/blog/cursor-rules-commands-skills-hooks-guide/)
- [Gemini CLI activate_skill Tool](https://geminicli.com/docs/tools/activate-skill/)
- [Gemini CLI Skills Tutorial](https://geminicli.com/docs/cli/tutorials/skills-getting-started/)
- [Gemini CLI Agent Skills Deep Dive](https://damimartinez.github.io/agent-skills-gemini-cli/)
- [Copilot CLI Skills Practical Guide](https://dxrf.com/blog/2026/03/03/copilot-cli-skills-practical-guide/)
- [Copilot CLI Plugin System (DeepWiki)](https://deepwiki.com/github/copilot-cli/5.5-plugin-system-and-skills)
- [VS Code Copilot Agent Skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills)
- [VS Code Making Agents Practical](https://code.visualstudio.com/blogs/2026/03/05/making-agents-practical-for-real-world-development)
- [Windsurf Cascade Skills](https://docs.windsurf.com/windsurf/cascade/skills)
- [Windsurf Context Engine 2026](https://markaicode.com/windsurf-flow-context-engine/)
- [Kiro Agent Skills](https://kiro.dev/docs/skills/)
- [Kiro Slash Commands](https://kiro.dev/docs/chat/slash-commands/)
- [Kiro Powers](https://kiro.dev/blog/introducing-powers/)
- [Codex CLI Skills](https://developers.openai.com/codex/skills)
- [Codex CLI Implicit Invocation Issue #10585](https://github.com/openai/codex/issues/10585)
- [Codex Testing Skills with Evals](https://developers.openai.com/blog/eval-skills)
- [Cline Skills Docs](https://docs.cline.bot/customization/skills)
- [Cline Skills System (DeepWiki)](https://deepwiki.com/cline/cline/7.4-skills-system)
- [OpenCode Skills](https://opencode.ai/docs/skills/)
- [OpenCode Skills System (DeepWiki)](https://deepwiki.com/sst/opencode/5.7-skills-system)
- [Roo Code Skills](https://docs.roocode.com/features/skills)
- [Roo Code Skill Tool](https://docs.roocode.com/advanced-usage/available-tools/skill)
- [Amp Agent Skills](https://ampcode.com/news/agent-skills)
- [Amp User-Invokable Skills](https://ampcode.com/news/user-invokable-skills)
- [Amp Lazy Load MCP with Skills](https://ampcode.com/news/lazy-load-mcp-with-skills)
- [Claude Code DMI Bug #19729](https://github.com/anthropics/claude-code/issues/19729)
- [Claude Code user-invocable Clarification #19141](https://github.com/anthropics/claude-code/issues/19141)
- [Progressive Disclosure Architecture](https://skills.deeptoai.com/en/docs/development/progressive-disclosure-architecture)
- [How Cursor Finds Skills](https://agenticthinking.ai/blog/skill-discovery/)
