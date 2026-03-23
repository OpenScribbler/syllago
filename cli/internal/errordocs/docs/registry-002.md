## What This Means

The specified registry name was not found in your syllago configuration. No registry with that name has been added.

## Common Causes

- Typo in the registry name
- The registry has not been added yet
- The registry was previously removed

## How to Fix

1. List your configured registries: `syllago registry list`
2. Check for typos in the registry name you specified.
3. If the registry isn't listed, add it: `syllago registry add <name> <url>`

## Example Output

```
Error REGISTRY_002: registry "my-reg" not found
  Suggestion: run 'syllago registry list' to see available registries
```
