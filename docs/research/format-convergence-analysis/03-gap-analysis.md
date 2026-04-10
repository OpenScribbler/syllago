# Gap Analysis for Syllago

**Section 3 of Format Convergence Analysis**
**Date:** 2026-03-30
**Source data:** `docs/research/provider-agent-naming.md`

---

## Methodology

Content type scores are calculated from the Content Type Support Matrix in the source document.

- ✅ (full support) = 1.0 point
- ⚠️ (partial support) = 0.5 points
- ❌ (not supported) = 0 points

Maximum possible score per agent = 6.0 (one point per content type: Rules, Skills, Hooks, MCP, Commands, Agents).

**Syllago's 12 currently supported agents:** Claude Code, Gemini CLI, Cursor, Windsurf, Codex, Copilot CLI, Zed, Cline, Roo Code, OpenCode, Kiro, Amp.

All analysis below covers agents NOT currently in syllago's 12.

---

## 1. Missing High-Value Agents — Ranked by Content Type Score

The following table ranks all out-of-scope agents from the source matrix by total content type score. Only agents with a score ≥ 2.0 are shown (agents scoring below 2.0 have minimal syllago relevance).

| Rank | Agent | Rules | Skills | Hooks | MCP | Commands | Agents | Score | Category |
|------|-------|-------|--------|-------|-----|----------|--------|-------|----------|
| 1 | **Cursor CLI** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6.0 | CLI |
| 1 | **gptme** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6.0 | CLI |
| 1 | **Qwen Code** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6.0 | CLI |
| 1 | **oh-my-pi** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6.0 | CLI |
| 1 | **OpenHands** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6.0 | Autonomous |
| 6 | **Qodo** | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | 5.5 | Extension |
| 6 | **Factory Droid** | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ | 5.5 | Extension |
| 8 | **Augment Code** | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | 5.0 | Extension |
| 8 | **Amazon Q Developer** | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | 5.0 | Extension |
| 8 | **JetBrains AI / Junie** | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | 5.0 | Extension |
| 8 | **Antigravity** | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | 5.0 | IDE |
| 8 | **v0** | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | 5.0 | Web |
| 13 | **Devin** | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | 4.5 | Autonomous |
| 13 | **Trae** | ✅ | ✅ | ❌ | ✅ | ⚠️ | ✅ | 4.5 | IDE |
| 13 | **Junie CLI** | ✅ | ✅ | ❌ | ✅ | ✅ | ⚠️ | 4.5 | CLI |
| 13 | **Mistral Vibe** | ⚠️ | ✅ | ❌ | ✅ | ✅ | ✅ | 4.5 | CLI |
| 13 | **Gemini Code Assist** | ✅ | ⚠️ | ✅ | ✅ | ✅ | ❌ | 4.5 | Extension |
| 18 | **Warp** | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | 4.0 | CLI |
| 18 | **Kilo Code** | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | 4.0 | Extension |
| 18 | **DeepAgents CLI** | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | 4.0 | CLI |
| 18 | **Pi** | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | 4.0 | CLI |
| 22 | **GitHub Copilot** | ✅ | ❌ | ⚠️ | ✅ | ❌ | ✅ | 3.5 | Extension |
| 22 | **Open SWE** | ✅ | ❌ | ✅ | ⚠️ | ❌ | ✅ | 3.5 | Autonomous |
| 24 | **Crush** | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | 3.0 | CLI |
| 24 | **Goose** | ✅ | ❌ | ❌ | ✅ | ⚠️ | ⚠️ | 3.0 | CLI |
| 24 | **Continue** | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | 3.0 | Extension |
| 24 | **Tabnine** | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | 3.0 | Extension |
| 28 | **Aider** | ✅ | ❌ | ❌ | ⚠️ | ✅ | ❌ | 2.5 | CLI |
| 28 | **Jules** | ✅ | ❌ | ❌ | ⚠️ | ✅ | ❌ | 2.5 | Autonomous |
| 28 | **Manus** | ⚠️ | ✅ | ❌ | ❌ | ✅ | ❌ | 2.5 | Autonomous |
| 28 | **Zencoder** | ⚠️ | ❌ | ❌ | ✅ | ❌ | ✅ | 2.5 | Extension |
| 32 | **JetBrains Air** | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ | 2.0 | IDE |
| 32 | **Replit** | ⚠️ | ❌ | ❌ | ✅ | ⚠️ | ❌ | 2.0 | Web |

