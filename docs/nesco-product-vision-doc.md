# Nesco — Product Vision (v2)

**February 2026**

Product: **Nesco** | CLI command: **nesco** | Language: Go
Status: Vision validated, building on existing CLI (v0.2.0)

---

## What Nesco Is

Nesco is a terminal application and MCP server that onboards AI coding agents to existing codebases and keeps their context current as the codebase evolves. It manages, converts, and distributes AI tool configurations — skills, agents, rules, hooks, commands, MCP configs — across every major coding agent from a single source of truth.

What makes nesco different from the dozen tools that sync AI config files: nesco doesn't just distribute rules you've already written. It scans your codebase, finds what's weird — the inconsistencies, non-standard patterns, and assumption-breakers that cause agents to produce wrong code — and then facilitates a structured conversation between the developer and their own LLM to capture the tacit knowledge no tool can detect mechanically.

### Architecture: Dumb Tool + Smart Skill

Nesco itself ships no LLM and requires no API keys. It's a zero-cost, offline-capable, single static binary written in Go. The intelligence lives in skills that ship alongside the CLI — one for each supported agent platform — that teach the user's own agent how to use nesco's capabilities. The user's existing LLM subscription powers the smart parts.

Three layers, independently useful, designed to work together:

**The CLI** — Handles all deterministic operations: content browsing and management, codebase scanning, agent config parity analysis, format parsing and emission, drift detection, import and conversion. Fast, predictable, testable. Everything it does could be verified by a human reading the source.

**The MCP Server** — `nesco mcp` starts a stdio server that any compatible agent can call. Exposes tools for on-demand context queries ("what should I know about this directory?"), scan triggering, drift checking, and format emission. Agents self-serve context at runtime instead of relying on static files.

**The Skills** — Markdown-based skill packages that ship alongside the CLI, one per supported agent platform. These teach the user's LLM how to drive the onboarding workflow, maintain context over time, use the MCP server, and contribute improvements back to nesco. Skills are versioned alongside the CLI but can be updated independently of binary releases.

---

## What Nesco Manages

Nesco handles 8 content types across all major AI coding tools:

| Content Type | Description |
|---|---|
| Rules | Provider-specific instruction files (CLAUDE.md, .cursor/rules/, copilot-instructions.md, GEMINI.md, AGENTS.md) |
| Skills | Multi-file reusable workflow packages |
| Agents | AI personality/subagent definitions |
| Commands | Slash commands (/review, /deploy, etc.) |
| Hooks | Lifecycle event scripts (session start, pre/post tool use, file edit, etc.) |
| MCP | Model Context Protocol server configurations |
| Prompts | Templates with variable substitution |
| Apps | Full application packages with provider compatibility filtering |

### Supported Providers

Claude Code, Cursor, Windsurf, Codex, Copilot, Gemini CLI — with the architecture designed to add new providers as they emerge. Each provider is supported at full depth: every content type that the provider supports, nesco manages.

---

## The Workflow

### Three Entry Points, One Canonical Model

All content flows through nesco's internal canonical representation before being emitted to any provider format.

**Scan path** — `nesco scan` analyzes the codebase and its existing AI tool configurations, producing two outputs: auto-maintained facts (tech stack, dependencies, build commands, directory structure) and detected surprises (inconsistencies, non-standard patterns, competing conventions, assumption-breakers). The scan has two dimensions:

- *Codebase analysis*: Patterns, inconsistencies, and surprises in the actual code
- *Agent config analysis*: Which AI tools are configured, how complete each is, parity gaps between them, what could be synced or created from existing configs

**Author path** — Developers write content in nesco's format and it emits to all configured providers. This is the workflow that tools like rulesync offer — nesco supports it too.

**Import path** — `nesco import --from cursor` pulls in existing tool-specific configurations, parses them into the canonical model, and can emit them to any other provider. If you have rich Cursor rules, nesco converts them to Claude Code format — hooks mapped to the right lifecycle events, rules restructured appropriately, with warnings about lossy mappings where platform-specific features can't be converted.

### Import Discovery

The import path needs to find content across 8 content types and 6 providers without the user pointing to every file. When `nesco import --from cursor` runs, it needs a discovery strategy:

