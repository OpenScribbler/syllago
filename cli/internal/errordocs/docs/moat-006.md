## What This Means

Syllago added this registry with a pinned signing identity (`signing_profile` in `config.json`), which means you explicitly opted into cryptographic verification on every add. But when it went to verify, the registry checkout on disk did not contain a `manifest.json` plus a `manifest.json.sigstore` bundle.

Once a profile is pinned, syllago refuses to silently fall back to unsigned git content. Either the publisher stopped signing, or the checkout is incomplete, or you pinned a profile against a registry that never published a manifest in the first place.

## Common Causes

- The registry was pinned optimistically but the operator never turned on the MOAT Publisher Action.
- A partial git clone or `.gitignore` accidentally excluded the manifest files.
- The manifest files live on a different branch or ref than the one you cloned.
- The registry moved to a different manifest location (e.g. a hosted URL instead of in-repo) without updating its README.

## How to Fix

Pick one of the following, depending on whether you expect this registry to be signed:

1. **You expected signatures.** Force a fresh checkout and check the branch:

   ```
   syllago registry sync <name>
   ls ~/.syllago/registries/<NAME>/manifest.json
   ls ~/.syllago/registries/<NAME>/manifest.json.sigstore
   ```

   If the files still aren't there, file an issue against the registry — the publisher needs to ship a MOAT manifest.
2. **You pinned optimistically and are OK with unsigned content.** Remove the registry and re-add it without any `--signing-*` flags:

   ```
   syllago registry remove <name>
   syllago registry add <url>
   ```

   That drops back to legacy git-only mode with no verification.
3. **The manifest lives at a URL, not in the repo.** Wire the manifest URI into config (slice 3+ feature — track progress in ADR 0007 follow-ups).

## Example Output

```
Error MOAT_006: registry has a pinned signing profile but no signed manifest in checkout
  Suggestion: Run `syllago registry sync` to refresh. If the registry does not publish a MOAT manifest, remove it and re-add without --signing-* flags.
  Details: MOAT_UNSIGNED: expected manifest.json + manifest.json.sigstore under ~/.syllago/registries/OpenScribbler/syllago-meta-registry/
```
