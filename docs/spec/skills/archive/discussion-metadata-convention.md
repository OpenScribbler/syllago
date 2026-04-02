# Proposal: Agent Skills Metadata Convention

**Author:** Holden Hewett
**Date:** 2026-03-31
**Status:** Draft for community discussion
**Review rounds:** 4 (5-persona review, behavioral data integration, attestation design, naming review, consensus fixes applied)

---

## Summary

This document proposes a **companion convention** to the Agent Skills specification that adds structured provenance, attestation, trigger mechanisms, dependency declarations, and agent compatibility metadata to SKILL.md files. Convention metadata lives in a **sidecar file** (`SKILL.meta.yaml`) referenced by a single `metadata_file` field in the SKILL.md frontmatter, preserving the spec's token-efficient design.

The proposal is backed by deep research across 20+ specifications (OCI, SLSA, npm, Cargo, Helm, SPDX, CycloneDX, Hugging Face, Git, Nix, Homebrew), analysis of 70+ AI coding agents, a 12-agent behavioral study validating 14 runtime checks against source code and official documentation, and a multi-perspective trust system analysis evaluating npm provenance, Sigstore, TUF, GitHub Attestations, and Mitchell Hashimoto's Vouch project.

---

## The Problem

The Agent Skills spec is deliberately minimal and that's one of its strengths — 32+ agents have adopted it precisely because it's easy to implement. But as the ecosystem grows, three gaps are creating real friction:

### 1. Skills lose their provenance when shared

When you find a skill repo with 20 skills and cherry-pick one, the connection to the original is lost. There's no way to:
- Track upstream updates without manually revisiting the source
- Know if you've diverged from the original after local edits
- Identify the original author when content passes through multiple hands
- Verify integrity — was this skill tampered with since publication?

No major package manager has solved fork/derivation tracking. npm, Cargo, and Helm all rely on naming conventions. The closest solutions exist in SPDX (relationship vocabulary), CycloneDX (pedigree model), and Hugging Face (base_model + relation type). This is an opportunity for the Agent Skills ecosystem to lead.

### 2. Skill activation is unreliable

A 650-trial study found that description-only activation achieves 50-77% reliability depending on wording. An entire cottage industry of workaround hooks exists to compensate. Every other trigger system in software separates "what something does" from "when it activates":

- VS Code: `description` (what) vs `activationEvents` (when)
- GitHub Actions: `name` (what) vs `on:` (when)
- Cursor: rule content (what) vs `alwaysApply`/`globs` (when)

The Agent Skills spec combines both into `description`. Structured trigger metadata would give authors deterministic activation alongside the existing probabilistic description-based routing.

### 3. Dependencies are freeform text

The `compatibility` field is a 500-character string. "Requires git, docker, jq, and access to the internet" is useful for humans but not machine-parseable. Agents can't check whether prerequisites are met, registries can't filter by requirements, and distribution tools can't warn about missing dependencies.

---

## The Approach

### Why a companion convention, not a spec change

We studied how 10 major specifications handle the "extend a minimal spec" problem:

| Pattern | Example | Outcome |
|---------|---------|---------|
| Vocabulary on mechanism | Schema.org on HTML microdata | Succeeded — vocabularies evolve independently |
| Stability levels | OpenTelemetry semantic conventions | Succeeded — Draft → Stable → Core graduation |
| Extension prefix | HTTP `X-` headers, CSS vendor prefixes | Failed — naming cliff prevents graduation |
| Extension registry | OpenAPI `x-` registry | Failed — no adoption incentives, ghost town |
| Ad hoc keys | npm package.json | Partial — works by accident, not design |

The best fit for our situation (minimal spec, 32+ adopters, eventual merger goal) is the **OpenTelemetry + Schema.org hybrid**:

1. Use a **sidecar file** as the extension surface — the SKILL.md frontmatter stays minimal, no existing adopter breaks
2. Publish a convention document defining what keys mean — like Schema.org sits on HTML
3. Assign stability levels — Draft fields may change, Stable fields won't
4. Graduate fields into the core spec based on demonstrated adoption, not theoretical utility

### How it stays compatible

A SKILL.md using this convention is **valid under the existing Agent Skills spec**. The convention adds a single optional top-level field (`metadata_file`) pointing to a sidecar file. Agents that don't know the convention ignore `metadata_file` — the skill's `name`, `description`, and body content are unaffected.

**SKILL.md** (what the model sees):

```yaml
---
name: my-skill
description: Does something useful.
metadata_file: ./SKILL.meta.yaml
---
```

