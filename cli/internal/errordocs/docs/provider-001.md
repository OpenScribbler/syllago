## What This Means

The provider name you specified is not recognized by syllago. The slug does not match any supported AI coding tool provider.

## Common Causes

- Typo in the provider name
- Using the full tool name instead of the slug (e.g., "Claude Code" instead of "claude-code")
- The provider is not yet supported by syllago

## How to Fix

1. List supported providers: `syllago info providers`
2. Use the exact provider slug shown in the list.
3. Check for typos in the provider name you specified.

## Example Output

```
Error PROVIDER_001: unknown provider "claudecode"
  Suggestion: run 'syllago info providers' to see supported providers
```
