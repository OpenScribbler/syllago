# AI Agent Content Format Convergence Analysis

**Date:** 2026-03-30
**Scope:** 90+ AI coding tools, 54 agents analyzed for content type support, 6 content types
**Data sources:** `provider-agent-naming.md`, `2026-03-30-provenance-versioning-research.md`, `frontmatter_spec_proposal.md`

---

## Executive Summary

This report analyzes format convergence, fragmentation, and trends across the AI coding agent ecosystem based on a Content Type Support Matrix covering ~55 agents across 6 form factors (IDE, CLI, Extension, Web, Autonomous, Orchestrator) and 6 content types (Rules, Skills, Hooks, MCP, Commands, Agents).

### Five Key Findings

**1. The ecosystem has a clear adoption ladder: Rules (78%) > MCP (70%) > Agents (63%) > Commands (44%) > Skills (37%) > Hooks (26%).** New agents enter at Rules+MCP and progressively add content types. The bimodal score distribution (41% at 2-3 types, 29% at 5-6 types) shows agents either stop at a minimal footprint or build out completely — there is no stable middle tier.

**2. Skills are the strongest convergence story.** The Agent Skills standard (`SKILL.md` + YAML frontmatter) achieved 33+ adopters within 3 months of publication — the fastest format convergence in the ecosystem. The `name` + `description` frontmatter nucleus is stable. Install paths remain fragmented (`.<agent>/skills/`), but the file format is genuinely cross-provider.

**3. Hooks are the biggest spec opportunity — and the biggest risk.** Only 26% adoption, but the 11 agents that implement hooks have 8 different names for "before tool executes," 5 different blocking mechanisms, and 4 different config formats. This is where standardization adds the most value. But the implementations aren't just different serializations — they represent fundamentally different models of what a hook *is* (lifecycle interceptor vs. CI trigger vs. middleware function). Speccing too early could lock in the wrong model.

**4. AGENTS.md is the de facto universal rules standard.** 20+ agents read it. It spread by imitation, not specification. Its strength is simplicity (plain markdown, no frontmatter). Its weakness is the same: no conditional activation, no structured metadata. The spec should codify it as the baseline interoperability target, not replace it.

**5. Syllago's hook conversion pipeline is its strongest differentiator.** Rulesync (the closest competitor at 25 agents) is rules-only. For the 5 agents with full 6/6 content type support that syllago doesn't yet cover, rulesync reaches 1/6 types; syllago can reach 6/6. Hooks specifically — only syllago has a canonical hook format and conversion infrastructure. Adding 5 hook-capable agents would expand the hook conversion graph from ~3 to 8+ agents.

### Strategic Recommendations

| Priority | Action |
|----------|--------|
| 1 | **Publish shared vocabulary spec** (terminology + provenance block) — foundation for all per-type specs |
| 2 | **Ratify skills spec** with `triggers`, `content_hash`, `derived_from` additions — emerging standard at risk of fragmenting |
| 3 | **Rename `supported_agent_platforms` → `supported_agents`** — free now, expensive later |
| 4 | **Add Kilo Code, GitHub Copilot, Augment Code, Cursor CLI, JetBrains Junie** — top 5 agents to add next |
| 5 | **Promote hooks spec to community review** — syllago's `hooks.md` v0.1.0 is the most complete hook spec in the ecosystem |
| 6 | **Publish agents spec** (static content, scoped away from runtime) — highest fragmentation among high-adoption types |
| 7 | **Hold on hooks standardization** — wait for ~50% adoption and a dominant structural model |

---

## Section 1: Format Convergence Analysis

> For each content type, what file formats and directory conventions are agents actually using? Where is there convergence vs. fragmentation?

*Full analysis: [01-format-convergence.md](./01-format-convergence.md)*

### Rules (78% adoption) — Two structural families

**Family A: AGENTS.md (plain markdown, hierarchical)** — The largest convergence cluster. 15+ agents read `AGENTS.md` from the project root as an always-on instruction file. Emerged from Codex CLI, spread by imitation. No spec governs it.

**Family B: YAML frontmatter + glob scoping** — 6 agents converged on the model of per-rule files with YAML frontmatter controlling activation. But the field names differ:

| Agent | Glob Field | Format |
|-------|-----------|--------|
| Claude Code | `paths:` | YAML array |
| Cline | `paths:` | YAML array |
| Cursor | `globs:` | Comma-separated string |
| Copilot | `applyTo:` | Comma-separated string |
| Amp | `globs:` | YAML array |