**SKILL.meta.yaml** (what tooling reads — the model never sees this):

```yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.1"
provenance:
  version: "1.0.0"
  authors:
    - "Alice <alice@example.com>"
triggers:
  mode: auto
  keywords:
    - "do the thing"
```

An agent implementing only the base Agent Skills spec sees:
- `name: my-skill` — used for activation
- `description: Does something useful.` — used for activation
- `metadata_file: ./SKILL.meta.yaml` — ignored (unknown field)

An agent implementing the convention resolves `metadata_file`, reads the sidecar, and gets structured provenance and triggers.

---

## Token Efficiency: Why a Sidecar File

The Agent Skills spec's original frontmatter design is deliberately minimal — just `name` and `description` — so that agents injecting skill content into model context waste as few tokens as possible. The metadata convention adds ~30 lines of structured YAML (provenance, triggers, expectations, tags, status, durability, supported agents). On agents that do not strip frontmatter before context injection (confirmed: Codex CLI, GitHub Copilot), all of this metadata would be injected into the model's context on every skill activation, directly contradicting the spec's token-efficient design.

Additionally, the base Agent Skills spec defines `metadata` as "a map from string keys to **string values**." The convention's nested YAML structures (provenance with sub-objects, triggers with arrays) exceed that definition.

### The decision

A [5-persona panel review](reference/token-efficiency-metadata-separation.md) (Solo Author, Agent Developer, Registry Operator, Spec Purist, Community Member) evaluated four options and reached 4-1 consensus on the **sidecar file pattern**:

1. **A single `metadata_file` field** in the SKILL.md frontmatter points to an external `SKILL.meta.yaml` file
2. **All convention metadata** moves to the sidecar file — the model never sees it
3. **The SKILL.md frontmatter** returns to near-original simplicity: `name`, `description`, `license`, and optionally `metadata_file` (3–4 lines instead of 30+)

### Why this matters for adoption

- **Zero cost for non-implementing agents.** Agents that don't know the convention see one extra line (`metadata_file`) instead of 30+ lines of opaque YAML. That's a 97% reduction in wasted tokens on non-stripping agents.
- **No behavior change required.** Agents that already strip frontmatter (Claude Code, Gemini CLI, Cline, Roo Code, OpenCode) continue working with zero changes.
- **Respects the spec's design philosophy.** The Agent Skills spec chose minimalism for a reason. A companion convention that adds 30 lines to every skill file undermines the adoption argument ("it's lightweight, just `name` and `description`"). The sidecar pattern preserves that pitch.
- **Cleaner spec layering.** Moving structured data out of a string→string `metadata` map and into a dedicated file eliminates the type mismatch with the base spec's definition.

The full design rationale — including all four options evaluated, debate progression, and conditions — is in the [token efficiency reference document](reference/token-efficiency-metadata-separation.md).

---

## What the Convention Adds

### Provenance (Stability: Stable)

| Field | Purpose | Why |
|-------|---------|-----|
| `version` | Semver version of the skill | Enables update detection and version pinning. **Immutable:** same version + different hash = registry rejection. |
| `source_repo` | HTTPS URL to canonical source | **REQUIRED for registry submissions.** Enables verification and update checking. Forge-agnostic (GitHub, GitLab, Codeberg, Gitea). |
| `source_commit` | Full git commit SHA (40 hex chars) | **New in v0.1.** Immutable reference point — unlike branches/tags, SHAs can't be rewritten. Auto-populated by tooling, not manually by authors. |
| `source_repo_subdirectory` | Path within repo for cherry-picked skills | Solves the "which skill in a mono-repo" problem |
| `content_hash` | SHA-256 hash of skill content files | Enables integrity verification and drift detection |
| `authors` | List of creators (git format) | Attribution — who wrote this |
| `publisher` | Entity that distributed the skill | Distribution chain — who gave this to me |
| `license_spdx_id` | SPDX license identifier | Machine-parseable license compliance (renamed from `license_spdx` for clarity) |
| `license_url` | URL to full license text | Legal reference with copyright attribution |
| `derived_from` | Upstream skill coordinates + relation type | Fork/derivation tracking — where this came from |

The `derived_from.relation` vocabulary (fork, convert, extract, merge) was informed by Hugging Face's `base_model_relation` and CycloneDX's pedigree model — the two systems that have actually solved derivation tracking in practice.

