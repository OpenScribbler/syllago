# Multi-Perspective Analysis: Trust System Design Tensions

**Date:** 2026-03-31
**Method:** Five adversarial personas analyzing the same proposed trust system — solo skill author, red team attacker, registry operator, minimalist skeptic, cross-ecosystem portability user.

---

## The Core Tensions

### Tension 1: Build for Current Scale vs Future Interop

**Skeptic:** At 50-200 publishers, social reputation propagates faster than cryptographic trust. `syllago verify` (clone source_repo, compare hashes) is the only thing that changes real behavior. Everything else is premature complexity.

**Spec boundary (from round 1):** If you don't standardize the format now, every tool invents its own. Vouch lists become non-portable. Trust gets siloed per-tool. Retrofitting interop is harder than designing it in.

**Resolution:** Both are right at different time horizons. The skeptic's `syllago verify` should be the v0.1 *implementation*. The spec should define the *format* that `syllago verify` reports its results in, so when other tools exist, they speak the same language. Standardize the vocabulary now; build the infrastructure incrementally.

### Tension 2: Author Friction vs Security Guarantees

**Skill author:** "Zero additional steps for the common case. If I have to run an extra command, some pushes don't happen. If I have to manage keys, I stop publishing."

**Red team:** "Self-attestation is the weakest link. A self-attesting registry is trivially exploitable. If anyone can attest their own content, 'attested' means nothing."

**Resolution:** Registry-side attestation. The author does nothing new — they push to GitHub like today. The registry fetches, verifies hash against source_repo, and publishes the attestation. Authors don't attest; registries do. This eliminates author friction while ensuring attestation comes from a third party.

The tradeoff: this makes registries the trust bottleneck. But the registry operator's analysis shows this is manageable at current scale if attestation is automated (hash verification) and only publisher vouching requires human judgment.

### Tension 3: Per-Skill vs Per-Publisher Trust

**Registry operator:** "Per-skill attestation doesn't scale at 5 hours/week. Publisher vouching is sustainable because publishers are roughly static; skills are not."

**Red team:** "Vouching for a publisher means all their skills inherit trust. If one skill is compromised, the trust delegation covers it."

**Resolution:** Three-tier model from the registry operator:
- `indexed` — in the registry, no attestation
- `source-verified` — automated hash check passes (content matches source_repo)
- `publisher-vouched` — a human has evaluated the publisher (not every skill)

`source-verified` is the high-volume automated tier that catches tampering. `publisher-vouched` is the human judgment tier that's sustainable at volunteer scale. Neither claims the content is *safe* — only that it's *authentic*.

### Tension 4: Content Integrity vs Content Safety

**Red team:** "The most dangerous attacks don't break cryptography. A trusted skill with a subtle prompt injection is more dangerous *after* attestation because users lower their guard."

**Gap analysis (round 1):** "No attestation system distinguishes 'this came from where it claims' from 'this is safe for an AI agent to execute.'"

**Portability agent:** "Different content types have radically different risk profiles. A style rule and an MCP config pointing to an external server should not have the same trust model."

**Resolution:** The spec must be explicit that attestation means **authenticity, not safety**. The trust UI must never display a simple "trusted ✓" — it must show what was actually verified. "Source verified by registry X" is honest. "Trusted" is misleading.

Content type risk stratification is a v0.2 concern, but the attestation schema should be designed with a `claim_type` field from day one, so different claim types (`source_verified`, `script_reviewed`, `content_safety_reviewed`) can be added without schema changes.

### Tension 5: Format Conversion Breaks Everything

**Portability agent:** "syllago's core value is converting content between providers. Conversion changes bytes. Changed bytes invalidate attestations. A trust system that breaks on conversion is useless for a cross-provider tool."

**This is uniquely syllago's problem.** No other trust system deals with format conversion as a first-class operation. The portability agent proposed:

- **Conversion attestation** as a distinct claim type: "syllago attests this Cursor artifact was mechanically derived from this Claude Code artifact that had attestation X"
- **Two-layer trust display**: "Original reviewed ✓" + "Conversion: mechanical, no semantic review"
- `derived_from` chain that propagates denouncement (if original is denounced, converted versions inherit a "source denounced" flag)

This means the attestation schema needs:
- `derived_from` reference (already in the metadata convention)
- `conversion_method` field (which tool, which version, which format pair)
- Clear distinction between "origin attestation" and "conversion attestation"

---

## Consensus Findings (All Five Agree)

1. **Don't make authors do anything new.** Every perspective agrees that author-side signing is a non-starter for adoption. Registry-side verification is the path.

2. **GitHub is the existing trust anchor.** Publisher identity = GitHub account ownership. Content verification = source_repo hash comparison. This is already stronger than self-asserted metadata fields.

3. **"Attested" must never mean "safe."** The distinction between authenticity and safety is critical. The UI must show *what* was verified and *by whom*, not a binary badge.

4. **Self-attestation is the biggest structural weakness.** A registry attesting its own content provides no independent verification. The system must distinguish self-attestation from third-party attestation.

5. **Quarantine is a necessary state.** Not just trusted/denounced — quarantine (flagged, pending review, not installable) is needed for operational reality.

---

## Key Attacks the System Must Address

From the red team analysis, ranked by difficulty × impact:

| Attack                                     | Difficulty         | Impact   | Mitigation                                          |
|--------------------------------------------|--------------------|----------|-----------------------------------------------------|
| Self-attesting registry                    | Trivial            | High     | Display attester identity; flag self-attestation    |
| Patient contributor social engineering     | Low                | High     | No technical fix — social/governance problem        |
| Identity hopping after denouncement        | Low                | Medium   | Cross-registry denouncement aggregation             |
| Prompt injection via trusted skill         | Low (once trusted) | Critical | Out of scope for attestation; agent-side sandboxing |
| Slow drift (incremental malicious changes) | Medium             | High     | Diff-from-originally-attested-version tooling       |
| Attestation staleness + fail-open clients  | Low                | Medium   | Spec must define fail-closed on expiry              |

---

## What This Means for v0.1

### The minimum viable attestation system:

1. **`syllago verify <skill>`** — clone source_repo, compare content_hash. This is the skeptic's proposal and it's the right starting point. No new file formats required.

2. **Attestation record format** — when a registry runs this verification and wants to publish the result, it needs a standardized format. This is where the spec adds value: defining what a machine-readable "I verified this" record looks like, so multiple tools can produce and consume them.

3. **Attester identity** — who published the attestation. Minimum: a URL or GitHub org. This is what prevents self-attestation from looking like independent verification.

4. **Claim types** — start with `source_verified` only. Design the field to be extensible for `script_reviewed`, `content_safety_reviewed`, `conversion_verified` later.

### What to explicitly defer:

- Vouch lists (TRUSTED_PUBLISHERS.td) — useful but not required for the attestation primitive to work
- Cross-registry federation — requires vouch lists first
- Denouncement propagation — requires federation first
- Content safety attestation — different trust hierarchy, different expertise
- Conversion attestation — important for syllago but not for the community spec v0.1

### The uncomfortable truth:

The skeptic is right that at current scale, `syllago verify` + git history + social reputation covers 95% of real threats. But the spec boundary analysis is right that standardizing the attestation format now (even if the only implementation is `syllago verify`) creates the interop foundation that prevents fragmentation later. The cost of defining a format is low. The cost of retrofitting one after multiple tools have invented their own is high.

**v0.1 = define the format, ship one implementation (`syllago verify`), defer everything else.**
