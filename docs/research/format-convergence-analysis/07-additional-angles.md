# Section 7: Additional Strategic Analysis

**Date:** 2026-03-30
**Series:** Format Convergence Analysis
**Data sources:** `docs/research/provider-agent-naming.md`, `docs/research/2026-03-30-provenance-versioning-research.md`, `docs/spec/skills/frontmatter_spec_proposal.md`, `docs/spec/hooks/hooks.md`

---

## 7.1 One Spec or Many?

### The Question

Does it make sense to create a single unified spec covering all six content types (rules, skills, hooks, MCP, commands, agents), or should spec work stay focused and ship one type at a time?

### Current State of the Landscape

Three content types already have living specs or strong community proposals:

- **Skills** — `agentskills.io` has a published spec; the Agent Ecosystem community frontmatter proposal (`frontmatter_spec_proposal.md`) is under active development. Adoption is growing fast: the agentskills.io site lists 33 adopters, with Cursor CLI, Crush, Junie, gptme, Mistral Vibe, Trae, Antigravity, v0, Manus, and OpenHands all converging on the `SKILL.md` + YAML frontmatter pattern.

- **MCP** — Anthropic's Model Context Protocol is an open spec with wide adoption (30 agents support it fully, 8 partially). It is not community-governed — Anthropic owns it — but it functions as a de facto standard for tool connectivity.

- **Hooks** — Syllago's own hooks spec (`docs/spec/hooks/hooks.md`, v0.1.0) is the most developed canonical format in this repo. It defines event names, matcher syntax, capability registry, degradation strategies, a full conversion pipeline, and conformance levels. No equivalent community spec exists elsewhere.

The other three types — rules, commands, and agents — have no formal cross-provider spec.

### Analysis

#### The Unified Spec Case

A single spec governing all six content types would provide a consistent governance model, a single place to define shared vocabulary (the agent/harness/model-provider terminology from Section 2 of the naming research), and a coherent story for distribution tools like syllago and rulesync.

The practical problem is that the six content types have almost nothing structurally in common. Rules are plain markdown with optional frontmatter. Skills are markdown with structured activation metadata. Hooks are JSON configuration with event bindings, matchers, and exit code contracts. MCP configs are JSON server definitions. Commands are markdown or TOML with a prompt body. Agents are YAML or markdown with persona and tool configuration. A single spec capable of describing all of these would be either a registry of sub-specs (one per type, with shared vocabulary) or an umbrella document that adds governance overhead without adding technical clarity.

There is also a sequencing problem. A unified spec requires consensus across all six content types before anything ships. Skills and hooks are ready for broader community adoption now. Waiting for rules and commands to reach the same maturity level would delay the parts of the spec that are actually needed.

#### The Per-Type Spec Case

Per-type specs can ship independently, can be governed by the communities that care about them, and can evolve at their own pace. Skills are the clearest candidate — the convergence is already happening, the `SKILL.md` pattern is spreading across 20+ agents, and a focused frontmatter spec has real immediate value. Hooks have the most pressing need and the most complete existing work.

The risk with per-type specs is coordination overhead: vocabulary drift, inconsistent versioning schemes, and no canonical place to resolve cross-cutting concerns (like provenance, which applies to all content types equally). The research in `2026-03-30-provenance-versioning-research.md` makes exactly this point — the `derived_from` / `content_hash` / `triggers` patterns that emerged from the provenance research apply to skills today but will apply to rules, hooks, and agents tomorrow.

#### The Focus-on-One Case

Shipping the skills spec first and well has the clearest ROI. Skills have the highest convergence momentum, an existing community around `agentskills.io`, and the most immediate demand. The Agent Ecosystem frontmatter proposal is already in active discussion. Finishing and shipping that spec while the momentum is high is a concrete win.

The downside is not that it misses cross-type synergies — it doesn't, because per-type specs can share vocabulary through a common glossary document. The real risk is that shipping skills first establishes patterns (provenance fields, versioning schemes, schema versioning identifiers) that other specs then need to align with retroactively. If the skills spec ships before those patterns are settled, fixing them later requires a major version bump and coordination across adopters.

### Recommendation

**Adopt a layered spec architecture:** a shared vocabulary and metadata layer, with per-type specs that build on it.

The shared layer defines:
- Terminology (agent, harness, model provider, form factor, distribution tool — as proposed in the naming research)
- Provenance fields (`source_repo`, `content_hash`, `derived_from`, `publisher`) that apply to all content types
- Versioning conventions (semver, `schema_version` field pattern)
- The `supported_agents` field with the agreed terminology

