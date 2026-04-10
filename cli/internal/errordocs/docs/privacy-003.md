## What This Means

A loadout being created or applied includes items that originate from private registries. This is a warning, not a blocking error.

## Common Causes

- A loadout mixes items from both private and public registries
- A loadout is being created from items that include private registry content
- An existing loadout was updated and now references private items

## How to Fix

1. This is a warning — no action is required if mixing private and public content is intentional
2. Review the listed items to confirm you intend to include private content
3. If the loadout will be shared publicly, consider replacing private items with public equivalents

## Example Output

```
Error PRIVACY_003: loadout "my-setup" contains 2 items from private registries
  Suggestion: review private items; this is a warning if mixing private/public content is intended
```
