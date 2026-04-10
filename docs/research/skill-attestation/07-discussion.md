# Attestation Convention v0.1 — Discussion Document

**Date:** 2026-03-31
**Companion to:** Agent Skills Metadata Convention v0.1
**Context:** Research and design rationale behind the v0.1 convention changes, informed by multi-perspective analysis across security, usability, operations, portability, and minimalism lenses. This document covers both the attestation mechanism (sections 8-9 of the spec) and the naming/structural changes informed by community feedback.

---

## Why Attestation, Why Now

The metadata convention defines provenance fields — `source_repo`, `content_hash`, `publisher`, `derived_from`. But every field is self-asserted. Any skill can claim any origin, and nothing verifies those claims.

This is not a theoretical concern. Of 12 surveyed AI coding agents, 7 auto-load skills from cloned repositories without user approval. Skills can contain executable scripts. Skills are natural language instructions that an AI agent with filesystem access will follow. The gap between "content claims to be from X" and "content was actually verified against X" is where supply chain attacks live.

The attestation mechanism fills that gap with the simplest useful primitive: a registry fetches a skill from its declared `source_repo` at its declared `source_commit`, computes the hash, confirms it matches `content_hash`, and publishes a structured record of that verification.

## Why Not Something Heavier

We evaluated five existing trust systems:

| System | What it does well | Why it doesn't fit |
|--------|------------------|--------------------|
| **npm provenance** | Binds packages to CI builds via Sigstore | Requires CI/CD. Skills are hand-authored — no build pipeline. |
| **Sigstore / cosign** | Keyless signing via OIDC identity | Container-centric UX. Puts author email in a permanent public transparency log. |
| **TUF** | Key rotation, threshold signatures, expiring metadata | Requires key ceremonies and multiple signing roles. Overkill for volunteer registries. |
| **GitHub Attestations** | Low-friction signing for GitHub Actions workflows | Requires GitHub Actions. Doesn't work for hand-published content. |
| **Vouch** | Social trust via flat files, no crypto | Vouches for people, not artifacts. Valuable but layers on top of attestation. |

Every production trust system assumes a CI/CD pipeline. The entire skill authoring workflow is a human in an editor running `git push`. A system that requires CI to produce attestations excludes the majority of legitimate skill authors.

### Why not just `git clone` + compare hashes?

That's actually the reference implementation (`syllago verify`). The spec adds value by standardizing the *output format* — when a registry performs that verification, the attestation record it produces follows a defined schema that any tool can consume. Without this, every tool invents its own format and trust signals become siloed per-tool. At current ecosystem scale (~50-200 publishers), social reputation propagates faster than cryptographic trust. But standardizing the vocabulary now prevents fragmentation when multiple tools exist.

## Why Registry-Side Verification

The central design decision: **authors do nothing new; registries attest.**

An author pushes to GitHub (or GitLab, Codeberg, self-hosted Gitea). The registry fetches, hashes, compares, and publishes the attestation. Zero author friction.

The alternative — author-side signing — was rejected for practical reasons: solo developers with 12 skills in a hobby repo will not manage signing keys, learn `ssh-keygen -Y sign`, or re-sign after every typo fix. We confirmed this through persona analysis: any additional step in the publish workflow causes some authors to stop publishing or to bypass the trust system entirely, which is worse than no trust system.

The tradeoff is that registries become the trust bottleneck. But registry-side `source_verified` attestation is automatable (clone repo, checkout commit, hash content, compare) and scales to hundreds of skills without human review. Only publisher vouching requires human judgment — and that's a v0.2 concern.

---

## Naming Decisions