The per-type specs then reference the shared layer and add their type-specific fields. Skills spec: `SKILL.md` frontmatter, `triggers`, `expectations`. Hooks spec: events, matchers, capability registry (as already developed in `hooks.md`).

This allows skills and hooks to ship now while ensuring that rules, commands, and agents specs — when they arrive — use the same provenance and vocabulary patterns rather than inventing their own.

Concretely, the sequence should be:
1. Finalize and publish the shared vocabulary + provenance layer (small document, already largely drafted in the naming research)
2. Ship the skills frontmatter spec (v1.0), building on the shared layer
3. Publish the hooks interchange spec (promote `hooks.md` v0.1.0 to community review)
4. Let rules, commands, and agents specs follow as community demand warrants

The "one spec for everything" approach is too ambitious for this stage of the ecosystem. The "focus on one and ignore the rest" approach leaves provenance and vocabulary problems unfixed, creating technical debt for every subsequent spec. The layered approach ships quickly on the high-value work while establishing the shared foundations that make future specs coherent.

---

## 7.2 Orchestrators as a Content Type

### The New Category

The landscape research identifies nine orchestrators: OpenAI Symphony, Claude Squad, Composio Agent Orchestrator, Superset, amux, AgentPipe, Conductor, Vibe Kanban, and IttyBitty. These tools manage multiple agent instances rather than augmenting a single agent's behavior.

Their content type support is sparse:

| Tool | Rules | Skills | Hooks | MCP | Commands | Agents |
|------|-------|--------|-------|-----|----------|--------|
| Symphony | ✅ | ❌ | ❌ | ❌ | ❌ | ⚠️ |
| Claude Squad | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠️ |
| Composio | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| Superset | ⚠️ | ❌ | ❌ | ✅ | ❌ | ✅ |
| amux | ⚠️ | ❌ | ❌ | ⚠️ | ❌ | ✅ |
| AgentPipe | ⚠️ | ❌ | ❌ | ❌ | ❌ | ✅ |
| IttyBitty | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |

No orchestrator fully supports skills or hooks. Every orchestrator supports agents (their core primitive). Most have thin or no rules support.

### Do Orchestrators Need Their Own Content Types?

Orchestrators have configuration concerns that don't map onto the six existing types:

**Workflow definitions** describe how multiple agents collaborate: task decomposition, agent selection, handoff conditions, parallelism strategy, and result aggregation. Symphony's `WORKFLOW.md` (YAML frontmatter + markdown body) is the clearest example. This is genuinely different from an agent definition — it describes coordination logic, not a single agent's persona or tools.

**Dispatch rules** define routing logic: which agent handles which task, under what conditions, with what priority. Composio's `agent-orchestrator.yaml` with swappable plugin slots models this. These rules operate at a higher level than agent-level hooks.

**Agent profiles** in the orchestration context describe how to instantiate a managed agent: which underlying agent binary to use, which workspace, which rules/skills to load, and what its role in the workflow is. Claude Squad's `config.json` profiles and IttyBitty's manager/worker hierarchy express this.

None of these map cleanly onto any of the six existing content types. Rules describe what a single agent should do; workflow definitions describe multi-agent coordination. Skills are capabilities of one agent; dispatch rules are routing logic across agents. Agent definitions describe a persona; agent profiles in orchestrators are instantiation instructions.

### The Delegation Question

The more important question is whether orchestrators primarily define their own content or primarily delegate to the content types of the agents they manage.

The evidence suggests mostly delegation. Claude Squad is a pure session manager — all content lives in the managed agents. Superset's `.superset/config.json` wraps MCP configs from `.mcp.json`. amux profiles point to agent instances. These tools are wrappers, not content systems.

Symphony and Composio are the exceptions. Symphony's `WORKFLOW.md` is a first-class content type that travels with the codebase. Composio's `agent-orchestrator.yaml` with reaction hooks and plugin slots is not expressible as any managed agent's content type.

### Implications for the Spec

The spec does not need to define orchestrator content types at v1.0. Orchestrators are still finding their form — nine tools, nine different approaches, no convergence signal yet. Defining an orchestrator content type now would be premature.

What the spec should do:

**Define orchestrator as a form factor**, not a content type. The naming research defines form factors as how an agent is delivered. Orchestrators are a distinct form factor (a tool that manages other agents rather than directly writing code). This gives them a named slot in the taxonomy without requiring new content types.