**New in v0.1: `script_hashes`.** Per-file SHA-256 hashes for executable scripts within the skill directory. This was promoted from an open question to a Draft field based on behavioral data: of 12 surveyed agents, only 3 gate skill loading behind user approval (Gemini CLI, Roo Code, OpenCode), while 7 auto-load skills with no consent (Codex CLI, Cursor, Windsurf, Copilot, Cline, Amp, Junie). On those 7 agents, `git clone` of a repo with a malicious skill containing an executable script would activate it without user awareness. `script_hashes` provides the building blocks for a trust chain — without hashes, there is nothing to verify even if an agent adds gating later.

**Trust boundaries.** `source_repo`, `publisher`, and `derived_from` are all **self-asserted claims** — any skill can claim any value. The convention spec explicitly warns that distribution tools and registries SHOULD NOT make authorization or trust decisions based solely on these fields without attestation (section 8 of the spec) or equivalent independent verification.

**Attestation.** v0.1 introduces a lightweight attestation mechanism where registries independently verify that `content_hash` matches the content at `source_repo` at `source_commit`, then publish a structured JSON record of that verification. Attestation means **authenticity, not safety** — it proves bytes came from where they claim, not that those bytes are safe. See the dedicated attestation section below.

**Immutable versions.** Registries MUST reject submissions where `version` matches an existing entry but `content_hash` differs. This prevents supply chain attacks where an attacker republishes at the same version with different (malicious) content.

**Registry versioning.** Registries MAY reject skill submissions that omit `provenance.version`, since versioning is essential for update detection, deduplication, and dependency resolution. The field remains optional for local-only skills. `source_repo` is REQUIRED for registry submissions — without it, there's nothing to verify against.

**Frontmatter visibility.** With the sidecar pattern, convention fields are no longer in the SKILL.md frontmatter — the model sees at most one extra line (`metadata_file: ./SKILL.meta.yaml`). This resolves the token cost concern: agents that don't strip frontmatter (confirmed: Codex CLI, GitHub Copilot) see 1 extra line instead of 30+.

**Content hash algorithm.** The hash algorithm was significantly tightened after review to eliminate cross-platform divergence:

- **Byte-level replacement** for the self-referential `content_hash` blanking — a raw string substitution, NOT YAML re-serialization (different YAML libraries produce different whitespace and quoting, which would break hash determinism)
- **Mandatory CRLF→LF normalization** before hashing, ensuring identical hashes regardless of platform checkout settings (e.g., `git core.autocrlf=true` on Windows converts LF to CRLF on checkout, which would produce different hashes for the same repository content)
- **UTF-8 encoding** specified for relative path bytes in the hash computation

These fixes address the most common failure mode in hash-based integrity systems: two compliant implementations producing different hashes from the same input.

### Triggers (Stability: Draft)

| Field | Purpose | Type |
|-------|---------|------|
| `mode` | Author's activation intent (auto/manual/always) | Deterministic |
| `activation_file_globs` | Activate when working with matching files | Deterministic |
| `activation_workspace_globs` | Activate when project contains matching files | Deterministic |
| `commands` | Explicit command invocation names | Deterministic |
| `keywords` | Structured hints for LLM routing | Probabilistic |
| `activation` | Dedup hint: single (once per session) or repeatable | Agent hint |

Every agent maps these to their native mechanism: `activation_file_globs` → Claude Code `paths`, Cursor `globs`, Copilot `applyTo`. `commands` → Claude `/slash-command`, Gemini `/command`. `mode: always` → Cursor `alwaysApply`.

**Trigger precedence (new in v0.1).** Mode is evaluated first. If mode resolves to a definitive state (`always` → active, `manual` → inactive unless commanded), trigger evaluation is skipped entirely. A full mode × trigger matrix is in the convention spec. This addresses a gap flagged in community review: what happens when `mode: always` is set but globs don't match? Answer: globs are ignored — `always` takes unconditional precedence.

**Command collisions.** When multiple skills register the same command name, agents are responsible for disambiguation. Registries SHOULD flag collisions across indexed skills.

**Behavioral data validates every trigger field.** A study of 12 agents' runtime behavior found that all 12 use description-only routing for skills today, but several already have the native mechanisms that convention trigger fields would map to — they just apply them to rules, not skills. The full mapping is in the convention spec's Appendix A. Key findings:

- `mode` is the **highest-value field** — every agent already has an implicit mode. Making it explicit gives distribution tools portable activation intent.
- `activation_workspace_globs` has **no native equivalent in any agent** — it's a genuinely new capability borrowed from VS Code's `activationEvents`. Agents will need to build this.
- `keywords` maps to the existing probabilistic description-based routing that all agents already implement.