**Five agents score a perfect 6.0:** Cursor CLI, gptme, Qwen Code, oh-my-pi, and OpenHands. All five support every content type syllago manages. These are the richest conversion targets in the unsupported landscape.

---

## 2. Conversion Opportunity Matrix

The highest-value targets are agents where syllago can convert multiple content types bidirectionally, the user base is meaningful, and the formats are documented well enough to implement.

### Tier 1 — Immediate High ROI (Score 5.0–6.0, open formats, active user base)

**Cursor CLI** (6.0) — Cursor's terminal agent inherits and extends the IDE's format. Rules in `.cursor/rules/`, skills as `SKILL.md` files, hooks via settings, `mcp.json`, slash commands, and subagents. Because syllago already supports Cursor IDE, Cursor CLI format compatibility is very high. This is the easiest 6/6 add in the entire landscape — most of the format work is already done.

**Kilo Code** (4.0) — Shares VS Code extension architecture with Cline and Roo Code (both already in syllago). Rules via `AGENTS.md`, MCP via `.kilocode/mcp.json` marketplace, slash commands, agent manager with orchestrator mode. Format patterns are near-identical to syllago's existing Cline/Roo Code support.

**Augment Code** (5.0) — Enterprise-focused VS Code + JetBrains extension. Supports all six content types with a clean directory-based layout: `.augment/rules/`, `.augment/skills/`, `.augment/hooks/`, MCP JSON config, and agent modes. Hooks are fully supported — one of only a handful of non-CLI agents with hook support.

**Amazon Q Developer** (5.0) — AWS's agent with hooks (agentSpawn, preToolUse matchers), MCP per-agent config, custom agent JSON, rules in `.amazonq/rules/*.md`, and slash commands. Strong enterprise footprint. Missing Skills only. Hook format is distinctive enough to be worth specific adapter work.

**JetBrains AI / Junie** (5.0) — Covers the JetBrains IDE ecosystem (IntelliJ, PyCharm, WebStorm, etc.). `AGENTS.md`, `.junie/skills/`, MCP via `.junie/mcp/mcp.json`, slash commands, and subagents. Junie CLI exists as a separate deployment mode that explicitly imports from other agents' directories — which makes it a strong format bridge.

**Qwen Code** (6.0) — Claude Code-inspired ecosystem with `.qwen/settings.json`, skills, hooks (experimental), MCP, commands, and agents. Because it tracks Claude Code's format closely, syllago's existing Claude Code adapter would cover 80%+ of the work. The Chinese developer market is large; Qwen Code is Alibaba's primary coding agent.

**OpenHands** (6.0) — Full 6/6. Micro-agents via markdown + YAML frontmatter, plugin hooks, native MCP with OAuth, slash commands, `.openhands/microagents/`. OpenHands is the leading open-source autonomous agent; its micro-agent format is well-documented. Strong signal for the autonomous-agent category.

**gptme** (6.0) — Full 6/6. `~/.config/gptme/config.toml`, agent skills standard, plugin hooks, dynamic MCP, 20+ built-in commands, agent templates. Niche user base (power users, researchers) but the richest CLI agent outside syllago's current 12. Open-source with detailed docs.

### Tier 2 — Solid ROI, Partial Format Coverage (Score 3.5–4.9)

**GitHub Copilot** (3.5) — Largest installed base of any coding agent by far (50M+ developers). Rules (`AGENTS.md`, `copilot-instructions.md`), MCP via JSON config, agent YAML profiles. Hooks are preview-only in JetBrains. No skills. Even partial support (rules + MCP + agents) would benefit more users than most full-support adds.

**Warp** (4.0) — CLI agent with `AGENTS.md`, rich MCP GUI, agent profiles, and slash commands. Growing user base. No hooks (requested, not shipped). Clean format; easily converted.

**Antigravity** (5.0) — Google's entry. `GEMINI.md`/`AGENTS.md` rules, AgentKit 2.0 skills (40+ built-in), MCP Store integration, workflow-as-commands. No hooks. The Google provenance means format stability and enterprise adoption.

**Devin** (4.5) — High-profile autonomous agent. Rules/skills as Playbooks, MCP marketplace, slash commands. Hooks via event-triggered API. Restricted API access limits format transparency, but Playbook format is documented.

