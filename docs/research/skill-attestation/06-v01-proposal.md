# Attestation Convention v0.1 — Proposal

**Date:** 2026-03-31
**Status:** Draft
**Companion to:** Agent Skills Metadata Convention v0.1

---

## The Problem

The metadata convention has provenance fields — `source_repo`, `publisher`, `content_hash`, `derived_from` — but they're all **self-asserted**. Any skill can claim any origin. Nothing verifies those claims.

This matters now because 7 of 12 surveyed AI agents auto-load skills without user approval, skills can contain executable scripts, and content passes through multiple hands (author → publisher → registry → user). Without verification, `content_hash` is decoration.

## What This Convention Does

Defines a lightweight mechanism for registries to publish **attestation records** — structured claims that a given `content_hash` was verified against a `source_repo` at a specific git commit.

**Attestation means authenticity, not safety.** An attested skill's origin has been independently verified. It has NOT been reviewed for security, prompt injection, or correctness. The convention is explicit about this distinction throughout.

### Why not something heavier?

We evaluated npm provenance (requires CI/CD), Sigstore (requires OIDC infrastructure), TUF (requires key ceremonies), and GitHub Artifact Attestations (requires GitHub Actions). All assume build pipelines. Skills are hand-authored files pushed to git repos — the entire authoring workflow is a human in an editor. A trust system that requires CI excludes the majority of legitimate skill authors.

We also evaluated Vouch-style social trust lists for publishers. These are valuable but layer on top of attestation — you need a way to verify artifacts before you need a way to vouch for people. Publisher trust is a v0.2 concern.

### Why not just `git clone` + compare hashes?

That's actually the reference implementation (`syllago verify`). The spec adds value by standardizing the *output format* — when a registry performs that verification, the attestation record it produces follows a defined schema that any tool can consume. Without this, every tool invents its own format and trust signals become siloed per-tool.

---

## 1. Provenance

Provenance answers: **where did this content come from?**

Four fields together identify origin:

| Field | Answers | Example |
|-------|---------|---------|
| `source_repo` | Where does it live? | `https://github.com/alice/skills` |
| `source_commit` | Which exact version? | `abc123def456...` (40 hex chars) |
| `content_hash` | Which exact bytes? | `sha256:a1b2c3d4...` |
| `derived_from` | Where did the original come from? | (defined in metadata convention) |

Without attestation, these are self-asserted claims. With attestation, a third party confirms: "the `content_hash` really does match what's at `source_repo` at `source_commit`."

### `source_repo` — REQUIRED for registry submissions

HTTPS URL of the canonical git repository. Registries MUST verify the URL resolves at submission time. OPTIONAL for local-only content.

**Rationale:** Without `source_repo`, there's nothing to verify against. Requiring it only for registry submissions (not local installs) preserves author freedom while enabling verification where it matters — in distribution.

### `source_commit` — strongly recommended (NEW)

Full git commit SHA (40 hex chars). Provides an immutable reference — unlike branch names or tags, SHAs can't be rewritten after the fact.

**Tooling captures this automatically.** Authors don't paste SHAs into YAML. Registries capture the commit at submission time; `syllago publish` captures the current HEAD.

---

## 2. Attestation Record

A JSON object published by an attester claiming to have verified content provenance.

