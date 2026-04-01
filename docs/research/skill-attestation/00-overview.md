# Skill Attestation & Trust Layers — Research

**Date:** 2026-03-31
**Status:** Active research
**Context:** Exploring Vouch-inspired trust mechanisms for the Agent Ecosystem metadata convention companion spec. Triggered by masukomi's feedback on metadata_convention v0.1 (provenance fields are self-asserted, no verification chain) and Mitchell Hashimoto's [vouch](https://github.com/mitchellh/vouch) project (social trust via flat files for open source contributor management).

---

## The Problem

The metadata convention already has provenance fields: `source_repo`, `publisher`, `content_hash`, `derived_from`, `script_hashes`. These are all **self-asserted claims**. Any skill can claim any publisher, any source repo, any hash. There is no verification chain.

This matters because:
- 7 of 12 surveyed agents auto-load skills without user approval
- Skills can contain executable scripts
- Skills are natural language instructions that an AI agent will follow — prompt injection is a real attack vector
- Content passes through multiple hands: author → publisher → registry → mirror → user

The existing provenance fields give you **integrity** (content_hash proves the bytes haven't changed) but not **authenticity** (who verified those bytes are what they claim to be) or **trust** (should you believe them).

## The Insight from Vouch

Mitchell Hashimoto's vouch project solves contributor trust with radical simplicity: a flat file listing trusted usernames, one per line. No crypto, no PKI, no central authority. Trust is social, explicit, auditable via git history, and propagates through a web where projects can import each other's trust lists.

The key adaptation for skill content: vouch trusts **people**. We also need to trust **artifacts**. The combination — a named person endorsing a specific content-addressed version — is the gap nobody has filled.

## Two Trust Dimensions

| Dimension | Question | Mechanism |
|-----------|----------|-----------|
| **People trust** | "Is this publisher who they say they are? Are they trustworthy?" | Vouch-style trust lists (TRUSTED_PUBLISHERS.td) |
| **Artifact trust** | "Is this specific version of this content verified and endorsed?" | Content-addressed attestations (ATTESTED.td) |

Neither alone is sufficient. Together they answer: "a known, trusted person stood behind these specific bytes."

## Research Files

| File | Contents |
|------|----------|
| [01-prior-art.md](01-prior-art.md) | npm provenance, Sigstore, TUF, GitHub Attestations, Vouch — what exists, what's applicable |
| [02-gap-analysis.md](02-gap-analysis.md) | Five gaps no existing system addresses for AI content |
| [03-people-trust.md](03-people-trust.md) | Publisher identity, vouch propagation, denouncement, role separation |
| [04-artifact-trust.md](04-artifact-trust.md) | Content attestation, hash chains, registry-to-registry trust |
| [05-spec-boundary.md](05-spec-boundary.md) | What belongs in spec vs tooling vs governance |
| [06-v01-proposal.md](06-v01-proposal.md) | Proposed scope for v0.1 of the trust/attestation companion spec |

## v0.1 Scope Thesis

The v0.1 companion spec should focus narrowly on **provenance attestation** — the mechanism by which a registry or trusted party can endorse a specific content_hash. Everything else (people trust lists, denouncement propagation, cross-registry federation, semantic safety review) can layer on top in later versions.

The minimum viable trust chain:
1. Skill has a `content_hash` (already in spec)
2. A trusted party publishes an attestation: "I verified this content_hash came from this source_repo at this commit"
3. Tools can check: does an attestation exist for this hash, from a party I trust?

This is the smallest useful unit. It turns `content_hash` from decoration into a verifiable claim.
