# Section 5: Spec Implications

**Date:** 2026-03-30
**Report:** Format Convergence Analysis
**Sources:** `provider-agent-naming.md`, `2026-03-30-provenance-versioning-research.md`, `docs/spec/skills/frontmatter_spec_proposal.md`

---

## Overview

The content type adoption data across 54 agents reveals a landscape that is simultaneously mature in some areas and still in active flux in others. Applying the wrong spec pressure at the wrong time causes harm: premature standardization locks in bad formats before the market has found natural equilibria; delayed standardization allows fragmentation to compound until interoperability becomes expensive to retrofit. This section identifies where the spec lever should be pulled now, where it should wait, and what specific fields the spec should define.

---

## 1. Standardization Priority Matrix

Plotting each content type by adoption rate and format fragmentation produces a clear action map.

**Fragmentation assessment** — derived from the number of distinct formats in active use, the degree of structural incompatibility between them, and the absence of a de facto leading approach:

| Content Type | Adoption Rate | Fragmentation | Quadrant | Recommendation |
|---|---|---|---|---|
| **Rules** | 78% | Low | Low frag / High adoption | **Codify** — spec what already exists |
| **MCP** | 70% | Low | Low frag / High adoption | **Defer** — protocol spec already owns this |
| **Agents** | 63% | High | High frag / High adoption | **URGENT** — standardize now |
| **Commands** | 44% | Medium | Medium frag / Medium adoption | **Targeted** — spec structure, not format |
| **Skills** | 37% | Low–Medium | Converging / Low–medium adoption | **Accelerate** — ratify the emerging standard |
| **Hooks** | 26% | High | High frag / Low adoption | **PREMATURE** — let the market settle |

### Quadrant Details

**URGENT (High fragmentation + High adoption): Agents**

Agent definition files have 63% adoption across the landscape, but no agreed format. AGENTS.md (plain markdown) co-exists with YAML frontmatter profiles (Copilot `agent.yaml`, OpenHands microagents), inline JSON configs (Zencoder), and agent marketplace registries (Kilo Code). This is the content type with the largest active user base and the most format divergence. Without a spec, distribution tools like syllago and rulesync must maintain a growing N×M conversion matrix with no stable target to converge on.

**Codify (Low fragmentation + High adoption): Rules**

Rules are near-universal at 78%, and AGENTS.md has emerged as a de facto cross-provider standard — recognized natively by Cursor, Windsurf, Copilot, Goose, Warp, Junie, Crush, Factory Droid, and 15+ others. The market has already solved this. The spec's job here is not to standardize but to *describe* what already exists, formalize the AGENTS.md convention, and document how agent-specific rule files (CLAUDE.md, CURSOR.md) relate to it. Codifying prevents drift but does not require inventing anything new.

**Defer (Low fragmentation + High adoption): MCP**

MCP is at 70% adoption and already governed by Anthropic's open MCP protocol spec. The content type is not fragmented — the format is `mcp.json` / `settings.json` entries following the protocol schema. There is no spec gap here for a skills spec to fill. The only MCP question for the community spec is: what metadata should describe an MCP configuration as installable content? That is a narrow registry/distribution concern, not a format concern.

**Targeted (Medium fragmentation + Medium adoption): Commands**

Slash commands appear in 44% of agents, but the fragmentation is at the format layer (Markdown vs TOML vs JSON vs YAML frontmatter), not the semantic layer. The *concept* of a named invocable command with a description and content body is already convergent. A spec can define the abstract structure without mandating file format, leaving format mapping to each agent's converter.

**Accelerate (Converging + Low adoption): Skills**

At 37% adoption, skills are the newest content type and adoption is accelerating — the Agent Skills standard (`SKILL.md` + YAML frontmatter) has been adopted by Cursor CLI, Crush, Junie, gptme, Mistral Vibe, Trae, Antigravity, v0, Manus, OpenHands, and more in a short window. The frontmatter_spec_proposal.md exists. The risk here is not standardizing too early — it is standardizing too slowly and allowing the current emerging consensus to fragment before the spec catches up. The spec should accelerate ratification of the existing proposal with the additions described in sections 4 and 5 below.

**PREMATURE (High fragmentation + Low adoption): Hooks**