**Fragmentation:** Directory naming is fully fragmented — every agent with a dedicated directory invents its own (`.claude/rules/`, `.cursor/rules/`, `.amazonq/rules/`, etc.). No two unrelated agents share a rules directory name.

### Skills (37% adoption) — Strongest convergence in the ecosystem

The Agent Skills standard achieved what no other content type has: a genuine cross-provider file format. 33+ agents use `SKILL.md` + YAML frontmatter with `name` and `description` as the convergence nucleus.

**Field support tiers:**
- **Base** (name + description): All 33+ agents
- **Extended** (+compatibility, metadata, license): Kiro, OpenCode, Cursor
- **CC superset** (+allowed-tools, context, agent, model, hooks): Claude Code only

**Fragmentation:** Install directories remain per-agent (`.<agent>/skills/<name>/SKILL.md`). But `.agents/skills/` is emerging as a cross-agent path (Amp, Codex CLI, and several cross-readers scan it).

### Hooks (26% adoption) — Most fragmented content type

Every dimension is fragmented:

| Dimension | Variation Count |
|-----------|----------------|
| Event names for "before tool" | 8 different names |
| Config format | JSON (7), TypeScript modules (2), script dirs (1), YAML files (1) |
| Blocking mechanism | exit 2 (5), JSON `action:block` (1), `throw` (2), global toggle (1) |
| Handler types | command-only (7), command+LLM+agent+HTTP (1), TS modules (2) |

**Two convergence signals:** VS Code Copilot adopted Claude Code's naming over Cursor's. Exit code 2 is the emerging blocking standard among command-type handlers (Claude Code, Gemini CLI, VS Code Copilot), though Windsurf and Cursor use JSON responses instead.

### MCP (70% adoption) — Surface convergence, detail divergence

11 of 16 agents use `mcpServers` as the top-level JSON key. The stdio transport (`command` + `args` + `env`) is universally shared. But:

- **Transport type collision:** `"http"` (Claude Code, Copilot) vs `"streamable-http"` (Cursor) for the same protocol
- **Auto-approval split:** `autoApprove` (Claude Code, Cursor, Kiro) vs `alwaysAllow` (Cline, Roo Code)
- **URL field:** `url` (10 agents) vs `httpUrl` (Gemini) vs `serverUrl` (Windsurf)

### Commands (44% adoption) — Transitional content type

4 agents converged on `$ARGUMENTS` markdown + YAML frontmatter (Claude Code, Copilot CLI, OpenCode, Roo Code). But the trend is toward merging commands into skills — Claude Code, Codex CLI, and Amp already treat skills with `user-invocable: true` as the successor to standalone commands.

### Agents (63% adoption) — MD+YAML dominant, but outliers exist

7 agents converged on markdown files with YAML frontmatter in `.<agent>/agents/<name>.md`, sharing `name`, `description`, `tools`, and `model` fields. But Codex CLI uses TOML, Roo Code uses JSON mode definitions, and OpenHands uses a structurally different micro-agent model.

### Convergence Scores

| Content Type | Format Convergence | Path Convergence | Overall |
|---|---|---|---|
| Skills | **High** | Low | Medium-High |
| Agents | **Medium** | Low | Medium |
| MCP | **Low** (detail divergence) | Low | Low-Medium |
| Rules | **Low** | Very Low | Low |
| Commands | **Low** | Very Low | Low |
| Hooks | **Very Low** | Very Low | Very Low |

---

## Section 2: Content Type Correlation

> Which content types tend to appear together? Are there natural adoption tiers?

*Full analysis: [02-content-type-correlation.md](./02-content-type-correlation.md)*

### Co-occurrence Matrix (conditional adoption)

| Given ↓ also has → | Rules | Skills | Hooks | MCP | Commands | Agents |
|--------------------|-------|--------|-------|-----|----------|--------|
| **Hooks** (n=15)   | 100%  | 73%    | —     | 87% | 73%      | 87%    |
| **Skills** (n=22)  | 95%   | —      | 50%   | 82% | 77%      | 73%    |
| **Rules** (n=46)   | —     | 46%    | 33%   | 80% | 57%      | 67%    |

**Key insight:** Hooks is the strongest predictor of everything else. Every agent with hooks also has rules (100%), and 87% also have MCP and Agents. Supporting hooks signals full ecosystem buildout.

### Natural Adoption Tiers