```json
{
  "schema": "https://agentskills.io/attestation/v0.1",
  "subject": {
    "content_hash": "sha256:a1b2c3d4e5f67890...",
    "source_repo": "https://github.com/alice/skills",
    "source_commit": "abc123def456789012345678901234567890abcd"
  },
  "attester": {
    "id": "https://registry.example.com",
    "display_name": "Community Skills Registry"
  },
  "claims": [
    { "type": "source_verified" }
  ],
  "self_attestation": false,
  "issued_at": "2026-03-31T12:00:00Z",
  "expires_at": "2027-03-31T12:00:00Z"
}
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `schema` | Yes | `"https://agentskills.io/attestation/v0.1"` |
| `subject.content_hash` | Yes | SHA-256 of content files. Format: `sha256:{64 hex}` |
| `subject.source_repo` | Yes | HTTPS URL of the source repository |
| `subject.source_commit` | Yes | Full 40-char git commit SHA |
| `attester.id` | Yes | HTTPS URL the attester controls. Used for self-attestation detection. |
| `attester.display_name` | No | Human-readable name |
| `claims` | Yes | Array of claim objects (extensible; unknown types MUST be ignored) |
| `self_attestation` | Yes | Boolean. See section 4. |
| `issued_at` | Yes | ISO 8601 timestamp |
| `expires_at` | Yes | ISO 8601 timestamp. Recommended: 12 months. Range: 6–24 months. |
| `review_policy` | No | URL to the attester's published review policy |

### Display requirement

v0.1 records are unsigned — trust depends on the registry's git access controls. Tooling MUST show *who* attested and SHOULD indicate records are unsigned. Never display a bare "verified" badge without attester context.

**Rationale:** Without this, "source verified by Community Skills Registry" looks identical to a cryptographic guarantee. Users need to understand the trust boundary.

---

## 3. Claim Types

v0.1 defines one claim type. The `claims` array is extensible for future types.

### `source_verified`

The attester fetched content from `source_repo` at `source_commit` and confirmed the computed hash matches `content_hash`.

**Asserts:** The bytes at the declared source match the bytes being distributed.

**Does NOT assert:** Content is safe, scripts are reviewed, content is free of prompt injection, publisher is trustworthy.

#### Renewal

Renewed attestations (same content_hash, new expiry) MUST include `last_reviewed` and `review_type`:

```json
{
  "type": "source_verified",
  "last_reviewed": "2027-03-15T00:00:00Z",
  "review_type": "automated_hash_check"
}
```

`review_type` is self-reported and informational — it cannot be independently verified in v0.1. Known values: `automated_hash_check`, `manual_audit`, `dependency_scan`. Unknown values MUST be accepted.

**Rationale:** Renewal without re-review evidence is meaningless — it's just extending a timestamp. Requiring `review_type` makes renewal honest about what was actually checked, even though the field is unverifiable until signing ships in v0.2.

### Future claim types (v0.2+)

- `script_reviewed` — human reviewed executable files
- `conversion_verified` — tool verified faithful format conversion from an attested original
- `publisher_vouched` — attester evaluated and vouches for the publisher
- `content_safety_reviewed` — reviewer checked for prompt injection / exfiltration

**`conversion_verified` is a v0.2 priority.** Format conversion (e.g., Claude Code → Cursor via syllago) changes bytes, invalidating attestation. v0.2 will define how trust chains survive conversion by referencing the original content_hash, conversion tool/version, and source/target formats.

---

## 4. Self-Attestation

Self-attestation (attester = content author/publisher) is **disclosed, not prohibited**. It provides weaker assurance than third-party attestation.

### Asymmetric enforcement

The dangerous lie is claiming independence when self-attesting. The safe direction is over-reporting.

- If `attester.id` and `source_repo` share the same hostname AND first path segment (user/org), `self_attestation` MUST be `true` regardless of declaration.
- Attester MAY freely declare `true` (safe — over-reporting).
- Attester MUST NOT declare `false` when domains match (dangerous — false independence claim).

**Example:** `attester.id: "https://github.com/alice"` + `source_repo: "https://github.com/alice/skills"` → same host + org → self-attestation.

**Acceptable error:** False positive (independent flagged as self) is acceptable. False negative (self-attester escaping to `false`) is a spec violation.

**Rationale:** A fully self-attesting registry (publishes content, attests its own content, presents it as "independently verified") is the easiest attack vector in the whole system. Asymmetric enforcement catches it without prohibiting legitimate self-attestation.

---

## 5. Trust States

Registry-level metadata (a registry flags content; content does not flag itself):

| State | Meaning |
|-------|---------|
| `active` | No issues known |
| `quarantined` | Flagged for review — not yet confirmed safe or malicious |
| `denounced` | Confirmed problematic |

Tooling SHOULD NOT silently install `denounced` content.

**Why quarantine exists:** Binary trusted/denounced doesn't match operational reality. A registry operator who receives a report needs time to investigate. Quarantine means "stop installing this while I check" without making a permanent judgment.

---

## 6. Verification Contract

### Three normative rules

1. Verification inputs are `source_repo` URL + `source_commit` SHA.
2. Implementations MUST resolve to the **exact commit SHA**. Branch names and tags MUST NOT be accepted.
3. Unresolvable SHA (deleted repo, force-push, private) = **verification failure**, not warning.

**Rationale for rule 3:** Lenient fallback on unresolvable SHAs creates a downgrade attack — an attacker deletes the original repo to force tools into a permissive state.

### Forge generality

The algorithm MUST be implementable using only standard `git` CLI operations. Works with GitHub, GitLab, Codeberg, Gitea, self-hosted — any forge. Forge-specific API calls are a tooling optimization, not a requirement.

---

## 7. Expiry

`expires_at` is REQUIRED. After expiry, the attestation provides no trust signal.

- Default: 12 months
- Range: 6–24 months (attester's choice based on review policy)

When content changes, `content_hash` changes, and all attestations for the old hash are automatically invalid. Re-attestation required.

If `source_repo` is unreachable at install time, cached attestations SHOULD NOT be relied upon regardless of expiry.

---

## 8. File Location

**Registry-side (canonical):** `.attestations/{content_hash}.json` in the registry repo, where the colon in `sha256:...` is replaced with a hyphen.

**Source-side (optional breadcrumb):** `.syllago/attestations/{content_hash}.json` in the author's repo. Informational only — not a trust anchor.

**Lifecycle:** Active-index attestations MUST be preserved. Superseded versions MAY be archived or deleted after 90 days.

**Portability:** When content moves between registries, the receiving registry verifies independently. `derived_from` in the metadata provides the link; the new registry issues its own attestation.

---

## 9. What Attestation Does NOT Provide

1. **Not safety.** Verified provenance from a malicious repo is still malicious.
2. **Not script review.** `content_hash` covers integrity, not safety.
3. **Not prompt injection prevention.** Natural language attacks are invisible to hash verification.
4. **Not conversion-proof.** Format conversion changes bytes, invalidating attestation. (Addressed in v0.2.)
5. **Not a substitute for judgment.** Attestation is one data point. The trust decision remains with the consumer.

---

## What's Explicitly Deferred

| Feature | Why deferred | Target |
|---------|-------------|--------|
| Cryptographic signing | Adds infrastructure complexity; unsigned records are still better than no records | v0.2 |
| Publisher vouch lists | Layers on top of attestation; need the artifact layer first | v0.2 |
| Cross-registry federation | Requires vouch lists | v0.3+ |
| Conversion attestation | Needs implementation experience with format conversion patterns | v0.2 |
| Content safety claims | Different trust hierarchy, different expertise, different review cadence | v0.3+ |

---

## Reference Implementation

syllago provides the reference implementation:

```bash
syllago verify alice/code-review              # verify against source_repo
syllago verify alice/code-review --publish \
  --attester-id https://registry.example.com  # publish attestation record
syllago install alice/code-review             # install with attestation display
```

---

## Appendix: Implementation Guidance (Informative)

### Expiry behavior

Recommended tiered escalation for expired attestations:

| Expiry Age | Suggestion |
|-----------|------------|
| 0–30 days | Subtle indicator. Install proceeds. |
| 31–90 days | Confirmation prompt before install. |
| 90+ days | Explicit acknowledgment required. |

Configurable policy: `default` (tiered), `strict` (fail-closed for enterprise), `permissive` (show status, never block).

### Registry maintenance status

Registries MAY publish `maintenance_status` (`active` / `maintenance-mode` / `archived`) to contextualize expiry.
