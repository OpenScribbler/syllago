## What This Means

You passed `--signing-identity` (or another `--signing-*` flag), but the flag set is incomplete. The most common case: the issuer is the GitHub Actions OIDC issuer (the default) but one or both numeric repository IDs is missing.

Syllago requires numeric IDs for GitHub-signed identities because they are the only immutable anchor — an attacker who persuades GitHub to transfer an owner/repo name can re-register the human-readable subject SAN but cannot forge the numeric ID extensions on the OIDC certificate.

## Common Causes

- Missing `--signing-repository-id`.
- Missing `--signing-repository-owner-id`.
- Typo in the `--signing-issuer` value that bypasses the GitHub check.

## How to Fix

Look up the numeric IDs for the target repository:

```
gh api repos/OWNER/REPO --jq '.id, .owner.id'
```

Then re-run the add:

```
syllago registry add https://github.com/OWNER/REPO.git \
  --signing-identity "https://github.com/OWNER/REPO/.github/workflows/moat.yml@refs/heads/main" \
  --signing-repository-id 123456789 \
  --signing-repository-owner-id 987654321
```

Non-GitHub issuers (custom OIDC providers, GitLab, Buildkite) do not require numeric IDs — those issuers have their own equivalent bindings that slice-3 will consume.

## Example Output

```
Error MOAT_002: GitHub Actions issuer requires --signing-repository-id and --signing-repository-owner-id
  Suggestion: Find numeric IDs with `gh api repos/OWNER/REPO --jq '.id, .owner.id'`. See https://openscribbler.github.io/syllago-docs/moat/registry-add-signing-identity/.
```
