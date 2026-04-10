# Gap Analysis: What No Existing System Solves for AI Content

**Date:** 2026-03-31
**Sources:** npm provenance docs, Sigstore/cosign docs, TUF spec, GitHub Artifact Attestations docs, mitchellh/vouch, SLSA spec, arXiv prompt injection research

---

## Gap 1: Version-Level Endorsement

**The problem:** Vouch trusts people. Hashes trust bytes. Nothing trusts "this version of this skill, right now."

Every social trust system (vouch, PGP web of trust, GitHub stars) endorses *people*. Cryptographic systems endorse *specific byte sequences*. Neither provides: "a named person with reputation endorsing a specific content-addressed version, with that endorsement becoming invalid when the content changes."

**Why it matters for skills:** A trusted publisher's endorsement of their skill at v1.0 should not silently cover v1.1. If content changes, the endorsement must be explicitly renewed. Without this, a compromised update inherits the trust of the previous version.

**What exists today:** Nothing combines social identity + content address + expiration in a single endorsement primitive.

---

## Gap 2: Semantic Content Safety vs Provenance

**The problem:** A signed, provenance-attested skill can still contain prompt injection.

Provenance tells you it came from the expected repo. A hash tells you it hasn't been tampered with in transit. Neither tells you whether the natural language instructions contain:
- Indirect prompt injection payloads
- Subtle exfiltration instructions
- Context manipulation patterns
- Instructions that appear normal to human reviewers but cause the AI agent to escalate privileges

**The signature-semantic gap:** npm packages contain compiled code that static analysis can scan. AI skills are English text interpreted by an LLM. The "code" is in natural language, the "runtime" is non-deterministic, and the attack surface is the model's tendency to follow embedded instructions.

**What exists today:** No attestation system distinguishes "this came from where it claims" from "this is safe for an AI agent to execute." This would require a new category of attestation with its own trust hierarchy (who is qualified to review AI content for safety) and its own refresh model.

**v0.1 implication:** Out of scope for v0.1, but the attestation format should be extensible enough that a `script_reviewed` or `content_safety_reviewed` claim type can be added later.

---

## Gap 3: Trust for Hand-Authored, Non-CI Content

**The problem:** Every production trust system assumes a CI/CD pipeline.

npm provenance requires a trusted CI runner's OIDC identity. GitHub Artifact Attestations require GitHub Actions. Sigstore keyless signing assumes a CI workflow context. Hand-published content either gets no provenance or a weaker "Level 0" that conveys no meaningful assurance.

AI skills are overwhelmingly hand-authored. A developer writes a SKILL.md in their editor and pushes to a git repo. There is no build step, no CI pipeline, no artifact registry. The entire authoring workflow is a human in an editor.

**What this means:** A trust system that requires CI to produce attestations will exclude the majority of legitimate skill authors from its trust model. The system must work for `git push` workflows, not just `npm publish --provenance` workflows.

**Possible approaches:**
- SSH signing of content hashes (keys already exist, GitHub publishes them at `github.com/{user}.keys`)
- Sigstore interactive keyless flow (OAuth login, sign blob, done) — technically works but UX is container-centric
- Registry-side verification: the registry fetches from source_repo and attests the match, rather than requiring the author to sign

**v0.1 implication:** The attestation model should not assume CI. Registry-side attestation ("we verified this hash matches what's at source_repo") is the most practical starting point.

---

## Gap 4: Mutable Artifact Trust Chains

**The problem:** Trust systems assume publish-once immutable artifacts. Skills update in-place.

An npm package at version 1.2.3 is expected to never change. A SKILL.md file lives at a stable path in a git repo and changes over time. The trust questions for mutable content are fundamentally different:

- Is *this version* trusted?
- Was the previous version trusted, and is this a legitimate update?
- Who authorized this change?
- Are there outstanding endorsements for the old content that should be invalidated?

TUF handles some of this for update systems (snapshot + timestamp metadata), but TUF assumes a registry operator, not distributed flat-file git repos.

**What this means for content_hash:** When a skill is updated, the content_hash changes. All existing attestations are invalidated. The skill appears unattested until re-attestation. This is **correct behavior** — but it creates friction. Re-attestation workflows must be fast enough that legitimate maintainers aren't bottlenecked.

**v0.1 implication:** Attestations must bind to content_hash (not skill name or path). Content_hash change = attestation invalidation. This is a feature, not a bug, but the spec must make it explicit.

---

## Gap 5: Cross-Context Trust Portability

**The problem:** A skill endorsed in one agent ecosystem carries zero trust signal in another.

syllago's goal is cross-provider portability: a skill authored for Claude Code and imported into Windsurf carries its content but not its trust provenance. There is no standard for expressing "this content was reviewed and endorsed by trusted party X in context Y, and that endorsement is considered valid in context Z."

SLSA provenance is portable across ecosystems in principle, but no ecosystem tooling consumes attestations from another ecosystem's trust roots.

**What this means:** If the attestation format is defined in the Agent Skills metadata convention (not a provider-specific spec), attestations are inherently portable across agents. This is a strong argument for putting the attestation format in the community spec rather than in syllago.

**v0.1 implication:** The attestation format must be agent-neutral. It references content_hash and source_repo, not any provider-specific concept.

---

## Summary: What v0.1 Must Get Right

| Gap | v0.1 Action | Later Version |
|-----|-------------|---------------|
| Version-level endorsement | Attestations bind to content_hash, not identity | Add expiration semantics |
| Semantic safety | Extensible claim types (leave room for `content_safety_reviewed`) | Define safety review attestation type |
| Hand-authored content | Don't require CI; support registry-side attestation | Add interactive signing workflow |
| Mutable artifacts | Hash change = attestation invalidation (explicit in spec) | Add re-attestation workflow guidance |
| Cross-context portability | Agent-neutral attestation format | Cross-registry trust federation |
