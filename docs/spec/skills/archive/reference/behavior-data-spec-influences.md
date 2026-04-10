# How Agent Behavior Data Informs the Metadata Convention

**Date:** 2026-03-31
**Status:** Reference document — informs `discussion-metadata-convention.md`
**Data source:** `docs/research/skill-behavior-checks/00-behavior-matrix.md` (12 agents, 14 behavioral checks, validated against source code and official docs)

---

## Overview

The [Agent Skills Metadata Convention](../discussion-metadata-convention.md) proposes structured provenance, triggers, expectations, and agent compatibility metadata for SKILL.md files. This document connects each convention section to empirical behavioral data from 12 agents, identifying where the data validates, strengthens, or extends the proposal.

The behavior matrix checked 14 runtime behaviors (from [agentskillimplementation.com/checks](https://agentskillimplementation.com/checks/)) across Claude Code, Codex CLI, Gemini CLI, Cursor, Windsurf, Kiro, GitHub Copilot, Cline, Roo Code, OpenCode, Amp, and JetBrains Junie CLI. Evidence levels range from source code inspection (SRC) to official documentation (DOC) to not documented (N/D).

---

## 1. Triggers — Validated and Strengthened

**Convention proposal:** A `triggers` section with `mode`, `file_patterns`, `workspace_contains`, `commands`, and `keywords` fields.

**What the data shows:** All 12 agents use description-only routing for skill activation today. No agent implements structured triggers for skills. But several agents already have the native mechanisms that convention trigger fields would map to — they just apply them to rules, not skills:

| Convention Field | Claude Code | Cursor | Kiro | Copilot |
|---|---|---|---|---|
| `mode: always` | Default behavior | `alwaysApply: true` | `inclusion: always` | Auto-attached instructions |
| `mode: manual` | `/skill-name` invocation | Manual rule type | `inclusion: manual` + `#name` | N/D |
| `file_patterns` | `paths:` (rules frontmatter) | `globs:` (rules frontmatter) | `fileMatch` + glob pattern | `applyTo:` (instructions frontmatter) |
| `workspace_contains` | No native equivalent | No native equivalent | No native equivalent | No native equivalent |
| `commands` | Slash command registration | N/D | N/D | N/D |
| `keywords` | `description` field (probabilistic) | `description` rule type | `description` for `auto` mode | Reminder prompt routing |

**Implications for the convention:**

1. **The mapping table above should be included as an informative appendix.** It proves every trigger field is implementable — agents already have the native mechanisms, just not wired to skills.

2. **`workspace_contains` has no native equivalent in any agent.** It's borrowed from VS Code's `activationEvents` and is a genuinely new capability. The convention should mark it as the least-adopted trigger field and set expectations accordingly — agents will need to build this, not just wire existing mechanisms.

3. **`mode` is the highest-value field.** Every agent already has an implicit mode (Claude Code defaults to auto, Cursor has 4 explicit modes, Kiro has 4 inclusion modes). Making mode explicit in the convention gives distribution tools a portable way to express author intent without per-agent format knowledge.

**Source:** Behavior matrix Q1 (discovery depth confirms progressive disclosure is universal — only name+description surface for inactive skills, meaning trigger metadata is available to the runtime at discovery without loading the full body).

---

## 2. Provenance — Validated, No Changes Needed

**Convention proposal:** `version`, `source_repo`, `content_hash`, `authors`, `publisher`, `derived_from` fields.

**What the data shows:** Provenance fields are intrinsic to content, not affected by runtime behavior. The behavior matrix does not change any provenance field. However, one data point strengthens the case:

**Frontmatter visibility varies.** 5 agents strip frontmatter before the model sees skill content (Claude Code, Gemini CLI, Cline, Roo Code, OpenCode). 2 agents include raw frontmatter in model context (Codex CLI, GitHub Copilot). 5 agents are undocumented.

This means `metadata.provenance` fields are **visible to the model on at least 2 agents**. This is not a security concern for version/source_repo/authors, but it adds noise tokens. The convention should note:

> Convention fields in `metadata` may be visible to the model on agents that do not strip frontmatter (confirmed: Codex CLI, GitHub Copilot). Skill authors should not store secrets in metadata fields. Distribution tools should be aware that metadata adds to token budget on these agents.

**Source:** Behavior matrix Q7 (frontmatter handling).

---

## 3. Script Security — Elevated from Open Question to Recommended Field

**Convention status:** Open Question #4 — "Should the convention add per-file hashes or capability declarations for scripts?"

**What the data shows:** This question is no longer theoretical. The behavior matrix reveals a concrete threat model:

| Security Tier | Agents | Count |
|---|---|---|
| **Gated** (approval required before skill loads) | Gemini CLI, Roo Code, OpenCode | 3 |
| **Permissioned** (tool access controlled, no per-skill gate) | Claude Code | 1 |
| **Unknown** (spec recommends gating, impl N/D) | Kiro | 1 |
| **Open** (auto-load, no approval) | Codex CLI, Cursor, Windsurf, Copilot, Cline, Amp, Junie | 7 |

On 7 of 12 agents, `git clone` of a repository with a malicious `.agents/skills/evil/SKILL.md` containing `scripts/payload.sh` would auto-load the skill with no user consent. The skill's instructions could direct the model to execute the script.

**Recommendation:** Promote script security from open question to a Draft-stability field:

```yaml
metadata:
  provenance:
    content_hash: "sha256:abc123..."
    script_hashes:
      "scripts/deploy.sh": "sha256:def456..."
      "scripts/validate.sh": "sha256:789abc..."
```

`script_hashes` enables:
- Distribution tools (syllago) to verify script integrity on install
- Agents that implement trust gating to compare hashes before execution
- Drift detection — "this script changed since the author published it"

This does not solve the gating problem (agents still need to implement approval), but it gives the ecosystem the building blocks for a trust chain. Without hashes, there is nothing to verify even if an agent adds gating later.

**Source:** Behavior matrix Q11 (trust gating).

---

## 4. New Proposed Field: `durability`

**Convention status:** Not currently proposed.

**What the data shows:** Context compaction protection is the biggest undocumented portability risk in the ecosystem:

| Agent | Skills Protected from Compaction? | Evidence |
|---|---|---|
| Claude Code | CLAUDE.md-embedded: yes. On-demand skills: N/D | DOC |
| OpenCode | **No** — compaction prompt is generic with no skill awareness | SRC |
| All others | N/D | — |

Only 1 of 12 agents definitively does NOT protect skills from compaction. The other 11 are undocumented — which means skill authors cannot know whether their instructions will survive long conversations.

**Recommendation:** Add a `durability` field at Draft stability:

```yaml
metadata:
  durability: persistent  # persistent | ephemeral
```

- `persistent` — Author intent: these instructions must remain in context for the full session. Agents with compaction should protect this skill's content from pruning.
- `ephemeral` — Author intent: these instructions are useful for the current task only and can be safely summarized or dropped during compaction.

This does not guarantee protection (agents must implement it), but it:
1. Signals author intent to agents that DO implement compaction protection
2. Gives distribution tools a basis for warnings: "This skill is marked `persistent` but Agent X has no compaction protection"
3. Creates a spec-level hook that agents can adopt incrementally

**Source:** Behavior matrix Q10 (context compaction).

---

## 5. New Proposed Field: `activation`

**Convention status:** Not currently proposed.

**What the data shows:** Skill deduplication varies dramatically:

| Mechanism | Agents | Quality |
|---|---|---|
| Code-level URI + content dedup | GitHub Copilot | Robust (prevents duplicate injection at both URI and content level) |
| Code-level per-turn dedup | Codex CLI | Partial (same skill blocked within one turn, but not cross-turn) |
| Prompt instruction only | Cline, Roo Code | Fragile (model compliance, no runtime enforcement) |
| None | OpenCode, Gemini CLI, and others | No dedup — skill re-injected on every activation call |

On most agents, a skill activated twice wastes tokens. On OpenCode, each re-activation adds the full `<skill_content>` block again.

**Recommendation:** Add an `activation` hint inside `triggers` at Draft stability:

```yaml
metadata:
  triggers:
    mode: auto
    activation: single  # single | repeatable
```

- `single` — This skill should activate at most once per session. Agents should deduplicate.
- `repeatable` — This skill may be meaningfully re-activated (e.g., a skill that produces different output based on conversation state).

Default is `single` (the common case). This gives agents a lightweight signal they can implement however they choose — Copilot's `hasSeen` set, Roo Code's prompt instruction, or a simple "already activated" check.

**Source:** Behavior matrix Q9 (reactivation deduplication).

---

## 6. `supported_agents` — Needs Behavioral Semantics

**Convention proposal:** A structured array declaring which agents the skill works with.

**What the data shows:** "Works with" is more nuanced than "can parse the file." The behavior matrix reveals runtime differences that affect skill correctness:

| Behavioral Dimension | Agents That Differ | Impact on Skills |
|---|---|---|
| Frontmatter visible to model | Codex, Copilot (include); 5 others strip | Skills with `metadata` instructions (e.g., "ignore the YAML above") break on stripping agents |
| Directory enumeration on activation | Gemini CLI, OpenCode (enumerate); others don't | Skills that assume the model knows about `references/` files break on non-enumerating agents |
| Loading scope | Windsurf (all files "available"); others body-only | Skills that depend on auto-loaded supporting files break on 11 agents |
| Nested discovery | Claude Code, Codex, OpenCode (deep); 6 agents flat only | Monorepo skill layouts invisible on flat-scan agents |
| Trust gating | 3 agents gate; 7 auto-load | Security-sensitive skills may need to declare "requires gated agent" |
| Path resolution | 10 agents use skill dir; Kiro may use workspace root | Scripts with relative paths break if base differs |

**Recommendation:** The convention should define what "supported" means in terms of behavioral assumptions. A skill entry like:

```yaml
metadata:
  supported_agents:
    - name: "Claude Code"
    - name: "Cursor"
    - name: "Gemini CLI"
```

...should mean "the skill's instructions, loading assumptions, and runtime behavior have been tested on these agents." The convention doc should include a non-normative checklist for skill authors:

> Before listing an agent as supported, verify:
> - Your skill does not depend on frontmatter being stripped (Codex CLI and Copilot include it)
> - Your skill does not assume supporting files are auto-loaded (only Windsurf may do this)
> - Your skill does not assume directory enumeration (only Gemini CLI and OpenCode enumerate)
> - Your scripts use `${CLAUDE_SKILL_DIR}` or equivalent path variables, not hardcoded relative paths
> - If your skill contains executable scripts, note that 7 agents auto-load without approval

**Source:** Behavior matrix Q2, Q5, Q7, Q11, Q12.

---

## 7. Loading Architecture — Informative Context for Implementers

The behavior matrix identified three distinct loading architectures. This is informative for convention implementers, not a convention field:

| Architecture | Agents | How Convention Fields Are Consumed |
|---|---|---|
| **Dedicated skill tool** | Codex, Gemini, Cline, Roo Code, OpenCode | Convention fields parsed by tool handler at activation time; can influence tool output format |
| **Standard read_file** | GitHub Copilot | Convention fields visible to model as raw YAML (if not stripped); agent can't act on them programmatically |
| **Implicit injection** | Claude Code, Cursor, Windsurf, Kiro, Amp, Junie | Convention fields parsed by runtime; can influence injection decisions (e.g., mode determines whether to inject) |

**Implication:** `triggers.mode` and `triggers.file_patterns` are only actionable on agents with dedicated skill tools or implicit injection. On Copilot (read_file architecture), the model sees triggers as YAML text and must interpret them — which brings us back to the probabilistic activation problem. The convention should note that trigger fields provide **deterministic activation on agents that implement them** and **structured hints on agents that don't**.

---

## Summary: Changes to the Convention

| Section | Change | Stability | Source |
|---|---|---|---|
| Triggers | Add mapping table appendix showing per-agent native mechanism equivalents | Informative | Q1, format convergence data |
| Provenance | Add note about frontmatter visibility on Codex/Copilot | Note | Q7 |
| Script Security | Promote from open question to `script_hashes` field | Draft | Q11 |
| New: `durability` | Signal whether skill content must survive context compaction | Draft | Q10 |
| New: `activation` | Signal whether skill should be deduplicated on re-activation | Draft | Q9 |
| `supported_agents` | Add behavioral assumptions checklist for skill authors | Informative | Q2, Q5, Q7, Q11, Q12 |
| Implementer guidance | Document three loading architectures and how each consumes convention fields | Informative | Q1, Q2, Q8 |

---

## Data Sources

All findings referenced in this document are validated against source code or official documentation. The full evidence chain is in:

- **Behavior matrix:** `docs/research/skill-behavior-checks/00-behavior-matrix.md`
- **Group 1 (Claude Code, Codex CLI, Gemini CLI):** `docs/research/skill-behavior-checks/agent-group-1-cc-codex-gemini.md`
- **Group 2 (Cursor, Windsurf, Kiro):** `docs/research/skill-behavior-checks/agent-group-2-cursor-windsurf-kiro.md`
- **Group 3 (GitHub Copilot, Cline, Roo Code):** `docs/research/skill-behavior-checks/agent-group-3-copilot-cline-roo.md`
- **Group 4 (OpenCode, Amp, Junie CLI):** `docs/research/skill-behavior-checks/agent-group-4-opencode-amp-junie.md`
- **Checks framework:** [agentskillimplementation.com/checks](https://agentskillimplementation.com/checks/)

Validation rounds: 4 total, 20 claims checked against live sources, all corrections applied, final sweep returned CLEAN.
