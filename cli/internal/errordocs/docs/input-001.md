## What This Means

A required flag or positional argument was not provided for the command.

## Common Causes

- Running a command without its required arguments (e.g., `syllago install` with no item name)
- Omitting a required flag (e.g., `--to` when a target provider is needed)

## How to Fix

1. Check the command's usage with `syllago <command> --help`
2. Provide all required arguments and flags

## Example Output

```
Error INPUT_001: missing required argument: item name
  Suggestion: see 'syllago install --help' for usage
```
