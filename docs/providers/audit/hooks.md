# Hooks systems across every AI coding agent: a deep technical comparison

**Claude Code leads the pack with 21 lifecycle events and four handler types, but a fragmented ecosystem of incompatible hooks implementations is emerging across all major AI coding tools — with no cross-platform standard in sight.** Every tool that ships hooks uses a different configuration format, different event names, and different capability boundaries. The practical result: hook scripts written for one tool cannot be reused in another without rewriting. This report dissects the hooks architecture of 18 AI coding tools, identifies which actually ship production-grade hooks (9 of 18), and maps the gaps where security enforcement, automation, and interoperability remain unsolved.

---

## Claude Code sets the bar with 21 events and four handler types

Claude Code's hooks system is the most mature and granular in the ecosystem. It supports **21 lifecycle hook events** organized across session, tool, agent, maintenance, and MCP domains — more than double the event count of any competitor.

### Lifecycle events

The full event catalog spans six categories. **Session events** include `SessionStart`, `SessionEnd`, and `Setup` (fired via `--init` or `--maintenance` flags). **User interaction** offers `UserPromptSubmit`, which fires before Claude processes input. **Tool execution** covers `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, and `PermissionRequest` — all of which accept regex matchers filtering by tool name (e.g., `Bash`, `Edit|Write`, `mcp__github__.*`). **Agent events** include `Stop`, `SubagentStart`, `SubagentStop`, `TeammateIdle`, and `TaskCompleted`. **Maintenance events** cover `PreCompact`, `PostCompact`, `Notification`, `InstructionsLoaded`, and `ConfigChange`. **Infrastructure events** add `WorktreeCreate`, `WorktreeRemove`, `Elicitation`, and `ElicitationResult`.

Sixteen built-in tools can be matched by name: `Read`, `Write`, `Edit`, `MultiEdit`, `Bash`, `Grep`, `Glob`, `LS`, `WebFetch`, `WebSearch`, `NotebookEdit`, `NotebookRead`, `TodoRead`, `TodoWrite`, `Task`, and `KillBash`. MCP tools use the `mcp__<server>__<tool>` naming convention with regex support.

### Four handler types make Claude Code unique

No other tool offers this breadth of hook execution models:

- **`command`** — Shell scripts receiving JSON on stdin. The workhorse for deterministic enforcement.
- **`http`** — POSTs event JSON to an HTTP endpoint with configurable headers and environment variable injection. Ideal for webhook-based workflows.
- **`prompt`** — Single-turn LLM evaluation (uses Haiku by default) for judgment-based decisions. Returns structured JSON with `approve`/`block` decisions.
- **`agent`** — Spawns a multi-turn sub-agent with read-only tool access (`Read`, `Grep`, `Glob`) to evaluate complex conditions before deciding.

The `prompt` and `agent` types blur the line between deterministic and intelligent enforcement — a capability unique to Claude Code. Combined with `async: true` for background execution, hooks can run non-blocking test suites or monitoring tasks that deliver results on the next conversation turn.

### Exit codes and structured output

Exit code **0** signals success; Claude parses stdout for JSON. Exit code **2** triggers blocking behavior that varies by event — `PreToolUse` blocks the tool call, `Stop` forces continued work, `PermissionRequest` denies permission. Other exit codes produce non-blocking warnings. Structured JSON output supports `permissionDecision` (allow/deny), `updatedInput` (transparent argument rewriting), `systemMessage` (context injection), `additionalContext`, `continue: false` (halt all processing), and `suppressOutput`.

### Configuration hierarchy

Hooks live in four locations with clear precedence: **user-level** (`~/.claude/settings.json`), **project-level** (`.claude/settings.json`, committable to repos), **project-local** (`.claude/settings.local.json`, gitignored), and **managed enterprise** (admin-controlled, cannot be overridden). Hooks can also be defined in skill/agent YAML frontmatter within `.claude/agents/*.md` files. A `disableAllHooks: true` flag exists for bulk disable, but managed hooks resist user-level disabling.

### Security: powerful but unsandboxed

**Command hooks run with full user-level permissions — there is no sandbox.** This is explicitly documented. A malicious `.claude/settings.json` committed to a cloned repository could execute arbitrary commands. Hooks fire recursively for subagent actions, preventing bypass through delegation. Claude Code snapshots hook configuration at startup to prevent mid-session tampering. The community project `lasso-security/claude-hooks` specifically targets prompt injection defense by scanning outputs for instruction override attempts.

---

## The competitive landscape: who actually ships hooks

Nine of eighteen researched tools have production or experimental hooks. The remaining nine rely on rules files, MCP servers, or have no extension mechanism at all.

| Tool | Hook Events | Config Format | Can Block | Can Modify Args | Handler Types | Maturity |
|------|------------|---------------|-----------|-----------------|---------------|----------|
| **Claude Code** | 21 | JSON (settings.json) | ✅ | ✅ (updatedInput) | command, http, prompt, agent | Production |
| **Gemini CLI** | 11 | JSON (settings.json) | ✅ | ✅ (rewrite) | command | Production |
| **OpenCode** | 25+ | TypeScript plugins | ✅ (throw) | ✅ (output object) | JS/TS modules | Production |
| **Windsurf** | 12 | JSON (hooks.json) | ✅ (exit 2) | ❌ | command | Production |
| **Pi Agent Rust** | 20+ | TypeScript extensions | ✅ | ✅ (middleware) | JS/TS via QuickJS | Production |
| **Cursor** | 6 | JSON (hooks.json) | ✅ (3 of 6) | ❌ | command | Beta |
| **Cline** | 4 | Script files in dirs | ✅ (PreToolUse) | ✅ (context inject) | executable scripts | Production |
| **Kiro** | 5+5 (IDE/CLI) | .kiro.hook / JSON | ✅ (CLI) | ❌ | askAgent + command | Preview |
| **GitHub Copilot CLI** | 3 | JSON (.github/hooks/) | ✅ (preToolUse) | ✅ | command | Production |
| **Codex CLI** | 2 | TOML | ❌ | ❌ | command | Experimental |
| **Amp** | 2+ | TypeScript plugins | ✅ | ❌ | JS/TS modules | Experimental |
| **Crush** | Plugin hooks | Go modules (compiled) | ❌ | ❌ | Go compiled | Emerging |

Tools with **no hooks system**: Aider, Roo Code, Continue.dev, Goose, Plandex, Mentat. Aider offers built-in `--auto-lint` and `--auto-test` flags plus git pre-commit integration. Roo Code uses declarative rules and custom modes. Continue.dev relies on `config.yaml` rules and context providers. Goose uses MCP-based extensions exclusively.

---

## Gemini CLI and OpenCode rival Claude Code in different dimensions

**Gemini CLI** ships 11 lifecycle events with particularly strong model-level hooks that no other tool offers. Its `BeforeModel` and `AfterModel` events fire before/after LLM requests, enabling prompt modification, model swapping, response mocking, and output redaction. `BeforeToolSelection` can filter which tools the LLM sees before it chooses. The extension system allows bundling hooks in distributable packages with a three-tier precedence (system → user → project). Hooks run in a **sanitized environment** — extension policies explicitly cannot set `allow` decisions or enable yolo mode, a security constraint absent from most competitors.

**OpenCode** takes a radically different approach with a full TypeScript/JavaScript plugin architecture built on Bun. Rather than shell scripts receiving JSON, plugins export typed event handlers with access to the OpenCode SDK client. The event catalog exceeds **25 event types** spanning tool execution, file changes, LSP diagnostics, session lifecycle, permissions, and TUI interactions. Plugins can register custom tools that override built-in ones, inject shell environment variables, and even manipulate the TUI. Distribution via npm packages (`opencode.json` config) with automatic Bun-based installation makes it the most developer-friendly hooks system for JavaScript/TypeScript teams.

**Windsurf** has the most symmetric pre/post coverage with **12 hook types** — five pre-hooks (`pre_read_code`, `pre_write_code`, `pre_run_command`, `pre_mcp_tool_use`, `pre_user_prompt`) and seven post-hooks. Its `post_cascade_response_with_transcript` event provides the complete conversation transcript including all tool calls and responses — useful for comprehensive audit logging. Enterprise deployment is mature: hooks can be distributed via cloud dashboard, MDM, and system-level config files at `/etc/windsurf/hooks.json`.

---

## Pi Agent Rust has the deepest extension security model

Pi Agent Rust (Dicklesworthstone's `pi_agent_rust`) deserves special attention for its **capability-gated security architecture** — the most sophisticated security model in the hooks ecosystem. Extensions are TypeScript modules running in an embedded QuickJS runtime (no Node.js or Bun dependency), with ~86,500 lines of Rust implementing the extension infrastructure.

The security model enforces **per-extension trust lifecycle** (`pending → acknowledged → trusted → killed`), **typed hostcall ABI boundaries** (extensions interact with the host through explicit opcode connectors for tool/exec/http/session/ui/events), and **command mediation** with rule-based risk scoring including heredoc AST inspection. Policy profiles (safe/balanced/permissive) control what extensions can do. A **tamper-evident runtime risk ledger** enables verification and replay of extension actions, and **shadow dual execution** samples compare fast-lane versus compatibility-lane results to catch silent behavioral drift.

The hook event catalog covers **20+ lifecycle events** across session, agent, tool, model, and input domains. The `tool_call` event can block with reasons; `tool_result` can modify results in a middleware chain (handlers execute in extension load order). The `context` event allows non-destructive message modification before each LLM call — enabling extensions to prune, reorder, or enrich the conversation history. The `before_agent_start` event can inject messages and modify the system prompt per-turn. The bash tool even supports a dedicated `spawnHook` that can rewrite commands, change the working directory, and inject environment variables before execution.

Of 224 cataloged extensions, **187 pass** automated conformance tests unmodified. Cold load time is sub-100ms (P95). The tradeoff: this sophistication comes with complexity. The QuickJS runtime doesn't support all Node.js APIs (compatibility shims cover most cases), and 13% of complex multi-dependency extensions fail conformance.

---

## What hooks can actually do across the ecosystem

The practical capabilities of hooks cluster into five categories, with varying support across tools:

**Security enforcement** is the highest-value use case. PreToolUse hooks can block dangerous bash commands (`rm -rf /`, `DROP TABLE`, `curl | sh`), prevent writes to protected files (`.env`, `.git/`, `package-lock.json`), and gate MCP tool calls. Cursor's unique `beforeReadFile` hook enables data loss prevention by intercepting file reads before content reaches the LLM — no other tool offers this. Endor Labs has built a production integration scanning `npm install`/`pip install` commands for malicious packages through Cursor's `beforeShellExecution` hook.

**Automated quality enforcement** covers auto-formatting (Prettier/ESLint via PostToolUse), TDD enforcement (Stop hooks that block completion until tests pass), and auto-linting. A critical gotcha in Claude Code: if formatters change files in PostToolUse hooks, Claude receives system reminders each time, consuming context window. Prefer formatting on `Stop` events instead.

**Workflow automation** includes auto-committing changes (GitButler integrates with Claude Code via PreToolUse/PostToolUse/Stop hooks for automatic branch management), desktop notifications when the agent needs input, and context injection at session start (e.g., injecting the current git branch).

**Observability** is an emerging pattern. Crush's OTLP hook exports OpenTelemetry traces for every session, message, and tool call. The `disler/claude-code-hooks-multi-agent-observability` project provides real-time dashboards with SQLite storage and Vue.js visualization. Windsurf's `trajectory_id` and `execution_id` fields enable precise correlation of hook events.

**AI-powered evaluation** is currently unique to Claude Code (via `prompt` and `agent` handler types) and Kiro IDE (via `askAgent` actions). This enables non-deterministic but context-aware decisions: "Are all tasks complete and were tests run and passing?" evaluated by a secondary LLM call rather than a shell script.

---

## No hooks standard exists, but convergence patterns are emerging

There is **no formal cross-platform hooks standard** — no RFC, no working group, no specification. Each tool uses different event names (`PreToolUse` vs `BeforeTool` vs `tool.execute.before` vs `preToolUse` vs `pre_run_command`), different configuration formats (JSON settings vs TypeScript plugins vs Go modules vs executable scripts in directories), and different communication protocols.

The closest thing to a cross-tool abstraction is **`assistantkit/hooks`**, a Go package published in March 2026 that provides canonical event types and adapter interfaces for converting between Claude Code, Cursor, and Windsurf hook formats. It's a community effort, not an official standard.

Several **convergence patterns** are visible. Most tools with hooks support the same core event pair: before-tool and after-tool execution. JSON-over-stdin with exit-code-based control flow is the dominant communication protocol (used by Claude Code, Gemini CLI, Cursor, Windsurf, Kiro CLI, and GitHub Copilot CLI). Three-tier configuration precedence (system/user/project) appears in Claude Code, Gemini CLI, Cursor, and Windsurf. The AGENTS.md standard (adopted by 60,000+ repos under the Linux Foundation's Agentic AI Foundation) standardizes *instructions* but explicitly does not cover hooks.

Community hook collections cluster heavily around Claude Code: `karanb192/claude-code-hooks` (safety and automation hooks), `disler/claude-code-hooks-mastery` (3k+ stars, multi-agent patterns), and `affaan-m/everything-claude-code` (cross-tool harness supporting Claude Code, Codex, OpenCode, and Cursor). GitHub Copilot has `github/awesome-copilot` with standardized hook templates. 1Password ships official agent hooks for Cursor and GitHub Copilot. No centralized hooks marketplace exists yet.

---

## Hooks and MCP servers solve fundamentally different problems

The relationship between hooks and MCP servers is **complementary, not competitive**. Hooks provide deterministic enforcement at specific lifecycle points — they always fire when their event occurs, execute as external processes with zero context window cost, and can block or modify agent behavior. MCP servers provide capabilities — external tools, data sources, and APIs that the agent decides whether to use, consuming context tokens in the process.

In practice, **hooks enforce discipline while MCP provides intelligence**. A combined workflow might use an MCP server to query a vulnerability database, while a PreToolUse hook deterministically blocks `npm install` of any package not on an allowlist. Hooks can intercept MCP tool calls (Claude Code's `mcp__github__.*` matchers, Cursor's `beforeMCPExecution`, Windsurf's `pre_mcp_tool_use`), creating a governance layer over MCP capabilities.

Hooks cannot replace MCP for dynamic tool discovery, structured data retrieval, or multi-step reasoning about external systems. MCP cannot replace hooks for guaranteed execution, zero-cost enforcement, or blocking operations before they happen.

---

## Security implications cut both ways

Hooks are simultaneously the strongest security enforcement mechanism and a significant attack surface in AI coding tools. **No tool sandboxes hook execution** — every implementation runs hooks with the user's full system permissions. This is an industry-wide gap.

The attack surface is real. Project-level hook configurations (`.claude/settings.json`, `.cursor/hooks.json`, `.github/hooks/`) committed to git repos mean a **malicious pull request could introduce hooks that exfiltrate data or execute arbitrary commands**. Input injection is a risk when hook scripts construct shell commands from unsanitized JSON stdin data. If an AI agent has write access to hook configuration files, it could disable or modify its own guardrails — the `affaan-m/everything-claude-code` project includes "config protection hooks" specifically to prevent this.

On defense, hooks enable patterns impossible through any other mechanism: deterministic command blocking, secret scanning before file reads (Cursor's `beforeReadFile`), dependency supply chain scanning (Endor Labs integration), complete audit trails for compliance, and MCP governance via tool-call interception. The enterprise deployment story is strongest in Windsurf (MDM + cloud dashboard + system-level config) and Claude Code (managed enterprise settings that resist user-level override).

---

## What's missing and where the ecosystem is headed

The most significant gap is **the absence of PostToolUse undo capability**. In every tool, post-execution hooks can only provide feedback after the action has already completed — they cannot roll back file writes, revert command side effects, or undo MCP operations. Claude Code's `PostToolUse` "block" decision merely prompts the agent with an error message. True transactional hooks with rollback would be transformative for safety-critical environments.

Other notable gaps: **no tool offers per-hook disable** (Claude Code's `disableAllHooks` is all-or-nothing), **no visual hook editor exists** (all configuration is JSON/YAML/TypeScript), **cross-platform shell compatibility** remains unsolved (macOS and Linux commands differ), and **matcher granularity is limited** — most tools only match by tool name, not by file path, argument content, or other contextual fields. Filtering by file path requires parsing `tool_input` inside the hook script itself.

The trajectory is clear: hooks are becoming more important, not less. Every major tool has shipped or announced hooks in the past year. Security vendors are building dedicated integrations. Multi-agent orchestration hooks (Claude Code's `SubagentStart`/`SubagentStop`, `TeammateIdle`) will become critical as agent swarms proliferate — Gartner reports a **1,445% surge** in multi-agent system inquiries from Q1 2024 to Q2 2025. The next frontier is likely a formal hooks specification emerging from the AGENTS.md working group or a parallel effort, driven by enterprise demand for consistent security enforcement across heterogeneous AI coding tool stacks.

## Conclusion

Claude Code's hooks system is the most comprehensive by event count (21), handler type diversity (4), and ecosystem maturity — but it is not the most secure (Pi Agent Rust's capability-gated architecture is significantly more hardened) nor the most developer-friendly (OpenCode's TypeScript plugin system is more ergonomic for JS/TS teams). The "best" hooks system depends on what you optimize for: **Claude Code for breadth and flexibility**, **Pi Agent Rust for security rigor**, **OpenCode for developer experience**, **Gemini CLI for model-level control**, and **Windsurf for enterprise deployment**. The fragmentation across tools is the ecosystem's biggest weakness — a cross-platform hooks standard would unlock reusable security policies, shared hook libraries, and portable automation, but no such standard is on the horizon.
