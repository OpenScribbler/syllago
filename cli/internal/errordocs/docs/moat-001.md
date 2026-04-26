## What This Means

You ran `syllago registry add` for a MOAT-signed registry, but syllago cannot find a signing identity to pin. Either the URL is not in the bundled allowlist and you did not pass `--signing-identity`, or you requested `--moat` without providing the required flags.

Syllago refuses to add MOAT registries without a pinned signing identity because silent "trust on first use" hides the most common supply-chain attack: an attacker-controlled first fetch establishing a fake root of trust.

## Common Causes

- The registry is newly published and not yet in the bundled allowlist.
- You forgot to pass `--signing-identity`, `--signing-repository-id`, or `--signing-repository-owner-id`.
- You passed `--moat` but did not supply the identity flags.

## How to Fix

Pick one of the three supported paths:

1. **Allowlist match** — if the registry is well-known, request an allowlist entry by opening a PR against syllago adding a new entry to `cli/internal/moat/signing_identities.json`.
2. **CLI flags** — pin the identity explicitly at add-time:
   ```
   syllago registry add https://github.com/OWNER/REPO.git \
     --signing-identity "https://github.com/OWNER/REPO/.github/workflows/moat.yml@refs/heads/main" \
     --signing-repository-id "$(gh api repos/OWNER/REPO --jq '.id')" \
     --signing-repository-owner-id "$(gh api repos/OWNER/REPO --jq '.owner.id')"
   ```
3. **Skip MOAT** — if you only need unsigned git content, omit `--moat` and the signing flags. Syllago will fall back to the legacy git-clone flow (no signature verification).

For the full workflow, see <https://syllago.dev/moat/registry-add-signing-identity/>.

## Example Output

```
Error MOAT_001: registry at https://github.com/newteam/new-registry.git has no pinned signing identity
  Suggestion: Pass --signing-identity <workflow-san> and --signing-repository-id / --signing-repository-owner-id (required for GitHub Actions issuers), or request an allowlist entry.
  Details: See https://syllago.dev/moat/registry-add-signing-identity/ for the full workflow and allowlist contribution process.
```