Community feedback (from masukomi, via [gist](https://gist.github.com/masukomi/38628c90e19a79d9d6feeb8d3500cfa0)) identified several naming problems. We resolved each through multi-persona debate (solo author, spec purist, community member, implementer, newcomer).

### `convention` → `metadata_spec`

**Problem:** "Convention" is a vague noun that could mean anything — a programming convention, a conference, a social norm. Reading `convention: "https://..."` doesn't communicate that this is a versioned specification identifier.

**Decision:** Rename to `metadata_spec`. Unanimous across five personas. It names the category (metadata), names the artifact type (spec), doesn't encode the value type (survives URI-vs-alias evolution), and is unambiguous on first read.

**Rejected alternatives:** `schema_version` (wrong — the value is a URI, not a version string), `convention_uri` (over-constrains the value type if short-form aliases are added later), `spec` (too minimal — ambiguous alongside `version` field).

### `license_spdx` → `license_spdx_id`

**Problem:** The name doesn't indicate whether the value is a URL, an ID, or an expression. masukomi suggested `license_spdx_id` to make it explicit.

**Decision:** Rename to `license_spdx_id` with docs linking to https://spdx.org/licenses/. Straightforward clarity improvement.

### `file_patterns` / `workspace_contains` → `activation_file_globs` / `activation_workspace_globs`

**Problem:** `file_patterns` doesn't specify the pattern type (glob vs regex vs gitignore syntax). `workspace_contains` reads like a boolean check, not a glob list.

**Decision:** Rename to `activation_file_globs` and `activation_workspace_globs`. Won 3-2 in persona debate. The `activation_` prefix carries semantic weight when fields appear outside their structural context (error messages, tooling output, documentation generators). The `_globs` suffix explicitly declares the pattern type.

**Rejected alternatives:** masukomi's proposed `use_on_files_matching_glob` / `use_when_workspace_contains_files_matching_glob` (too long to write from memory), `file_globs` / `workspace_globs` (relies on `triggers:` section heading for context — fields surfaced in isolation lose meaning).

**Added:** The spec now explicitly states patterns use "standard glob syntax (not regex, not gitignore syntax)" to close the remaining ambiguity.

---

## Key Attestation Design Decisions

### Attestation means authenticity, not safety

The most important boundary in the convention. A `source_verified` attestation proves the bytes came from where they claim. It does NOT prove those bytes are safe. A skill with verified provenance from a malicious repository is still malicious.

We enforce this through explicit "does NOT assert" language and a display requirement: tooling MUST show who attested and MUST NOT display bare "verified" badges. The goal is to prevent users from equating "attested" with "reviewed for safety."

### `source_repo` required for registry submissions

Without `source_repo`, there's nothing to verify against. Making it required only for registry submissions (not local installs) preserves author freedom while enabling verification where it matters.

A reachability check fires at submission time — the URL must resolve. Key design constraint: don't require a field until the validator exists. Otherwise the requirement is theater.

### `source_commit` auto-populated by tooling

Authors should not paste SHA strings into YAML. Registries capture the commit SHA at submission time; distribution tools capture the current HEAD automatically. This was a strong consensus from the solo author persona: any manual step in the authoring workflow reduces adoption.

### Immutable versions

Registries MUST reject submissions where `version` matches an existing entry but `content_hash` differs. This prevents a supply chain attack where an attacker republishes at the same version with different (malicious) content. Identified by the red team persona as a real attack vector — without immutability, the `version` field is meaningless for integrity.

### Exact commit SHA, never branches or tags

`source_commit` anchors attestation to an immutable point in git history. Branch names and tags can be rewritten after the fact. The spec requires implementations to resolve to the exact SHA — never a branch HEAD, never a tag.

Unresolvable SHAs (deleted repo, force-push) are treated as verification failure, not a warning. Lenient fallback creates a downgrade attack: an attacker deletes the original repo to force tools into a permissive state.

### Self-attestation: disclosed, not prohibited

A self-attesting registry (publishes content, attests its own content, presents it as "independently verified") is the easiest attack in the system. But prohibiting self-attestation would block legitimate use cases.

The solution is asymmetric enforcement: if the attester's domain matches the source_repo's domain, `self_attestation` MUST be `true` regardless of declaration. This catches the dangerous lie (claiming false independence) without prohibiting the safe direction.

**Important caveat:** The hostname comparison is a heuristic disclosure mechanism, not a security boundary. It catches the common case (same GitHub org) but cannot detect all self-attestation scenarios (e.g., an attester using a different org on the same forge). Future versions may use cryptographic identity for stronger binding.

### Unsigned records in v0.1

v0.1 attestation records are unsigned JSON files committed to a registry's git repository. Their trustworthiness depends on the registry's git access controls. This is a conscious tradeoff: unsigned records with clear attester identity are still better than no records, and they don't require signing infrastructure.

The display requirement compensates: tooling MUST indicate records are unsigned. This prevents users from treating them as cryptographic guarantees.

### Expiry with renewal evidence

Attestations expire (default 12 months, range 6–24). After expiry, the attestation provides no trust signal. Renewal requires `last_reviewed` and `review_type` — it's not a silent re-signing.

`review_type` is self-reported and unverifiable in v0.1. This is a known limitation, documented in the spec.

**Tiered expiry behavior:** The spec moved client behavior guidance to an informative appendix rather than normative requirements. The recommended tiered escalation (0-30 days: subtle indicator; 31-90 days: confirmation; 90+ days: explicit acknowledgment) is implementation guidance for tooling, not a spec mandate. This was a key decision from persona debate — the spec defines the `expires_at` field semantics; what clients do with expiry is their business. A `strict` policy option (fail-closed) is recommended for enterprise deployments.

### Trust states: active, quarantined, denounced

Binary trusted/denounced doesn't match operational reality. A volunteer registry operator who receives a report needs time to investigate. Quarantine means "stop installing this while I check" without making a permanent judgment. It's a registry-level flag, not skill-level metadata.

### Trigger precedence

masukomi raised: what happens when `mode: always` is set but globs don't match? The spec now includes a full precedence table (mode × trigger type) and an evaluation-order rule: "Mode is evaluated first. If mode resolves to a definitive state, trigger evaluation is skipped."

This table also forced us to fully specify `auto` mode × trigger interaction, which was previously underspecified.

### Command collision

The `commands` trigger field has no namespacing. If two skills register the same command name, agents are responsible for disambiguation. The spec notes this and recommends registries flag collisions. This was identified as a potential skill-squatting attack vector by the red team persona.

### Forge generality

The verification algorithm works with any git hosting service — GitHub, GitLab, Codeberg, self-hosted Gitea. The spec requires only standard `git` CLI operations. Forge-specific API calls are explicitly tooling optimizations, not requirements.

---

## Open Questions We Resolved

Six design questions were debated across three rounds by five adversarial personas. Summary of outcomes:

| Question | Resolution |
|----------|-----------|
| `source_repo` required? | Yes, for registry submissions. Ships with a reachability validator. |
| Attestation location? | Registry-side canonical (`.attestations/{hash}.json`), source-side optional breadcrumb. |
| Expiry duration? | 12-month default, 6–24 range. Renewal requires re-review evidence. |
| Fail-closed vs fail-open on expiry? | Tiered escalation as informative guidance. `strict` mode available for enterprise. |
| Self-attestation detection? | Asymmetric enforcement — computed in dangerous direction only. Heuristic, not security boundary. |
| Git forge generality? | Three normative rules (exact SHA, hard failure on unresolvable). Everything else is tooling. |

Full debate transcripts and analysis are in the research directory (`03-multi-perspective-analysis.md`, `05-open-questions-resolved.md`).

---

## What We Explicitly Deferred

| Deferred | Why | Target |
|----------|-----|--------|
| **Cryptographic signing** | Adds infrastructure complexity. Unsigned records still improve on zero records. SSH signing is the likely mechanism. | v0.2 |
| **Publisher vouch lists** | Social trust for people layers on top of artifact attestation. Need the artifact layer first. Inspired by Mitchell Hashimoto's Vouch project. | v0.2 |
| **Cross-registry federation** | Requires standardized vouch lists to propagate trust decisions. | v0.3+ |
| **Denouncement propagation** | Requires federation. Denouncements are registry-local in v0.1. | v0.3+ |
| **Conversion attestation** | Format conversion (e.g., Claude Code → Cursor) changes bytes, invalidating attestation. Need implementation experience before specifying. | v0.2 |
| **Content safety review** | Different trust hierarchy, different expertise, different review cadence than source verification. | v0.3+ |

---

## The Path to v0.2 and v0.3: Vouch-Inspired Trust

v0.1 establishes artifact trust: "are these the right bytes?" The next layers add people trust and ecosystem trust, inspired by Mitchell Hashimoto's [Vouch](https://github.com/mitchellh/vouch) project.

### v0.2: Signing + Publisher Trust + Conversion

**Signing:** SSH key signatures on attestation records. Most developers already have SSH keys (GitHub publishes them at `github.com/{user}.keys`). This turns attestation from "trust the registry's git access controls" into "trust the attester's cryptographic identity."

**Publisher vouch lists:** A standardized format (inspired by Vouch's `.td` flat files) for registries to publish which publishers they've evaluated. `TRUSTED_PUBLISHERS.td` — one publisher per line, with scope and evidence. This separates "I verified the artifact" (attestation) from "I trust the person" (vouching).

Key design decisions from research:
- Vouch propagation is one-hop, explicit only — registries must explicitly re-vouch rather than auto-inheriting
- Denouncements propagate more aggressively than vouches (asymmetric: trust is local, revocation is urgent)
- Publisher trust is per-publisher, not per-skill — sustainable at volunteer scale (5 hours/week)
- Three trust tiers: indexed → source-verified (automated) → publisher-vouched (human judgment)

**Conversion attestation:** A `conversion_verified` claim type that references the original `content_hash`, the conversion tool/version, and the source/target formats. This enables trust chains through format conversion — critical for cross-provider tools like syllago. Conversion creates a new attestation (distinct claim type), not an amendment to the original.

### v0.3+: Ecosystem Trust

**Cross-registry federation:** Registries can import each other's vouch decisions. Asymmetric propagation: denouncements propagate aggressively (safety-critical), vouches stay local (require explicit re-endorsement).

**Content safety claims:** A `content_safety_reviewed` claim type with its own trust hierarchy. Who is qualified to review AI content for prompt injection? How often does that review need refreshing? Different expertise, different cadence than source verification.

**Denouncement propagation:** When a publisher is denounced in one registry, subscribing registries receive notification and can confirm/quarantine/ignore. Not automatic — requires human acknowledgment to prevent denial-of-service via false denouncements.

### The trust layer model

The goal across v0.1–v0.3 is a lightweight web of trust where each layer is independently useful and incrementally deployable:

| Version | Trust Layer | Question Answered |
|---------|------------|-------------------|
| v0.1 | Artifact trust | "Are these the right bytes?" |
| v0.2 | Publisher trust | "Do I trust the person who published them?" |
| v0.3+ | Ecosystem trust | "Do I trust the registry that verified them?" |

---

## Key Attacks the System Must Address

From red team analysis, ranked by difficulty × impact:

| Attack | Difficulty | Impact | Mitigation |
|--------|-----------|--------|------------|
| Self-attesting registry | Trivial | High | Display attester identity; flag self-attestation (v0.1) |
| Patient contributor social engineering | Low | High | No technical fix — social/governance problem (v0.2 vouch lists help) |
| Identity hopping after denouncement | Low | Medium | Cross-registry denouncement aggregation (v0.3) |
| Prompt injection via trusted skill | Low (once trusted) | Critical | Out of scope for attestation; agent-side sandboxing needed |
| Slow drift (incremental malicious changes) | Medium | High | Diff-from-originally-attested-version tooling |
| Attestation staleness + fail-open clients | Low | Medium | Tiered expiry guidance; `strict` policy for enterprise |
| Version republish with different content | Low | High | Immutable versions — same version + different hash = rejection (v0.1) |

---

## Research Materials

Full research is in the `docs/research/skill-attestation/` directory:

| File | Contents |
|------|----------|
| `00-overview.md` | Problem statement and trust dimensions |
| `02-gap-analysis.md` | Five gaps no existing system addresses for AI content |
| `03-multi-perspective-analysis.md` | Nine-agent synthesis — consensus findings and design tensions |
| `04-v01-shape.md` | Early spec vs implementation sketch (superseded by final spec) |
| `05-open-questions-resolved.md` | Six questions debated and resolved across five personas |