Hooks have only 26% adoption — the lowest of any content type — and the implementations are structurally incompatible: Claude Code's JSON settings merge (`hooks` key in `settings.json`), Amazon Q's YAML matchers with typed event names, Augment's `.augment/hooks/` directory, Composio's reaction hooks for CI events, Qwen Code's experimental hook system, and Open SWE's middleware hooks. These are not different serializations of the same model. They represent fundamentally different design decisions about what a hook even *is*: lifecycle interceptor, event reactor, CI trigger, or middleware function. Speccing this now would either produce a lowest-common-denominator spec that covers no real use case well, or back a particular model that turns out not to win. The hook standardization window opens after the market reaches ~50% adoption and the leading structural model becomes visible.

---

## 2. Per-Content-Type Spec Assessment

### Rules (78% adoption) — Codify what exists

AGENTS.md is the de facto standard and the spec should bless it. This means:

- **Define AGENTS.md as the canonical cross-agent rules file** at the project root. No new structure needed — the existing convention is the spec.
- **Define the agent-specific override pattern**: `CLAUDE.md`, `CURSOR.md`, `.cursor/rules/`, etc. are valid agent-specific complements, not replacements. The spec should describe the priority order (agent-specific file takes precedence over AGENTS.md for that agent).
- **Do not specify AGENTS.md content structure.** The strength of AGENTS.md is that it is unstructured natural language. Adding required sections or frontmatter would break backward compatibility with every existing AGENTS.md file in the wild. The spec's role here is to name and locate the file, not to structure its contents.

**What the spec should NOT do for rules:** require frontmatter, mandate section headers, or invent a new format. The market has spoken.

### Skills (37% adoption) — Ratify with targeted additions

The `frontmatter_spec_proposal.md` is the right foundation. It has the right shape: `provenance`, `expectations`, `supported_agent_platforms`. The three additions that transform it from a good draft into a durable spec are covered in sections 4 and 5 below (triggers, content hash, derived_from).

**Terminology fix required:** The current spec uses `supported_agent_platforms`. The naming research establishes that `agent` is the correct term and `platform` is overloaded. The field should be renamed to `supported_agents` before the spec is ratified. This is a breaking change that costs nothing to make now and compounds as a terminology debt if deferred.

**The spec should also define:**
- The `SKILL.md` filename convention as the canonical entrypoint
- The `metadata:` top-level key as the namespace for all frontmatter
- That additional fields outside `metadata:` are reserved for agent-native use and should not conflict

### Hooks (26% adoption) — Hold

