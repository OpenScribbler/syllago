# v0.1 Shape: Attestation in Spec vs Implementation

**Date:** 2026-03-31
**Status:** Early sketch — not a proposal yet, just exploring what it looks like

---

## What the Spec Would Define

The spec defines three things: the attestation record schema, the trust states, and the claim type vocabulary.

### Attestation Record

A JSON object that says: "attester X verified claim Y about content Z."

```json
{
  "schema": "https://agentskills.io/attestation/v0.1",
  "subject": {
    "content_hash": "sha256:a1b2c3d4...",
    "source_repo": "https://github.com/alice/skills",
    "source_ref": "v1.2.0",
    "source_commit": "abc123def456..."
  },
  "attester": {
    "id": "https://registry.example.com",
    "display_name": "Community Skills Registry",
    "self_attestation": false
  },
  "claims": [
    {
      "type": "source_verified",
      "detail": "content_hash matches source_repo at source_commit"
    }
  ],
  "issued_at": "2026-03-31T12:00:00Z",
  "expires_at": "2027-03-31T12:00:00Z"
}
```

**Key design decisions in the schema:**

- `subject.content_hash` — this is what the attestation is *about*. Hash change = attestation invalid. Non-negotiable.
- `subject.source_ref` — git tag or branch. Human-readable. Optional (HEAD is fine).
- `subject.source_commit` — git commit SHA. Immutable reference. Strongly recommended.
- `attester.id` — URL identifying the attester. Not a username — a dereferenceable identifier. Works for any git forge.
- `attester.self_attestation` — boolean. True when the attester is also the publisher/author. **Honest disclosure, not prohibition.**
- `claims[].type` — extensible vocabulary. v0.1 defines only `source_verified`. Future versions add `script_reviewed`, `conversion_verified`, etc.
- `expires_at` — attestations don't live forever. Forces re-verification.

### Trust States

The spec defines three states for content in a registry index:

| State | Meaning | Install behavior |
|-------|---------|-----------------|
| `active` | No issues known. May or may not have attestations. | Install normally. Show attestation status if available. |
| `quarantined` | Flagged for review. Not yet confirmed safe or malicious. | Refuse install. Show reason. |
| `denounced` | Confirmed problematic. Removed from active index. | Refuse install. Show reason and evidence. |

These are **registry-level states**, not skill-level metadata. A skill doesn't declare itself quarantined — a registry flags it.

### Claim Type Vocabulary

v0.1 defines one claim type:

**`source_verified`** — The attester fetched the content from `source_repo` at `source_commit` and confirmed that the computed content_hash matches `subject.content_hash`.

This claim does NOT mean:
- The content is safe to execute
- The scripts have been reviewed
- The instructions are free of prompt injection
- The publisher is trustworthy

It means: **the bytes you have are the bytes at that repo at that commit.** That's it. Authenticity, not safety.

Future claim types (not in v0.1, but the schema supports them):
- `script_reviewed` — a human reviewed executable files for safety
- `conversion_verified` — a tool verified this is a faithful format conversion of an attested original
- `publisher_vouched` — the attester has evaluated and vouches for this publisher
- `content_safety_reviewed` — a reviewer checked for prompt injection / exfiltration patterns

---

## What syllago Would Implement (Reference Implementation)

### `syllago verify <skill>`

The simplest form. No attestation files needed:

```
$ syllago verify alice/code-review

  Skill:        alice/code-review
  Source:        https://github.com/alice/skills
  Commit:        abc123d (v1.2.0)
  Content hash:  sha256:a1b2c3d4...

  Verification:  PASS
    content_hash matches source_repo at commit abc123d

  Attestations:  none found
```

What happens:
1. Read the skill's `.syllago.yaml` for `source_repo`
2. Clone/fetch the source_repo
3. Checkout the commit or tag referenced in the skill metadata
4. Compute content_hash of the skill directory
5. Compare to the content_hash in the installed skill
6. Report match or mismatch

No attestation infrastructure needed. Works today. This is the skeptic's proposal and it's correct as a starting point.

### `syllago verify --publish`

When a registry operator wants to publish the result as an attestation:

```
$ syllago verify alice/code-review --publish --attester-id https://registry.example.com

  Verification:  PASS
  Attestation written to: .attestations/sha256-a1b2c3d4.json
```

This writes the attestation record (the JSON above) to the registry's attestation directory. The registry commits and pushes it. Now other tools can find it.

### `syllago install` behavior with attestations

When installing a skill, syllago checks for attestations:

```
$ syllago install alice/code-review

  Installing alice/code-review v1.2.0...

  Source verified by:
    Community Skills Registry (https://registry.example.com)
    Verified: 2026-03-31  Expires: 2027-03-31

  Installed successfully.
```

If no attestation exists:

```
$ syllago install bob/quick-hack

  Installing bob/quick-hack...

  ⚠ No attestations found. Source not independently verified.
    Source repo: https://github.com/bob/tools

  Install anyway? [y/N]
```

If quarantined:

```
$ syllago install mallory/sus-tool

  ✗ Cannot install: skill is quarantined
    Reason: Reported by peer registry for suspicious script behavior
    Quarantined: 2026-03-30
    Review status: pending

  This skill is under review and cannot be installed until
  the review is resolved.
```

### Attestation discovery

Where does syllago look for attestations? Two places:

1. **Registry index** — the registry that the skill was found in may include attestation records alongside its index. Convention: `.attestations/{content_hash}.json`

2. **Source repo** — the skill's own source_repo may contain attestations from third parties. Convention: `.syllago/attestations/{content_hash}.json`

Tooling checks both. If multiple attestations exist from different attesters, all are shown.

---

## What This Does NOT Cover (Explicitly Deferred)

- **Signing attestations** — v0.1 attestations are unsigned. They're trust-on-first-fetch from the registry URL. Signing (SSH, Sigstore, etc.) is a v0.2 concern. The schema has room for a `signature` field but doesn't require it.

- **Publisher vouch lists** — TRUSTED_PUBLISHERS.td is deferred. v0.1 relies on GitHub/GitLab account ownership as the implicit identity layer.

- **Cross-registry federation** — registries don't import each other's attestations in v0.1. Users configure which registries they trust manually.

- **Denouncement propagation** — quarantine and denouncement are local to each registry. No cross-registry notification mechanism in v0.1.

- **Conversion attestation** — how attestations survive syllago format conversion is deferred. The `derived_from` field in the metadata convention provides the link; a `conversion_verified` claim type will formalize this later.

---

## Open Questions

1. **Should `source_repo` become REQUIRED in the metadata convention?** The skeptic argues it should be. Without it, there's nothing to verify against. Currently it's optional.

2. **Attestation location convention** — `.attestations/` in the registry? In the source repo? Both? Is there a well-known URL pattern (like `.well-known/attestations/`)?

3. **Expiry duration** — what's the default? 1 year? 6 months? Should the spec recommend a range, or leave it to attesters?

4. **Fail-closed vs fail-open on expiry** — when an attestation expires and no new one exists, should tooling refuse to install or warn-and-allow? The red team analysis argues fail-closed, but the skeptic argues that fail-closed on attestation expiry will break installs for skills whose registries stop maintaining attestations.

5. **Self-attestation flag** — should this be computed by tooling (compare attester.id to source_repo domain) or declared by the attester? If declared, what prevents an attacker from setting `self_attestation: false` on a self-attestation?

6. **Git forge generality** — the verification algorithm needs to work for GitHub, GitLab, Codeberg, self-hosted Gitea, etc. The `source_repo` URL is forge-agnostic, but fetching commit data differs per forge. Is this a tooling concern or does the spec need to define a fetch protocol?
