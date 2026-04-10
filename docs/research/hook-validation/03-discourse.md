# Community Discourse: Cross-Platform AI Coding Tool Hooks

Research date: 2026-03-21

This document captures community discourse around cross-platform AI coding tool hooks — what people are asking for, problems they're hitting, solutions they've tried, and whether there's demand for standardization.

---

## 1. Schalk Neethling — "Writing Cross-Platform Hooks for AI Coding Agents"

**Source:** https://schalkneethling.com/posts/writing-cross-platform-hooks-for-ai-coding-agents/

### Problem
AI agents bypass project-specific test runners and conventions, pattern-matching from training data instead. Documentation (AGENTS.md, CLAUDE.md) provides guidance but does not enforce behavior. Agents "flip-flop between using the test runner and calling tools directly."

### Approach
A shared hook script in a `hooks/` directory, with platform-specific config files (Claude Code and Cursor) that point to the same script. The hook intercepts tool calls and enforces project conventions deterministically.

### Key technical findings — input normalization challenges
- **Input differences:** Claude Code nests commands in `tool_input.command`; Cursor places them at the top level under `command`. A `detectHookSystem()` function normalizes these.
- **Response format differences:** Claude Code uses `hookSpecificOutput` with `permissionDecision`; Cursor uses a `permission` field. The hook builds platform-specific responses dynamically.
- **Pattern matching:** Uses a "fail-safe" approach — allowed patterns checked first, then blocked patterns, enabling short-circuit evaluation.

### Relevance to syllago
This is exactly the problem syllago's hook normalization solves at the format level. Neethling's manual approach (write a detectHookSystem function per hook) is what syllago would automate through its canonical format and per-provider adapters.

---

## 2. Sondera AI — Cross-Agent Reference Monitor

**Source:** https://github.com/sondera-ai/sondera-coding-agent-hooks
**Blog:** https://blog.sondera.ai/p/hooking-coding-agents-with-the-cedar

### Problem
Agent security vulnerabilities (EchoLeak, CurXecute). Existing protections are either unreliable ("prompt and pray") or overly restrictive (full sandboxing).

### Approach — Rust hook binaries + Cedar policies
Sondera is the most sophisticated cross-platform hook project found. Architecture:
- **Hook Adapters:** Per-agent binaries (Claude Code, Cursor, Copilot, Gemini CLI) communicate via stdin/stdout JSON, normalizing agent-specific events into common types.
- **Normalization model:** Four event categories — Actions (ShellCommand, FileRead, FileWrite, WebFetch), Observations (post-execution responses), Control (lifecycle), State (environment context).
- **Cedar Policy Engine:** Deterministic allow/deny/escalate decisions. "A single matching forbid overrides any permit."
- **Harness Service Layer:** Coordinates YARA-X signatures, LLM classifiers, and information flow control via Unix socket RPC.

### Key design decisions
1. Deterministic (YARA + Cedar) combined with optional probabilistic (LLM classifiers)
2. Agent-agnostic rules — single Cedar policies work identically across all four agents through normalization
3. Composition over monolith — multiple policy files evaluated together

### Relevance to syllago
Sondera's normalization model (mapping agent-specific tool names to common types) is very close to what syllago's hook conversion would do. Their four-event-category taxonomy (Actions, Observations, Control, State) could inform syllago's canonical hook event model. Key difference: Sondera is a runtime enforcement system; syllago is a content distribution system. They're complementary — syllago could distribute hooks that Sondera enforces.

