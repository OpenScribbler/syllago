## What This Means

The content was parsed successfully but could not be rendered into the target provider's format.

## Common Causes

- The content uses features not supported by the target provider
- Encoding issues when writing the output format
- The target format has constraints that the source content violates (e.g., field length limits)

## How to Fix

1. Review the content for provider-specific features that may not translate
2. Simplify the content to use only features common across providers
3. Check provider compatibility with `syllago info formats`
4. Try converting to a different target provider to isolate the issue

## Example Output

```
Error CONVERT_003: failed to render content to gemini format: unsupported hook event type "on_tool_error"
  Suggestion: simplify content or check target provider compatibility
```