- **Provider fingerprinting** — Each provider has known filesystem locations for each content type. Cursor keeps rules in `.cursor/rules/*.mdc`, hooks in specific config locations, MCP configs in their own files. Nesco maps each provider's filesystem layout for all 8 content types.
- **Content type detection** — Once files are found, nesco identifies what content type each file represents. A `.mdc` file with lifecycle event frontmatter is a hook, not a rule. A markdown file in a skills directory is a skill, not a rule.
- **Completeness reporting** — Import reports what it found, what it couldn't find, and what it couldn't classify. "Found 12 Cursor rules, 3 hooks, 1 MCP config. No skills or agents detected. 2 files couldn't be classified."
- **Selective import** — `nesco import --from cursor --type rules` narrows discovery to a single content type. `nesco import --from cursor --preview` shows what would be imported without writing anything.

The discovery maps are provider-specific and maintained as part of provider parity (Phase 1).

### Phase 1 — Initial Onboarding (run once per project)

The developer runs `nesco scan` or invokes the nesco skill in their agent. The deterministic layer produces auto-maintained facts and detected surprises. The skill then drives a guided interview — asking targeted questions informed by what the scan found:

- "I see two different test frameworks coexisting. Which is canonical, or is this intentional?"
- "Your Cursor rules reference a testing convention that isn't mentioned in CLAUDE.md."
- "There's a `legacy/` directory imported from production code — what's the policy on that?"
- "You have 12 Cursor rules but only 3 Claude Code rules. Want me to sync these up? Create Gemini configs from what you have?"

The developer's answers become curated context. Everything gets stored in `.nesco/` and emitted to whatever tool formats are configured.

### Phase 2 — Ongoing Maintenance (continuous)

Two categories of maintenance run in parallel:

**Auto-maintenance** (deterministic, silent): Dependency versions, build commands, directory structure, tech stack facts. Updated programmatically. The janitor.

**Surprise surfacing** (requires attention): When drift detection finds new inconsistencies or assumption-breakers — not just updated facts — it surfaces them. A CI integration can comment on PRs: "This PR introduces a second logging library. Is this intentional? If so, nesco needs to know about it." The skill can re-engage the interview for new surprises that need human context.

Drift runs via multiple trigger points:
- `nesco drift` on demand
- CI check that exits non-zero when new surprises are found
- Pre-commit or post-merge hook
- The MCP server itself can flag drift when an agent queries a changed directory

### Phase 3 — Runtime Serving (always on)

The MCP server delivers targeted context when an agent needs it. When an agent is about to work in `src/auth/`, it queries nesco for relevant context about that directory — getting minimal, targeted information rather than a static dump. The server draws from both auto-maintained facts and curated interview answers, returning only what's relevant to the current task.

Static files and runtime serving coexist. Teams that want CLAUDE.md and `.cursor/rules/` get them. Agents that prefer on-demand queries use the MCP server. Both draw from the same underlying state.

---

## Two Categories of Context

Nesco produces and maintains two categories of output, with strict boundaries between them:

**Deterministic context** (auto-maintained): Dependency versions, build commands, directory structure, tech stack facts. Updated programmatically whenever the codebase changes. Never requires human input after initial scan. Boring but necessary — saves people from manually updating "React 18" → "React 19" in their context files.

**Curated context** (human-authored via guided interview): Architecture rationale, domain terminology, gotchas, historical decisions, convention explanations. Captured through the skill-driven interview process. Never overwritten by automation. This is the high-value 40% that changes agent behavior — the stuff that can't be detected from code alone.

Boundary markers in emitted files distinguish the two, so regeneration never overwrites human-authored content.

### The Reconciler

The boundary between deterministic and curated context doesn't enforce itself. Something needs to sit between "emitter produces a string" and "files appear on disk" — deciding where to write, whether to preserve curated sections, when to update baselines, and how to handle conflicts when both auto-maintained facts and curated content exist in the same output file.

This is the reconciler — the orchestration layer that:

- **Reads existing output files** before writing, identifies `nesco:auto` and `nesco:human` boundary markers
- **Merges new deterministic output** with preserved curated sections, maintaining section ordering
- **Appends new auto-detected sections** that didn't exist in the previous version
- **Warns on conflicts** — if a curated section covers something the scan now detects differently, it surfaces the discrepancy rather than silently overwriting or silently ignoring
- **Coordinates baseline updates** — writes `.nesco/baseline.json` only after successful file emission, never leaving baseline and emitted files out of sync

