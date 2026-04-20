## What This Means

Syllago refused to install a content item because its resolved MOAT trust tier is below the caller-configured minimum. MOAT defines three trust tiers (ADR 0007):

- **UNSIGNED** — no manifest, or the item is not attested. Lowest trust.
- **SIGNED** — the registry manifest entry is signed by an identity you pinned, and the signature verifies. Medium trust.
- **DUAL-ATTESTED** — signed at the registry level AND each source artifact carries a matching in-toto attestation whose subject digest matches the manifest's `content_hash`. Highest trust.

When the caller (CLI flag, config policy, or programmatic installer) sets a minimum tier, syllago will refuse any item whose resolved tier falls below it. Unlike MOAT_008, this is not a registry-operator signal — it's your own policy floor rejecting an otherwise valid install.

Tier resolution is defensive: if attestation verification silently fails (G-13 `attestation_hash_mismatch`), syllago downgrades the effective tier rather than promoting unverified content. So an item the registry labels DUAL-ATTESTED can still be treated as SIGNED at gate time, which may fall below your policy floor.

## Common Causes

- You set `--min-tier DUAL_ATTESTED` but the item carries only a registry-level signature, no in-toto attestation on the source artifact.
- You set `--min-tier SIGNED` but the registry has no manifest yet (tier resolved as UNSIGNED).
- An attestation exists but its subject digest does not match the manifest's `content_hash` — G-13 downgrade kicked in and demoted the effective tier.
- Your organization-wide policy file enforces a tier floor that this registry's items do not meet.

## How to Fix

1. Confirm the tier you expected. `syllago registry items <name>` prints each entry's resolved tier.
2. If the registry operator intended DUAL-ATTESTED but you're seeing SIGNED, the attestation pipeline in their publisher is misconfigured — contact them.
3. If you deliberately set a strict floor and want to accept this item anyway, re-run without `--min-tier` (or lower it) — but only for content where you have an out-of-band reason to trust a weaker tier.
4. For production pipelines, prefer fixing the root cause (ask the publisher to dual-attest) over lowering your floor.

## Example Output

```
Error MOAT_009: install refused — trust tier SIGNED is below policy floor DUAL_ATTESTED for syllago-guide
  Suggestion: The registry item does not meet your --min-tier policy. Either lower the policy or ask the publisher to dual-attest their release pipeline.
  Details: MOAT_TIER_BELOW_POLICY: resolved tier=SIGNED, required=DUAL_ATTESTED
```
