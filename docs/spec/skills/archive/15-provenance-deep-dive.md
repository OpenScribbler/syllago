# Provenance Deep Dive: 6-Persona Adversarial Review

**Date:** 2026-04-01
**Focus:** Provenance section of metadata_convention.md
**Framing:** Community-level provenance, not syllago-specific

---

## JORDAN — Spec Maintainer (15 years W3C/IETF)

### Field Set Assessment

**Missing: `published_at` timestamp.** Every provenance system needs temporal ordering independent of git. When registries import from multiple sources, "when did this version first appear?" has no answer. SLSA, SPDX, and CycloneDX all include timestamps. Add `provenance.published_at` as ISO 8601, populated by tooling.

**Underspecified: `publisher`.** Freeform string means two registries can assign same publisher string to different entities. Should be either a URI (matching `attester.id`) or explicitly registry-scoped (`{registry_id}/{publisher_name}`).

**The rest is solid.** Don't add more fields. "The temptation in provenance specs is to keep adding fields until you have SPDX's 130-field monster. Resist that."

### Content Hash: Three Breaking Edge Cases

**1. Binary files + CRLF normalization.** PNG containing `0x0D 0x0A` silently corrupted in hash. **Recommendation: Drop CRLF normalization entirely.** Mandate LF in published skills via `.gitattributes`. If source uses CRLF and consumer uses LF, hashes SHOULD differ — that's a real portability issue.

**2. Blanking step is fragile.** What if file contains the pattern as an example? What about leading spaces, single quotes? **Specify the exact regex.** Propose: `^(\s*content_hash:\s*)"sha256:[0-9a-f]{64}"` replaced with `$1""`. Tooling MUST generate exact format `content_hash: "sha256:..."` (double quotes, no trailing space).

**3. Symlinks undefined.** What when symlink target is within skill directory? **Resolve symlinks if target is within directory, exclude if external.**

### Derived-From Assessment

Right vocabulary, wrong enforcement model. Parallel to FDA DSCSA: transaction history travels with the product, each handler adds their record, each handler VERIFIES the previous handler's claims.

**Add `adapt` as relation type.** `convert` = mechanical transformation. `adapt` = rewritten for different domain. Art provenance distinguishes "copy" from "after."

**State forward contract now:** `derived_from` becomes verifiable via `conversion_verified` attestation in v0.2. Document this so implementers don't treat it as permanently unverifiable.

### Immutable Versions: Enforceable Within, Not Across Registries

Registry A publishes `cool-skill@1.0.0` hash X. Registry B independently publishes same name, hash Y. No detection mechanism.

**Make `content_hash` the canonical cross-registry identifier, not name+version.** This is how OCI works — digest is the true identifier, tag is convenience.

### Concrete Recommendations

1. Add `provenance.published_at` (ISO 8601, tooling-populated)
2. Make `publisher` a URI or define scoping convention
3. **Drop CRLF normalization; mandate LF in published skills**
4. Specify exact regex for content_hash blanking
5. Define symlink resolution behavior
6. Add `adapt` to derived_from vocabulary
7. Document forward contract: derived_from becomes verifiable via attestation
8. Make content_hash the canonical cross-registry identifier
9. Adopt SLSA-style trust levels for communication
10. **Define denouncement record format in v0.1** even without federation

---

## DR. PRIYA ANAND — Security Researcher

### Complete Attack Surface

1. **Registry compromise** — unsigned JSON, anyone with write access forges records
2. **Self-attestation domain evasion** — `github.com/alice-skills-registry` attesting `github.com/alice-skills` evades heuristic (different first path segment)
3. **Attestation record substitution** — predictable paths, no integrity protection on records themselves
4. **Phantom publisher attack** — `publisher: "google-security-team"` with no prevention
5. **Derived-from laundering** — claiming derivation from reputable upstream for implied trust
6. **Slow drift after attestation** — incremental changes that individually look harmless, auto-re-attestation misses content-level attacks

### Content Hash Security

- SHA-256 collision resistance is not the concern
- **TOCTOU between hash computation and consumption** — files can change between verification and agent loading. Hash protects transit, not runtime.
- **Blanking step ambiguity** — SKILL.md body could contain hash pattern as example. Clarify blanking operates on sidecar only.
- **Symlink exclusion as attack vector** — symlink points outside directory, hash doesn't cover it, agent executes it

