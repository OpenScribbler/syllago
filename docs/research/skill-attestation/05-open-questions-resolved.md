# Open Questions: Multi-Persona Debate Results

**Date:** 2026-03-31
**Method:** 5 adversarial personas (solo author, red team, registry operator, minimalist skeptic, cross-ecosystem portability user) debated each question for 3 rounds, responding to each other's arguments.

---

## Q1: Should `source_repo` become REQUIRED?

**Consensus: Required for registry submissions, optional for local use.**

The portability user's scoping was the key insight: registry submissions and personal use are different threat models. The skeptic's sequencing constraint was the most actionable: don't require the field until a reachability validator ships. The requirement and the check must ship together.

| Decision | Detail |
|----------|--------|
| Required for | New registry submissions |
| Optional for | Local installs, personal use |
| Prerequisite | Automated reachability check must exist before requirement is enforced |
| Transition | 90-day deprecation window for existing entries without source_repo |
| Validation | At minimum: URL resolves, repo exists. At submission time AND periodically. |

---

## Q2: Attestation Location Convention

**Consensus: Registry-side canonical, source-side optional breadcrumb.**

The solo author evolved from "source-side only" to recognizing that self-authored attestation files are claims, not attestations. The red team correctly separated content hash portability from trust portability. The group converged on a "request vs approval" model: source-side is the audit breadcrumb, registry-side is the countersigned attestation.

| Decision | Detail |
|----------|--------|
| Canonical location | `.attestations/{content_hash}.json` in the registry repo |
| Optional breadcrumb | `.syllago/attestations/{content_hash}.json` in the source repo |
| Well-known URLs | Deferred — no server infrastructure commitment required |
| Portability | Carry hash + prior attestation *pointers* across registries, not attestation records |
| Immutability | Registry-side attestations are append-only (new versions = new hashes, old attestations preserved) |
| Cross-reference | Registry-side record MAY include `source_attestation_url` pointing to source-side breadcrumb |

---

## Q3: Expiry Duration

**Consensus: 12-month default, 6–24 month range, renewal requires re-review evidence.**

The red team moved from "6 months mandatory" after the skeptic argued that duration is a proxy — the real variable is re-review quality. The group agreed that silent re-signing (renewing without re-checking) is worse than a longer honest cycle.

| Decision | Detail |
|----------|--------|
| Spec default | 12 months |
| Allowed range | 6–24 months, attester's choice |
| Attester requirement | Must publish documented review policy |
| Renewal requirement | Must include `last_reviewed` date and `review_type` (e.g., `automated_hash_check`, `manual_audit`, `dependency_scan`) |
| Key insight | Expiry duration is the wrong lever. Attestation content quality at renewal time is the real variable. |

---

## Q4: Fail-Closed vs Fail-Open on Expiry

**Consensus: Tiered warn-and-allow as default, fail-closed as configurable policy.**

The skeptic proposed an escalation ladder that satisfied everyone. The red team moved from "fail-closed mandatory" after recognizing that binary blocks train users to find workarounds. The registry operator's SLA concern was addressed by the 0–30 day grace window.

| Expiry Age | Behavior | Rationale |
|-----------|----------|-----------|
| 0–30 days | Subtle indicator, install proceeds | Administrative lag, not adversarial |
| 31–90 days | Interstitial confirmation required | Getting stale, user should be aware |
| 90+ days | Explicit "I understand" block screen | Likely abandoned, user must acknowledge risk |

| Decision | Detail |
|----------|--------|
| Default | Tiered warn-and-allow (ladder above) |
| Config option | `attestation.policy: strict` for fail-closed in enterprise/workspace contexts |
| Registry metadata | `maintenance_status` field (active / maintenance-mode / archived) contextualizes expiry |
| Format conversion | Attestation expiry clock inherits through syllago conversions, does not reset |

---

## Q5: Self-Attestation Flag — Computed vs Declared

**Consensus: Asymmetric enforcement — computed in the dangerous direction only.**

The red team identified the asymmetric attack surface: lying "I'm independent" (false) is dangerous, lying "I'm self-attesting" (true) is benign. The group converged on enforcing only the dangerous direction.

| Decision | Detail |
|----------|--------|
| Rule | If `attester.id` domain matches `source_repo` domain → `self_attestation` MUST be `true` regardless of declaration |
| Declaration | Attester can freely declare `self_attestation: true` (safe direction) |
| Override | Attester cannot declare `self_attestation: false` when domains match (dangerous direction) |
| Attester ID format | Must be a typed URL/identifier (not freeform string) for deterministic domain comparison |
| Spec requirement | Normative comparison algorithm with canonical test vectors |
| Error tolerance | False positive (independent flagged as self) = acceptable. False negative (self-attester escapes to false) = spec violation. |

---

## Q6: Git Forge Generality

**Consensus: Three normative rules, everything else is tooling.**

The red team and solo author converged quickly: the spec anchors on commit SHA, not forge-specific APIs. The registry operator's strongest constraint was "implementable with standard `git` CLI only."

**Three normative rules:**
1. Verification inputs are `source_repo` URL + `commit_sha`
2. Tools MUST resolve to that exact commit SHA (never a branch name or tag as substitute)
3. Unresolvable SHA = verification failure (hard failure, not warning)

| Decision | Detail |
|----------|--------|
| Forge-specific APIs | Not in spec. Tooling concern. |
| Informative appendix | MAY document known forge patterns (GitHub, GitLab, Codeberg, Gitea) as non-normative guidance |
| Converted skills | New format = new attestation with new commit SHA. Don't amend original. |
| Implementation bar | Must be implementable using only standard `git` CLI operations |

---

## Summary: How These Answers Shape v0.1

The six questions produced a consistent design philosophy:

1. **Anchor on what's verifiable** — commit SHAs, not branch names. Content hashes, not skill names. Domain comparison, not declared flags.

2. **Tiered, not binary** — trust states (active/quarantined/denounced), expiry behavior (ladder, not cliff), self-attestation (disclosed, not prohibited).

3. **Spec defines the what, tooling defines the how** — three normative rules for verification, no forge-specific APIs. Attestation format standardized, policy enforcement delegated.

4. **Honest about what's certified** — renewal requires re-review evidence. Self-attestation is disclosed. "Attested" never means "safe."

5. **Volunteer-sustainable** — 12-month default, no SLA obligations, tiered expiry that doesn't create outages from administrative lag.