The reconciler is the most complex piece between scan and output, and the place where bugs around lost curated content would live. It's invisible to the user but critical to the dual-maintenance model working correctly.

### Baseline Management

Baselines track what nesco last saw so drift detection can report what changed. Baseline creation is decoupled from file generation:

- **`nesco scan`** generates context files AND updates the baseline by default
- **`nesco baseline`** accepts the current codebase state as the baseline WITHOUT regenerating files — useful when drift is expected and acknowledged, or when adopting an existing project that already has context files
- **`nesco baseline --from-import`** creates a baseline after importing from another provider's format, enabling drift tracking for converted content

This decoupling matters for three scenarios: adopting existing context files you don't want regenerated, enabling drift tracking after format conversion, and joining a project where someone else already set up nesco.

---

## The Contribution Model

Nesco ships a contribution skill that any user's agent can run against the nesco repository itself. A "report a problem" or "improve nesco" flow is always available. When a user encounters an issue — a surprise detector missed something, a convention wasn't recognized, a format conversion lost something important, the interview asked the wrong question — their agent launches a guided question chain:

- What were you doing?
- What went wrong?
- Which part of nesco was involved? (detection, conversion, interview, drift)
- Can you show an example?

Each branch leads to the right artifact type. The skill produces a PR containing only **structured artifacts in known formats** — detector configurations, pattern definitions, test fixtures, interview question templates, conversion mappings. Not arbitrary code.

This gives security properties by construction: the PR can't contain executable injections because the skill only emits predefined artifact types. The maintainer reviews *what to detect*, not *how to detect it*.

The flywheel: usage surfaces gaps → agents generate structured contributions → nesco gets better at detecting that class of surprise → fewer misses → more trust → more users.

This embodies the Nesco "ideas not code" contribution philosophy — contributors describe what they encountered, the skill structures it, the maintainer reviews the specification.

### Contribution Security (Research Required)

Open question: how to prevent tampered local nesco installations from producing PRs that *look* legitimate but contain poisoned content. Directions to explore:

- Binary and skill file hashing at contribution time, included in PR metadata
- Version verification against latest release — flag PRs from modified installations
- Git working tree integrity checks for uncommitted modifications to nesco's own files
- Reproducibility verification — given the same inputs, would a clean installation produce the same PR?

---

## Differentiation

### What nesco does that nothing else does

**Surprise detection** — Finds what's *weird* about your codebase: inconsistencies, non-standard patterns, competing conventions, assumption-breakers. Every other tool starts from human-authored rules. Nesco finds the things humans forget to write down.

**Guided interview driven by scan results** — Asks smart questions informed by mechanical analysis. The scan tells the skill what to ask about, so the human only answers questions that matter for their specific codebase.

**Agent config parity analysis** — Examines existing AI tool configurations, finds gaps between them, offers to sync and create missing configs. Existing tools assume you start from scratch in their format. Nesco meets you where you are.

**Dual maintenance** — Deterministic facts auto-update silently. Curated context is protected. Drift surfaces new surprises, not just stale facts.

**Dumb tool + smart skill** — No LLM baked in, no API costs, no vendor lock-in. The user's own agent powers the intelligent parts.

**Structured contribution model** — The tool improves from usage through safe, auditable PRs that contain specifications, not code.

### What nesco does that others also do (aiming for best-in-class)

**Format emission** across all major AI tools — rules, skills, commands, subagents, hooks, MCP configs. Informed by competitive analysis of rulesync (dyoshikawa), ai-rulez (Goldziher), Ruler, and ai-rules-sync.

**Import and convert** between tool-specific formats, including lossy mapping warnings.

**Content management** — Browse, preview, install, and manage reusable AI tool content via interactive TUI.

---

## Competitive Landscape

The "write once, emit everywhere" space has 5+ active tools:

| Tool | Language | Key Strength |
|---|---|---|
| Rulesync (dyoshikawa) | Node.js | Most complete — rules, hooks, MCP, commands, subagents, skills across 21+ tools. Anthropic customer story. |
| ai-rulez (Goldziher) | Go/npm/pip | Most ambitious — 18 presets, remote includes, team profiles, context compression, MCP server. LLM-powered init. |
| Ruler | ? | Broadest tool support — 30+ agents including niche ones. Skills propagation. |
| ai-rules-sync | npm | Multi-repo focus — rules from company standards, team protocols, open-source collections. Git integration. |
| rulesync (jpcaparas) | PHP | Simplest — single rulesync.md, emit everywhere. TypeScript rewrite exists. |