**Ensure agent definitions support composition metadata.** Orchestrators frequently reference managed agents by name or profile. If agent definitions (wherever they land in the spec) include a stable `id` field and a `role` field indicating how the agent can be composed, orchestrators can reference them by identity. This is a small addition to the agents content type, not a new type.

**Reserve the workflow definition space.** The spec glossary should define "workflow definition" and note that it is out of scope for v1.0 but expected in a future version. This prevents the term from being used inconsistently across tools before the ecosystem has enough evidence to standardize it.

The practical guidance: syllago should support the six existing content types for orchestrators where applicable (Symphony's workflow rules, Composio's hooks), but not add a seventh orchestrator-specific content type until the convergence is clearer.

---

## 7.3 Cross-Agent Discovery Conventions

### The Pattern

Two agents in the landscape already implement systematic cross-agent discovery:

**Junie CLI** imports skills from `.cursor/skills/`, `.claude/skills/`, `.codex/skills/` — explicitly reading other agents' skill directories as if they were native. This is a pragmatic decision: the content is structurally identical, the paths are predictable, and reusing existing skills eliminates duplicate authoring.

**oh-my-pi** discovers content from 8+ agent formats natively. As a batteries-included fork of Pi, it treats the agent ecosystem's collective content as a unified pool rather than requiring content to be authored specifically for oh-my-pi.

Rulesync (25 agents supported) and Agent Rules Sync / AI Rules Sync (VS Code extensions) also implement this pattern from the distribution side: one source of truth, multiple target formats.

### What Cross-Agent Discovery Actually Needs

Three mechanisms have been proposed. They solve different problems and are not substitutes for each other.

**Standard discovery paths** (e.g., `.agent/skills/`, `~/.agent/skills/`) would let any agent find any skill without needing to know other agents' specific paths. The practical problem: retroactive path standardization is extremely difficult. Claude Code uses `~/.claude/`, Cursor uses `.cursor/`, Codex uses `.codex/`. These paths are embedded in documentation, tutorials, shell completions, and user muscle memory. The ecosystem is not going to converge on `.agent/` voluntarily — each vendor has a brand reason to use their own path. Junie's approach (explicitly listing known paths) scales better: it requires no coordination and works immediately with existing installations.

**A manifest file** listing available content would solve the discovery problem without requiring path standardization. A `.syllago/manifest.json` (or a more neutral `.ai-content/index.json`) at the project root could list all content items, their types, and their paths. Any agent that reads this manifest can discover all content regardless of where it lives. This is the same approach used by package managers: you don't need a standard install path if you have an index. The manifest approach is opt-in, backward compatible, and doesn't require vendors to change their paths.

**A content type registry** (central, URL-based) would serve a different purpose: searchable, community-curated content. This is what agentskills.io is building for skills. It solves the distribution and discovery-at-scale problem, not the local-project discovery problem.

### Recommendation

