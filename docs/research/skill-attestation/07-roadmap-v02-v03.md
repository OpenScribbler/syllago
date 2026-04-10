# Attestation Convention Roadmap: v0.2 and v0.3

**Date:** 2026-03-31
**Status:** Planning — builds on the v0.1 proposal (06-v01-proposal.md)
**Prerequisite:** v0.1 ships attestation records, `source_verified` claims, trust states, and `syllago verify`

---

## How the Layers Stack

The attestation convention creates a three-layer trust architecture. v0.1 covers Layer 2. Future versions build upward and downward.

```
Layer 3: PEOPLE TRUST (v0.2)
  "Which registries/attesters do I trust?"
  "Which publishers are vouched for?"
  "Who's been denounced?"
         ↓ informs
Layer 2: ATTESTATION (v0.1 — shipping)
  "A trusted party verified this content_hash"
  "Here's who attested, when, and what they checked"
         ↓ verifies
Layer 1: PROVENANCE (metadata convention — exists)
  "source_repo, source_commit, content_hash, derived_from"
  (self-asserted claims)
```

Without Layer 2, Layer 1 is just claims nobody checked. Without Layer 3, Layer 2 is verification by parties nobody evaluated. Each layer adds value independently, but the full chain — provenance → attestation → people trust — is the end goal.

---

## v0.2: Signing + People Trust

### What v0.1 leaves unresolved

v0.1 attestation records are unsigned JSON files. Their trustworthiness depends on the registry's git access controls. Users manually decide which registries to trust. There's no structured way to share or discover trust decisions.

### v0.2 scope

**Cryptographic signing of attestation records**