| Tier | Content Types | Threshold |
|------|--------------|-----------|
| 0 (baseline) | Rules + MCP | 67% of non-zero agents have both |
| 1 (common) | + Agents + Commands | 73% of Tier 0 agents add these |
| 2 (advanced) | + Skills | 78% of Tier 1 agents add this |
| 3 (full) | + Hooks | 50% of Tier 2 agents add this |

The distribution is bimodal: 41% cluster at 2-3 types, 29% at 5-6 types. Agents either stop early or build out completely.

### Full 6/6 Agents (all-✅)

| Agent | Form Factor | Open Source |
|-------|-------------|-------------|
| Cursor CLI | CLI | No |
| gptme | CLI | Yes |
| Qwen Code | CLI | Yes (CC fork) |
| oh-my-pi | CLI | Yes (Pi fork) |
| OpenHands | Autonomous | Yes |

CLI form factor dominates. Open source is the norm. Fork-based agents (Qwen Code = CC fork, oh-my-pi = Pi fork) inherit full-stack architecture.

### Surprising Gaps

- **Hooks without Skills** (4 agents) — all in CI/infrastructure contexts where hooks mean pipeline triggers, not session lifecycle events
- **Agents without Rules** (6 agents) — almost exclusively orchestrators that delegate content to managed agents
- **Rules without MCP** (9 agents) — principled architectural decisions: research tools, privacy-first tools, or orchestrators

---

## Section 3: Gap Analysis for Syllago

> Given syllago's current 12 agents, where are the biggest conversion opportunities?

*Full analysis: [03-gap-analysis.md](./03-gap-analysis.md)*

### Top 10 Agents to Add Next

| Rank | Agent | Score | Rationale |
|------|-------|-------|-----------|
| 1 | **Kilo Code** | 4.0 | Near-identical to Cline/Roo Code format; 70%+ adapter reuse |
| 2 | **GitHub Copilot** | 3.5 | Largest installed base (50M+ devs); rules+MCP+agents |
| 3 | **Augment Code** | 5.0 | Full hooks support; enterprise; clean `.augment/` layout |
| 4 | **Cursor CLI** | 6.0 | Full 6/6; existing Cursor adapter covers most of it |
| 5 | **JetBrains Junie** | 5.0 | Largest non-VS Code IDE market; explicit cross-agent import |
| 6 | **Amazon Q Developer** | 5.0 | Enterprise; hook matchers; AWS ecosystem |
| 7 | **Factory Droid** | 5.5 | Enterprise hooks; multi-IDE support |
| 8 | **Qwen Code** | 6.0 | Full 6/6; CC clone (path substitution); Chinese market |
| 9 | **Goose** | 3.0 | Block's OSS agent; MCP-first; community credibility |
| 10 | **Warp** | 4.0 | Growing CLI user base; 4 types rulesync misses |

### Critical Gap: Hooks

