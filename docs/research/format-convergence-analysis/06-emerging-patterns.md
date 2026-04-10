# Section 6: Emerging Patterns and Surprising Findings

**Date:** 2026-03-30  
**Sources:** provider-agent-naming.md (Content Type Support Matrix), 2026-03-30-provenance-versioning-research.md, hook-research.md, competitive-landscape-2026-03-27.md, agents-research.md, skills-research.md, rules-research.md

---

## 1. Novel Content Types Outside the Standard Six

The six content types syllago currently models (rules, skills, hooks, MCP, commands, agents) do not cover everything in the ecosystem. Several agents have introduced content types that fit none of the six cleanly or represent genuinely new ideas.

### Goose's Recipes

Goose supports "Recipes" — named, parameterized workflow definitions stored as YAML or markdown files in `.goose/recipes/`. A Recipe is a higher-level construct than a skill: it describes a multi-step task sequence the agent should execute, including which tools to use, what context to load, and how to handle intermediate results. Unlike a skill, which attaches context or behavior to an existing conversation, a Recipe is itself the task — it describes a complete autonomous workflow.

**Closest existing type:** Commands (in that they are invocable named artifacts) or Agents (in that they define a task objective + execution strategy). But Recipes are neither slash-command shortcuts nor persistent agent profiles — they are workflow templates closer to GitHub Actions `workflow.yml` files than to anything in syllago's current model.

**Syllago recommendation:** Do not force-map Recipes to commands. If Goose adoption grows, introduce a `workflow` content type with the semantic of "a named, parameterized multi-step task." For now, treat Recipes as unmapped/non-portable and document the gap.

---

### Devin's Playbooks

Devin's Playbooks are documented as "guides for completing specific tasks" — long-form structured documents that combine rules (what Devin should and shouldn't do), embedded skills (how to accomplish a specific class of operation), and process knowledge (what the right sequence of steps is for a recurring workflow). In practice, a Playbook is a rules document with task-specific scope and skill-like reusability.

**Closest existing type:** Rules (for the constraint content) and Skills (for the reusable process logic), but the combination is architecturally distinct. A Playbook is not simply a long rule or a named skill — it is a bounded operational manual for a specific recurring task.

**Syllago recommendation:** Map to rules as a conservative default. The Playbook format is Devin-proprietary and Devin's content type matrix shows partial rule support and partial hook support, with no clean analog in the six-type model. Flag on import with a provenance note indicating the original was a Playbook and semantic fidelity is partial.

---

### Warp's Agent Prompts and WARP.md

Warp uses `WARP.md` as its rules equivalent. More interestingly, Warp supports named "Agent Profiles" — configuration documents that define a specific persona or mode for the Warp agent, including a system prompt, allowed tools, and context scope. Separately, Warp's "Oz" platform hosts cloud agents that can be invoked by name.

`WARP.md` maps cleanly to syllago's rules type. Agent Profiles map partially to syllago's agents type. The Oz cloud agents have no equivalent in any other platform — they are hosted, versioned, remotely invocable agents with their own identity and billing model, closer to a platform product than a content type.

**Syllago recommendation:** `WARP.md` → rules. Local Agent Profiles → agents. Oz cloud agents → out of scope (platform feature, not portable content).

---

### Kiro's Specs

Kiro has a distinct content type that has no equivalent anywhere else in the ecosystem: **Specs** (`.kiro/specs/`). A Spec is a structured document describing a software feature or change to be implemented — it includes requirements, acceptance criteria, and implementation tasks that the agent uses to drive autonomous implementation. The Kiro task lifecycle hooks `Pre Task Execution` and `Post Task Execution` exist specifically to fire before and after Spec-driven task execution.

This is genuinely novel. A Spec is not rules (it's not behavioral guidance), not a skill (it's not reusable knowledge), not an agent (it's not a persona), and not a command (it's not invocable). It is a **work specification**: the agent reads the Spec, derives a task list, executes the tasks, and marks them complete. The closest external analogs are Linear issues or GitHub issues — structured work items — except that Specs are read and acted on autonomously without human dispatch.