**Portable glob syntax.** `activation_file_globs` and `activation_workspace_globs` use standard glob syntax (not regex, not gitignore syntax) — a portable subset (`*`, `**`, `?`, `[...]`) defined inline in the convention. The initial draft referenced micromatch (a JavaScript library) — this was changed after review because micromatch is not a specification, and its behavior differs from POSIX glob, Python fnmatch, Go filepath.Match, and gitignore patterns. Brace expansion (`{a,b}`) and extglobs (`+(pattern)`) are explicitly excluded as non-portable.

**Activation dedup.** Skill re-activation behavior varies dramatically across agents: GitHub Copilot has robust URI + content dedup, Codex CLI deduplicates per-turn only, Cline/Roo Code use fragile prompt instructions, and OpenCode/Gemini CLI have no dedup at all (each re-activation injects the full skill content). The `activation` field (default: `single`) gives agents a lightweight signal they can implement with whatever mechanism they have.

### Expectations (Stability: Draft)

| Field | Purpose |
|-------|---------|
| `software` | CLI tools required (name + version constraint) |
| `services` | Remote services needed (name + URL) |
| `runtimes` | Programming language runtimes (name + version constraint) |
| `operating_systems` | Supported OSes (name + optional version constraint) |

Version constraints use semver range syntax (`>=`, `~>`, etc.) with the grammar defined inline in the convention, not by reference to an external spec. The `~>` (pessimistic constraint) operator follows RubyGems semantics with a fully specified algorithm and explicit validity rules (2 or 3 numeric components only — `~>1` and `~>1.2.3.4` are invalid).

**Field requirements.** After review, all expectations sub-types now specify `name` as REQUIRED and `version`/`url` as OPTIONAL. Canonical operating system values are defined (`linux`, `macos`, `windows`) to prevent fragmentation (`macOS` vs `macos` vs `darwin`).

### Durability (Stability: Draft)

**New in v0.1.** A signal for whether skill content must survive context compaction (`persistent` or `ephemeral`). Default is `persistent`.

This addresses the biggest undocumented portability risk in the ecosystem: of 12 surveyed agents, only Claude Code (for CLAUDE.md-embedded content) confirms compaction protection. OpenCode confirmed no protection — its compaction prompt is generic with no skill awareness. The remaining 10 agents are undocumented. Skill authors currently have no way to know whether their instructions will survive long conversations.

### Supported Agents (Stability: Draft)

Structured array declaring which agents the skill works with, including integration requirements. Complements the existing freeform `compatibility` field.

**Canonical identifiers.** Agent names now use canonical lowercase-hyphenated identifiers (e.g., `claude-code`, `cursor`, `gemini-cli`) from Appendix C of the convention spec. The initial draft used freeform strings — this was changed after review because `"Claude Code"` vs `"claude-code"` vs `"Claude"` would fragment search and filtering immediately. The convention defines 12 initial identifiers matching the regex `[a-z][a-z0-9-]*`, with new agents added in convention patches.

**Behavioral semantics.** "Works with" is more nuanced than "can parse the file." Behavioral data from 12 agents reveals runtime differences that affect skill correctness:

| Dimension | What Varies | Impact |
|---|---|---|
| Frontmatter handling | 5 strip, 2 include, 5 N/D | Skills with metadata-referencing instructions break on stripping agents |
| Directory enumeration | 2 enumerate, 10 don't | Skills assuming model knows about `references/` files break on non-enumerating agents |
| Nested discovery | 3 deep-scan, 6 flat-only, 3 N/D | Monorepo skill layouts invisible on flat-scan agents |
| Trust gating | 3 gate, 1 permissioned, 7 auto-load, 1 N/D | Security-sensitive skills need to know the agent's trust model |

The convention spec includes a non-normative behavioral checklist (6 items: frontmatter handling, supporting file loading, directory enumeration, nested discovery, path resolution, trust gating) for skill authors to verify before listing an agent as "supported."

### Attestation (Stability: Draft)

**New in v0.1.** All provenance fields are self-asserted — any skill can claim any `source_repo` or author. Attestation provides a mechanism for independent verification of these claims.

A registry (or any trusted party) fetches content from `source_repo` at `source_commit`, computes the hash, confirms it matches `content_hash`, and publishes a structured JSON attestation record. This is **registry-side verification** — authors do nothing new. They push to GitHub (or GitLab, Codeberg, Gitea) like today; the registry attests.