No spec recommendation at this time. The syllago hook data layer and spec (see `docs/spec/hooks.md`) is the right investment — building conversion infrastructure prepares for standardization without forcing it. When adoption reaches ~50% and one structural model dominates (likely Claude Code's `settings.json` JSON pattern, given its role as the ecosystem reference implementation), the spec should move fast. The canonical event names and tool names work underway in syllago's `toolmap.go` is exactly the right pre-work: a provider-neutral vocabulary ready to anchor a future spec.

### MCP (70% adoption) — Narrow scope only

The community spec should not duplicate the MCP protocol spec. The only MCP-adjacent spec gap is in the **registry/distribution layer**: what metadata describes an MCP server configuration as something that can be installed, versioned, and shared? This is a narrow `provenance` question, not a format question. If the community spec addresses MCP at all, it should be in the form of an "MCP config as installable content" profile — a thin wrapper that adds `version`, `source_repo`, and `content_hash` to an otherwise standard MCP configuration block.

### Commands (44% adoption) — Structural spec, format-agnostic

The spec should define the abstract structure of a command:
- A name (the invocation key, e.g., `review`)
- A description (what it does, used for routing and display)
- A content body (the prompt or instructions)
- Optional `arguments` schema if the command accepts parameters

The spec should explicitly *not* mandate file format — Markdown with frontmatter (Claude Code, Copilot), TOML (Gemini CLI), JSON (OpenCode), and plain Markdown with a naming convention (Continue) all express this same structure. Format is a per-agent concern. A structural spec gives distribution tools a stable conversion target without locking any agent's native format.

### Agents (63% adoption) — URGENT, but careful scope

Agent definition is the most urgent standardization need, but it also spans the widest behavioral surface area. The risk is writing a spec broad enough to cover Devin's autonomous task agents, Cursor's inline subagents, and Copilot's workflow agents — and producing something that fits none of them well.

**Recommended scope boundary:** The spec should cover *content-addressable agent definitions* — files that describe an agent's identity, instructions, tool permissions, and activation conditions. It should explicitly exclude runtime orchestration (how agents call other agents, task decomposition, state management). That is a protocol concern, closer to ACP (Agent Communication Protocol) than to a content spec.

**Minimum viable agent spec fields:**
- Name and description
- System instructions (content body, same as AGENTS.md)
- `supported_agents` (which harnesses can run this agent definition)
- `tools` or `permissions` (what the agent is allowed to do)
- Provenance fields (version, source_repo, content_hash) — same as skills

This intentionally reuses the skill spec's provenance structure. A unified provenance block across all content types is better than inventing per-type variants.

---

## 3. Spec Scope Recommendation

The research supports **option (b): separate specs per content type**, organized under a shared vocabulary and provenance standard.

**Rationale:**

A single unified spec covering all six content types would face two structural problems. First, the content types are at very different maturity levels — skills are ready to ratify, hooks are premature, agents are urgent but complex. A unified spec would either hold the mature types hostage to the immature ones, or ship with major sections marked "TBD," which undermines the spec's credibility as a stable target.

Second, the content types have different primary audiences. Skills are authored by individual developers for personal and team sharing. Agents are increasingly authored by enterprises and tool vendors for distribution. MCP configs are infrastructure-adjacent and operated by platform teams. Bundling these into a single spec creates a document that is simultaneously too broad for any one audience to navigate and too narrow for any one use case to satisfy completely.

**The recommended structure:**

1. **A shared vocabulary and provenance standard** — defines the terminology (agent, harness, model provider, form factor), the `provenance` block structure (version, source_repo, content_hash, derived_from), and the `supported_agents` field. This is the common foundation all per-type specs build on.

2. **A skills spec** — extends the shared provenance standard with `expectations`, `triggers`, and the `SKILL.md` file convention. This is closest to ready and should be the first published spec.

3. **An agents spec** — extends the shared provenance standard with agent-specific fields (tools, permissions, system instructions). Should follow the skills spec by one version cycle.

4. **A commands spec** — structural-only, format-agnostic. Can be published alongside the agents spec.

5. **A rules guidance doc** — not a formal spec, but a reference document that codifies the AGENTS.md convention. Rules are natural language; a spec would be overengineering.

6. **Hooks** — deferred until the market converges. The syllago canonical event vocabulary is the pre-work, not the spec.

**Why not option (c) — focus on 1-2 types?**

Skills + agents is a compelling focus. But the terminology confusion between agents (the products) and agents (the content type) argues for getting the shared vocabulary spec published first, separately, before the per-type specs proliferate the ambiguity further. The naming research documents this problem directly.

---

## 4. Trigger Mechanism Standardization

The provenance-versioning research documents a fundamental design gap in the current skills spec: **there is no structured trigger mechanism**. The `description` field does double duty as both a human-readable explanation and an LLM routing signal, conflating "what this skill does" with "when the skill should activate."

The research findings on this are sharp:
- A 650-trial study found passive descriptions ("Docker expert for containerization") achieve only ~77% activation
- Directive descriptions ("ALWAYS invoke this skill when...") hit 100%
- The gap spawned a cottage industry of activation workarounds (ALWAYS/NEVER directives in descriptions, manual `/` commands as the reliable fallback)

Every established system that handles triggered behavior separates "what" from "when":
- VS Code: `description` vs `activationEvents` (onLanguage, onCommand, workspaceContains)
- GitHub Actions: workflow `name` vs `on:` block (push, PR, schedule, paths)
- Cursor: rule content vs four activation modes (always, intelligent, file globs, manual)
- Dialogflow: fulfillment vs intent training phrases

The current skills spec is an outlier in combining both into a single field.

**Spec recommendation: add a `triggers` block**

```yaml
triggers:
  mode: auto          # auto | manual | always
  file_patterns:
    - "**/*.test.ts"
    - "**/*.spec.ts"
  workspace_contains:
    - "jest.config.*"
  commands:
    - "/test-review"
  keywords:
    - "review tests"
    - "check test coverage"
```

All fields are optional. Skills without a `triggers` block fall back to description-only activation, preserving full backward compatibility with every existing SKILL.md.

**Cross-provider mapping:** The structured trigger fields map cleanly to native provider mechanisms:
- `file_patterns` → Claude Code `paths`, Cursor `globs`, Copilot `applyTo`
- `commands` → Claude slash commands, Gemini `/command`, Copilot prompt file names
- `mode: always` → Cursor `alwaysApply`, Copilot auto-attached instructions
- `workspace_contains` → VS Code `workspaceContains` activation event (and equivalent patterns in other agents)

The `keywords` field serves agents that use semantic routing (LLM-based dispatch) rather than deterministic pattern matching. It provides structured hints rather than embedding trigger intent in prose, which is easier for distribution tools to extract, transform, and inject into agent-native routing configs.

**Note on the `mode` field:** The three values map to the three dominant activation paradigms visible in the landscape. `auto` is description/pattern-guided (the current default behavior). `manual` is explicit invocation only (user types `/skill-name`). `always` is unconditional inclusion in every context (Cursor's `alwaysApply`, system-prompt-level rules). Authors who want reliable activation should use `mode: always` sparingly and `file_patterns`/`workspace_contains` for context-specific precision.

---

## 5. Provenance and Versioning

The current spec has `version` and `source_repo` in the `provenance` block. The research identifies two additional fields that upgrade the spec from "attribution metadata" to "supply chain integrity":

### 5.1 Content Hash

**Add `content_hash` to the `provenance` block**, using the format `algorithm:hex_value` (e.g., `sha256:abc123...`).

The research documents the gap clearly: without a content hash, `source_repo` is trusted implicitly. If the upstream repository is force-pushed or the file is modified in transit, there is no way to detect it. The hash covers the skill's content files, computed deterministically over a sorted, normalized file list.

This is the OCI/Nix pattern: **mutable names + immutable digests**. The `version` field (semver) is the human-readable mutable name. The `content_hash` is the canonical immutable identity. They serve different purposes and both belong in the spec.

Distribution tools like syllago use `content_hash` to:
- Detect local drift (user modified an installed skill — hash no longer matches)
- Verify download integrity (what arrived matches what the author published)
- Enable reliable deduplication (same content from two different sources = same hash)

### 5.2 Derived From

**Add a `derived_from` block** inside `provenance` for skills that are forks, conversions, or extracts of other skills:

```yaml
provenance:
  version: "2.0.0"
  source_repo: "https://github.com/bob/skills"
  source_repo_subdirectory: "/code-review"
  content_hash: "sha256:abc123..."
  derived_from:
    source_repo: "https://github.com/alice/skills"
    source_repo_subdirectory: "/code-review"
    version: "1.0.0"
    content_hash: "sha256:def456..."
    relation: "fork"    # fork | convert | extract | merge
```

The `relation` types map directly to syllago's conversion operations:
- `fork` — author copied and modified the original
- `convert` — syllago (or similar tool) transformed the content to a different agent's format
- `extract` — skill was pulled from a larger bundle (loadout, registry collection)
- `merge` — skill combines content from multiple sources

This is the Hugging Face typed derivation pattern adapted to the skills domain. The research established that "derived from" without a type is insufficient — a `convert` relationship (format-transformed, semantically equivalent) carries very different downstream implications than a `fork` (content-modified, possibly diverged).

**Single-level only.** The spec should define `derived_from` as pointing to the immediate parent only. Full chain traversal is done by following links, not by embedding the entire history. This is the SPDX/Hugging Face pattern, and it was chosen for good reason: embedding full chains creates stale metadata whenever any ancestor updates its fields.

### 5.3 Spec vs. Tool-Specific Split

The research establishes a clean principle: **intrinsic properties of the content belong in the spec; a consumer's relationship to the content is tool-specific state.**

| Field | Belongs In | Rationale |
|---|---|---|
| `version` | Spec (provenance) | Intrinsic — author's stated version |
| `source_repo` | Spec (provenance) | Intrinsic — canonical upstream location |
| `content_hash` | Spec (provenance) | Intrinsic — content identity |
| `derived_from` | Spec (provenance) | Intrinsic — authorship chain |
| `authors`, `license_name`, `license_url` | Spec (provenance) | Intrinsic — attribution |
| Sync strategy (track/pin/detach) | Tool-specific (install state) | Consumer's choice, not content property |
| Last-checked timestamp | Tool-specific | Operational state |
| Installed version history | Tool-specific | Local install log |
| Drift detection result | Tool-specific | Computed from `content_hash` comparison |
| Trust tier | Tool-specific | Security model varies by tool |
| Registry stars/downloads | Tool-specific | Platform metadata |

Syllago's `.syllago/installed.json` is exactly the right place for the tool-specific fields. The spec should not absorb them.

---

## 6. Risks of Premature Standardization

### 6.1 Hooks — The Largest Active Risk

Hooks are the content type where premature standardization would do the most damage. The 26% adoption rate masks a deeper problem: the agents that implement hooks have not converged on what hooks *are*.

- **Claude Code** treats hooks as settings-file entries that intercept named lifecycle events with shell commands, filtered by tool name and input patterns
- **Amazon Q** treats hooks as YAML matchers with typed event categories (agentSpawn, preToolUse, etc.) and structured response types
- **Augment Code** treats hooks as file-based scripts in a dedicated `.augment/hooks/` directory
- **Composio** treats hooks as CI/CD reaction triggers for code review and PR events
- **Open SWE** treats hooks as middleware functions in the agent's LangGraph execution pipeline

These are not format variations on a shared conceptual model. They are different answers to the question "what should a hook do?" A spec that tries to unify them would either:
1. Choose one model and alienate the other 74% — forcing Claude Code's JSON model onto agents that have fundamentally different hook architectures
2. Design an abstract superset — producing a spec so general it provides no implementation guidance and functions as marketing rather than engineering

The risk compounds because hooks are frequently security-relevant (they can intercept tool calls and modify agent behavior). A spec that gets the security model wrong — even subtly — could give users false confidence that hooks installed from a registry are safe when the spec's trust model is actually a lowest-common-denominator that doesn't match their agent's security architecture.

**The correct pre-spec work:** syllago's canonical event names (`before_tool_execute`, `after_tool_execute`, `before_message`, `after_message`) and canonical tool names (`shell`, `file_read`, `file_edit`) in `toolmap.go` are the right foundation. Build the vocabulary. Build the conversion infrastructure. Wait for the adoption curve to produce a dominant structural model. Then spec it.

### 6.2 Agents — Risk of Scope Creep

Agents are URGENT but also the most complex content type. The risk of premature standardization here is not standardizing too early — it is over-scoping the spec to cover runtime behavior, orchestration, and agent communication, which are protocol concerns that should go to ACP or a dedicated agent runtime spec.

An agent definition spec that tries to cover spawning, state handoff between agents, and task delegation will:
- Be immediately obsolete as orchestration patterns evolve (JetBrains Air's ACP, OpenAI Symphony's dispatch model, and Claude Squad's worktree approach are all actively competing)
- Conflict with runtime protocols if/when those protocols are standardized
- Be unpublishable until the hardest parts of the design are resolved, holding back the easy parts (provenance, identity, instructions) indefinitely

**The correct scope boundary:** an agent definition spec covers the *static content* that defines an agent (who it is, what it can do, what instructions it follows). Runtime behavior (how it executes, how it communicates with other agents, how it manages state) is explicitly out of scope.

### 6.3 Commands — Risk of Format Lock-In

Commands are at 44% adoption with four active formats in common use (Markdown frontmatter, TOML, JSON, plain Markdown). Speccing a single format now would exclude the three largest agent ecosystems that don't use it. The more durable move — a structural spec that is format-agnostic — avoids this, but requires the spec to be explicit about what it is *not* doing, or implementers will treat it as an implicit format mandate.

### 6.4 The Meta-Risk: Premature Unification of Vocabulary

The naming research identified that "provider" means three different things across syllago, the community spec, and the broader industry. Publishing specs that entrench any of these ambiguous usages before the terminology is settled creates a vocabulary debt that grows with every agent that adopts the spec. The recommendation: publish the shared vocabulary spec *first*, before any per-type spec that uses the terms `agent`, `harness`, `model_provider`, or `supported_agents`. Retrofitting terminology into a ratified spec is significantly more expensive than getting it right before publication.

---

## Summary: Recommended Spec Action Queue

| Priority | Action | Rationale |
|---|---|---|
| 1 | Publish shared vocabulary spec (terminology + provenance block) | Foundation for all other specs; fixes terminology fragmentation now |
| 2 | Ratify skills spec with `triggers`, `content_hash`, `derived_from` additions | Emerging standard at risk of fragmenting; additions are ready |
| 3 | Rename `supported_agent_platforms` → `supported_agents` in skills spec | Breaking change that's free now and expensive later |
| 4 | Publish agents spec (static content only, scoped away from runtime) | Highest fragmentation among active content types |
| 5 | Publish commands spec (structural, format-agnostic) | Medium-urgency; can ship with agents spec |
| 6 | Publish rules guidance doc (codify AGENTS.md convention) | Low-urgency; market has already solved this |
| 7 | Monitor hooks adoption; build vocabulary now, spec later | Premature; wait for dominant structural model |