**Factory Droid** (5.5) — Enterprise-focused with `AGENTS.md`, skills, full hooks (with global toggle), MCP, commands, and agents. One of only ~11 agents with hook support. Hooks via `~/.factory/hooks/`. Clean enterprise format makes this a strong add despite smaller user base.

### Tier 3 — Niche or Format-Constrained (Score 2.0–3.4)

**Continue** (3.0), **Tabnine** (3.0) — Rules + MCP + commands only. Useful for coverage breadth but no hooks or skills to convert. Lower strategic priority.

**Goose** (3.0) — Block's open-source agent with `AGENTS.md` and MCP-first design. Strong developer credibility, growing community. Limited content types (rules + MCP mainly) but recipes (workflow definitions) could map to commands.

**Aider** (2.5) — `CONVENTIONS.md`/`AGENTS.md` + built-in slash commands. Aider exposes itself as an MCP server, not a client, which inverts the typical relationship. Limited syllago conversion value.

---

## 3. Rulesync Overlap Analysis

Rulesync supports 25 targets. Two of those are meta-targets rather than agents (`AGENTS.md` as a generic target, `AgentSkills` as a spec target). Of the 23 real agents, 11 overlap with syllago's 12 (all of syllago's except Amp). That leaves **13 agents rulesync supports that syllago does not**:

| Agent | Rulesync Target ID | Content Score | Syllago Should Add? | Reasoning |
|-------|--------------------|---------------|---------------------|-----------|
| **Goose** | `goose` | 3.0 | Yes — moderate priority | Rules + MCP. Block's open-source project with growing community. Rulesync-only means rules today; syllago adds MCP + commands conversion. |
| **GitHub Copilot** | `copilot` | 3.5 | Yes — high priority | Largest user base of any agent. Rules + MCP + agents alone justifies the add. Rulesync covers rules only. |
| **DeepAgents CLI** | `deepagents` | 4.0 | Yes — moderate priority | Rules + skills + MCP + agents. Smaller user base but rich content types. Syllago adds skills and agents conversion over rulesync. |
| **Factory Droid** | `factorydroid` | 5.5 | Yes — high priority | One of only ~11 agents with hooks. Rulesync covers rules; syllago can convert hooks, skills, MCP, commands, agents. |
| **Kilo Code** | `kilo` | 4.0 | Yes — high priority | Near-identical format to Cline/Roo Code. Shares VS Code extension architecture with syllago's existing adapters. Rulesync covers rules; syllago covers MCP + commands + agents. |
| **Rovo Dev / Atlassian** | `rovodev` | Unknown | Low priority | Atlassian-internal agent. Limited public format documentation. Content type matrix not researched (not in the source matrix). |
| **Qwen Code** | `qwencode` | 6.0 | Yes — high priority | Full 6/6 support. Close Claude Code format affinity. Rulesync covers rules; syllago can convert all 6 types. Large Chinese developer market. |
| **Antigravity** | `antigravity` | 5.0 | Yes — moderate priority | Google's agent with skills and MCP. Rulesync covers rules; syllago adds skills (AgentKit), MCP, commands, agents. |
| **JetBrains Junie** | `junie` | 5.0 | Yes — high priority | JetBrains ecosystem coverage. Skills, MCP, commands, agents. Rulesync covers rules; syllago covers the rest. Active import-from-other-agents feature is a natural format bridge. |
| **Augment Code** | `augmentcode` | 5.0 | Yes — high priority | Full hooks support, enterprise focus. Rules + skills + hooks + MCP + agents. Rulesync covers rules; syllago covers 5 additional types. |
| **Warp** | `warp` | 4.0 | Yes — moderate priority | Rules + MCP + commands + agents. Growing CLI user base. Rulesync covers rules; syllago adds MCP + commands + agents. |
| **Replit** | `replit` | 2.0 | Low priority | Rules (partial) + MCP only. Web-based; limited content type depth. Rules-only target is adequate for syllago to add eventually but not urgent. |

**Key insight:** For 9 of these 13 agents, rulesync's rules-only approach leaves 3–5 content types unconverted. Syllago's multi-type support makes it strictly more capable than rulesync on every one of these agents except Rovo Dev and Replit.

---

## 4. Market Position Analysis

### Where Syllago Has Structural Advantage

Rulesync is explicitly a rules-sync tool — its own documentation frames it as converting a single source-of-truth rules directory into per-agent formats. That design ceiling is the foundation of syllago's differentiation.

