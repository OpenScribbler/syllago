## What This Means

The registry name or structure is invalid. Syllago requires registry names to follow a specific format and registry directories to contain the expected layout.

## Common Causes

- Registry name contains invalid characters (spaces, special characters)
- Registry directory is missing required structure or metadata
- Corrupted registry clone

## How to Fix

1. Use alphanumeric characters and hyphens for registry names (e.g., `my-registry`).
2. Verify the registry repository has the expected directory structure.
3. If the registry was partially cloned, try removing it and re-adding: `syllago registry remove <name>` then `syllago registry add <url>`.

## Example Output

```
Error REGISTRY_004: invalid registry "my registry!"
  Suggestion: use alphanumeric characters and hyphens only
```
