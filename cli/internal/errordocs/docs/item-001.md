## What This Means

The specified item could not be found in your local library or any configured registries.

## Common Causes

- Typo in the item name
- The item does not exist in the local library
- The item exists in a registry that is not configured or has not been synced
- The item was removed or renamed

## How to Fix

1. Check the spelling of the item name
2. Run `syllago list` to browse available items in your library
3. Run `syllago list --registry <name>` to browse items in a specific registry
4. If the item is in a registry, run `syllago registry sync` to pull the latest content

## Example Output

```
Error ITEM_001: item "my-ruel" not found
  Suggestion: run 'syllago list' to browse available items; check spelling
```