**Content types rulesync cannot touch:**
- **Hooks** — lifecycle event handlers require format-specific JSON/YAML schemas, not just markdown conversion. Only syllago's content model can represent and convert these.
- **Skills** — structured skill files with frontmatter, tool declarations, and input schemas. Rulesync has no concept of this.
- **MCP configs** — JSON merge into provider settings files. Rulesync doesn't handle JSON settings files.
- **Agents** — agent definition YAML/JSON profiles. Outside rulesync's markdown-centric model.
- **Commands** — slash command definitions with structured invocation schemas. Beyond rulesync's scope.

**Quantifying the gap:** Of the 25 agents rulesync supports, syllago can convert all 6 content types for agents that support them. For the 5 full-6/6 agents (Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands), rulesync reaches only rules; syllago reaches all 6. That's a 6x content coverage multiplier on the richest targets.

### Which Agents Benefit Most from Syllago's Multi-Type Support

These are agents where syllago's content depth matters most — ranked by the number of content types rulesync misses:

| Agent | Types Rulesync Covers | Types Syllago Adds | Gain |
|-------|-----------------------|--------------------|------|
| Cursor CLI | 1 (rules) | hooks, skills, MCP, commands, agents | +5 |
| gptme | 1 (rules) | hooks, skills, MCP, commands, agents | +5 |
| Qwen Code | 1 (rules) | hooks, skills, MCP, commands, agents | +5 |
| OpenHands | 1 (rules) | hooks, skills, MCP, commands, agents | +5 |
| Augment Code | 1 (rules) | hooks, skills, MCP, agents | +4 |
| Amazon Q Developer | 1 (rules) | hooks, MCP, commands, agents | +4 |
| JetBrains Junie | 1 (rules) | skills, MCP, commands, agents | +4 |
| Factory Droid | 1 (rules) | hooks, skills, MCP, commands, agents | +5 |
| Kilo Code | 1 (rules) | MCP, commands, agents | +3 |
| Warp | 1 (rules) | MCP, commands, agents | +3 |

The pattern is consistent: rulesync delivers 1/6 content types; syllago delivers up to 6/6. The value proposition is not incremental — it's a categorical difference for teams that use hooks, MCP configs, or skills.

### Hooks Are Syllago's Strongest Differentiator

Only 11 agents in the full landscape support hooks. Rulesync has no hooks concept at all. Syllago's hook adapter system (canonical format in `docs/spec/hooks.md`, provider-native mappings in `toolmap.go`) means that for any two hook-supporting agents a user works with, syllago is the only tool that can convert between them. This is a capability gap that rulesync cannot close without a fundamental redesign.

**Hook-supporting agents not yet in syllago's 12:**
Augment Code, Amazon Q Developer, Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands, Factory Droid, Pi, Gemini Code Assist, Open SWE. Adding these would expand syllago's hook conversion graph from ~3 agents to 14+.

---

## 5. Priority Recommendations — Top 10 Agents to Add Next

These rankings weight four factors: content type score, user base size, format compatibility with existing adapters, and strategic value (hook coverage, ecosystem positioning).

### 1. Kilo Code (Score: 4.0)

**Rationale:** Format is near-identical to Cline and Roo Code, both already in syllago. Shares VS Code extension architecture; `.kilocode/mcp.json` mirrors Cline's MCP config pattern. The adapter can likely reuse 70%+ of the Cline implementation. Rulesync already supports it, so not adding it leaves a gap for users who want MCP + commands + agents beyond rules. Self-describes as "500+ model" BYOK agent — strong power-user overlap with syllago's audience.

### 2. GitHub Copilot (Score: 3.5)

**Rationale:** Largest installed base of any coding agent. Even if content type support is limited to rules + MCP + agents (no skills, hooks partial/preview), the sheer number of Copilot users who also use other agents makes this a high-impact add. `.github/copilot-instructions.md` and `AGENTS.md` formats are stable and well-documented. This is the highest-reach target in the entire landscape.

### 3. Augment Code (Score: 5.0)

**Rationale:** One of only ~11 agents with full hooks support. Clean directory-based layout (`.augment/rules/`, `.augment/skills/`, `.augment/hooks/`) maps naturally to syllago's content model. Enterprise-focused VS Code + JetBrains extension — the enterprise positioning aligns with syllago's workspace use case. Adding Augment expands syllago's hook conversion graph significantly.

### 4. Cursor CLI (Score: 6.0)

