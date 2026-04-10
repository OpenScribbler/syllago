## What This Means

The item you tried to install could not be found. It is not present in your local library or any configured registry.

## Common Causes

- Typo in the item name
- The item hasn't been imported into your local library yet
- The item exists in a registry that hasn't been added
- The item was removed or renamed

## How to Fix

1. List available items: `syllago list`
2. Check for typos in the item name.
3. If the item is in a registry, make sure the registry is added and synced: `syllago registry list`
4. Import the item first if it's from an external source.

## Example Output

```
Error INSTALL_002: item "my-rule" not found
  Suggestion: run 'syllago list' to see available items
```