The spec should define **a project-level content manifest** as an optional mechanism for cross-agent discovery. The manifest lives at a predictable path (`.syllago/manifest.json` is already established in syllago's architecture), lists content items with their types, paths, and supported agents, and is written by distribution tools (syllago, rulesync) when they install content.

The spec does not need to standardize discovery paths. That ship has sailed. What it can standardize is the manifest format so that tools like Junie, oh-my-pi, and future cross-agent readers have one format to parse rather than each implementing their own crawler.

The manifest format should record:
- Content type and canonical file path
- Which agents the item has been confirmed to work with (`supported_agents` from the skills spec)
- Install timestamp and version (local operational state, not in the portable spec but useful for the manifest)
- Optional content hash for drift detection

This aligns with the research finding in `2026-03-30-provenance-versioning-research.md` that sync state, install history, and relationship-to-upstream are local concerns (not spec concerns), while the content's intrinsic identity (hash, version, source) belongs in the spec itself.

---

## 7.4 AGENTS.md as Universal Rules Standard

### The Adoption Evidence

AGENTS.md is being read by a remarkable breadth of agents: Antigravity, Goose, Junie, Crush, gptme, Jules, Open SWE, OpenHands, Lovable, Replit, Pi, oh-my-pi, Warp, DeepAgents CLI, Factory Droid, Stagewise, Frontman, Bolt.new, and Claude Code (via the `AGENTS.md` file support added alongside `CLAUDE.md`). Rulesync includes `agentsmd` as a first-class target among its 25 supported formats.

No other rules file name comes close to this breadth. `CLAUDE.md` is Claude Code-specific. `.cursorrules` is Cursor-specific. `.kilocode/rules/` is Kilo Code-specific. `AGENTS.md` is read by tools across all form factors: IDEs, CLIs, extensions, web agents, and autonomous agents.

### What AGENTS.md Actually Is

AGENTS.md started as a community convention, not a vendor-imposed format. The file is plain markdown with no required frontmatter. There is no formal spec defining what it must contain. Agents treat it as a high-priority context file — they read it and inject its contents into the system prompt or initial context window.

The breadth of adoption is exactly because there is no spec. Any tool can add AGENTS.md support without a standards process, a format negotiation, or a compatibility matrix. The cost of that flexibility is that AGENTS.md means different things to different agents: some use it as the only rules file they read; others use it as a fallback when their preferred format is absent; others read both AGENTS.md and their native format and merge them.

### The Relationship Between AGENTS.md and Native Rule Files

Three patterns exist in the wild:

**AGENTS.md as primary, native as override.** Goose and Crush read AGENTS.md as their main rules file and fall back to `GOOSE.md` / `CRUSH.md` only for agent-specific settings. This is the "write once, run everywhere" model.

**AGENTS.md as fallback, native as primary.** Claude Code reads `CLAUDE.md` as primary; AGENTS.md is a secondary option for projects that don't have a Claude-specific file. Junie CLI reads `.junie/AGENTS.md` (its own namespaced version) as primary.

**AGENTS.md as supplement.** Factory Droid reads AGENTS.md alongside `~/.factory/` content. Warp reads both `WARP.md` and AGENTS.md. The agent merges both into context.

### Should Syllago Treat AGENTS.md as Canonical?

AGENTS.md is the de facto community rules standard in the sense that it is the broadest-reaching rules file in the ecosystem. But "canonical" in syllago's context means something specific: it means the internal hub format through which conversions flow. Treating AGENTS.md as canonical would mean converting all other rules formats into AGENTS.md-compatible markdown, then converting outward.

This would work for simple rules files, but breaks down for rules with structured metadata. CLAUDE.md supports inheritance (`@import`). Cursor rules have activation mode selectors (`alwaysApply`, `autoApply`, glob-filtered). Amazon Q's `.amazonq/rules/` uses per-file matchers. Kiro's spec-style rules have structured sections. None of this survives a conversion through plain AGENTS.md markdown.

The right role for AGENTS.md in syllago is as a **high-fidelity target format** for rules export, not as the canonical internal format. When a user installs rules into an agent that reads AGENTS.md, syllago writes a well-structured AGENTS.md. When syllago imports rules from AGENTS.md, it treats the content as a flat markdown rules file and converts into the canonical format (which retains the structured metadata that AGENTS.md cannot express).

The spec's rules section should define AGENTS.md as the baseline interoperability target: every agent that supports rules MUST be able to read and write AGENTS.md-compatible content. Agents MAY also support their own richer formats. This gives AGENTS.md its correct status — universal minimum, not maximum.

---

## 7.5 Hooks Spec Opportunity

### The Case for Prioritizing Hooks

Hooks have only 26% full adoption across the landscape (11 agents fully, 3 partially), but they have the highest fragmentation of any content type. The providers that do implement hooks have invented entirely different systems:

| Provider | Event Model | Config Format | Blocking Mechanism | Notable Unique Feature |
|----------|------------|---------------|-------------------|----------------------|
| Claude Code | ~25 named events | JSON in `settings.json` | Exit code 2 / `decision: deny` | `input_rewrite`, LLM-evaluated hooks, HTTP handlers |
| Cursor CLI | Split by category (shell/MCP/file) | JSON in `.cursor/mcp.json` | Exit code 2 | `beforeReadFile` access control |
| Amazon Q | Lifecycle phases (agentSpawn, preToolUse, etc.) | Per-agent JSON | Block/proceed decision | Agent-scoped hooks |
| Augment | `.augment/hooks/` file-based | YAML | Unknown | Workspace-scoped |
| gptme | Plugin hooks, TypeScript | `.gptme/` | Programmatic | 30+ events, LSP integration |
| OpenHands | Plugin hooks, Python | `.openhands/` | Programmatic | In-process, full access |
| Pi | Extension lifecycle | JS/TS | Programmatic | Browser-side hooks |
| Open SWE | Middleware hooks, 4 built-in | Python config | Middleware injection | LangGraph integration |
| Factory Droid | Global toggle, reaction hooks | `~/.factory/hooks/` | Global on/off | Enterprise compliance toggle |
| Composio | Reaction hooks, CI/review events | `agent-orchestrator.yaml` | Event-driven | CI/review integration |
| Qodo | Webhook hooks, external | `mcp.json` + YAML | HTTP webhook | External webhook target |

No two providers use the same event names, the same output schema, or the same blocking semantics. A hook written for Claude Code cannot run on Amazon Q without manual adaptation. A hook written for gptme requires Python plugin infrastructure that Cursor doesn't have.

This is precisely the scenario where a canonical interchange format adds the most value. The lower the current adoption, the more painful the fragmentation — early adopters are writing hooks for one tool, discovering they can't share them, and either abandoning hooks or maintaining multiple versions.

### What Syllago Already Has

Syllago's `docs/spec/hooks/hooks.md` (v0.1.0) is the most complete hook spec in the ecosystem. It defines:

- A canonical JSON format with a versioned `spec` field (`"hooks/0.1"`)
- A typed event registry with canonical `snake_case` names (`before_tool_execute`, `session_start`) and full provider-native name mapping tables across 8 providers
- A four-type matcher system: bare string (vocabulary lookup), regex pattern, MCP object, and array OR
- A four-stage conversion pipeline: decode → validate → encode → verify
- A capability registry with inference rules and safe default degradation strategies for 8 capabilities
- A tool vocabulary abstracting over provider-specific tool names (`shell`, `file_read`, `file_edit`, etc.)
- Exit code semantics (0 = allow, 1 = warn, 2 = block) and interaction rules with JSON `decision` fields
- Three conformance levels (Core, Extended, Full)
- Provider-exclusive event round-tripping for lossless conversions
- Per-capability degradation strategies with safety-critical defaults (`input_rewrite` defaults to `block`)

This spec is further along than anything the Agent Ecosystem community has published. The skills spec proposal in `frontmatter_spec_proposal.md` is a field listing. The hooks spec is a full protocol definition with conformance requirements.

### What the Hooks Spec Should Add

The existing spec covers the conversion and format layer thoroughly. Two areas would strengthen it for community adoption:

**Provenance fields aligned with the shared layer.** Hooks distributed as reusable content (not project-specific config) need the same provenance fields proposed for skills: `version`, `source_repo`, `content_hash`, `derived_from`. A hook that implements a safety check should be traceable to its author and verifiable against drift. The current spec does not include provenance. Adding a top-level `provenance` block (parallel to what the skills spec defines) would complete the portable content story.

**Skill-like trigger metadata for hook distribution.** When hooks are shared via a registry, authors need a way to describe what the hook does and when it should be used. A `description` field and a `tags` array on the manifest level would enable searchable, browsable hook registries. This is distinct from the per-hook `name` field that already exists — this is metadata about the manifest as a whole.

### How the Hooks Spec Connects to the Broader Ecosystem

The hooks spec is already positioned as syllago's contribution to the community spec effort, not a syllago-internal document. Promoting it to community review alongside the skills frontmatter spec would establish syllago as a reference implementation for both. This is consistent with the agent ecosystem spec relationship document: syllago is the reference implementation, not the spec owner.

The recommended path:
1. Add provenance block to the hooks spec (small addition, consistent with skills spec)
2. Publish `hooks.md` v0.1.0 to the Agent Ecosystem community for review alongside the skills spec
3. Implement both specs in syllago as the reference implementation
4. Let the community governance process handle future versions

The hooks spec's existing quality — full event registry, capability model, degradation strategies — is exactly the kind of careful design work that earns trust in a community standards process.

---

## Summary of Recommendations

| Question | Recommendation |
|----------|---------------|
| One spec or many? | Layered: shared vocabulary + provenance layer, per-type specs that reference it. Ship skills and hooks now; let rules, commands, agents follow. |
| Orchestrator content types | Do not add new types at v1.0. Define orchestrator as a form factor. Reserve the workflow definition space in the glossary. |
| Cross-agent discovery | Define a project-level content manifest (`.syllago/manifest.json`) as the discovery mechanism. Do not attempt to standardize discovery paths. |
| AGENTS.md status | Universal baseline export target, not canonical internal format. All agents MUST support it; richer formats are additive. |
| Hooks spec | Syllago's hooks spec is ready for community promotion. Add provenance block, publish alongside skills spec, position syllago as reference implementation. |