### source_commit Weaker Than Claimed

- Force-push rewrites history, GitHub GCs unreferenced objects after 90 days
- Repository transfers change org context, break self-attestation heuristic
- Repository deletion: attestation exists but unverifiable

### Concrete Security Improvements

1. **Transparency log for attestation records** — append-only log (even simple protected git repo) makes forgery detectable after-the-fact
2. **Sigstore over SSH for v0.2 signing** — identity-bound ephemeral signing without key management. SSH creates revocation problem. "I believe Sigstore wins on UX."
3. **Normative `script_hashes` verification** — MUST verify before execution, MUST reject on mismatch
4. **Hash pinning in `derived_from`** — registry SHOULD verify upstream content_hash actually existed
5. **Attestation record integrity via content-addressing** — name files by their own content hash, not subject's hash
6. **Cross-registry denouncement feed from v0.1** — simple JSON feed of denounced hashes, don't wait for v0.3 federation
7. **Blanking step clarification** — operates on sidecar only, never on SKILL.md body

---

## MARCUS CHEN — OSS Implementer (50K+ stars)

### Content Hash Implementation Reality

8-step algorithm = 8 divergence points across 33+ tools.

**Where implementers WILL get it wrong:**
- **Unicode normalization** — macOS NFD vs Linux NFC for filenames. `é` composed vs decomposed = different hashes from same checkout.
- **Binary file detection** — CRLF normalization destroys binary files
- **Empty directories** — git doesn't track them, some tools create them

### What He'd Demand

**Non-negotiable: reference implementation + test vectors.**
1. Standalone CLI binary: `provenance-hash ./directory` prints hash
2. Test corpus: 20+ directories covering unicode, symlinks, empty files, binary, mixed line endings
3. Compliance test suite: `provenance-test my-tool-hash-command`

"Without this, you'll get 33 implementations that agree on simple cases and diverge on everything else."

### Fields He Actually Uses vs Parse-and-Ignore

| Field | Action |
|-------|--------|
| version | Display |
| source_repo | Render as hyperlink |
| source_commit | **Ignore — never displayed, never verified** |
| content_hash | Verify at install |
| authors | Display |
| publisher | Display |
| license_spdx_id | Display + filter |
| script_hashes | Verify at install |
| derived_from | "Based on:" display line |
| source_repo_subdirectory | **Ignore** |
| license_url | **Ignore** — SPDX ID sufficient |

"4 fields I actively use, 4 I display, 3 I parse and throw away."

### Versioning Reality

"Parse and display, don't compare. The moment you add version resolution, you need a SAT solver and you've reinvented npm."

### The ISRC Parallel

ISRC (International Standard Recording Code) is a **dumb identifier** — 12 characters, no embedded semantics, nothing to recompute. Every tool reads the same code.

**Concrete proposal: Add an optional `id` field** — registry-assigned opaque string. Make content_hash a verification mechanism, not an identity mechanism. "Identities are assigned; integrity is computed. Conflating them is the fundamental design tension."

### Implementation Recommendations

1. Reference implementation as standalone binary + test vectors
2. Be honest about which fields are "for humans" vs "for machines"
3. Add registry-assigned `id` field — separate identity from integrity
4. `source_repo` is a display string, not machine-actionable provenance

---

## SARAH PARK — VP Engineering, Fortune 500

### The Three Compliance Questions

1. "Can you prove this skill is the same one we approved?" (Integrity) — **content_hash answers this**
2. "Can you trace who touched it and when?" (Chain of custody) — **partially answered, missing approval layer**
3. "Can you kill it everywhere in under four hours?" (Incident response) — **missing deployment tracking**

### What's Missing for Enterprise

**1. The Approval Gap (biggest concern):**

Provenance tracks WHERE content came from, not WHO approved it for this environment. Need environment-scoped approvals:

```yaml
approvals:
  - approved_by: "alice@acme.com"  # must integrate with enterprise IdP
    approved_for: "trading-desk/production"  # scope
    approved_at: "2026-04-01T09:00:00Z"
    expires_at: "2026-07-01T09:00:00Z"  # forces re-review
    policy_ref: "https://wiki.acme.com/skill-approval-policy"
```

