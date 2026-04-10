## What This Means

The content type you are trying to export is not supported for the specified target provider.

## Common Causes

- The content type has no equivalent in the target provider's format
- The export path between the content type and target provider is not implemented
- The content type is internal to syllago and not exportable

## How to Fix

Check which export targets are supported for your content type:

```
syllago info formats
```

If the target provider is not supported, consider exporting to a different provider or manually adapting the content.

## Example Output

```
Error EXPORT_001: export not supported for content type "agents" to provider "gemini"
  Suggestion: check 'syllago info formats' for supported export targets
```