Attestation records gain a `signature` field. Signing provides non-repudiation (the attester can't deny having attested) and tamper evidence (a modified record fails verification).

Approach TBD — SSH signing is the lightest weight option (keys already exist, GitHub publishes them). Sigstore keyless is more robust but has higher adoption friction. The v0.1 schema has room for a `signature` field without breaking existing records.

**`TRUSTED_PUBLISHERS.td` format**

A flat-file format for registries to publish their trust decisions about publishers. Vouch-inspired — one entry per publisher, human-readable, git-native.

Each registry maintains its own file. It answers: "who is allowed to publish to this registry, and who vouched for them?"

Shape (from research):
```
# Vouched publishers
alice
  github: alice
  vouched-by: registry-maintainer-bob
  since: 2026-01-15

# Denounced publishers
-mallory
  reason: distributed hook with credential exfiltration
  evidence: https://github.com/registry/incidents/1
  denounced-by: registry-maintainer-bob
  since: 2026-02-15
```

Key design decisions from the multi-persona debates:
- Trust targets the **publisher role** (controls distributed bytes), not just the author role
- Denouncements require **evidence links** — no evidence = not actionable
- Vouching is **per-registry, not transitive** — Registry A vouching for alice does not auto-vouch her at Registry B
- New publishers bootstrap by **introducing themselves** (issue/PR on the registry) — the social mechanism, not a technical one

**`TRUSTED_REGISTRIES.td` format**

A flat-file format for users/organizations to declare which registries' attestations they accept. This is the bridge between the people layer and the attestation layer.

Shape:
```
# Which registries' attestations do I accept?
community-skills
  url: https://github.com/community/skills-registry
  accept-attestations: yes
  accept-denouncements: yes

alice-personal
  url: https://github.com/alice/skills
  accept-attestations: no
  accept-denouncements: no
```

Key design decisions:
- **Denouncements propagate more aggressively than vouches** — `accept-denouncements: yes` can be on even when `accept-attestations: no`
- Each registry/user sets their own policy — no central authority
- The file lives in the user's syllago config or in an organization's shared config

**`publisher_vouched` claim type**

A new claim type for attestation records: "the attester has evaluated and vouches for this publisher." Distinct from `source_verified` — this is about the person, not the artifact.

### What v0.2 changes about v0.1

Nothing breaks. v0.2 is additive:
- Existing unsigned attestations remain valid
- The `TRUSTED_PUBLISHERS.td` and `TRUSTED_REGISTRIES.td` files are new, not replacements
- The `publisher_vouched` claim type is a new entry in the vocabulary
- Signing is optional — unsigned records still work, signed ones are stronger

---

## v0.3: Federation + Conversion Attestation

### What v0.2 leaves unresolved

Denouncements are local to each registry. Format conversion (syllago's core use case) breaks attestation chains. There's no cross-registry notification mechanism.

### v0.3 scope

**Cross-registry denouncement propagation**

When a registry denounces a publisher, registries that subscribe to its denouncement list receive a notification. Not auto-propagation — each registry must explicitly confirm the denouncement before applying it locally.

Key design decisions from research:
- **Quarantine first, denounce after review** — incoming denouncements from peer registries trigger quarantine (not auto-denouncement)
- **Severity tiers** — distinguish "published a buggy hook" from "actively malicious"
- **Grace period on cascading revocations** — if a registry is itself denounced, its vouches aren't immediately revoked; they're flagged as "vouching registry under review"
- **Notification mechanism TBD** — feed/webhook/manual check. Must work for volunteer-operated registries.

**`conversion_verified` claim type**

The attestation chain for format-converted content. When syllago converts a skill from Claude Code format to Cursor format:

1. The content bytes change (different frontmatter, different conventions)
2. The original attestation is invalidated (different content_hash)
3. A new attestation with `conversion_verified` links back to the original

Shape:
```json
{
  "type": "conversion_verified",
  "detail": "mechanical conversion from claude-code to cursor format",
  "original_content_hash": "sha256:abc123...",
  "original_attestation_ref": "sha256-abc123.json",
  "converter": "syllago@1.4.2",
  "source_format": "claude-code",
  "target_format": "cursor"
}
```

Key design decisions from the portability debate:
- Conversion attestation is a **different claim** than origin attestation — "this is a faithful translation" vs "this came from where it claims"
- Trust display should show both layers: "Original reviewed ✓ by Registry X" + "Conversion: mechanical by syllago"
- Denouncement of the original propagates to converted versions (flagged as "source denounced")
- Attestation expiry **inherits** through conversion — doesn't reset

**`script_reviewed` claim type**

A human reviewed executable files for safety. High-burden claim — few attesters will make it, but meaningful when present. Requires defining what "reviewed" means (checklist? free-form? linked to a review report?).

**`content_safety_reviewed` claim type**

A reviewer checked for prompt injection and exfiltration patterns. This is the hardest claim type because it requires domain expertise in AI agent security. Likely requires a separate trust hierarchy (who is qualified to make this claim?).

---

## What Each Version Enables

| Capability | v0.1 | v0.2 | v0.3 |
|-----------|------|------|------|
| "Did this content come from where it claims?" | ✓ | ✓ | ✓ |
| "Who verified it?" | ✓ (unsigned) | ✓ (signed) | ✓ (signed) |
| "Do I trust the verifier?" | Manual | Structured (TRUSTED_REGISTRIES.td) | Structured + federated |
| "Is the publisher trustworthy?" | Manual | Structured (TRUSTED_PUBLISHERS.td) | Structured + cross-registry |
| "Is this conversion faithful?" | No | No | ✓ (conversion_verified) |
| "Has someone been denounced?" | Local only | Local only | Cross-registry propagation |
| "Are the scripts safe?" | No | No | ✓ (script_reviewed) |
| "Is the content safe for AI agents?" | No | No | Partial (content_safety_reviewed) |

---

## The End-to-End Flow (v0.3)

When all three versions are shipped, the full trust chain for a user installing a skill:

1. **Provenance** (Layer 1) — skill metadata declares source_repo, source_commit, content_hash
2. **Attestation** (Layer 2) — a registry has a signed `source_verified` record for that content_hash
3. **Attester trust** (Layer 3) — the user's `TRUSTED_REGISTRIES.td` includes that registry
4. **Publisher trust** (Layer 3) — the registry's `TRUSTED_PUBLISHERS.td` includes the publisher
5. **No denouncements** — neither the publisher nor the registry appears in any denouncement list the user subscribes to
6. **Display** — "Source verified by Community Registry (signed, trusted). Publisher alice (vouched by bob, Jan 2026). No denouncements."

If any link in the chain breaks, the user sees exactly where and why. Provenance is the data. Attestation is the verification. People trust is the judgment. Each layer is independently useful but together they form a complete picture.

---

## Open Research Questions for Future Versions

These emerged from the multi-persona debates and aren't resolved yet:

1. **Content type risk stratification** — hooks (shell commands) and MCP configs (external servers) have fundamentally different risk profiles than rules (text injection). Should different content types require different attestation levels?

2. **Content_hash normalization across providers** — if two providers normalize whitespace differently, the "same" content produces different hashes. This must be addressed before `conversion_verified` ships.

3. **Legal liability for volunteer registry operators** — denouncement carries reputational and potentially legal risk. Should the spec provide standard indemnification language?

4. **Self-hosted forge identity** — the domain comparison algorithm for self-attestation is GitHub-centric. Test vectors are needed for GitLab, Codeberg, and self-hosted Gitea instances.

5. **Semantic safety attestation** — who is qualified to claim a skill is safe for AI agent execution? This is the hardest unsolved problem and may require a separate working group.