**Rationale:** Full 6/6 content type support. Format inherits Cursor IDE (already in syllago), so adapter work is largely reusing and extending existing Cursor logic. Cursor CLI launched Jan 2026 and is gaining adoption fast. Being the only distribution tool supporting Cursor CLI is a differentiated position, given Cursor's large IDE user base migrating workflows to the terminal.

### 5. JetBrains AI / Junie (Score: 5.0)

**Rationale:** JetBrains IDEs (IntelliJ, PyCharm, WebStorm) represent the largest non-VS Code developer IDE market. Junie CLI explicitly imports from other agents' directories — Claude Code's `CLAUDE.md`, Cursor's `.cursor/rules/`, etc. This makes it a format bridge by design. Adding syllago support closes the loop: Junie users can export their Claude Code or Cursor configurations directly.

### 6. Amazon Q Developer (Score: 5.0)

**Rationale:** AWS's native coding agent with enterprise deployment at scale. Hook support is a differentiator — `agentSpawn`, `preToolUse`, and other matchers go beyond what most extension agents support. Rules in `.amazonq/rules/*.md`, MCP per-agent config, custom agent JSON. Enterprise teams using AWS who also use Claude Code or Cursor are a natural syllago audience.

### 7. Factory Droid (Score: 5.5)

**Rationale:** Enterprise agent with clean hooks support (`~/.factory/hooks/`) and a rich multi-type layout (`~/.factory/mcp/`, `~/.factory/droids/`, `~/.factory/commands/`). Supports VS Code + JetBrains + Zed. The Zed overlap with syllago's existing Zed support means some format patterns already apply. Enterprise focus aligns with syllago's positioning.

### 8. Qwen Code (Score: 6.0)

**Rationale:** Full 6/6. Tracks Claude Code's format closely — `.qwen/settings.json` mirrors Claude Code's settings structure, hooks are experimental but follow the same pattern. Alibaba's coding agent for the Chinese developer market (large, underserved by Western distribution tools). Adding Qwen Code costs little format work given Claude Code overlap and opens a large new market segment.

### 9. Goose (Score: 3.0)

**Rationale:** Block's (Square/Cash App) open-source agent with a strong developer community. `AGENTS.md` + `goosehints` rules, MCP-first design. Rulesync already supports it, so syllago adding it closes the gap. The MCP-first philosophy means Goose users likely have rich MCP configurations worth converting. Moderate content type depth but high community credibility.

### 10. Warp (Score: 4.0)

**Rationale:** CLI/terminal agent with a growing user base. `AGENTS.md` rules, rich MCP GUI, agent profiles, and slash commands. Users who work in the terminal and use both Warp and Claude Code (common overlap) want to share configurations between them. No hooks (feature requested but not shipped), so syllago's value here is rules + MCP + commands + agents — still 4 types rulesync misses.

**Honorable mentions:** gptme (6.0) and oh-my-pi (6.0) are technically the richest targets after Cursor CLI, but both serve niche power-user communities. Prioritize after the top 10 once mainstream agents are covered.

---

## 6. Content Type Coverage Gaps

This analysis asks: for each of syllago's 6 content types, what fraction of the total hook-supporting agent population is already covered by syllago's 12?

### Rules Coverage

Rules are the most universally supported content type (78% adoption across the landscape). Syllago's 12 cover Claude Code, Gemini CLI, Cursor, Windsurf, Codex, Cline, Roo Code, OpenCode, Kiro, and Zed — all major rules-supporting agents. **Gap:** GitHub Copilot, Augment Code, Amazon Q Developer, Kilo Code, Factory Droid, and ~20 others. Rules coverage is good but not broad enough given how many agents support them.

### MCP Coverage

MCP is the second-most-supported type (70% adoption). Syllago's 12 cover the major MCP hosts. **Gap is moderate** — agents like Kilo Code, Continue, GitHub Copilot, Augment Code, and Warp all have MCP configs syllago cannot yet convert. Since MCP configs are JSON-structured and relatively standardized, format compatibility is high — the gap is agent support count, not format complexity.

### Skills Coverage

Skills support is growing rapidly (37% adoption, accelerating). Syllago's 12 include several strong skills implementations (Claude Code, Codex, Cursor, Cline). **Gap:** Augment Code, JetBrains Junie, Antigravity, Qwen Code, gptme, oh-my-pi, OpenHands, DeepAgents CLI, v0 — all unsupported agents with skills. The Agent Skills standard is gaining traction; syllago's skills coverage should track that standard's adoption.

