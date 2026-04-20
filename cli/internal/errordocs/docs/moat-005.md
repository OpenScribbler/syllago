## What This Means

Syllago ships a snapshot of the Sigstore public-good trusted root — the Fulcio CA bundle plus Rekor public keys. That snapshot has a 365-day expiration cliff baked in, which is how syllago ensures verifications keep working as Sigstore rotates its keys every 6-12 months.

MOAT_005 means the bundled root crossed that cliff: it is more than 365 days old, and syllago refuses to verify against keys that old because any result (pass or fail) would be untrustworthy.

This is an operator action, not a registry action. Updating your syllago binary ships a fresher bundled root.

## Common Causes

- You have been running the same syllago binary for more than a year.
- Your syllago distribution channel (package manager, internal mirror) is not pushing updates.
- You built from an old git tag without a fresh `trusted_root.json` refresh.

## How to Fix

Run the self-updater to pick up the latest release, which always carries a refreshed trusted root:

```
syllago self-update
```

If your environment pins syllago to a specific version, bump that pin. You can also check staleness at any time without running a verify:

```
syllago moat trust status
```

which prints the current root's issue date, age in days, and the cliff date.

For air-gapped environments, download a newer release artifact on an online machine and copy the binary into place. There is no supported path to refresh just the trusted root without updating the binary — reproducibility of signed releases depends on the root travelling with the client.

## Example Output

```
Error MOAT_005: bundled Sigstore trusted root expired
  Suggestion: Run `syllago self-update` to refresh. The bundled root passed its 365-day cliff; verification refuses to proceed.
  Details: MOAT_TRUSTED_ROOT_STALE: trusted root issued 2025-03-01, age 420 days, cliff 2026-03-01
```
