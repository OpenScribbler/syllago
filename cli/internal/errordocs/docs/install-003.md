## What This Means

The requested install method is not supported for this content type. Different content types require different installation strategies.

## Common Causes

- Trying to symlink a content type that requires JSON merge (hooks, MCP configs)
- Specifying an install method that doesn't exist for the content type
- Mismatched content type and provider capabilities

## How to Fix

1. Hooks and MCP configs are installed via JSON merge into provider settings files, not as filesystem symlinks.
2. Rules, skills, agents, commands, and prompts use filesystem installation (files, directories, or symlinks).
3. Let syllago choose the default install method by omitting the method flag.

## Example Output

```
Error INSTALL_003: invalid install method "symlink" for content type "hooks"
  Suggestion: hooks use JSON merge — omit the method flag to use the default
```
