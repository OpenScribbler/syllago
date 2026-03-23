## What This Means

The specified provider was not detected on your system. Syllago looks for provider-specific configuration directories to determine which AI coding tools are installed.

## Common Causes

- The AI coding tool is not installed on this machine
- The tool's configuration directory is missing or in a non-standard location
- The tool was installed but hasn't been run yet (config directory created on first run)

## How to Fix

1. Install the AI coding tool if you haven't already.
2. Run the tool at least once so it creates its configuration directory.
3. Check if the provider's config directory exists in the expected location (e.g., `~/.claude/` for Claude Code).

## Example Output

```
Error PROVIDER_002: provider "claude-code" not detected
  Suggestion: install Claude Code or verify its config directory exists
```