**None of them** scan codebases for surprises, detect agent config parity gaps, provide drift detection, drive guided interviews, or serve context on demand via MCP. They all start from human-authored canonical rules and distribute them.

---

## Implementation Roadmap

Building on the existing CLI (v0.2.0), which already provides content browsing/management, provider detection, and install mechanics for Claude Code and Gemini CLI:

### Phase 1: Provider Parity
Expand all providers (Cursor, Windsurf, Codex, Copilot) to full depth — every content type they support, nesco manages. This includes building the import discovery maps for each provider's filesystem layout across all 8 content types. This is the foundation everything else builds on. Can't convert between formats or detect config gaps without deep provider knowledge.

### Phase 2: Scan + Surprise Detection
Add the `scan` subcommand with dual-dimension analysis (codebase patterns + agent config parity). Implement the surprise detection engine — finding inconsistencies, competing conventions, and assumption-breakers. Store baselines in `.nesco/`. Ship `nesco baseline` for decoupled baseline management.

### Phase 3: Onboarding Skill
Ship the Claude Code skill that drives the guided interview using scan results. This is where "dumb tool + smart skill" comes alive — the skill interprets surprises and asks the developer targeted questions. Include the contribution flow ("report a problem" entry point).

### Phase 4: Format Conversion
Add import parsing for all supported provider formats. Convert between formats through the canonical model. Smart lossy-mapping warnings. The scan + parity analysis already tell you *what* needs converting.

### Phase 5: MCP Server
`nesco mcp` exposes scan data, curated context, drift status, and format emission as tools any compatible agent can call. On-demand context serving. By this point, there's a rich data model and proven results to serve.

### Ongoing: Drift Detection
Added incrementally — `nesco drift` can ship as early as Phase 2 for deterministic facts, with surprise-aware drift arriving alongside Phase 3.

---

## Research Follow-ups

Queued for separate deep-dive sessions:

1. **Competitive gap analysis** — Deep study of rulesync, ai-rulez, Ruler, ai-rules-sync: what gaps exist, what users want done better, what issues they're experiencing. Inform best-in-class emission layer.

2. **Contribution security** — How to ensure PRs from the contribution skill aren't produced by tampered installations. Provenance, binary hashing, git integrity checks, reproducibility verification.

3. **Surprise detection taxonomy** — What categories of codebase surprises can be detected mechanically? Build the full catalog of detectors.

4. **Skill distribution** — How to version and distribute skills for multiple agent platforms. Update mechanics independent of binary releases.

---

## Decisions Log

| Decision | Choice | Rationale |
|---|---|---|
| Architecture | Dumb tool + smart skill | No LLM cost, no vendor lock-in, user's agent powers intelligence |
| LLM requirement | None in CLI/MCP. Skills use user's LLM. | Zero-cost, offline-capable, deterministic CLI |
| Context strategy | Surprises > facts | Agents can discover boring facts themselves; surprises cause failures |
| Maintenance model | Dual — auto for facts, protected for curated | Prevents overwriting human knowledge while keeping boring stuff current |
| Contribution model | Structured artifacts, not code | Security by construction, specification-level review |
| MVP foundation | Existing CLI v0.2.0 | Content management, provider detection, install mechanics already built |
| Build order | Provider parity → scan → skill → convert → MCP | Each phase unlocks the next |
| Competitive position | Superset — do what sync tools do, plus what nobody does | Best-in-class emission + surprise detection + guided interview |
| Static vs runtime | Both — emit files AND serve via MCP | Teams choose their preference, same underlying state |
| Nesco relationship | Independent tool, branding heritage | No ecosystem dependency |
| Baseline management | Decoupled from file generation | Supports adoption, conversion, and team onboarding scenarios |

---

## Implementation Reference

The original nesco design doc (v1) contained detailed implementation patterns — Go interfaces, project structure, detector specifications, emitter details, exit codes, CLI flags, and distribution strategy. Many of these patterns carry over to v2's expanded scope.

See **nesco-v1-implementation-reference.md** for a curated catalog of what carries over cleanly, what needs expanding, and what v2 supersedes. That document is a reference for the implementation brainstorm, not a spec.
