## What This Means

A path override in the configuration is invalid. The path is either not absolute or references an unknown configuration path key.

## Common Causes

- A relative path was used where an absolute path is required
- An unknown or misspelled configuration path key was specified
- The path override was set manually with an invalid value

## How to Fix

1. Use absolute paths (starting with `/` on Linux/macOS or a drive letter on Windows) for all config path overrides
2. Check valid path keys:

```
syllago config paths show
```

3. Correct the path override to use a valid absolute path and recognized key

## Example Output

```
Error CONFIG_003: invalid path override: "library" path must be absolute, got "./my-library"
  Suggestion: use absolute paths for config overrides; see 'syllago config paths show' for valid keys
```
