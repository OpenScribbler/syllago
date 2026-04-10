## What This Means

The specified loadout could not be found in your local library or any configured registry.

## Common Causes

- Typo in the loadout name
- The loadout is not in your local library
- The loadout is not available in any configured registry
- The registry has not been synced recently

## How to Fix

1. Check available loadouts:

```
syllago loadout list
```

2. Verify the exact name of the loadout you are looking for
3. If the loadout is in a registry, ensure the registry is configured and synced

## Example Output

```
Error LOADOUT_001: loadout "kubernetes-setup" not found
  Suggestion: run 'syllago loadout list' to see available loadouts
```