Should be **separate layer** from content provenance — enterprise-local, shouldn't leak to community registries. Suggest `.syllago-approvals.yaml` or registry-level ledger.

**2. Cross-format provenance:**

When skill converts from canonical to Cursor format, SHA-256 changes. Add `conversion_source_hash` — records canonical hash before conversion. Auditable chain across format boundaries.

**3. Deployment tracking for incident response:**

Provenance tracks where content CAME FROM but not where it WENT TO. Define standard format for deployment receipts that enterprise registries can optionally collect.

### Patterns from Regulated Industries

**Financial messaging (ISO 20022):** Every transaction carries unique end-to-end ID surviving transformation. Skills need immutable `content_id` distinct from hash.

**Pharmaceutical (DSCSA):** Each handler adds signed record — append-only chain of custody. Author publishes (record 1), registry mirrors (record 2), approval clears (record 3), developer installs (record 4).

**Defense (AS6081):** Counterfeit avoidance requires testing samples, not just inspecting paperwork. **Runtime attestation** — periodic re-verification that installed skill still matches provenance.

### What Gets CISO Sign-Off

Four additions: mandatory hash verification at runtime (not just install), enterprise-local approval layer with IdP integration, deployment receipts for incident response, expiring approvals forcing periodic re-review.

---

## KAI NAKAMURA — AI-Native Futurist

### Agent-Generated Skills: The `origin` Field

```yaml
provenance:
  origin:
    type: "agent_generated"      # | "human_authored" | "hybrid"
    agent_id: "claude-code/4.0"
    session_context:
      workspace: "acme/deploy-service"
      timestamp: "2026-04-01T14:32:00Z"
    human_principal: "alice@acme.com"  # who was in the session
```

"Agent-generated skills still have a human principal. The agent is the tool, the human is the author of record."

### Expanded Derivation Vocabulary

| Relation | Meaning |
|----------|---------|
| `summarize` | Compressed while preserving intent |
| `translate` | Natural language translation |
| `specialize` | Narrowed from general to domain-specific |
| `generalize` | Broadened from specific to general |
| `compose` | Combined multiple skills preserving parent boundaries |

Plus `transformation_description` field — short note about what changed and why.

### Accretive Provenance (Not Static Snapshots)

Provenance is a living record. Define `provenance_history` array capturing state transitions:

```yaml
provenance_history:
  - event: "created"
    timestamp: "2026-04-01T14:32:00Z"
    context: "agent_session"
  - event: "promoted"
    timestamp: "2026-04-02T09:00:00Z"
    context: "committed to acme/skills at abc123"
  - event: "attested"
    timestamp: "2026-04-03T12:00:00Z"
    context: "source_verified by community-registry"
```

### Machine Attestation: Testing as Claim Type

```json
{
  "type": "functional_verified",
  "test_agent": "claude-code/4.0",
  "test_suite": "sha256:...",
  "pass_rate": "14/14"
}
```

Inherently weaker than `source_verified` — test suites can be incomplete, adversarial content can pass tests.

### Content-Addressed Identity

**Semantic fingerprinting:** Secondary hash over normalized content (stripped of whitespace, comments, formatting). Two skills with different `content_hash` but identical `semantic_hash` are formatting variants.

### Provenance Fingerprint in SKILL.md

```yaml
---
name: code-review
description: Reviews code for quality and security.
metadata_file: ./SKILL.meta.yaml
provenance_fingerprint: "a1b2c3d4e5f6"
---
```

12 characters. One line. Survives copy-paste, agent forwarding, sidecar loss. Reconnects orphaned content to lineage via registry lookup.

### Unified Contributors Model

```yaml
contributors:
  - type: "human"
    identity: "Alice Smith <alice@example.com>"
    role: "author"
  - type: "agent"
    identity: "claude-code/4.0"
    role: "generator"
    principal: "alice@example.com"
```

Backward-compatible: `authors` becomes convenience alias filtering to `type: human, role: author`.

---

## AISHA WILLIAMS — Daily Skill Author

### What She'd Actually Fill Out

**Always:** `version`, `license_spdx_id`, `authors` — already tracks informally

**If tooling does it:** `source_repo`, `source_commit`, `content_hash`, `script_hashes` — zero effort