| Component | What it is |
|-----------|-----------|
| Attestation record | JSON object binding a `content_hash` to a verified `source_repo` + `source_commit` |
| Claim type | What was verified. v0.1 defines only `source_verified`. |
| Self-attestation | Disclosed via asymmetric enforcement — attester can't falsely claim independence |
| Trust states | Registry-level flags: `active`, `quarantined` (under review), `denounced` (confirmed bad) |
| Expiry | Attestations expire (default 12 months, range 6–24). Renewal requires re-review evidence. |
| Location | Registry-side canonical: `.attestations/{content_hash}.json`. Source-side optional breadcrumb. |

**Why not something heavier?** We evaluated npm provenance (requires CI), Sigstore (container-centric UX), TUF (requires key ceremonies), and GitHub Attestations (requires GitHub Actions). Every production trust system assumes a CI/CD pipeline. Skills are hand-authored — no build step. A system that requires CI excludes most skill authors.

**Why not just `git clone` + compare hashes?** That's actually the reference implementation (`syllago verify`). The spec standardizes the *output format* so multiple tools can produce and consume attestation records without each inventing their own.

**Authenticity, not safety.** A `source_verified` attestation proves the bytes came from where they claim. It does NOT prove those bytes are safe. The spec enforces this through display requirements: tooling MUST show who attested and MUST NOT display bare "verified" badges.

**Self-attestation.** Asymmetric enforcement: if `attester.id` and `source_repo` share the same hostname and org, `self_attestation` MUST be `true` regardless of declaration. The dangerous lie (false independence) is blocked; the safe direction (declaring self-attestation) is always allowed. This hostname comparison is a heuristic, not a security boundary — future versions may use cryptographic identity.

**v0.1 records are unsigned.** Trust depends on registry git access controls. The display requirement compensates: tooling MUST indicate records are unsigned. Signing (SSH keys, likely) is a v0.2 concern.

**Verification contract.** Three normative rules: (1) resolve to exact commit SHA, never branches/tags; (2) unresolvable SHA = hard failure; (3) implementable with standard `git` CLI only. Forge-agnostic — works with any git host.

Full attestation design rationale is in `docs/research/skill-attestation/07-discussion.md`, including prior art analysis, multi-perspective persona debates, and the v0.2/v0.3 Vouch-inspired trust roadmap.

---

## Terminology

The convention proposes standard terms for the ecosystem:

| Term | Definition |
|------|-----------|
| **Agent** | A software product that uses LLMs to help write, review, or maintain code. Ex. Claude Code, Cursor, Copilot. |
| **Model provider** | The company providing the underlying LLM. Ex. Anthropic, OpenAI, Google. |
| **Form factor** | How an agent is delivered: IDE, CLI, extension, web, autonomous, hybrid. |

"Agent" is the public-facing term. Research across 70+ agents shows it's the dominant self-description (16+ tools use "coding agent"), it's the directional term (post-2025 launches almost universally use it), and it matches the ecosystem name.

---

## Graduation Path

Fields in this convention follow a lifecycle:

```
Draft → Stable → Proposed for Core → Agent Skills Spec
```

**Draft → Stable:** Requires multiple implementations and community consensus that the field's semantics are correct.

**Stable → Proposed for Core:** Requires demonstrated adoption across at least **3 agents and 2 registries**. "Demonstrated adoption" means the agent or registry actively parses and acts on the field — not merely tolerates it as opaque metadata. The field is formally proposed as a top-level frontmatter field in the Agent Skills spec.

**Proposed → Core:** The Agent Skills spec maintainers accept the field. During transition, the sidecar field location becomes the canonical source, with any legacy inline `metadata` entries treated as deprecated aliases.

---

## Research Basis

This proposal is informed by:

- **Provenance research** across 20+ specs: OCI, SLSA, npm, Cargo, Helm, SPDX, CycloneDX, Hugging Face, Git, Nix, Homebrew, MLflow, W&B, LangChain Hub, GPT Store, MCP
- **Trigger mechanism research**: VS Code activationEvents, GitHub Actions, Cursor rules, Copilot instructions, Dialogflow intents, and a 650-trial activation rate study
- **Landscape analysis** of 70+ AI coding agents across IDE, CLI, extension, web, and autonomous form factors
- **Content type support matrix** showing adoption of Rules, Skills, Hooks, MCP, Commands, and Agents across the ecosystem
- **Spec evolution research** across 10 patterns: Schema.org, HTTP headers, OpenAPI, Docker labels, JSON-LD, CSS properties, npm, Kubernetes CRDs, W3C process, OpenTelemetry
- **Behavioral data** from a 12-agent study (14 behavioral checks validated against source code and official docs), informing trigger mapping, security tiers, compaction protection, and deduplication behavior

