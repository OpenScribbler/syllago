## What This Means

A registry with the specified name already exists in your configuration. Registry names must be unique.

## Common Causes

- Trying to add a registry that was previously added
- Using a name that collides with an existing registry

## How to Fix

1. List existing registries to confirm: `syllago registry list`
2. Choose a different name for the new registry.
3. If you want to replace the existing one, remove it first: `syllago registry remove <name>`

## Example Output

```
Error REGISTRY_005: registry "community" already exists
  Suggestion: use a different name or remove the existing registry first
```