**Honestly skip:** `publisher` (redundant), `license_url` (SPDX sufficient), `derived_from` (unless captured at fork time)

### Semver Is Wrong for Skills

"What's a 'breaking change' to a markdown file?"

**What she actually does:** Integer versions. `v3`, `v4`. "Am I on the latest?" is the only question.

**Recommendation:** Default to integer versions. Let people opt into semver for complex skills with script interfaces.

### Derived-From Only Works If Captured At Fork Time

"If `syllago fork registry://community/code-review` auto-populates derived_from, I'll use it. If I have to remember and type it later, it'll be empty in 95% of skills."

Recipe parallel: "Food bloggers write 'adapted from Smitten Kitchen' because they had that tab open. They don't reconstruct sources two weeks later."

### Content Hash: Publish-Time, Not Authoring-Time

"If I have to run `syllago rehash` after every edit, I will forget."

Right model: hashes computed at publish/export time only. While editing, no hash. `syllago publish` computes fresh. Hash reflects published artifact, not working copy.

### The Core Principle

**"Provenance should be captured at the moment of action, computed from existing context, and required only at the point of distribution."**

### Concrete Authoring Improvements

1. `syllago init` generates everything derivable (author from git, repo from remote, license from LICENSE file)
2. `syllago fork` captures derived_from at fork time
3. Hashes are publish-time artifacts, not authoring-time
4. `syllago bump` increments version — no semver decisions
5. **Tiered provenance requirements** — local requires nothing, team requires version+authors, public requires license+source_repo
6. `syllago status` shows incomplete provenance for current sharing scope

---

## CROSS-PANEL CONSENSUS ON PROVENANCE

### Universal Agreement (6/6)

- **Content hash algorithm is solid foundation** but needs test vectors and reference implementation
- **Tooling must generate provenance** — hand-authoring kills adoption
- **Provenance value scales with distribution radius** — local needs nothing, public needs everything

### Strong Agreement (4-5/6)

- **Drop CRLF normalization, mandate LF** (Jordan, Priya, Marcus agree; cleaner and avoids binary file corruption)
- **Add `published_at` timestamp** (Jordan, Sarah, Kai, Aisha all identify the need)
- **Separate identity from integrity** — registry-assigned `id` (Marcus), `provenance_fingerprint` (Kai), end-to-end `content_id` (Sarah) all point the same direction
- **Denouncement feed from v0.1** — don't wait for federation (Jordan, Priya)

### The Split

- **Semver vs integer versions** — Aisha says semver is wrong for markdown files; Jordan and Sarah want semver for enterprise tooling
- **How much future-proofing** — Kai wants `origin`, `contributors`, `provenance_history`; Marcus says "seven fields, ship it"
- **Signing timeline** — Jordan and Priya say ship with v0.1 or don't ship attestation; Sarah and Aisha say ship v0.1 fast, signing in v0.2

### Actionable Improvements (Prioritized)

**Tier 1: Must-have for v0.1**
1. Reference implementation + test vectors for content_hash (33+ tools must agree)
2. Drop CRLF normalization, mandate LF
3. Specify exact regex for hash blanking step
4. Define symlink resolution behavior
5. Rename "attestation" to "provenance checks" until signing ships
6. Normative MUST for script_hash verification before execution
7. Define denouncement record format (even without federation)

**Tier 2: High-value additions for v0.1**
8. Add `published_at` timestamp
9. Add `provenance_fingerprint` (12-char hash) in SKILL.md frontmatter for orphan reconnection
10. Make `publisher` a URI or define scoping convention
11. Add `adapt` to derived_from vocabulary
12. Tiered provenance requirements matching distribution radius

**Tier 3: v0.2 candidates**
13. Registry-assigned `id` field (separate identity from integrity)
14. `origin` field for agent-generated skills
15. `contributors` model replacing `authors` (human + agent)
16. Sigstore evaluation alongside SSH signing
17. Enterprise approval layer (separate sidecar)
18. Cross-format `conversion_source_hash`
19. SLSA-style trust level communication

**Tier 4: v0.3+ vision**
20. Accretive provenance_history
21. Semantic fingerprinting
22. Machine attestation (functional_verified claim type)
23. Deployment tracking / receipts
24. Cross-registry denouncement federation
