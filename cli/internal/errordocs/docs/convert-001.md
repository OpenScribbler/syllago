## What This Means

The content type you are trying to convert does not support conversion between the specified providers.

## Common Causes

- The content type is provider-specific and has no equivalent in the target provider
- The conversion path between the source and target providers is not implemented
- The content type does not have a canonical format defined

## How to Fix

Check which conversion paths are supported:

```
syllago info formats
```

If the content type cannot be converted, you may need to manually recreate it for the target provider.

## Example Output

```
Error CONVERT_001: conversion not supported for content type "commands" (claude-code → gemini)
  Suggestion: check 'syllago info formats' for supported conversion paths
```
