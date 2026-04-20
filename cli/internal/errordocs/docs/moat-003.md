## What This Means

Syllago fetched a MOAT registry's manifest and signing bundle, but the certificate on that bundle did not match the `signing_profile` you pinned for this registry. The most common cause is that the pinned `repository_id` or `repository_owner_id` does not match the numeric IDs baked into the GitHub OIDC certificate — exactly the repo-transfer scenario ADR 0007 exists to catch.

Syllago refuses to proceed on any mismatch because the whole point of pinning the numeric IDs is to make repo-rename / repo-transfer attacks impossible: the SAN subject string can be re-registered by an attacker who takes over the owner/repo name, but the numeric IDs cannot.

## Common Causes

- The pinned profile is stale — the owner/repo was legitimately transferred and never re-approved.
- An attacker captured the owner/repo name after the legitimate publisher released it, and is now signing with a different numeric repo ID.
- You pinned the wrong numeric IDs when running `registry add --signing-repository-id / --signing-repository-owner-id`.
- The registry publisher rotated to a different workflow subject or issuer without updating its profile.

## How to Fix

1. Confirm the current, legitimate owner of the registry — use an out-of-band channel (project homepage, existing contributor, prior CI logs).
2. Re-derive the numeric IDs from the canonical source:

   ```
   gh api repos/OWNER/REPO --jq '.id, .owner.id'
   ```

3. Remove the existing registry, then re-add it with the refreshed profile:

   ```
   syllago registry remove OWNER/REPO
   syllago registry add https://github.com/OWNER/REPO.git \
     --signing-identity "https://github.com/OWNER/REPO/.github/workflows/moat.yml@refs/heads/main" \
     --signing-repository-id <NEW_ID> \
     --signing-repository-owner-id <NEW_OWNER_ID>
   ```

If you cannot verify the legitimacy of the transfer out-of-band, assume compromise: leave the registry uninstalled until you can.

## Example Output

```
Error MOAT_003: manifest cert does not match pinned profile for registry OpenScribbler/syllago-meta-registry
  Suggestion: Re-verify the registry's signing identity out-of-band, then re-add with refreshed --signing-repository-id / --signing-repository-owner-id.
  Details: MOAT_IDENTITY_MISMATCH: repository_id mismatch: got="9999999" want="1193220959"
```
