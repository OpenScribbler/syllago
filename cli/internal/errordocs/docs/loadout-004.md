## What This Means

The loadout references a provider that syllago does not recognize or support.

## Common Causes

- The `provider` field in `loadout.yaml` contains a typo (e.g., `cladue-code` instead of `claude-code`)
- The loadout was created for a provider that has since been removed or renamed
- The loadout was authored for a provider that your version of syllago does not support

## How to Fix

1. Open the loadout's `loadout.yaml` and check the `provider` field
2. Run `syllago info providers` to see the list of supported provider slugs
3. Update the `provider` field to match one of the supported slugs

## Example Output

```
Error LOADOUT_004: loadout references unknown provider "cladue-code"
  Suggestion: check the provider field in loadout.yaml; run 'syllago info providers' for supported providers
```
