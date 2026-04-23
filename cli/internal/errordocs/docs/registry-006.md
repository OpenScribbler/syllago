## What This Means

The registry is listed in your configuration but its local clone is missing. Syllago knows about the registry but cannot access its content.

## Common Causes

- The local cache was cleared or deleted
- The registry directory was manually removed
- First run after importing a config from another machine
- The clone was interrupted and never completed

## How to Fix

1. Sync your registries to re-clone missing ones: `syllago registry sync`
2. If sync fails, try removing and re-adding the registry: `syllago registry remove <name>` then `syllago registry add <url>`

## Example Output

```
Error REGISTRY_006: registry "community" not cloned
  Suggestion: run 'syllago registry sync' to fetch missing registries
```