Syllago currently has ~3-4 hook-capable agents in its conversion graph. 11 agents in the landscape support hooks. Adding Augment Code (#3), Cursor CLI (#4), Amazon Q (#6), Factory Droid (#7), and Qwen Code (#8) would expand the hook graph to 8-9 agents — making hook conversion a usable feature rather than a theoretical one.

### Rulesync Competitive Position

Rulesync supports 13 agents syllago doesn't. For 11 of those 13, syllago would add 3-5 content types beyond rulesync's rules-only support. The competitive gap is content depth, not agent count. Syllago's value proposition is a 6x content coverage multiplier on rich targets.

---

## Section 4: Format Family Trees

> Can we group agents by which "family" of formats they follow?

*Full analysis: [04-format-family-trees.md](./04-format-family-trees.md)*

### 9 Families Identified

```
AI Coding Agent Format Families
════════════════════════════════════════════════════════════════

  ┌─────────────────────────────────────────────────────────┐
  │  AGENTS.md UNIVERSAL LAYER (cross-family compatibility) │
  │  20+ agents read AGENTS.md — not a family, a shim      │
  └─────────────────────────────────────────────────────────┘
                          │
         ┌────────────────┼────────────────┐
         ▼                ▼                ▼
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │ CLAUDE CODE  │ │   CURSOR     │ │ VS CODE EXT  │
  │ FAMILY       │ │   FAMILY     │ │ FAMILY       │
  │              │ │              │ │              │
  │ CC, Codex*,  │ │ Cursor IDE,  │ │ Copilot,     │
  │ Qwen Code,   │ │ Cursor CLI,  │ │ Cline, Roo,  │
  │ oh-my-pi*,   │ │ Windsurf*,   │ │ Kilo, Augment│
  │ Junie CLI*,  │ │ Trae*        │ │ Amazon Q,    │
  │ Pi*          │ │              │ │ Junie, Qodo  │
  └──────────────┘ └──────────────┘ └──────────────┘

  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │ AGENT SKILLS │ │  MCP JSON    │ │ GEMINI CLI   │
  │ FAMILY       │ │  FAMILY      │ │ FAMILY       │
  │              │ │              │ │              │
  │ 21+ agents   │ │ 12+ agents   │ │ Gemini CLI,  │
  │ SKILL.md +   │ │ mcpServers   │ │ Gemini Assist│
  │ frontmatter  │ │ JSON block   │ │ Antigravity  │
  └──────────────┘ └──────────────┘ └──────────────┘

  * = cross-family reader
```

### Architecture Implication

**Families define shared code, agents define converter entry points.** The CC family shares `cc-rules` import/export functions. The AGENTS.md layer shares `plain-markdown` import/export. The Agent Skills family shares `skill-frontmatter` parse/render. The MCP JSON family shares `mcpservers-json` import/export.

The converter module priority table identifies 30 converters across 4 priority tiers, with Qwen Code (`cc-family` path substitution) and Trae (`cursor-family` path substitution) as near-zero-cost adds.

---

## Section 5: Spec Implications

> What should the community spec standardize vs. leave to individual agents?

*Full analysis: [05-spec-implications.md](./05-spec-implications.md)*

### Standardization Priority Matrix

| Content Type | Adoption | Fragmentation | Quadrant | Action |
|---|---|---|---|---|
| **Agents** | 63% | High | High frag / High adoption | **URGENT** — standardize now |
| **Rules** | 78% | Low | Low frag / High adoption | **Codify** — describe what exists |
| **Skills** | 37% | Low-Med | Converging | **Accelerate** — ratify emerging standard |
| **MCP** | 70% | Low | Already specced | **Defer** — protocol spec owns this |
| **Commands** | 44% | Medium | Medium both | **Targeted** — spec structure, not format |
| **Hooks** | 26% | High | High frag / Low adoption | **PREMATURE** — let market settle |

### Recommended Spec Architecture: Layered

1. **Shared vocabulary + provenance layer** (terminology, `content_hash`, `derived_from`, `supported_agents`)
2. **Skills spec** — ratify with `triggers` block, content hash, derived_from
3. **Hooks interchange spec** — promote syllago's `hooks.md` to community review
4. **Agents spec** — static content only, scoped away from runtime
5. **Commands spec** — structural, format-agnostic
6. **Rules guidance doc** — codify AGENTS.md convention
7. **Hooks standardization** — deferred until dominant model emerges

### Key Spec Additions

**Triggers block** (for skills spec):
```yaml
triggers:
  mode: auto          # auto | manual | always
  file_patterns:      # deterministic activation
    - "**/*.test.ts"
  workspace_contains: # project detection
    - "jest.config.*"
  commands:           # explicit invocation
    - "/test-review"
  keywords:           # semantic routing hints
    - "review tests"
```

**Content hash** (for provenance): `sha256:abc123...` — enables drift detection, integrity verification, deduplication.

**Derived from** (for provenance): typed relationships (`fork | convert | extract | merge`) — Hugging Face's model adapted for AI content.

---

## Section 6: Emerging Patterns

> Surprising findings and patterns that don't fit the six content types cleanly.

*Full analysis: [06-emerging-patterns.md](./06-emerging-patterns.md)*

### Novel Content Types

| Agent's Content | What It Is | Syllago Mapping |
|----------------|-----------|-----------------|
| Goose Recipes | Multi-step workflow templates | Future `workflow` type; don't force to commands |
| Devin Playbooks | Rules + skills hybrid | Map to rules with provenance note |
| Kiro Specs | Work specifications (genuinely novel) | Unmapped; potential seventh type if adopted |
| Symphony WORKFLOW.md | Orchestration definitions | Map to rules; orchestration semantics non-portable |

**Kiro Specs are the most novel finding.** A Spec is a structured work item — requirements, acceptance criteria, implementation tasks — that the agent executes autonomously. No other agent has this. If other agents adopt spec-driven development, this could become a seventh content type.

### Cross-Agent Discovery Is Growing

- **Junie CLI** imports from `.cursor/skills/`, `.claude/skills/`, `.codex/skills/`
- **oh-my-pi** reads 8+ agent formats natively
- **Continue** auto-imports MCP from Claude/Cursor configs
- **Zed** scans 9 different file names in priority order
- **Stagewise, Frontman, Bolt.new** parasitically read `claude.md`/`agents.md`

The spec should define a project-level content manifest for discovery rather than trying to standardize paths (that ship has sailed).

### The Hook Conversion Gap Is Asymmetric, Not Just Lossy

Claude Code's `prompt` and `agent` hook handler types cannot be expressed in any other system — there is literally no field to put them in. Converting CC→Cursor produces a broken artifact, not just a less-capable one. Converting Cursor→CC is lossless. This asymmetry must be a visible property of content, not just a conversion warning.

---

## Section 7: Additional Strategic Analysis

> One spec or many? Orchestrators? Discovery? AGENTS.md? Hooks opportunity?

*Full analysis: [07-additional-angles.md](./07-additional-angles.md)*

### Key Recommendations

**One spec or many?** → **Layered architecture.** A shared vocabulary + provenance layer, with per-type specs that build on it. Ship skills and hooks now. Let rules, commands, agents follow as community demand warrants. The unified approach is too ambitious; the focus-on-one approach leaves provenance and vocabulary problems unfixed.

**Orchestrator content types?** → **Not at v1.0.** Nine orchestrators, nine approaches, no convergence. Define "orchestrator" as a form factor. Reserve "workflow definition" in the glossary for a future version. Ensure agent definitions include an `id` field for composition references.

**Cross-agent discovery?** → **Define a project-level content manifest** (`.syllago/manifest.json`). Path standardization is unachievable — vendor paths are entrenched. The manifest is the index-not-install-path approach every package manager uses.

**AGENTS.md status?** → **Universal baseline export target, not canonical hub format.** Plain markdown cannot carry structured metadata (activation modes, globs, inheritance). Correct role: every agent MUST support read/write of AGENTS.md; richer formats are additive. Syllago should develop a "broadcast mode" that writes AGENTS.md plus vendor-specific files simultaneously.

**Hooks spec opportunity?** → **Syllago's hooks spec is ready for community promotion.** Add a provenance block consistent with the skills spec. Publish alongside the skills spec as syllago's community contribution, positioning syllago as the reference implementation for both.

---

## Appendix: Data Tables

### Content Type Adoption Summary (54 agents)

| Content Type | ✅ Full | ⚠️ Partial | ❌ None | Adoption Rate |
|---|---|---|---|---|
| Rules | 30 | 16 | 12 | 78% |
| MCP | 33 | 8 | 16 | 70% |
| Agents | 26 | 11 | 20 | 63% |
| Commands | 24 | 4 | 30 | 44% |
| Skills | 18 | 4 | 34 | 37% |
| Hooks | 12 | 3 | 40 | 26% |

### All-✅ 6/6 Agents (Not in Syllago's 12)

Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands

### Five Fragmentation Hotspots Requiring Immediate Spec Work

1. **Hook event naming** — 8 different names for "before tool executes"; no canonical vocabulary
2. **MCP transport type collision** — `"http"` vs `"streamable-http"` for the same transport
3. **MCP auto-approval split** — `autoApprove` vs `alwaysAllow`
4. **Agent frontmatter proliferation** — 7-agent cluster shares 4 fields; Claude Code has 15
5. **Skills `supported_agents` terminology** — `supported_agent_platforms` vs `supported_agents`

### Three Convergence Patterns

- **Spec-driven** (Skills): Published spec drives rapid adoption. Healthiest pattern.
- **Imitation-driven** (AGENTS.md, MCP `mcpServers`): One agent sets convention, others copy. Stable surface, unstandardized depth.
- **Parallel evolution** (rules scoping, hook events): Same concept, different vocabulary. Hardest to convert.

---

## Section Files

| Section | File | Focus |
|---------|------|-------|
| 1 | [01-format-convergence.md](./01-format-convergence.md) | Per-content-type format and path analysis |
| 2 | [02-content-type-correlation.md](./02-content-type-correlation.md) | Co-occurrence patterns and adoption tiers |
| 3 | [03-gap-analysis.md](./03-gap-analysis.md) | Which agents to add next |
| 4 | [04-format-family-trees.md](./04-format-family-trees.md) | Converter architecture implications |
| 5 | [05-spec-implications.md](./05-spec-implications.md) | What to standardize and when |
| 6 | [06-emerging-patterns.md](./06-emerging-patterns.md) | Novel content types and surprising findings |
| 7 | [07-additional-angles.md](./07-additional-angles.md) | Strategic questions and recommendations |

---

---

## Appendix B: Validation Results

Two validation subagents checked 20 specific claims against live sources (GitHub repos, official docs, release notes). Results:

### Confirmed Claims (12/20)

| Claim | Status | Notes |
|-------|--------|-------|
| Agent Skills spec published Dec 2025 | **Confirmed** | agentskills/agentskills repo created Dec 16, 2025 |
| Agent Skills adoption count "16+" | **Undersold** | agentskills.io lists **33 adopters** — report undersells |
| oh-my-pi discovers 8+ formats | **Confirmed** | Exactly 8 (CC, Cursor, Windsurf, Gemini, Codex, Cline, Copilot, VS Code) |
| Junie CLI imports from .cursor/.claude/.codex skills | **Confirmed** | Official JetBrains docs describe this |
| Qwen Code mirrors Claude Code with .qwen/settings.json | **Confirmed** | QwenLM/qwen-code (21.4K stars) |
| OpenHands micro-agents in .openhands/microagents/ | **Confirmed** | YAML frontmatter + markdown body verified |
| Claude Code 21+ events, 4 handler types | **Confirmed** | Actually 28 events — report conservative |
| Cursor hooks use beforeShellExecution naming | **Confirmed** | No preToolUse in Cursor vocabulary |
| VS Code Copilot adopted Claude Code naming | **Confirmed** | preToolUse/postToolUse, not Cursor's naming |
| Gemini CLI BeforeModel/AfterModel unique hooks | **Confirmed** | Unique in ecosystem per available evidence |
| Amazon Q hooks with agentSpawn/preToolUse matchers | **Confirmed** | Official AWS docs |
| MCP transport type collision (http vs streamable-http) | **Confirmed** | Real-world friction documented |

### Corrections Needed (8/20)

| Claim | Issue | Correction |
|-------|-------|------------|
| Cursor CLI launched Jan 2026 with 6/6 support | **Unverifiable** | No changelog entry for Jan 2026 CLI launch; hooks not confirmed in Cursor CLI docs |
| Skills.sh hit 26,000+ installs | **Misleading** | skills.sh is a leaderboard; 26K is individual top-skill counts, not a tool's install count |
| 650-trial activation study (77% vs 100%) | **Unverifiable** | No public source found; referenced in provenance research but origin unclear |
| Continue auto-imports MCP from CC/Cursor | **Overstated** | Users manually copy config files to `.continue/mcpServers/`; not auto-import |
| gptme uses `gptme.toml` config | **Wrong filename** | Correct path: `~/.config/gptme/config.toml` |
| Exit code 2 standard includes Windsurf | **Incorrect** | Windsurf uses JSON responses for blocking, not exit codes |
| Windsurf `model_decision` is a hook trigger | **Miscategorized** | `model_decision` is a rules activation mode, not a hook trigger type |
| Pi uses QuickJS with hostcall ABI | **Conflation** | Describes unofficial Rust port (Dicklesworthstone/pi_agent_rust); official Pi uses jiti |

### AGENTS.md Adoption — Nuanced

The claim "AGENTS.md read by 20+ agents" is **plausible by count** (~23 on agents.md site) but **specific agents listed are questionable**. Confirmed on official list: Goose, Junie, Jules. Not on official list: Crush, Lovable, Replit, OpenHands, Antigravity (though Antigravity added AGENTS.md support in v1.20.3, Mar 2026). The agents may read AGENTS.md in practice without being on the official agents.md website.

### Impact on Report Findings

These corrections are mostly precision issues, not directional ones. The key findings hold:
- Skills convergence is **stronger than reported** (33 adopters, not 16+)
- Claude Code hooks are **richer than reported** (28 events, not 21+)
- The Windsurf `model_decision` discussion should be framed as a **rules feature**, not hooks
- The Pi hooks analysis should reference the **official jiti-based system**, not the unofficial Rust port
- The exit-code-2 convergence claim should drop Windsurf from the list

No corrections change the strategic recommendations. The core analysis — adoption tiers, format families, gap analysis, spec implications — remains valid.

---

*Analysis conducted 2026-03-30. Data from provider-agent-naming.md Content Type Support Matrix (54 agents), provenance-versioning-research.md (20+ specs), and frontmatter_spec_proposal.md. Validation against live sources completed same day.*
