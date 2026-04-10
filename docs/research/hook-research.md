# AI Coding Agent Hooks: Ecosystem Research & Opportunity Analysis

> Research compiled March 2026. Covers hooks systems across 12+ AI coding tools, the conversation history search ecosystem, skills/MCP tooling landscape, and the cross-platform normalization gap.

---

## Table of Contents

1. [Context: How We Got Here](#context)
2. [The Conversation History Ecosystem](#conversation-history)
3. [The Skills, MCP, and Configuration Ecosystem](#skills-mcp-config)
4. [Hooks: Deep Technical Comparison](#hooks-deep-dive)
5. [The Cross-Platform Normalization Problem](#normalization)
6. [Existing Normalization Attempts](#existing-attempts)
7. [Why This Hasn't Been Solved](#why-not-solved)
8. [The Real Gap](#the-gap)
9. [Key Projects Reference](#key-projects)

---

## Context

This research emerged from exploring what tools exist for searching and managing AI coding agent artifacts — conversation history, skills, hooks, MCP configs, commands — across the increasingly fragmented multi-tool developer landscape. The question evolved from "does a tool like this exist?" into "where is the actual gap worth building into?"

The short answer: the conversation history space has `cass`. The skills space has Skills.sh. MCP config sync has CC Switch. **Hooks have nothing serious.**

---

## Conversation History Ecosystem

### The Problem

Every AI coding CLI tool stores conversation history in a different format and location. None of them are searchable across tools, and none provide a unified view of what you've built, debugged, or discussed across sessions.

### Format Zoo

| Tool | Format | Location |
|------|--------|----------|
| Claude Code | JSONL (one event per line) | `~/.claude/projects/<hash>/` |
| Codex CLI | JSONL (rollout format) | `~/.codex/sessions/YYYY/MM/DD/` |
| Gemini CLI | Monolithic JSON (JSONL migration in progress, see [#15292](https://github.com/google-gemini/gemini-cli/issues/15292)) | `~/.gemini/tmp/<project-hash>/chats/` |
| OpenCode | SQLite | `.opencode/` directories |
| Aider | Markdown | `.aider.chat.history.md` per project |
| Cursor | Multiple SQLite DBs (two schema versions) | `AppData/Cursor/User/workspaceStorage/<md5>/state.vscdb` |
| Cline | JSON in VS Code extension storage | VS Code global storage |
| oh-my-pi | JSONL (Snowflake hex IDs, not UUIDs) | `~/.omp/agent/sessions/` |
| Simon's `llm` | SQLite with FTS5 | `~/.config/llm/logs.db` |
| ChatGPT export | Nested JSON (`mapping` field) | Export ZIP |
| Claude.ai export | JSON in ZIP | Export ZIP |

Gemini's migration issue is worth watching — they're explicitly moving to the Claude/Codex JSONL pattern for performance (~100x faster appends vs full-file rewrites).

### cass — The Solution That Already Exists

**[coding_agent_session_search](https://github.com/Dicklesworthstone/coding_agent_session_search)** — 550 stars, 77 forks, 1,759 commits, actively maintained.

The most comprehensive tool in this space. Written in Rust by Jeff Emanuel (Dicklesworthstone). Sole author, does not accept contributions, builds with Claude/Codex.

**What it does:**
- Auto-discovers sessions from 15+ coding agents via `franken_agent_detection` library
- Normalizes all formats into a unified SQLite schema via the `Connector` trait (one struct per agent, `detect()` + `scan()` methods, runs in parallel via rayon)
- Builds BM25 full-text index via `frankensearch` (Tantivy-backed)
- Optional semantic search via local MiniLM ONNX model (hash embedder fallback when model not installed)
- Hybrid search via Reciprocal Rank Fusion (BM25 + vector)
- Three-pane TUI + `--robot`/`--json` modes for agent consumption
- Remote multi-machine search via SSH/rsync
- HTML export with AES-256-GCM encryption
- Robot mode with structured exit codes, cursor pagination, token budget controls
- Sub-60ms search latency

**Connector architecture (the elegant part):**

```rust
trait Connector {
    fn detect() -> DetectionResult;
    fn scan(ctx: ScanContext) -> Vec<NormalizedConversation>;
}
```

Each connector handles its own format complexity (integer vs ISO timestamps, tool-use block flattening, ChatGPT's v1/v2/v3 encryption, Cursor's multi-schema SQLite) and emits clean `NormalizedConversation` objects. The indexer runs all connectors in parallel. The normalization problem is real but bounded — the connector absorbs the mess, the downstream system sees clean data.

**Lessons for hook normalization:**
The connector pattern is directly applicable. Each tool gets an adapter that reads native format and emits canonical events. The canonical schema is the hard design work, not the adapters themselves.

**The gap cass doesn't fill:**
- No coverage of web chat exports (Claude.ai ZIP, ChatGPT export) — this is still an open opportunity
- No semantic search over web exports
- No cross-provider unified search (cass covers CLI agents; Claude.ai and ChatGPT web are separate)

**Related tools in this space:**

| Tool | What it does | Format |
|------|-------------|--------|
| `claude-history` (raine) | Fuzzy TUI search for Claude Code JSONL | Rust, fzf-style |
| `claude-parser` (alicoding) | Git-like commands + file recovery from JSONL | Python |
| `ccusage` (ryoppippi) | Token/cost analytics from JSONL | TypeScript, ~11k stars |
| `claude-historian-mcp` (Vvkmnn) | MCP server for in-session history search | TypeScript, 68 stars |
| `chatgpt2md` (NextStat) | ChatGPT export → Markdown + Tantivy index + MCP server | Rust |
| `continues` (yigitkonur) | Session handoff between 14 tools | TypeScript |
| `casr` (Dicklesworthstone) | Cross-provider session conversion with canonical IR | Rust |

---

## Skills, MCP, and Configuration Ecosystem

### Skills

Agent skills (SKILL.md files) are the most mature cross-platform configuration artifact. They became a de facto standard in late 2025/early 2026, adopted by Claude Code, Codex, Gemini CLI, Cursor, Copilot, OpenCode, Windsurf, Amp, and Manus.

**Registry/distribution landscape:**

| Tool | What it is | Coverage |
|------|-----------|---------|
| **Skills.sh** (Vercel) | Official registry + CLI, launched Jan 20 2026, 26k+ installs in weeks | 40+ agent platforms |
| **VoltAgent/awesome-agent-skills** | Community catalog, 6,900+ stars, 300+ skills | Claude Code, Codex, Gemini, Cursor, Copilot |
| **SkillPort** (gotalab) | CLI + MCP server, lazy-loads metadata (~100 tokens/skill), install on demand | Claude Code, Codex, Cursor, Copilot, Windsurf |
| **tech-leads-club/agent-skills** | Security-hardened registry, static analysis in CI, content hashing | Claude Code + adapters |
| **microsoft/skills** | 132 Azure-focused skills, wizard-based install, symlinks across agents | GitHub Copilot primary |

**Key insight:** Skills are a solved content distribution problem. The format is stable (YAML frontmatter + Markdown body), the install locations are known per-tool, and Skills.sh handles discovery well. Building another skills registry is not a good use of time.

### MCP Config Management

| Tool | What it does |
|------|-------------|
| **CC Switch** (farion1231/cc-switch) | Desktop app managing MCP, skills, providers, prompts across Claude/Codex/Gemini/OpenCode/OpenClaw |
| **CC Switch CLI** (SaladDay) | CLI fork of CC Switch |
| **mcp-config-manager** (holstein13) | CLI for syncing MCP configs between Claude/Gemini/Codex, preset modes, backups |

**Key insight:** MCP config sync is partially solved. What's missing is a unified *search* layer over your accumulated MCP configs — a "what do I have installed and where" query tool.

### Commands

Largely absorbed into the skills ecosystem. Slash commands in Claude Code (`/.claude/commands/*.md`) follow the same SKILL.md pattern and are treated interchangeably. Rulesync handles bidirectional conversion between Claude Code commands and other agents' equivalents.

### Hooks

**Nothing serious exists for cross-platform hooks.** This is the gap. See the next two sections.

---

## Hooks: Deep Technical Comparison

### Who Has Production Hooks

Nine of eighteen researched tools have production or experimental hooks:

| Tool | Events | Config Format | Can Block | Can Modify Args | Handler Types |
|------|--------|---------------|-----------|-----------------|---------------|
| **Claude Code** | 21 | JSON (settings.json) | ✅ | ✅ (updatedInput) | command, http, prompt, agent |
| **Gemini CLI** | 11 | JSON (settings.json) | ✅ | ✅ (rewrite) | command |
| **OpenCode** | 25+ | TypeScript plugins | ✅ (throw) | ✅ | JS/TS modules (Bun) |
| **Windsurf** | 12 | JSON (hooks.json) | ✅ (exit 2) | ❌ | command |
| **Pi Agent Rust** | 20+ | TypeScript extensions | ✅ | ✅ (middleware chain) | JS/TS via QuickJS |
| **Cursor** | 6 | JSON (hooks.json) | ✅ (3 of 6) | ❌ | command |
| **Cline** | 4 | Script files in dirs | ✅ | ✅ (context inject) | executable scripts |
| **Kiro** | 5+5 | .kiro.hook / JSON | ✅ (CLI) | ❌ | askAgent + command |
| **GitHub Copilot CLI** | 3 | JSON (.github/hooks/) | ✅ | ✅ | command |
| **VS Code Copilot** | 3 (preview) | JSON (hooks.json) | ✅ | ❌ | command |
| **Codex CLI** | 2 | TOML | ❌ | ❌ | command (experimental) |

**No hooks:** Aider (uses `--auto-lint`/`--auto-test` flags), Roo Code (declarative rules), Continue.dev (config.yaml rules), Goose (MCP-only extensions).

---

### Claude Code — The Most Comprehensive System

**Event catalog (21 events):**

*Session:* `SessionStart`, `SessionEnd`, `Setup`  
*User interaction:* `UserPromptSubmit`  
*Tool execution:* `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest`  
*Agent:* `Stop`, `SubagentStart`, `SubagentStop`, `TeammateIdle`, `TaskCompleted`  
*Maintenance:* `PreCompact`, `PostCompact`, `Notification`, `InstructionsLoaded`, `ConfigChange`  
*Infrastructure:* `WorktreeCreate`, `WorktreeRemove`, `Elicitation`, `ElicitationResult`

**Built-in tools matchable by name:** `Read`, `Write`, `Edit`, `MultiEdit`, `Bash`, `Grep`, `Glob`, `LS`, `WebFetch`, `WebSearch`, `NotebookEdit`, `NotebookRead`, `TodoRead`, `TodoWrite`, `Task`, `KillBash`

**MCP tool matching:** `mcp__<server>__<tool>` pattern with regex support

**Four handler types** (unique in the ecosystem):
- `command` — shell script, JSON on stdin, structured JSON on stdout
- `http` — POST to endpoint with configurable headers + env var injection
- `prompt` — single-turn LLM eval (Haiku default), returns approve/block + reason
- `agent` — multi-turn sub-agent with read-only tools (`Read`, `Grep`, `Glob`)

**Exit code contract:**
- `0` → success, parse stdout for JSON
- `2` → block (PreToolUse blocks tool; Stop forces continuation; PermissionRequest denies)
- Other → non-blocking warning

**Structured output fields:**
```json
{
  "permissionDecision": "allow|deny",
  "permissionDecisionReason": "string",
  "updatedInput": {},
  "systemMessage": "injected context",
  "additionalContext": "string",
  "continue": false,
  "suppressOutput": true
}
```

**Configuration locations (precedence order):**
1. Enterprise managed (`/etc/claude/` or MDM-deployed) — cannot be overridden
2. User-level (`~/.claude/settings.json`)
3. Project-level (`.claude/settings.json`) — committable
4. Project-local (`.claude/settings.local.json`) — gitignored
5. Agent YAML frontmatter in `.claude/agents/*.md`

**Example hook config:**
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/safety-check.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/run-tests.sh",
            "async": true
          }
        ]
      }
    ]
  }
}
```

---

### Gemini CLI — Model-Level Hooks Are Unique

**Event catalog (11 events):**

`SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeToolExecution`, `AfterToolExecution`, `ToolError`, `BeforePromptTemplate`, `AfterPromptTemplate`, `OnNotification`

**Notable capabilities:**
- `BeforeModel` / `AfterModel` fire before/after LLM API calls — enables prompt modification, response mocking, output redaction. No other tool offers model-level hooks.
- `BeforeToolSelection` filters the tool list before the LLM decides which tool to use.
- Extensions bundle hooks into distributable packages (three-tier: system → user → project).
- **Security constraint unique to Gemini:** extensions cannot set `allow` decisions or enable yolo mode.

**Config location:** `~/.gemini/settings.json`

```json
{
  "hooks": {
    "BeforeToolExecution": [
      {
        "command": "gemini-hook-safety",
        "args": ["--check"],
        "timeout": 5000
      }
    ]
  }
}
```

---

### OpenCode — Most Developer-Friendly (TypeScript Plugins)

**Mechanism:** TypeScript/JavaScript plugin files in `.opencode/plugins/` or `~/.config/opencode/plugins/`, loaded via Bun. Distributed as npm packages.

**25+ event types** spanning tool execution, file changes, LSP diagnostics, session lifecycle, permissions, and TUI interactions.

**Unique capabilities:**
- Register custom tools that override built-ins
- Inject shell environment variables
- Manipulate TUI
- Access full OpenCode SDK client from plugin context

**Example plugin:**
```typescript
import type { Plugin } from "opencode";

export default {
  name: "my-hook",
  version: "1.0.0",
  hooks: {
    "tool.execute.before": async (ctx) => {
      if (ctx.tool === "bash" && ctx.args.command.includes("rm -rf")) {
        throw new Error("Blocked: dangerous command");
      }
    }
  }
} satisfies Plugin;
```

---

### Windsurf — Best Enterprise Deployment

**12 hook types:** `pre_read_code`, `pre_write_code`, `pre_run_command`, `pre_mcp_tool_use`, `pre_user_prompt` + seven post variants including `post_cascade_response_with_transcript` (provides complete conversation transcript — useful for audit logging).

**Enterprise features:**
- Distributable via cloud dashboard, MDM, system-level `/etc/windsurf/hooks.json`
- `trajectory_id` and `execution_id` for event correlation
- No input modification (post-only for most events)

---

### Pi Agent Rust — Most Sophisticated Security Model

**Mechanism:** TypeScript extensions via embedded QuickJS (~86,500 lines of Rust infrastructure). No Node.js or Bun dependency.

**Capability-gated security architecture:**
- Per-extension trust lifecycle: `pending → acknowledged → trusted → killed`
- Typed hostcall ABI: explicit opcode connectors for tool/exec/http/session/ui/events
- Command mediation with rule-based risk scoring (heredoc AST inspection)
- Policy profiles: safe/balanced/permissive
- Tamper-evident runtime risk ledger
- Shadow dual execution (fast-lane vs compatibility-lane comparison)

**20+ lifecycle events** including `context` (non-destructive message modification before each LLM call), `spawnHook` (rewrite commands, change working directory, inject environment variables), `before_agent_start` (inject messages + modify system prompt per-turn).

**Middleware chain for tool results:**
```typescript
export function onToolResult(result: ToolResult): ToolResult {
  // handlers execute in extension load order
  return { ...result, content: sanitize(result.content) };
}
```

---

### Cursor — Most Novel: Pre-Read Interception

**6 hooks (3 blocking, 3 non-blocking):**

| Hook | Blocking | Description |
|------|---------|-------------|
| `beforeShellExecution` | ✅ | Intercepts bash commands before execution |
| `beforeFileWrite` | ✅ | Intercepts file writes |
| `beforeMCPExecution` | ✅ | Intercepts MCP tool calls |
| `afterShellExecution` | ❌ | Post-execution observation |
| `afterFileWrite` | ❌ | Post-write observation |
| `beforeReadFile` | ✅ | **Unique: intercepts file reads before content reaches LLM** |

`beforeReadFile` is unique in the ecosystem — it enables data loss prevention by controlling what the agent can *see*, not just what it can *do*. Endor Labs uses this to scan for malicious packages before `npm install` reaches the LLM.

**Input format (differs from Claude Code):**
```json
{
  "hook_event_name": "beforeShellExecution",
  "command": "npm install lodash",
  "workingDir": "/project"
}
```

**Output format:**
```json
{
  "action": "block",
  "reason": "Package scanning failed"
}
```

---

### GitHub Copilot CLI / VS Code Copilot

**3 events:** `preToolUse`, `postToolUse`, `preUserMessage`

`preToolUse` supports both blocking and input modification (unique among the limited three). Config lives at `.github/hooks/` for project scope, `~/.github/hooks/` for global.

VS Code Copilot hooks launched in preview with the same three events in `.vscode/hooks.json`. Notably, VS Code adopted `PreToolUse` naming (same as Claude Code) rather than Cursor's `beforeShellExecution`, hinting at convergence.

---

### Cline — Filesystem-Based Discovery

Cline (v3.36+) uses a filesystem-based hook discovery model. Drop executable scripts into:
- `.cline/hooks/pre-tool-use/` → runs before tool execution
- `.cline/hooks/post-tool-use/` → runs after
- `.cline/hooks/pre-task/` → runs at session start
- `.cline/hooks/post-task/` → runs at session end

Scripts receive JSON on stdin with `tool`, `args`, and `context`. Can inject additional context back to the agent. This is the most Unix-philosophy approach — no config file, just scripts in directories.

---

## The Cross-Platform Normalization Problem

### The Core Issue

Every tool uses different names for the same concept:

| Canonical Concept | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot |
|-------------------|-------------|------------|--------|----------|-----------------|
| Before tool runs | `PreToolUse` | `BeforeToolExecution` | `beforeShellExecution` | `pre_run_command` | `preToolUse` |
| After tool runs | `PostToolUse` | `AfterToolExecution` | `afterShellExecution` | `post_run_command` | `postToolUse` |
| Session starts | `SessionStart` | `SessionStart` | N/A | N/A | N/A |
| Block decision | exit `2` + JSON | exit `2` + JSON | `{"action":"block"}` | exit `2` | exit `2` + JSON |
| Modify input | `updatedInput` in JSON | `rewrite` field | N/A | N/A | N/A |

The input JSON shapes also differ significantly. A bash command interception in Claude Code:
```json
{
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf /tmp/old" }
}
```

The same interception in Cursor:
```json
{
  "hook_event_name": "beforeShellExecution",
  "command": "rm -rf /tmp/old",
  "workingDir": "/project"
}
```

Same semantic event. Completely different structure.

### The Asymmetry Problem

Conversion is **not symmetric**. Claude Code has capabilities that no other tool offers:

- `prompt` and `agent` handler types (intelligent evaluation, not just deterministic scripts)
- `updatedInput` transparent argument rewriting
- `TeammateIdle`, `SubagentStart`, `SubagentStop` (multi-agent coordination events)
- `Elicitation` / `ElicitationResult` (structured user input requests)
- `http` handler type (webhook-based hooks)

Writing a Claude Code hook and converting it *down* to Cursor format is lossy. Converting a Cursor hook *up* to Claude Code is potentially lossless. Any normalization system must represent this — not pretend it's symmetric.

### What A Real Normalization Looks Like

The Schalk Neethling article on cross-platform hooks demonstrates the fundamental pattern:

```typescript
function detectHookSystem(input: HookInput): DetectedHook {
  const eventName = input.hook_event_name ?? "";
  
  if (eventName === "beforeShellExecution") {
    return { system: "cursor-shell", command: input.command ?? "" };
  }
  if (eventName === "PreToolUse" || input.tool_name === "Bash") {
    return { system: "claude-code", command: input.tool_input?.command ?? "" };
  }
  return { system: "unknown", command: "" };
}
```

That's the read side. The write side requires producing the correct response shape per tool:

```typescript
// Claude Code expects:
{ "hookSpecificOutput": { "hookEventName": "PreToolUse", "permissionDecision": "deny" } }

// Cursor expects:
{ "action": "block", "reason": "..." }

// Gemini CLI expects:
{ "decision": "block", "reason": "..." }
```

A normalization layer handles this: you write the hook logic once against a canonical API, and adapters handle read/write per tool.

### Canonical Event Taxonomy (Proposed)

Based on the overlapping capabilities across all tools, a minimal canonical event set:

```
before_tool_execute   → maps to PreToolUse / beforeShellExecution / BeforeToolExecution / pre_run_command
after_tool_execute    → maps to PostToolUse / afterShellExecution / AfterToolExecution / post_run_command
before_file_read      → maps to beforeReadFile (Cursor only currently)
before_file_write     → maps to beforeFileWrite (Cursor) / Write matching (Claude Code)
session_start         → maps to SessionStart (Claude Code, Gemini)
session_end           → maps to SessionEnd (Claude Code, Gemini) / Stop (Claude Code)
before_prompt         → maps to UserPromptSubmit (Claude Code) / BeforePromptTemplate (Gemini) / preUserMessage (Copilot)
before_mcp_tool       → maps to mcp__* matching (Claude Code) / beforeMCPExecution (Cursor) / pre_mcp_tool_use (Windsurf)
```

Platform-specific events that don't map cross-platform (must be flagged as non-portable):
- `before_model` / `after_model` (Gemini only)
- `before_tool_selection` (Gemini only)
- `teammate_idle`, `subagent_start`, `subagent_stop` (Claude Code multi-agent only)
- `elicitation` (Claude Code only)
- `worktree_create`, `worktree_remove` (Claude Code only)
- `before_compact`, `after_compact` (Claude Code, OpenCode)

---

## Existing Normalization Attempts

### sondera-ai/sondera-coding-agent-hooks

The most architecturally sophisticated attempt. **2 stars, 5 commits** — basically undiscovered.

Built in Rust. Uses **AWS Cedar** (the same policy language behind AWS IAM) for hook evaluation. Architecture:

1. **Adapters** per tool normalize native JSON into four canonical event categories
2. **Harness server** runs as a Unix socket daemon, evaluates Cedar policies against normalized events
3. **Hook scripts** for each tool connect to the harness server instead of implementing their own logic

Cedar policies evaluate identically across all agents — write once, enforce everywhere:
```cedar
permit(
  principal,
  action == Action::"execute_shell",
  resource
) when {
  !resource.command.contains("rm -rf")
};
```

Curl-to-bash install, production hardening docs (socket ownership, restricted permissions). Genuinely well-designed.

**Why it has 2 stars:** Published in early 2026, niche intersection of Cedar policy knowledge + multi-agent hooks, no marketing.

### assistantkit/hooks

A Go package providing canonical event types and adapter interfaces. Very early, community effort, not an official standard.

### Schalk Neethling's Cross-Platform Hook Pattern

Published March 2026. Blog post showing the detect-normalize-respond pattern for Claude Code + Cursor in TypeScript. The practical foundation of what a normalization library looks like, but it's a tutorial, not a library.

### casr (Dicklesworthstone)

`cross_agent_session_resumer` — converts session formats between tools with a canonical IR. The closest architectural analog to what hook normalization needs:

```
Read source → Canonical IR → Validate → Write target → Verify
```

Key design: read-back verification after write, rollback on failure, explicit lossy conversion warnings. This architecture applies directly to hook conversion.

---

## Why This Hasn't Been Solved

### 1. The Tools Are Too New

Hooks only became widespread in late 2024/early 2025. Claude Code's hook system expanded significantly in 2024, Gemini CLI followed, Cursor hooks are still in beta, VS Code Copilot hooks just landed in preview. The formats haven't been stable long enough for confident cross-tool abstraction. Gemini CLI is actively migrating its session format and has an open issue requesting hook expansion. Building on moving targets is risky.

### 2. The Problem Looks Smaller Than It Is Until You're In It

From the outside: "just rename the events and map the JSON." From the inside: you discover that the response shapes differ, blocking semantics differ (exit codes vs JSON fields vs throw exceptions), some events are asymmetric by nature (model-level hooks, multi-agent events), and maintaining adapters as tools release updates is an ongoing tax.

### 3. Solo Developers Solved It For Themselves

The people who encountered the problem (running multiple agents with consistent enforcement policies) wrote private solutions. The sondera-ai project is the only public attempt, and it's essentially undiscovered. The HumanLayer blog (Feb 2026) explicitly noted hooks as a key harness component but gave no cross-platform guidance.

### 4. The Capability Asymmetry Feels Unsolvable

Claude Code's `prompt` and `agent` handler types have no equivalent anywhere. If you're a Claude Code power user, moving to a normalized format means losing your most powerful hooks. This creates resistance — why normalize when you lose capability?

The answer is: you don't lose capability for users who want cross-platform portability. You just need to clearly model which features are portable vs platform-specific. The asymmetry is a documentation/schema problem, not a technical impossibility.

### 5. There's No Obvious Business Model Anchor

Skills.sh grew because Vercel could anchor it to their deployment platform. Skills help you build things you deploy on Vercel. The hooks normalization problem is pure infrastructure — there's no natural product to attach it to. This reduces incentive for well-funded teams to build it.

---

## The Real Gap

The gap is a **cross-platform hook normalization layer** that:

1. **Defines a canonical hook manifest format** — normalized event names, normalized input schema, normalized response contract — with explicit lossy conversion flags for platform-specific capabilities
2. **Provides adapters per tool** — read native → canonical, write canonical → native
3. **Exposes a CLI** for hook script authors — write once, emit configuration for any target platform
4. **Maintains a registry** of community hooks in the canonical format — installable to any supported tool

The scope that's tractable today: **Claude Code + Gemini CLI + Cursor + VS Code Copilot**. These four cover the tools where hooks are production-stable, semantically overlapping, and have the largest user bases.

### How This Fits Into a Provider Conversion Tool

If you already have a tool that converts between agent providers, hooks normalization is a natural extension:

- The canonical IR pattern is identical (`casr` uses it for sessions; hook normalization uses it for hook configs + scripts)
- You already have the platform detection logic
- Hook configuration files (`.claude/settings.json`, `.cursor/hooks.json`, etc.) are format-converted alongside provider configs — the same mechanics apply

The conversion for hooks is:
```
.claude/settings.json (hooks section) → canonical manifest → .cursor/hooks.json
```

Same pipeline you'd use for MCP server configs, prompts, and skill paths.

### What Makes This Hard

- **Ongoing maintenance burden**: every time a tool updates its hook event format, an adapter needs updating. This is the real cost.
- **Testing**: you need real instances of each tool to validate that converted hooks actually work. Snapshot tests help but aren't sufficient.
- **Semantic drift**: tools may use the same event name for slightly different semantics. `PreToolUse` in Claude Code fires before any tool including MCP; `preToolUse` in Copilot may have subtly different scope.
- **The `updatedInput` capability**: Claude Code can transparently rewrite tool arguments before the tool runs. No other tool offers this. Hooks that rely on it cannot be portably converted.

### What Makes This Approachable

- The core normalization for the most common case (bash command interception → block or allow) is straightforward and covers 80% of real-world hook use cases
- The cass connector pattern is a working proof of concept for exactly this architecture applied to a different domain
- TypeScript/Node is the right implementation language (all major tools have Node-based toolchains, hook scripts are language-agnostic but TypeScript gives you the type safety to define the canonical schema cleanly)
- The spec/schema design work is a genuine technical writing problem, not primarily a programming problem — that's a comparative advantage for this particular builder

---

## Key Projects Reference

### Essential Tooling

| Project | Stars | Language | What It Does |
|---------|-------|----------|-------------|
| [cass](https://github.com/Dicklesworthstone/coding_agent_session_search) | 550 | Rust | Unified CLI/TUI search across 15+ agent session histories |
| [casr](https://github.com/Dicklesworthstone/cross_agent_session_resumer) | — | Rust | Cross-agent session format converter with canonical IR |
| [continues](https://github.com/yigitkonur/cli-continues) | — | TypeScript | Session handoff between 14 CLI tools |
| [claude-history](https://github.com/raine/claude-history) | — | Rust | Fuzzy TUI search for Claude Code JSONL |
| [claude-parser](https://github.com/alicoding/claude-parser) | — | Python | Git-like commands + file recovery from JSONL |
| [chatgpt2md](https://github.com/NextStat/chatgpt2md) | — | Rust | ChatGPT export → Markdown + Tantivy + MCP server |
| [CC Switch](https://github.com/farion1231/cc-switch) | — | Rust/React (Tauri) | Desktop app: MCP/skills/providers across 5 CLI tools |
| [cc-switch-cli](https://github.com/SaladDay/cc-switch-cli) | — | Rust | CLI fork of CC Switch |
| [SkillPort](https://github.com/gotalab/skillport) | — | Python | CLI + MCP server for skill management |
| [sondera-coding-agent-hooks](https://github.com/sondera-ai/sondera-coding-agent-hooks) | 2 | Rust | Cross-platform hook normalization via Cedar policies |
| [franken_agent_detection](https://github.com/Dicklesworthstone/franken_agent_detection) | 5 | Rust | Agent installation detection library |

### Hook Collections

| Project | Coverage | Notes |
|---------|---------|-------|
| [karanb192/claude-code-hooks](https://github.com/karanb192/claude-code-hooks) | Claude Code | Safety + automation hooks, copy-paste ready |
| [disler/claude-code-hooks-mastery](https://github.com/disler/claude-code-hooks-mastery) | Claude Code | 3k+ stars, multi-agent observability patterns |
| [1Password/agent-hooks](https://github.com/1Password/agent-hooks) | Cursor + Copilot | Official 1Password hooks for credential protection |
| [fcakyon/claude-codex-settings](https://github.com/fcakyon/claude-codex-settings) | Claude Code + Codex | Bundled skills/commands/hooks/agents via plugin system |

### Hook Documentation

| Tool | Hook Docs |
|------|----------|
| Claude Code | [code.claude.com/docs/en/hooks](https://code.claude.com/docs/en/hooks) |
| Gemini CLI | [geminicli.com/docs/hooks](https://geminicli.com/docs/hooks/) |
| OpenCode | [opencode.ai/docs/plugins](https://opencode.ai/docs/plugins/) |
| Windsurf | [docs.windsurf.com/windsurf/cascade/hooks](https://docs.windsurf.com/windsurf/cascade/hooks) |
| Cursor | [GitButler deep dive](https://blog.gitbutler.com/cursor-hooks-deep-dive) |
| VS Code Copilot | [code.visualstudio.com/docs/copilot/customization/hooks](https://code.visualstudio.com/docs/copilot/customization/hooks) |

### Context and Analysis

| Resource | Why It's Useful |
|----------|----------------|
| [HumanLayer: Harness Engineering](https://www.humanlayer.dev/blog/skill-issue-harness-engineering-for-coding-agents) | Best framing of hooks as a harness component; honest about tool capability gaps |
| [Schalk Neethling: Cross-Platform Hooks](https://schalkneethling.com/posts/writing-cross-platform-hooks-for-ai-coding-agents/) | Practical TypeScript pattern for Claude Code + Cursor normalization |
| [GitButler: Cursor Hooks Deep Dive](https://blog.gitbutler.com/cursor-hooks-deep-dive) | Technical detail on Cursor's hook system including `beforeReadFile` |
| [Endor Labs: Malware Detection via Cursor Hooks](https://www.endorlabs.com/learn/bringing-malware-detection-into-ai-coding-workflows-with-cursor-hooks) | Real-world security use case; shows what's possible |
| [Gemini Hooks PR #9070](https://github.com/google-gemini/gemini-cli/issues/15292) | Active development; format may change |

---

*Report compiled from research across 80+ sources, March 2026. Ecosystem is moving fast — verify current hook API docs before implementing adapters.*
