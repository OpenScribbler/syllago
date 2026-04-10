## What This Means

The specified content type is not recognized by syllago.

## Common Causes

- Typo in the content type (e.g., `--type=ruls` instead of `--type=rules`)
- Using a singular form instead of plural (e.g., `rule` instead of `rules`)
- Using a content type that does not exist in syllago

## How to Fix

1. Use one of the valid content types: `rules`, `skills`, `agents`, `commands`, `hooks`, `mcp`, `prompts`, `loadouts`
2. Check command help for accepted values: `syllago <command> --help`

## Example Output

```
Error ITEM_003: unknown content type "ruls"
  Suggestion: valid types are: rules, skills, agents, commands, hooks, mcp, prompts, loadouts
```
