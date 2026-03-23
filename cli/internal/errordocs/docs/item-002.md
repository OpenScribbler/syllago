## What This Means

The item name matches content in multiple content types, so syllago cannot determine which one you mean.

## Common Causes

- The same name is used for both a rule and a skill (e.g., `my-config` exists as both `rules/my-config` and `skills/my-config`)
- A registry contains duplicate names across different content types

## How to Fix

1. Qualify the item with its content type prefix: `syllago install rules/my-item` instead of `syllago install my-item`
2. Run `syllago list` to see all items grouped by type and identify the correct one

## Example Output

```
Error ITEM_002: item "my-config" is ambiguous: found in rules, skills
  Suggestion: qualify with content type, e.g. 'syllago install rules/my-config'
```