### Hooks Coverage — Critical Gap

Hooks are the rarest content type (26% adoption) but syllago's most strategically important. Only ~11 agents in the full landscape support hooks. Syllago's 12 currently include:

- Claude Code (hooks supported)
- Codex (hooks supported)
- Kiro (hooks supported)
- Gemini CLI (hooks limited — commands only, no lifecycle hooks)
- Others in syllago's 12: limited or no hooks

**Agents outside syllago's 12 with hooks support:** Augment Code, Amazon Q Developer, Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands, Factory Droid, Pi, Gemini Code Assist, Open SWE.

If syllago does not add hook-supporting agents, syllago's hook conversion graph remains thin. With only 3–4 active hook agents in the graph, users have few conversion paths. This is the most critical coverage gap: hooks are syllago's strongest differentiator over rulesync, but that advantage is only realized when multiple hook-supporting agents are in the conversion graph.

**Recommendation:** Of the top 10 priority adds above, prioritize the hook-capable ones — Augment Code (#3), Cursor CLI (#4), Amazon Q Developer (#6), Factory Droid (#7), Qwen Code (#8) — to maximize the hook conversion graph quickly.

### Commands Coverage

Commands are supported by ~44% of agents. Syllago's 12 cover Claude Code, Gemini CLI, Codex, and Cursor for commands. **Gap:** Kilo Code, Continue, Tabnine, Amazon Q Developer, Warp, JetBrains Junie, Factory Droid, gptme, oh-my-pi, and others all support commands that syllago cannot convert. This is a meaningful gap — but commands are agent-specific enough (slash command registries, invocation schemas) that format compatibility must be assessed agent by agent.

### Agents Coverage

The "Agents" content type (agent definition profiles — YAML/JSON files defining subagents or agent modes) is supported by ~63% of agents. Syllago's 12 include several agents-capable targets. **Gap:** GitHub Copilot, Kilo Code, JetBrains Junie, Augment Code, Factory Droid, Qwen Code, OpenHands, Warp, gptme, oh-my-pi, DeepAgents CLI — all unsupported. This is the content type with the most missed agents by raw count.

### Summary of Coverage Gap Severity

| Content Type | Syllago Strength | Gap Severity | Priority |
|--------------|-----------------|--------------|----------|
| Rules | Covers most major agents | Moderate — 20+ missing agents | Medium |
| MCP | Covers major hosts | Moderate — standardized format, just needs more agents | Medium |
| Skills | Covers core agents | Moderate — skills standard growing fast | Medium-high |
| Hooks | 3–4 agents, thin graph | **Critical** — differentiation only works with more hook agents | **High** |
| Commands | Covers primary agents | Moderate — agent-specific formats | Medium |
| Agents | Covers primary agents | Moderate — high count of missing agents | Medium |

Hooks demand priority investment. Every other content type has enough agents in syllago's 12 to provide useful conversion paths today. Hooks do not — syllago's hook conversion graph is so thin that the feature is nearly theoretical without adding Augment Code, Cursor CLI, Amazon Q Developer, Factory Droid, or Qwen Code.

---

## Key Findings Summary

1. **Five agents score 6/6:** Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands. All are high-value adds; Cursor CLI is the most accessible given existing Cursor adapter work.

2. **Rulesync covers 13 agents syllago doesn't.** For 11 of those 13, syllago would provide 3–5 additional content types beyond rulesync's rules-only support. The competitive gap is not agent count — it's content depth.

3. **Hooks are the critical gap.** Only 11 agents in the full landscape support hooks. Syllago's current hook conversion graph is too thin to be useful. Priority adds (Augment Code, Cursor CLI, Amazon Q Developer, Factory Droid, Qwen Code) would expand the graph to 8–9 agents and make hook conversion a real, usable feature.

4. **GitHub Copilot is the highest-reach add.** Even with limited content type depth (3.5), its installed base dwarfs all other agents. Rules + MCP + agents conversion for Copilot alone justifies the adapter work.

5. **Format affinity reduces implementation cost:** Kilo Code ≈ Cline/Roo Code, Cursor CLI ≈ Cursor IDE, Qwen Code ≈ Claude Code, Junie ≈ self-imports from existing formats. Several high-value adds are near-free in implementation terms given existing adapters.
