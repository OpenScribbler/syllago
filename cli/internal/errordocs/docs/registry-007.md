## What This Means

Syllago failed to update a registry's local clone during sync. The git pull operation did not complete successfully.

## Common Causes

- Network connectivity issues
- Merge conflicts in the local clone (local modifications)
- The remote branch was force-pushed, causing history divergence
- Remote server is temporarily unavailable

## How to Fix

1. Check your network connection.
2. Try syncing again: `syllago registry sync`
3. If the problem persists, remove and re-add the registry to get a fresh clone: `syllago registry remove <name>` then `syllago registry add <name> <url>`

## Example Output

```
Error REGISTRY_007: failed to sync registry "community"
  Suggestion: check your network connection, or try removing and re-adding the registry
```
