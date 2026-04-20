## What This Means

You (or your config) asked syllago to verify a registry's manifest against a specific Sigstore trusted root supplied from disk — either via `--trusted-root <path>` on the command line, or via a `trusted_root` field on the registry in `config.json`. Syllago tried to load that file and could not.

This error fires *before* any signature verification happens. Rather than silently fall back to the bundled trusted root (which would be a trust downgrade you did not authorize), syllago refuses to proceed until the override is fixed or removed.

## Common Causes

- The path points at a file that does not exist, or has permissions that block reading.
- The file exists but is not valid JSON — e.g. a download got truncated, or someone hand-edited it and introduced a syntax error.
- The path is relative. Syllago requires absolute paths for trusted-root overrides because the resolved file would otherwise depend on the caller's working directory.
- The file was replaced with the wrong thing (a certificate bundle, a keyring, a README).

## How to Fix

1. **Verify the path.** Make sure the file exists and is readable:

   ```
   ls -l /etc/syllago/corp-trusted-root.json
   cat /etc/syllago/corp-trusted-root.json | jq .mediaType
   ```

   The file must be a Sigstore `trusted_root.json` — the `mediaType` should look like `application/vnd.dev.sigstore.trustedroot+json;version=0.1`.

2. **Refresh the file** from your internal trust-root distribution process (TUF mirror, shared secret, etc). Air-gapped deployments typically pin this file to a specific release — re-fetch against the current pin.

3. **If you want to drop the override**, either pass `--trusted-root ""` (CLI) or remove the `trusted_root` field from that registry's entry in `config.json`. Verification then falls back to the bundled root shipped with syllago.

## Example Output

```
Error MOAT_007: trusted-root override unusable
  Suggestion: Fix the path or remove it to fall back to the bundled trusted root.
  Details: reading trusted-root override /etc/syllago/corp-trusted-root.json: open /etc/syllago/corp-trusted-root.json: no such file or directory
```