Full research documents are available at:
- `docs/research/2026-03-30-provenance-versioning-research.md` — provenance and versioning across 20+ specs
- `docs/research/provider-agent-naming.md` — 70+ agent landscape with content type support matrix
- `docs/research/skill-behavior-checks/00-behavior-matrix.md` — 14 behavioral checks across 12 agents (validated against source code and official docs)
- `docs/spec/skills/reference/behavior-data-spec-influences.md` — how behavioral data informed convention field design
- `docs/research/skill-attestation/` — attestation design research including gap analysis, multi-perspective persona debates, and trust system prior art (npm provenance, Sigstore, TUF, GitHub Attestations, Vouch)

---

## Changes from Review

Two review rounds (5 simulated personas: skill author, agent developer, registry maintainer, enterprise security, spec pedant) identified issues ranging from interop-breaking ambiguities to security gaps. All consensus fixes (4/5+ agreement) have been applied to the convention spec.

### Consensus fixes applied (5/5 or 4/5 agreement)

| Issue | Personas | Fix | Data |
|---|---|---|---|
| `content_hash` self-referential replacement ambiguous | All 5 | Byte-level regex substitution specified; no YAML re-serialization | Different YAML libraries produce different whitespace/quoting — tested with PyYAML, ruamel, Go yaml.v3, js-yaml |
| `supported_agents.name` freeform string breaks interop | All 5 | Canonical lowercase-hyphenated identifiers (Appendix C, 12 agents) | Even 2 variations (`"Claude Code"` vs `"claude-code"`) fragments search; every package manager uses canonical names |
| `file_patterns` references micromatch (JS library) | 3/5 | Portable glob subset defined inline (`*`, `**`, `?`, `[...]`); brace expansion excluded | Micromatch edge cases differ from Go filepath.Match, Python fnmatch, and gitignore; agents span 5+ languages |
| `content_hash` line endings not normalized | 3/5 | Mandatory CRLF→LF normalization before hashing | `git core.autocrlf=true` (Windows default) converts LF→CRLF on checkout; same repo, different hash |
| `expectations` sub-fields lack REQUIRED/OPTIONAL | 3/5 | `name` REQUIRED, `version`/`url` OPTIONAL; canonical OS values defined | Conformance tests need to know if `{name: "bat"}` (no version) is valid |

### Strong-signal fixes applied (3/5 agreement)

| Issue | Personas | Fix | Data |
|---|---|---|---|
| `~>` version operator underspecified | Agent dev, Spec pedant, Skill author | Full algorithm with edge cases; `~>1` and `~>1.2.3.4` explicitly invalid | Ruby/Bundler-specific; no libraries in Go, Rust, or Python without custom parsing |
| `source_repo`/`publisher`/`derived_from` self-asserted with no trust warning | Security, Spec pedant, Registry | Explicit "self-asserted claim" warnings on all three; attestation section added | Enterprise teams ask "what trust does this confer?" — answer must be "none without attestation" |
| `provenance.version` should be REQUIRED for registries | Registry, Spec pedant, Skill author | Registries MAY reject submissions without version | Without version, registries cannot distinguish two publishes of the same skill |

### Round 3-4 fixes (community feedback + attestation design)

| Issue | Source | Fix |
|---|---|---|
| `convention` is handwavey as a field name | masukomi (community feedback) | Renamed to `metadata_spec` — names the category and artifact type without encoding the value type |
| `license_spdx` doesn't indicate value type | masukomi (community feedback) | Renamed to `license_spdx_id` with docs linking to spdx.org/licenses |
| `file_patterns` / `workspace_contains` ambiguous | masukomi (community feedback) + 5-persona debate | Renamed to `activation_file_globs` / `activation_workspace_globs` — explicit about activation context and pattern type |
| Trigger precedence unspecified when `mode: always` conflicts with globs | masukomi (community feedback) | Full mode × trigger precedence table added with evaluation-order rule and worked example |
| Immutable versions not enforced | Red team persona | Registries MUST reject same version + different hash |
| No attestation mechanism for provenance claims | Multi-perspective analysis (9 agents) | Attestation section added (spec sections 8-9) with `source_verified` claim type |
| Command collision unaddressed | Red team persona | Agents responsible for disambiguation; registries SHOULD flag collisions |