**Syllago recommendation:** Treat Kiro Specs as an unmapped content type for now. They represent a new category — **work items** or **task specifications** — that no other agent has implemented at the same level of formalization. If other agents adopt spec-driven development workflows (Traycer, Qodo, and JetBrains AI are all moving in this direction with their plan/spec features), this could become a seventh content type. Track adoption before committing to a type.

---

### Symphony's WORKFLOW.md

OpenAI Symphony uses `WORKFLOW.md` — a YAML-frontmatter markdown document that defines an orchestration workflow: which agents to invoke, in what order, with what inputs, and with what success criteria. The frontmatter contains routing metadata; the markdown body contains context and objectives. Symphony dispatches to Codex agent instances and integrates with Linear for issue tracking.

**Closest existing type:** Rules (it's a markdown document with frontmatter) or Agents (it defines agent dispatch behavior). But `WORKFLOW.md` is specifically an orchestration artifact — it defines how multiple agents should collaborate on a task, not how a single agent should behave.

**Syllago recommendation:** Map to rules as the conservative default (it is a markdown+frontmatter document that provides context to an agent). Document that the orchestration semantics are Symphony-specific and non-portable. Like Kiro Specs, `WORKFLOW.md` hints at a future `workflow` or `orchestration` content type, but the ecosystem needs more convergence before standardizing.

---

### Other Notable Cases

**Kiro's Steering Files (`.kiro/steering/`):** Kiro calls its rules "Steering" rather than rules. These are YAML-frontmatter markdown files with three `inclusion` modes: `always`, `fileMatch` (glob-triggered), and `manual` (slash-command-accessible). The `fileMatch` mode is more structured than any other agent's rules activation — it's essentially a declarative trigger mechanism baked into the rules format itself. The `manual` mode makes steering files accessible as slash commands, blurring the rules/commands boundary. Kiro's steering maps cleanly to syllago's rules type but is the most feature-rich rules implementation in the ecosystem.

**gptme's Agent Templates:** gptme has agent templates (in `~/.config/gptme/config.toml`) that configure personas, tool availability, and default model parameters. These are more structured than most agents' agent definitions and include explicit tool enable/disable lists. Maps to syllago's agents type with good fidelity.

**Pi's SYSTEM.md:** Pi supports both `AGENTS.md` (rules) and `SYSTEM.md` (system prompt override). The distinction between a rules document that augments the system prompt and a `SYSTEM.md` that replaces it is functionally significant but structurally invisible from syllago's perspective. Both map to rules; the semantic difference needs flagging on conversion.

---

## 2. Cross-Agent Content Discovery

A notable and growing pattern is agents that actively look for and import content formats from *other* agents rather than defining their own format exclusively.

**Junie CLI** is the clearest example. It stores its own content in `.junie/AGENTS.md` and `.junie/skills/`, but it also imports skills from `.cursor/skills/`, `.claude/skills/`, and `.codex/skills/` — reading other agents' skill directories natively. This means a developer who has already set up Cursor skills gets them in Junie for free, without any conversion tool.

**oh-my-pi** takes this further. It "discovers content from 8+ agent formats natively," meaning it ships with built-in readers for CLAUDE.md, AGENTS.md, .cursor/rules/, GEMINI.md, and several others. Instead of requiring the user to install content in oh-my-pi's own format, oh-my-pi reads wherever the content already lives.

**Continue** discovers MCP server configurations from Claude Code's and Cursor's config files when users copy them into `.continue/mcpServers/`. This is passive discovery of dropped files rather than active auto-import, but the effect is the same: MCP configs authored for other agents work in Continue without reformatting.

**Stagewise, Frontman, and Bolt.new** all read `agents.md` and/or `claude.md` from the project root. They are browser-based agents that have no content format of their own — they parasitically read Claude Code's rules format as a de facto universal context file.

**What this means for the spec:**

These discovery behaviors are essentially organic convergence toward a universal format — not because anyone mandated it, but because Claude Code's CLAUDE.md and the AGENTS.md convention are where the most content already lives. The spec should acknowledge this by defining a standard discovery convention:

- A compliant agent SHOULD check `AGENTS.md` in the project root as a baseline rules file
- A compliant agent SHOULD check `.claude/skills/` or a vendor-specific equivalent when the Agent Skills standard is supported
- A compliant agent MAY check peer agent directories (`.cursor/`, `.claude/`, etc.) for additional content

This would formalize what Junie, oh-my-pi, Continue, Stagewise, Frontman, and Bolt.new are already doing informally, and give syllago a basis for declaring which agents "discover" existing syllago-managed content automatically.

---

## 3. The AGENTS.md Convergence

The following agents read `AGENTS.md` as a rules format, based on the Content Type Support Matrix:

**CLI agents:** Crush, Goose, Aider, Warp, Junie CLI, DeepAgents CLI, Pi, oh-my-pi  
**Extension agents:** GitHub Copilot, Kilo Code, JetBrains AI/Junie, Factory Droid  
**Web agents:** Replit, Stagewise, Lovable, v0  
**Autonomous agents:** Jules, Open SWE  
**Orchestrators:** OpenAI Symphony (WORKFLOW.md is effectively an AGENTS.md variant)

That is at minimum 20 agents that either primarily use or explicitly support `AGENTS.md` as a rules format. The file is rulesync's explicit first-class target (listed as `agentsmd` in its 25-agent target list). The "Agent Rules Sync" VS Code extension exists solely to sync `AGENTS.md` / `CLAUDE.md` / `.cursor` between formats.

**Is AGENTS.md the universal rules standard?**

It is becoming one, but with an important nuance: `AGENTS.md` is treated differently by different agents. Most agents treat it as a project-level context file (analogous to CLAUDE.md or GEMINI.md), placed at the repo root. It is not always the same as the agent's primary rules format — for example, Junie stores its own rules in `.junie/AGENTS.md` (scoped), while Copilot reads `AGENTS.md` at the repo root as a general context source, but its own instructions live in `.github/copilot-instructions.md`.

**Relationship to syllago's rules conversion:**

Syllago's rules converter already produces AGENTS.md as one target (via Copilot CLI). The practical implication of the AGENTS.md convergence is that syllago should elevate this to a first-class canonical output: if a user wants maximum compatibility across all agents, writing to `AGENTS.md` at the repo root reaches more agents than any other single rules file. A syllago "broadcast" mode for rules could write to `AGENTS.md` plus vendor-specific files (`CLAUDE.md`, `GEMINI.md`, `.cursor/rules/`, etc.) simultaneously.

---

## 4. The Orchestrator Layer

The research identified a new category of tools — Agent Orchestrators — that sit above individual agents: Symphony, Claude Squad, Composio Agent Orchestrator, Superset, amux, AgentPipe, IttyBitty, Conductor, Vibe Kanban.

**Do orchestrators need their own content types?**

OpenAI Symphony: Yes, in a limited way. `WORKFLOW.md` is an orchestration artifact that doesn't map to any existing syllago type. Its YAML frontmatter defines routing behavior that is meaningless in a non-orchestrator context.

Claude Squad, amux, IttyBitty: No. These are pure session managers — they manage which agent processes are running and provide a UI for interacting with multiple sessions. Content lives in the managed agents (typically Claude Code). Claude Squad has no content format of its own.

Composio Agent Orchestrator: Partial. It has an `agent-orchestrator.yaml` config and reaction hooks for CI/review triggers, but these are Composio-specific and not generalizable.

**Impact on the spec:**

Orchestrators introduce a new kind of agent relationship: the orchestrator delegates to workers, which have their own full content models. This creates a composition challenge — if a user is running Symphony over multiple Codex agent instances, do Symphony's `WORKFLOW.md` rules apply to each Codex instance? Do Codex's own rules take precedence? The spec should address content scope for delegated/managed agents:

- Content authored for a specific agent (Claude Code rules, Cursor skills) applies only to that agent
- Orchestration artifacts (Symphony WORKFLOW.md, Composio `agent-orchestrator.yaml`) apply only to the orchestrator layer
- There is no standard for "rules that apply to all agents in an orchestration" — this is an open design space

For syllago's practical purposes: orchestrator content types are low priority. They are platform-specific, not generalizable, and the orchestrators themselves mostly delegate content to their managed agents.

---

## 5. The "Assistant to Agent" Migration

The research shows a clear directional shift in both terminology and feature adoption:

**Terminology:** Tools launched after 2025 almost universally use "agent" rather than "assistant." Amp explicitly declared the "assistant" framing dead. Even tools in the "assistant" camp (Copilot, Windsurf) are adding agentic features. The 16+ tools that self-describe as "coding agent" are the directional majority.

**Feature adoption velocity:**

The agents actively expanding content type support:

- **GitHub Copilot:** Added MCP (stable), hooks (JetBrains preview, VS Code preview). From 3/6 to 5/6 active.
- **Kiro:** Adopted Agent Skills standard (Feb 2026) alongside its existing Steering + Specs + Hooks. Now approaching full coverage.
- **Junie CLI:** Added skills import from peer agents. Hooks listed as actively requested.
- **Warp:** Hooks explicitly listed as "requested" in the research — community pressure is there. WARP.md is already agent-profile capable.
- **Qwen Code:** Full 6/6 with experimental hooks, actively expanding.
- **v0 (Vercel):** Adopted Agent Skills via Skills.sh, Vercel MCP, ContextKit commands. Most complete web agent — moved from near-zero to 6/6 support within a year.
- **Trae:** Added Agent Skills, custom agents, MCP support — moved from a minimal IDE to competitive feature coverage.
- **Antigravity:** Full content type support using Google's AgentKit 2.0, which bundles 40+ skills natively.

The five agents with full 6/6 coverage (Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands) represent the leading edge. Based on trajectory, the next group to reach full coverage is likely GitHub Copilot, Kiro, and Warp — all of which are one or two content types away.

**The adoption gradient:**

Rules and MCP are the established baseline (78% and 70% adoption). Agents at 63% and commands at 44% are mid-adoption. Skills at 37% is growing fast via the Agent Skills standard. Hooks at 26% is the laggard. The pattern suggests a natural adoption order: rules → MCP → agents → commands → skills → hooks. New agents entering the space tend to launch with rules and MCP support, then add agents/commands, then skills, then hooks last (if at all).

---

## 6. Hooks: The Biggest Opportunity

At 26% adoption, hooks are the content type least supported — but the fragmentation among those that do support them is extreme. This is where a spec would add the most value.

### Full Hook Implementation Details

#### Claude Code — 28 events, 4 handler types

**Format:** JSON in `settings.json` (project, user, or enterprise level). Configuration is an object with event names as keys, each mapping to an array of `{matcher, hooks[]}` objects.

**Event trigger model:** Events fire at specific lifecycle moments. `PreToolUse` and `PostToolUse` accept a `matcher` field that filters by tool name (supports regex). Most other events have no matcher — they fire unconditionally.

**Blocking model:** Exit code 2 blocks. For `PreToolUse`, this prevents the tool from running. For `Stop`, it forces the agent to continue. For `PermissionRequest`, it denies the permission. Any other non-zero exit is a non-blocking warning. Exit 0 with structured JSON stdout is the signal/modify path.

**Capabilities unique to Claude Code:**
- `prompt` handler type — runs an LLM evaluation (Haiku default) to make the approve/block decision. No script required.
- `agent` handler type — runs a multi-turn read-only sub-agent that can read files and grep code before deciding.
- `http` handler type — POSTs to a webhook endpoint with configurable headers and env var injection.
- `updatedInput` — hooks can rewrite tool arguments transparently before execution.
- Multi-agent events: `TeammateIdle`, `SubagentStart`, `SubagentStop` (unique to CC's multi-agent model).
- Infrastructure events: `WorktreeCreate`, `WorktreeRemove`, `Elicitation`, `ElicitationResult`.

**Config path:** `.claude/settings.json`, `~/.claude/settings.json`, `/etc/claude/` (enterprise managed).

---

#### Gemini CLI — 11 events, command handler only

**Format:** JSON in `~/.gemini/settings.json` or project-level equivalent.

**Event trigger model:** Linear sequence: session start → (for each turn) before model → before tool selection → before tool execution → [tool runs] → after tool execution → after model → session end. The model-level events (`BeforeModel`, `AfterModel`) fire around the actual LLM API call — no other agent in the ecosystem has this.

**Capabilities unique to Gemini CLI:**
- `BeforeModel` / `AfterModel` — allows prompt modification and response interception at the LLM call layer, not the tool layer. Enables response mocking, redaction, logging of raw completions.
- `BeforeToolSelection` — filters the tool list before the LLM decides what to call. Enables dynamic capability restriction.
- **Security constraint unique to Gemini:** Extensions (bundled hooks + config) cannot set `allow` decisions or enable permissive ("yolo") mode. Hooks can block, but they cannot whitelist.

**Blocking model:** Exit code 2 blocks. JSON response includes `decision` and `reason` fields.

---

#### OpenCode — 25+ events, TypeScript/JavaScript plugin modules

**Format:** TypeScript or JavaScript plugin files in `.opencode/plugins/` or `~/.config/opencode/plugins/`. Loaded via Bun. Distributed as npm packages.

**Event trigger model:** Plugin exports a default object with a `hooks` record mapping event names to async handler functions. Handlers receive a context object with full OpenCode SDK access.

**Capabilities unique to OpenCode:**
- Full TypeScript SDK — plugins can register new tools, override built-in tool behavior, inject shell environment variables, and manipulate the TUI.
- npm distribution — hooks are distributable as standard npm packages, not just local scripts.
- Exception-based blocking — `throw new Error(...)` blocks, rather than exit codes or JSON responses.

**This is a fundamentally different model.** Every other hook system uses shell scripts or commands. OpenCode uses a plugin module system. Converting from OpenCode hooks to any other system requires extracting the hook logic from the TypeScript module and reimplementing it as a shell script — a non-trivial transformation.

---

#### Windsurf — 12 events, command handler only (JSON response blocking)

**Format:** JSON in `hooks.json` at user or system level. Distributable via cloud dashboard or MDM to `/etc/windsurf/hooks.json`.

**Event trigger model:** Events split into per-operation categories. Rather than one `before_tool_execute` event with a matcher, Windsurf has separate events for each operation category: `pre_read_code`, `pre_write_code`, `pre_run_command`, `pre_mcp_tool_use`, `pre_user_prompt` plus post variants.

**Capabilities unique to Windsurf:**
- `post_cascade_response_with_transcript` — fires after agent turn with the full conversation transcript. Uniquely suited for audit logging and compliance use cases.
- `trajectory_id` and `execution_id` — correlation identifiers included in all hook payloads for tracing.
- Enterprise deployment via MDM/cloud dashboard — hooks can be deployed to developer machines centrally without developer action.
- No input modification — post events are observation-only. Pre events can block but not rewrite.

---

#### Pi Agent — 20+ events, TypeScript extensions via jiti

**Format:** TypeScript extensions loaded via **jiti** (a TypeScript module loader). Extensions use an `ExtensionAPI` object passed by the runtime.

**Event trigger model:** Middleware chain model. Extensions export handlers that execute in load order, each receiving and returning the event object. Any extension can modify the event or block it.

**Capabilities unique to Pi:**
- Per-extension trust lifecycle: `pending → acknowledged → trusted → killed`. Each extension must be explicitly trusted.
- `context` event — fires before each LLM call, allowing non-destructive message modification.
- `spawnHook` — can rewrite commands, change working directories, inject environment variables.

**Note:** An unofficial Rust port (`pi_agent_rust` by Dicklesworthstone) exists that uses embedded QuickJS with a typed hostcall ABI and more elaborate security features (shadow dual execution, tamper-evident risk ledger). The official Pi project (badlogic/pi-mono) uses the simpler jiti-based system described above.

---

#### Cursor — 6 events, 3 blocking + 3 non-blocking

**Format:** JSON in `hooks.json` (project-level `.cursor/hooks.json` or global).

**Event trigger model:** Six named events, each mapped to a specific operation type. No matcher system — each event is already scoped to a specific operation.

**Capabilities unique to Cursor:**
- `beforeReadFile` — intercepts file reads before content reaches the LLM. This is the only hook in the ecosystem that controls what the agent can *see*, not just what it can *do*. Enables data loss prevention, source control for sensitive files. Endor Labs uses this for malware detection in `npm install` output before it reaches the model.

**Input/output format:** Different from all other systems. Input uses `hook_event_name` with flat command fields (not nested tool_input). Output uses `{"action": "block", "reason": "..."}` rather than exit codes.

---

#### GitHub Copilot CLI — 3 events, command handler only

**Format:** JSON in `.github/hooks/` (project) or `~/.github/hooks/` (global).

**Events:** `preToolUse`, `postToolUse`, `preUserMessage`. Note that `preToolUse` naming matches Claude Code, not Cursor's `beforeShellExecution` — a data point suggesting VS Code/GitHub is aligning on Claude Code naming conventions.

**Capabilities:** `preToolUse` supports both blocking (exit 2) and input modification. Minimal system overall — the smallest hook surface in the ecosystem.

---

#### VS Code Copilot — 3 events, preview

Same three events as Copilot CLI, in `hooks.json` at `.vscode/hooks.json`. In preview as of March 2026. No input modification on post events.

---

#### Cline — 4 events, filesystem-based discovery

**Format:** Executable scripts in directories: `.cline/hooks/pre-tool-use/`, `.cline/hooks/post-tool-use/`, `.cline/hooks/pre-task/`, `.cline/hooks/post-task/`. Any executable in these directories is discovered and run automatically — no config file required.

**Event trigger model:** Directory-based convention. Script placement equals registration. Scripts receive JSON on stdin with `tool`, `args`, and `context`.

**This is the most Unix-native hook model.** No config file, no manifest, just scripts in predictable directories. This makes Cline hooks the easiest to add to version control and the easiest to understand for developers unfamiliar with hook systems — but also the least expressive (no event filtering, no matcher, no blocking semantics beyond exit codes).

---

#### Kiro — 10+ events (5 IDE + 5 CLI), dual format

**Format:** `.kiro.hook` files (YAML frontmatter + markdown) for IDE, or JSON config for CLI. Each hook file is a separate document declaring a single hook.

**Event sets differ between IDE and CLI:**
- IDE events: `Pre Tool Use`, `Post Tool Use`, `File Save`, `File Create`, `File Delete` plus spec-related events (`Pre Task Execution`, `Post Task Execution`)
- CLI events: `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `AgentStop` plus manual trigger

The spec-related hook events (`Pre/Post Task Execution`) are Kiro-specific and tied to its Spec-driven development workflow. No other agent in the ecosystem has equivalent events.

**Handler types:** Standard `command` plus `askAgent` — a type that routes the hook decision to a specified agent with credits billed from the user's Kiro account. Similar to Claude Code's `prompt` handler but cloud-hosted.

---

### The Normalization Picture

The core fragmentation across these 11 implementations:

| Dimension | Variation |
|-----------|-----------|
| Event names | 8 different names for "before tool executes" |
| Config format | JSON (7 tools), TypeScript module (2 tools), executable directories (1 tool), YAML frontmatter files (1 tool) |
| Blocking mechanism | Exit code 2 (most), `throw` (OpenCode), `{"action":"block"}` (Cursor), exit code 2 + JSON field (CC) |
| Input modification | `updatedInput` (CC), `rewrite` (Gemini), `throw + context inject` (Cline), not supported (most) |
| Handler types | command-only (7 tools), command + LLM eval + agent + HTTP (CC), TS modules (OpenCode), TS extensions + QuickJS (Pi) |
| Scope | Project-level config (most), per-hook files (Kiro), npm packages (OpenCode) |

No two agents handle hooks the same way. This is the content type with the highest fragmentation and the most technical depth — and the one where syllago's canonical hook format and conversion pipeline represent the most distinctive value in the ecosystem. The sondera-ai hook normalization project (2 stars, essentially undiscovered) is the only prior art attempting exactly this, and it relies on AWS Cedar policies rather than a portable YAML spec.

---

## 7. Other Surprising Findings

### The "Full 6/6" Club Is Very Small

Only five agents out of 54+ researched support all six content types: Cursor CLI, gptme, Qwen Code, oh-my-pi, and OpenHands. Three of these are relatively obscure (gptme, oh-my-pi, Qwen Code). Two have very small user bases by any market share measure. Full content type coverage is a rare property, and none of the full-coverage agents are dominant-market tools.

The practical implication: syllago's value is greatest for teams using mainstream agents (Claude Code, Cursor, Copilot, Windsurf) that *don't* have full coverage. The users who would most benefit are not using the most fully-featured agents.

---

### Web Agents Are Parasitic Content Consumers

Stagewise, Frontman, and Bolt.new do not define their own rules format. They read `agents.md` and/or `claude.md` from wherever they already exist in the project. They are content-format parasites on Claude Code's ecosystem in the most practical sense: they add themselves to the audience of Claude Code's content without requiring any new authoring.

This is a convergence signal but also a fragility signal. If Anthropic changes CLAUDE.md semantics or format, these tools break silently.

---

### Aider Can Be Exposed as an MCP Server

Aider's hook support is listed as partial/none — it uses `--auto-lint` and `--auto-test` flags instead of a formal hook system. But Aider can be exposed *as* an MCP server. This reverses the typical relationship: instead of Aider consuming MCP tools, other agents consume Aider as a tool. This inverts the content type model — Aider becomes MCP configuration in other agents' settings files, not a target for syllago's conversion pipeline.

---

### Completion-Only Tools Have Zero Content Types But Large User Bases

Supermaven (completion engine, acquired by Cursor), CodeGeeX (Zhipu AI's extension), and Tabnine's core product are completion-only with no content type support. These tools collectively have enormous user bases, particularly in enterprise and non-Western markets. They will never be syllago targets because they have no extensibility surface — but they represent a ceiling on syllago's addressable market that is worth acknowledging.

---

### The Agent Skills Standard Is Outpacing the Spec

The Agent Skills standard (`SKILL.md` + YAML frontmatter) is being adopted faster than the formal spec is being written. Kiro adopted it in February 2026. v0, Manus, Antigravity, Mistral Vibe, and Trae all landed support within the last six months. Skills.sh hit 26,000+ installs within weeks of launch. The community convergence on the SKILL.md format is happening through network effects, not governance.

This is both encouraging (the spec is influencing reality) and concerning (the spec is lagging behind adoption and may solidify in ways that diverge from what the spec ends up saying). The provenance/versioning research finding is directly relevant here: without `content_hash` in the frontmatter, there is no way to detect when a skill in a repo has drifted from the version the user installed, and no way to verify integrity across the community distribution chain.

---

### Skill Activation Reliability Is a Systemic Problem

The provenance research cites a 650-trial study finding that passive skill descriptions achieve only ~77% activation while directive descriptions ("ALWAYS invoke this skill when...") hit 100%. This is not a single-agent quirk — it is a fundamental property of LLM-based routing across all agents that use description-based skill dispatch. The problem affects Claude Code, Cursor (intelligent mode), Copilot (natural language activation), and every agent that routes via description rather than deterministic triggers.

This finding argues for the `triggers` section recommended in the provenance research — structured `file_patterns`, `workspace_contains`, `commands`, and `keywords` fields that give agents deterministic activation paths. Cursor already has deterministic triggers (file globs, always mode, manual mode). The spec should bring this to the other 37% of agents that support skills but rely on description-only routing.

---

### The Conversion Gap Is Asymmetric, Not Just Lossy

Every conversion discussion in syllago focuses on lossiness — fields that don't map, capabilities that don't exist in the target format. But the hook research reveals a more structural problem: the conversion gap for some content types is *asymmetric*, not just lossy.

Claude Code's `prompt` and `agent` hook handler types have no analog in any other system. A Claude Code hook that uses `prompt` type (LLM evaluation) cannot be expressed in Cursor, Gemini CLI, Windsurf, or Copilot formats — there is literally no field to put it in. Converting *down* (CC → Cursor) is lossy in a way that produces a broken artifact, not just a less-capable one. Converting *up* (Cursor → CC) is lossless.

The same asymmetry appears in Gemini CLI's model-level hooks (`BeforeModel`, `AfterModel`) — unique to Gemini, no target exists for these in any other system. And in Pi's trust lifecycle model — converting Pi extensions to command-based hooks loses the entire security layer.

Syllago's converter must represent this asymmetry explicitly — not as a warning that fires on conversion, but as a property of the source content that is visible before conversion starts. A hook authored with `prompt` type is a CC-only artifact by definition. Users should know this before they try to install it on Cursor.

---

*End of Section 6.*
