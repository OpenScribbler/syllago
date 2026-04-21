## What This Means

Syllago refused to install a content item because the registry manifest — or a prior install recorded in your lockfile — said the item is revoked. MOAT defines two revocation flavors (ADR 0007 G-8):

- **Registry-source revocation** — the registry operator marked the item revoked in the current manifest. This is a hard block: syllago will not install revoked content, even interactively.
- **Archival revocation (G-15)** — a prior install recorded the item, and your lockfile shows it was later superseded by a revoked manifest entry. Once archived as revoked, that exact `(registry, name, content_hash)` tuple can never be re-installed, even if a later manifest un-flags it.

This is intentional. A registry operator who has issued a revocation is telling you "do not run this content." Syllago honors that without giving the caller a bypass, because the alternative — letting operators opt through revocations interactively — defeats the whole point of a revocation signal.

Note: publisher-source revocation is handled differently (warn-once, operator confirms interactively). That path uses MOAT_004 on refusal, not MOAT_008.

## Common Causes

- The registry operator revoked a release after discovering a vulnerability, supply-chain compromise, or accidental publish of unintended content.
- A content item in your lockfile was previously installed, then revoked upstream, and you are attempting to re-install the same `(name, content_hash)` pair.
- Your local clock is correct but the manifest contains an explicit `revoked: true` flag for this entry.
- You are attempting to install from a registry whose manifest uses a stricter revocation policy than your previous client version did.

## How to Fix

1. Do not bypass this error. Revocations are operational signals from the registry operator.
2. Check whether the registry has published a non-revoked replacement:

   ```
   syllago registry sync <name>
   syllago registry items <name>
   ```

   Look for a superseding entry — often a higher version of the same content name.
3. Install the replacement instead of the revoked item.
4. If you genuinely need the old content and believe the revocation is in error, contact the registry operator. Do not try to work around syllago's refusal — use a different content source.

## Example Output

```
Error MOAT_008: install refused — registry revoked item syllago-guide (source: registry)
  Suggestion: The registry operator marked this item as revoked. Sync the registry and install the superseding entry.
  Details: archival revocation: lockfile recorded install on 2026-04-12, current manifest revokes content_hash sha256:abc123...
```