---

## Open Questions for Discussion

### Resolved

4. ~~**Script security**~~ — **Resolved.** `provenance.script_hashes` added as a Draft field. Behavioral data showed 7 of 12 agents auto-load skills with no user consent, making per-file integrity hashes essential for any future trust chain.

6. ~~**Tags and categories**~~ — **Resolved.** `tags` added as a Draft field. Freeform string array for categorization and discovery. No controlled vocabulary — authors choose tags that describe the skill's domain and purpose.

7. ~~**Deprecation lifecycle**~~ — **Resolved.** `status` added as a Draft field with values `active`, `deprecated`, and `archived`. Default is `active`.

8. ~~**Agent name registry**~~ — **Resolved.** Canonical lowercase-hyphenated identifiers defined in Appendix C of the convention spec (12 agents). New identifiers added in convention patches. Format constrained to `[a-z][a-z0-9-]*`.

15. ~~**Signing and verified provenance**~~ — **Partially resolved.** v0.1 introduces unsigned attestation records as the verification foundation. Signing (SSH keys) deferred to v0.2. The attestation format is designed to accommodate signatures without schema changes.

16. ~~**Trigger precedence**~~ — **Resolved.** Full mode × trigger precedence table added with evaluation-order rule: mode is evaluated first, definitive modes skip trigger evaluation.

17. ~~**Field naming**~~ — **Resolved.** `convention` → `metadata_spec`, `license_spdx` → `license_spdx_id`, `file_patterns` → `activation_file_globs`, `workspace_contains` → `activation_workspace_globs`. All from community feedback with 5-persona debate confirmation.

### Still Open

1. **Graduation thresholds** — We propose **3 agents + 2 registries** as the threshold for Stable → Proposed for Core. This is deliberately low to match the ecosystem's current size — thresholds can increase as adoption grows. Does this feel right, or should the bar be higher/lower?

2. **Convention versioning** — The `convention` URL includes a version (`/v0.1`). How should convention versions evolve? Is semver appropriate for a vocabulary document? If v0.2 adds fields but doesn't change v0.1 semantics, can a v0.1-only agent handle v0.2 skills? The current exact-string-match rule means no — a v0.1 agent ignores ALL metadata from a v0.2 skill, even fields it could handle. The consequences of this should be explicitly documented.

5. **Signing** — v0.1 attestation records are unsigned. v0.2 will add SSH key signatures on attestation records (most developers already have SSH keys; GitHub publishes them at `github.com/{user}.keys`). This turns attestation from "trust the registry's git access controls" into "trust the attester's cryptographic identity."

9. **`script_hashes` verification requirement** — The convention spec says script hashes "enable" verification, but includes no MUST statement requiring tools to actually check them. Without normative verification language, the field risks being security theater. Should distribution tools be REQUIRED to verify hashes before executing scripts? The enterprise security persona argues yes; the skill author persona worries about adoption friction. *Data: 7 of 12 agents auto-load skills without consent, making unverified scripts an active attack vector.*

10. **`mode: always` as context injection vector** — A skill with `mode: always` loads into every session unconditionally. Combined with 7 agents that auto-load without consent, a malicious skill in a cloned repo could inject arbitrary instructions into every conversation. Should the convention include a security consideration warning about this? Should agents be RECOMMENDED to restrict `mode: always` to vetted sources? *Data: Behavior matrix Q11 — 7/12 agents auto-load with no approval gate.*

11. **`tags` constraints** — Tags are currently unconstrained freeform strings. A skill with 200 tags could game search rankings. Should we add limits (e.g., max 20 tags, 1-50 chars each, `[a-z0-9][a-z0-9-]*` format)? *Data: Every major registry enforces tag limits — npm caps at 50, Docker Hub at 100, PyPI at 20.*

12. **`durability` default** — The default is `persistent`, meaning every skill silently claims compaction protection. In practice, most skills are ephemeral (fire for a specific task, user moves on). Should the default be `ephemeral` instead (skills that need persistence opt in)? Or should absence mean "agent decides" (current behavior, no new obligation)? *Data: Of 12 agents, only 1 (Claude Code) confirms compaction protection for skills — making `persistent` the default creates an obligation most agents can't meet.*

13. **`triggers.mode: manual` interaction with `activation`** — The spec says "all trigger fields except `commands` are ignored" when mode is manual, but `activation` lives under `triggers`. Is `activation` also ignored? If so, what governs dedup for manual skills? Should the ignored fields be enumerated explicitly? *This is a spec precision issue, not a design question — but the answer affects conformance tests.*

