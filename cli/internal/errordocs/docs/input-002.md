## What This Means

Two or more flags that cannot be used together were provided in the same command.

## Common Causes

- Using `--local` and `--registry` together when only one source is allowed
- Combining `--json` with interactive-only flags
- Specifying conflicting output or filter options

## How to Fix

1. Review the command help with `syllago <command> --help` to understand which flags conflict
2. Use only one of the conflicting flags per invocation

## Example Output

```
Error INPUT_002: flags --local and --registry are mutually exclusive
  Suggestion: use only one of the conflicting flags
```
