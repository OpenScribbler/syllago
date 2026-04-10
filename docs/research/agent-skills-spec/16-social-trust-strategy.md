# Social-First Trust Strategy for Agent Skills

**Date:** 2026-04-01
**Status:** Draft — under adversarial review
**Context:** Builds on research files 01-15, vouch research, web-of-trust history, viral adoption patterns

## Core Thesis

The Agent Skills ecosystem has a trust crisis (36.82% of skills flawed, 76 confirmed malicious payloads, zero spec-level security). Previous approaches to solving this (signing, capabilities fields, security harnesses) are technically correct but have zero viral adoption potential. The proposal: **split trust into two layers — a human-readable social layer that drives adoption, and a technical layer that provides actual security guarantees underneath.**

## The Two-Layer Model

### Layer 1: Social Trust (What Humans See)

Adapted from mitchellh/vouch's model. People vouch for skills they trust. People denounce skills (or authors) that are harmful. Trust is expressed in human language, attached to human identities.

**What users see:**
```
$ syllago install community/prompt-engineering

  prompt-engineering v2.1.0
  by @swyx
  Vouched by: @ThePrimeagen, @fireship, +9 others
  "Best prompt skill I've found" — @ThePrimeagen

  Install? [Y/n]
```

**What authors see:**
```
$ syllago vouch community/prompt-engineering --reason "I use this daily"
  ✓ Vouch recorded for community/prompt-engineering
  Your GitHub identity: @alice (verified)
```

**What the community sees:**
```
# VOUCHED.td (in a registry or collection repo)
github:swyx
github:ThePrimeagen
github:fireship
-github:slopmaster3000  Published skills with obfuscated shell commands
```

**Key properties:**
- Vouching costs social capital (reputation risk)
- Denouncement propagates faster than trust (shared blocklists across registries)
- One command to vouch, one command to denounce
- Reasons are quotable (testimonial content, viral potential)
- GitHub identity is the anchor — no new accounts

### Layer 2: Technical Trust (What Machines Verify)

Invisible to users, provides actual security guarantees:

- **Content hashes** — SHA-256 of skill contents, verified at install time
- **Identity binding** — GitHub OIDC ties vouches to verified accounts
- **Behavioral scanning** — flag suspicious patterns (shell commands, network requests, obfuscated content)
- **Transparency log** — append-only record of all vouches, denouncements, and publishes
- **Script hash verification** — mandatory check before executing any skill scripts

## Why This Split (Historical Evidence)

| System | Person + Artifact merged? | Outcome |
|--------|--------------------------|---------|
| PGP | Yes (keys = identity = integrity) | Both failed — catastrophic UX |
| Keybase | Person only (social proof) | Identity went viral, no artifact story |
| Sigstore | Artifact only (ephemeral signing) | Technical success, zero virality |
| Let's Encrypt | Both split (auto identity + auto certs) | Massive adoption — UX hid everything |
| npm provenance | Both split (OIDC identity + Sigstore signing) | Working, mild adoption pressure via badges |

**The pattern:** Systems that merge person-trust and artifact-trust into one mechanism fail because the UX of cryptographic operations kills adoption. Systems that split them can optimize each layer independently — make the human layer intuitive and viral, make the technical layer invisible and correct.

## Adoption Mechanics

### The Viral Hook

"Alice vouched for this" is tweetable. "Verified via ephemeral OIDC-bound Ed25519 signature" is not.

The vouch IS the marketing:
- Registry listings show voucher names and reasons
- README badges: `![Vouched by 12 devs](syllago.dev/badge/...)`
- Absence is a signal: "0 vouches" feels alarming, like HTTP without the padlock
- Influencer vouches are prominently displayed and linkable

### Cold Start Bootstrap

Piggyback on existing trust (how every successful system bootstrapped):
- **Day 1:** GitHub identity signals (account age, followers, org membership, contribution history)
- **Week 1:** Behavioral scanning provides artifact trust before any vouches exist
- **Month 1:** Early adopters vouch for skills they use. Vouch counts become visible.
- **Month 3:** "Developers you follow also use this" becomes available
- **Month 6:** Org-level trust policies for enterprise ("trust skills vouched by 3+ team members")

### Denouncement Network

Vouch's killer feature for security: shared blocklists.

1. Registry A discovers malicious skill by `github:badactor`
2. Registry A denounces `github:badactor` with reason
3. Registry B (which subscribes to A's denouncement feed) auto-blocks
4. Denouncement propagates faster than the malware

This maps to supply chain security best practice: "Every system (diamonds, aviation, pharma) propagates revocation faster than endorsement."

## What This Replaces in the Previous Strategy

The earlier adversarial panel (files 14-15) recommended an 8-field sidecar convention focused on technical metadata (content_hash, version, source_repo, authors, mode, file_globs, commands, metadata_spec). That work remains valid as Layer 2 plumbing.

What changes:
- **The adoption driver** shifts from "better metadata for tools" to "social trust that humans spread"
- **The entry point** shifts from "tool authors implement sidecar parsing" to "developers vouch for skills they like"
- **The security story** shifts from "signed attestation records" to "community-driven trust + automated scanning + denouncement networks"

## Open Questions

1. How does vouch-of-skill differ from vouch-of-person? (Vouch trusts people; we need to trust artifacts)
2. What prevents vouch manipulation (sock puppets, bought vouches)?
3. How do vouches interact with skill versions? (Vouch for v1.0 — does it carry to v2.0?)
4. What's the governance of denouncement? (Who can denounce? What's the appeal process?)
5. How does enterprise trust (org policies) layer on top of community trust?
6. What happens when a vouched author's account is compromised?
7. Does this create a popularity contest that marginalizes good but obscure skills?
8. Is the GitHub identity dependency acceptable? (Not everyone uses GitHub)