14. **`license_spdx_id` vs top-level `license` conflict** — When both are present, `license_spdx_id` is "authoritative for machine processing." But a skill can have `license: BSD` and `license_spdx: MIT` with no error. Should this be a validation error? The skill author persona notes this creates two fields to maintain for the same datum. *Data: The base Agent Skills spec defines `license` as freeform; the convention can't change that, but should define conflict behavior.*

15. **`expectations` as informational vs machine-verifiable** — The spec says expectations are "runtime dependencies that MUST be present" but provides no mechanism to check them (how do you get the version of `bat`?). Should `expectations` be explicitly informational (no agent verification expected), or should a `check_command` field be added? The agent developer persona argues that without a verification mechanism, this is just structured documentation. *Data: CLI version output formats vary wildly — `bat --version`, `bat -V`, `bat version` all exist across different tools.*

---

## Next Steps

1. **Community review** of this proposal in the Agent Ecosystem group
2. **Reference implementation** in syllago (content package manager) — `syllago verify` for source verification, attestation record generation, install-time attestation display
3. **Behavioral data publication** — release the 12-agent behavior matrix as a community resource
4. **Adoption** by interested agents and registries — target: 3 agents + 2 registries for Draft → Stable graduation
5. **v0.2 planning** — signing, publisher trust, and conversion attestation (see roadmap below)
6. **Graduation** of Stable fields (provenance) into the Agent Skills core spec

---

## Roadmap: v0.2 and v0.3 — Vouch-Inspired Trust Layers

v0.1 establishes **artifact trust**: "are these the right bytes from the right source?" The next versions layer on **people trust** and **ecosystem trust**, inspired by Mitchell Hashimoto's [Vouch](https://github.com/mitchellh/vouch) project — a social trust system using flat files where maintainers vouch for contributors, with a web-of-trust model where projects can import each other's trust decisions.

### v0.2: Signing + Publisher Trust + Conversion

**Signing.** SSH key signatures on attestation records. Most developers already have SSH keys (GitHub publishes them at `github.com/{user}.keys`). This turns attestation from "trust the registry's git access controls" into "trust the attester's cryptographic identity."

**Publisher vouch lists.** A standardized format (inspired by Vouch's `.td` flat files) for registries to publish which publishers they've evaluated and trust. `TRUSTED_PUBLISHERS.td` — one publisher per line, with scope, evidence, and accountability. This separates "I verified the artifact" (attestation) from "I trust the person" (vouching).

Key design decisions from research:
- Vouch propagation is **one-hop, explicit only** — registries must explicitly re-vouch for publishers rather than auto-inheriting trust from other registries
- **Denouncements propagate more aggressively than vouches** — trust is local (requires opt-in), revocation is urgent (safety-critical)
- Publisher trust is **per-publisher, not per-skill** — sustainable at volunteer scale (5 hours/week for a 200-skill registry)
- Three trust tiers: `indexed` (no attestation) → `source-verified` (automated hash check) → `publisher-vouched` (human judgment)

**Conversion attestation.** A `conversion_verified` claim type that references the original `content_hash`, the conversion tool/version, and the source/target formats. This enables trust chains through format conversion (e.g., Claude Code → Cursor) — critical for cross-provider portability.

### v0.3+: Ecosystem Trust

**Cross-registry federation.** Registries can import each other's vouch decisions. Asymmetric propagation: denouncements propagate aggressively, vouches stay local (require explicit re-endorsement by each registry).

**Content safety claims.** A `content_safety_reviewed` claim type with its own trust hierarchy. Different expertise, different review cadence than source verification. Addresses the gap between "verified origin" and "safe to execute."

**Denouncement propagation.** When a publisher is denounced in one registry, subscribing registries receive notification and can confirm, quarantine, or ignore. Not automatic — requires human acknowledgment to prevent denial-of-service via false denouncements.

### The trust layer model

Each layer is independently useful and incrementally deployable:

| Version | Trust Layer | Question Answered |
|---------|------------|-------------------|
| **v0.1** | Artifact trust | Are these the right bytes from the right source? |
| **v0.2** | Publisher trust | Do I trust the person who published them? |
| **v0.3+** | Ecosystem trust | Do I trust the registry that verified them? |

Full attestation design rationale, prior art analysis, and multi-perspective debate transcripts are in `docs/research/skill-attestation/`.
