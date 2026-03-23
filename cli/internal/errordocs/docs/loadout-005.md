## What This Means

A loadout was created or modified with no content items selected.

## Common Causes

- All items were deselected during interactive loadout creation
- A loadout was created programmatically with an empty items list
- Items were removed from a loadout without adding replacements

## How to Fix

1. When creating a loadout, select at least one content item (rule, skill, agent, etc.)
2. If using the TUI, ensure at least one item is checked before confirming
3. If editing `loadout.yaml` directly, ensure the `items` list is non-empty

## Example Output

```
Error LOADOUT_005: no items selected for loadout
  Suggestion: select at least one content item when creating a loadout
```