### Community reception
Hacker News thread (https://news.ycombinator.com/item?id=47322752) had limited discussion — mostly an announcement post. No significant debate or pushback, suggesting the space is still early.

---

## 3. runkids/ai-hooks-integration — Cross-Tool Hook Skill

**Source:** https://github.com/runkids/ai-hooks-integration

### Problem
Developers using multiple AI coding tools face: duplicating hook logic across incompatible config formats, no unified interface for common tasks (linting, security, CI/CD), risk of data loss during manual configuration edits.

### Approach
A reusable AI skill (not a standalone tool) that integrates CLI hooks across four platforms:
- Claude Code (~/.claude/settings.json)
- Gemini CLI (~/.gemini/settings.json)
- Cursor (~/.cursor/hooks.json)
- OpenCode (~/.config/opencode/plugins/*.js)

Cross-tool event mapping: `PreToolUse <-> beforeShellExecution <-> tool.execute.before <-> BeforeTool`

Uses idempotent JSON merging scripts (merge_hooks.py, remove_hooks.py) that preserve existing configurations.

### Relevance to syllago
This project validates the core need syllago addresses — cross-platform hook distribution with format translation. The event mapping table (`PreToolUse <-> beforeShellExecution <-> tool.execute.before <-> BeforeTool`) is essentially what syllago's canonical format would encode. Notable gap: Gemini IDE lacks hook APIs entirely, requiring Gemini CLI instead.

---

## 4. GitHub Copilot Agent Hooks — Official Specification

**Source:** https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks

### Design
- Hooks stored in `.github/hooks/*.json` (repository-bound, team-shared)
- Event types: sessionStart, sessionEnd, userPromptSubmitted, preToolUse, postToolUse, agentStop, subagentStop, errorOccurred
- Hooks run synchronously, blocking agent execution
- Performance target: under 5 seconds per hook

### Key difference from Claude Code
Copilot treats the repository as the trust anchor (hooks on default branch). Claude Code starts from the user's local environment and layers organizational controls on top. This philosophical difference affects how hooks are distributed and who controls them.

### Relevance to syllago
Copilot's repo-bound model vs Claude Code's user-local model is a key tension syllago needs to navigate. A hook distributed via syllago might install to `~/.claude/settings.json` for Claude Code but to `.github/hooks/` for Copilot — different trust models, different filesystem locations.

---

## 5. VS Code Agent Hooks (Preview)

**Source:** https://code.visualstudio.com/docs/copilot/customization/hooks

VS Code hooks use the same format as Copilot CLI for compatibility. Hooks execute at lifecycle points during agent sessions: automate workflows, enforce security policies, validate operations, integrate with external tools. Designed to work across agent types (local, background, cloud agents).

### Relevance to syllago
VS Code converging on the Copilot hook format means one fewer format to support — Copilot and VS Code agent hooks are the same target.

---

## 6. Cursor Hooks — Deep Dive (GitButler)

**Source:** https://blog.gitbutler.com/cursor-hooks-deep-dive
**Docs:** https://cursor.com/docs/hooks

### Architecture
Six lifecycle hooks in `.cursor/hooks.json` (project) or `~/.cursor/hooks.json` (global):
1. `beforeSubmitPrompt` — captures prompt context before LLM processing
2. `beforeShellExecution` — validates shell commands (allow/deny/ask)
3. `beforeMCPExecution` — controls MCP tool calls
4. `beforeReadFile` — filters file contents before LLM sees them (secret redaction)
5. `afterFileEdit` — logs file modifications post-edit
6. `stop` — signals task completion with follow-up capability

Hooks receive JSON over stdin, return JSON to Cursor. Scripts resolved relative to hooks.json file.

### Pain points identified
- **Input-only hooks lack output control:** `beforeSubmitPrompt` and `afterFileEdit` cannot influence agent behavior — "you cannot do much here other than record this information."
- **Limited response capabilities:** Only `beforeShellExecution` and `beforeMCPExecution` return JSON responses.
- **Beta instability:** API explicitly in beta with potential breaking changes, creating friction for production adoption.
- **Silent failures:** Scripts must be chmod +x or hooks fail silently. Windows requires PowerShell workarounds.
- **No diff in afterFileEdit:** Provides old/new file strings, not a diff.

### Missing capabilities developers want
- Output/feedback from all hooks (not just execution gates)
- Real-time prompt modification before submission
- Cross-hook state sharing for coordinated workflows
- Stronger isolation guarantees between concurrent hook executions

### Relevance to syllago
Cursor's hook format is simpler than Claude Code's but has significant asymmetries: different event names, different JSON schemas, different response formats. The beta status and limited response capabilities mean syllago needs to handle "lossy" conversion — some Claude Code hook behaviors have no Cursor equivalent.

---

## 7. Kiro (AWS) — Agent Hooks with Natural Language Configuration

**Source:** https://kiro.dev/docs/hooks/
**Blog:** https://kiro.dev/blog/automate-your-development-workflow-with-agent-hooks/

### Architecture
Hooks stored in `.kiro/hooks/` as individual files. Two action types:
- **Agent Prompt** ("Ask Kiro") — invokes the agent with natural language instructions (consumes credits)
- **Shell Command** — deterministic execution (no credits)

Events: file operations (created, saved, deleted), prompt submit, agent stop, pre/post tool use, spec task events, manual triggers.

### Key distinction from other systems
Kiro emphasizes natural language configuration — hooks can be created by describing what you want in plain English. This is a fundamentally different model from the JSON+shell approach used by Claude Code, Cursor, and Copilot.

### Relevance to syllago
Kiro's natural-language hook definition creates a conversion challenge. A Claude Code hook (JSON config + shell script) doesn't have a direct equivalent in Kiro's model. Syllago might need to treat Kiro as a partial-support target where some hooks convert cleanly (shell command actions) and others require manual adaptation (agent prompt actions).

---

## 8. Gemini CLI Issue #17475 — Bridge Ecosystems Request

**Source:** https://github.com/google-gemini/gemini-cli/issues/17475

### The request
A feature request to import plugin bundles (including hooks) from other ecosystems into Gemini CLI. Proposed a CLI command like `gemini extensions import <source>` or documented conversion flow, starting with Claude Code as first reference target.

### Problem statement (from the issue)
> "Even when ecosystems provide the same building blocks (commands, skills, hooks, MCP), you currently have to re-author everything because the formats and folder structures differ. That slows migration, duplicates work, and limits reuse of community extensions."

### Proposed hook mapping
The issue included a detailed hook event mapping table identifying direct mappings, approximate translations, and unsupported events between Gemini CLI and Claude Code.

### Status
Closed as duplicate, redirected to issue #17505. Only 2 comments — low engagement suggests the community hasn't coalesced around this yet.

### Relevance to syllago
This is direct validation of syllago's value proposition, stated by a community member asking for exactly what syllago provides. The fact that it was filed as a Gemini CLI feature request (rather than as a standalone tool) suggests people don't yet know that a cross-provider solution could exist as a separate tool.

---

## 9. HumanLayer — Harness Engineering for Coding Agents

**Source:** https://www.humanlayer.dev/blog/skill-issue-harness-engineering-for-coding-agents

### Philosophy on hooks
HumanLayer/CodeLayer (built on Claude Code) takes a pragmatic approach: hooks are added reactively when agents actually fail, not preemptively. The author states: "I have thrown away many more hooks than we actually use today."

### What they keep vs discard
- **Keep:** Verification hooks (typecheck + format on stop, exit code 2 to re-engage agent), notification hooks, dangerous-command blockers
- **Discard:** Hooks that flood context windows, overly specific hooks, hooks that try to do too much

### Key insight — context efficiency
Hooks must be context-efficient. Running full test suites as hooks floods the context window. HumanLayer suppresses all passing output, surfacing only errors.

### Cross-platform gap
The article notes that Claude Code and OpenCode support hooks, but "Sadly, Codex doesn't have an equivalent." This highlights the fragmented support landscape.

### Back-pressure pattern
The core insight: "your likelihood of successfully solving a problem with a coding agent is strongly correlated with the agent's ability to verify its own work." Hooks are the mechanism for building this back-pressure.

### Relevance to syllago
HumanLayer's experience validates that hooks are high-leverage but need curation. A syllago registry of hooks could help teams skip the trial-and-error phase. The "thrown away many more than we use" observation suggests community-vetted hook collections would have significant value.

---

## 10. GSD — One Codebase, Three Runtimes

**Source:** https://medium.com/@richardhightower/one-codebase-three-runtimes-how-gsd-targets-claude-code-opencode-and-gemini-cli-29c98cfe96c6

### Approach
GSD maintains a single canonical codebase and generates platform-specific deployments for Claude Code, OpenCode, and Gemini CLI. The installer acts as a translation layer, transforming file structures, naming conventions, and tool vocabularies.

### Hook handling
"Each platform has its own hook configuration mechanism, so GSD handles these in platform-specific code paths. This is one area where a unified abstraction does not yet exist; the hook logic for each runtime is maintained separately."

### Relevance to syllago
GSD explicitly calls out that hooks are the hardest thing to abstract across platforms — they maintain separate code paths for each runtime. This validates that a canonical hook format (as syllago proposes) is solving a real unsolved problem.

---

## 11. rulesync — Unified Rule Management

**Source:** https://dev.to/dyoshikawatech/rulesync-published-a-tool-to-unify-management-of-rules-for-claude-code-gemini-cli-and-cursor-390f

### Problem
"Various AI coding tools have emerged, each defining their own rule file specifications. Managing these files individually is quite tedious — you have to write rules in different locations and formats for each tool."

### Approach
A single source rule file that auto-generates tool-specific outputs for `.github/instructions/*.instructions.md`, `.cursor/rules/*.mdc`, `CLAUDE.md`, `GEMINI.md`, etc.

### Relevance to syllago
Rulesync solves the rules version of what syllago solves for hooks. Its existence validates that the multi-tool format translation problem is real and people are building tools to solve it. Rulesync is rules-only; syllago covers hooks, MCP, skills, and more.

---

## 12. CCManager — Coding Agent Session Manager

**Source:** https://github.com/kbwo/ccmanager

### Overview
Session manager spanning 8+ coding agents (Claude Code, Gemini CLI, Codex CLI, Cursor Agent, Copilot CLI, Cline CLI, OpenCode, Kimi CLI). Supports context preservation across branches and custom commands on session status changes.

### Hook-adjacent features
Worktree hooks execute custom commands when worktrees are created — post-creation hooks, environment variables, async execution. Not agent-level hooks per se, but shows the demand for lifecycle automation across multiple tools.

---

## 13. Dippy — Cross-Platform Command Approval

**Source:** https://github.com/hesreallyhim/awesome-claude-code (listed project)

Auto-approves safe bash commands using AST-based parsing while prompting for destructive operations. Supports Claude Code, Gemini CLI, and Cursor. Solves "permission fatigue" — the constant approve/deny dialog that slows down agentic workflows.

### Relevance to syllago
A concrete example of a hook that works across three platforms. The author had to implement per-platform adapters manually — exactly what syllago would automate.

---

## 14. Security Concerns — Hooks as Attack Vectors

**Sources:**
- https://blakecrosley.com/blog/claude-code-hooks-tutorial
- https://paddo.dev/blog/claude-code-hooks-guardrails/

### CVE disclosures (Feb 2026)
Check Point Research disclosed CVE-2025-59536, CVE-2026-21852, CVE-2026-24887 — RCE through Claude Code. Hooks in `.claude/settings.json` were part of the attack vector: malicious project files could define hooks that execute automatically when Claude loads an untrusted repo.

### Mitigation
Claude Code now snapshots hooks at session start and asks users to review changes via `/hooks`. Settings edits don't hot-apply.

### Community sentiment
Multiple blog posts emphasize "hooks > prompts for security" because hooks are deterministic. But the CVE disclosures show that hooks themselves need security — untrusted hooks are dangerous. This creates a trust-chain problem: who wrote the hook, and should you run it?

### Relevance to syllago
Hook distribution via syllago needs a trust/provenance story. If syllago distributes hooks from a registry, users need confidence that those hooks are safe. The CVE history shows this is not hypothetical — hooks can be weaponized.

---

## 15. Medium/Blog Ecosystem — Common Hook Use Cases

**Sources:**
- https://medium.com/coding-nexus/claude-code-hooks-5-automations-that-eliminate-developer-friction-7b6ddeff9dd2
- https://medium.com/@peterphonix/using-command-hooks-to-keep-your-ai-agent-on-track-740a097c8cbc
- https://www.datacamp.com/tutorial/claude-code-hooks

### Most popular hook patterns
1. **Auto-formatting** — run Prettier/Black/gofmt after every file edit
2. **Dangerous command blocking** — prevent `rm -rf`, `DROP TABLE`, force pushes
3. **Test verification** — run tests on stop, re-engage agent if they fail
4. **Desktop notifications** — alert when agent finishes or needs input
5. **Lint enforcement** — run linters after edits, surface errors to agent
6. **Secret scanning** — block commands that would expose credentials

### Community sentiment
Strong enthusiasm for hooks as the "missing piece" between prompt-based guidance and deterministic enforcement. The phrase "hooks > rules" appears frequently. Friction points: setup complexity (JSON editing, chmod, debugging silent failures), lack of cross-platform portability.

---

## Cross-Cutting Themes

### Theme 1: Convergent event models, divergent formats
Every platform has pre-tool-use and post-tool-use hooks. The concept is identical; the naming, JSON schema, and response format differ. This is the core problem syllago solves.

| Claude Code | Cursor | Copilot | Gemini CLI | Kiro | OpenCode |
|-------------|--------|---------|------------|------|----------|
| PreToolUse | beforeShellExecution | preToolUse | BeforeTool | Pre Tool Use | tool.execute.before |
| PostToolUse | afterFileEdit | postToolUse | AfterTool | Post Tool Use | tool.execute.after |
| Stop | stop | agentStop | AgentStop | Agent Stop | session.idle |

### Theme 2: Trust model fragmentation
- Claude Code: user-local (`~/.claude/settings.json`) + project (`.claude/settings.json`)
- Cursor: project (`.cursor/hooks.json`) + user (`~/.cursor/hooks.json`)
- Copilot: repo-bound (`.github/hooks/*.json`, must be on default branch)
- Kiro: project (`.kiro/hooks/`)
- Gemini CLI: user-local (`~/.gemini/settings.json`)

Syllago needs to handle these different trust anchors during installation.

### Theme 3: Lossy conversion is inevitable
Not every hook capability exists across all platforms:
- Claude Code has 21 lifecycle events; Cursor has 6
- Claude Code supports input modification; most others don't
- Kiro has file-system triggers; no one else does
- Copilot has subagentStop; not all platforms have subagent hooks
- Claude Code has prompt and agent handler types; others only have shell commands

Syllago's canonical format needs to represent the superset, with per-provider converters handling the lossy mapping and documenting what's lost.

### Theme 4: Enterprise demand exists but is underserved
Enterprise users need: policy enforcement across multiple tools, audit logging, dangerous-command blocking, secret scanning. These are the highest-value hook use cases, and they're the ones most painful to maintain across platforms manually. The Sondera project and Copilot's enterprise hooks feature are early attempts to serve this need.

### Theme 5: The ecosystem is early and fragmented
- No dominant standard has emerged for cross-platform hooks
- Multiple projects (Sondera, ai-hooks-integration, GSD, rulesync) are solving pieces of the problem independently
- Community engagement on cross-platform hook topics is moderate — most discussion is platform-specific
- The Gemini CLI feature request for plugin import got only 2 comments before being closed
- Hacker News discussion of Sondera's Cedar-based approach was minimal

### Theme 6: Hooks vs rules — complementary, not competing
Community consensus is that hooks enforce deterministic behavior (always runs, exit-code-based), while rules/prompts guide probabilistic behavior (LLM interprets, might ignore). Both are needed. Hooks handle "must never" and "must always" constraints; rules handle "prefer" and "when possible" guidance.

---

## Demand Signals for Standardization

### Direct evidence
1. **Gemini CLI #17475** — explicit request for cross-ecosystem plugin import with hook mapping
2. **ai-hooks-integration** — community skill built specifically for cross-tool hook deployment
3. **Sondera** — Rust-based normalization layer across 4 agents, proving the approach works
4. **GSD** — explicitly calls out hooks as the area where "a unified abstraction does not yet exist"
5. **rulesync** — solves the same problem for rules, validating the format-translation approach

### Indirect evidence
1. Multiple blog posts describe manually writing cross-platform hooks (Neethling)
2. Developers building notification hooks that span Claude Code + Gemini CLI + Codex (code-notify)
3. Session managers spanning 8+ agents (CCManager) need per-tool hook configuration
4. Enterprise security tools (Dippy, Parry) implementing per-platform adapters manually

### Counter-signals
1. Low engagement on cross-platform hook discussions (HN, GitHub issues)
2. Most community content is platform-specific (Claude Code hooks tutorials dominate)
3. No major vendor has proposed a cross-platform hook standard
4. The space is still early enough that many developers use only one AI coding tool

---

## Implications for Syllago's Hook Design

1. **The canonical event model should cover the union of all platforms** — PreToolUse, PostToolUse, SessionStart, SessionEnd, Stop, FileEvent, PromptSubmit, SubagentEvents, ErrorOccurred. Per-provider converters drop unsupported events with documentation.

2. **Input/output normalization is the core value** — Neethling's detectHookSystem() pattern, Sondera's four-event taxonomy, and ai-hooks-integration's event mapping table all validate this approach.

3. **Trust model differences need explicit handling** — where a hook installs (user-local vs repo-bound) varies by provider. Syllago should make this transparent, not hide it.

4. **Security/provenance is non-negotiable** — the CVE history shows hooks can be weaponized. Registry-distributed hooks need at minimum: author attribution, content hashing, and clear review prompts before installation.

5. **Lossy conversion should be documented, not hidden** — when a Claude Code hook with input modification is exported to Cursor (which doesn't support input modification), the user needs to know what they're losing.

6. **Start with the overlap** — PreToolUse command blocking and PostToolUse verification are the most portable patterns. Build the canonical format around these first, then expand to platform-specific capabilities.
