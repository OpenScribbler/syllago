## What This Means

The command requires an interactive terminal (TTY) but is running in a non-interactive context.

## Common Causes

- Running the TUI or an interactive command through a pipe (e.g., `echo | syllago`)
- Running syllago in a CI/CD pipeline without interactive support
- Redirecting stdin from a file

## How to Fix

1. Run the command in an interactive terminal
2. For scripted or CI usage, use `--json` or other non-interactive flags
3. Use specific CLI subcommands instead of the TUI for automation

## Example Output

```
Error INPUT_004: this command requires an interactive terminal
  Suggestion: use --json or non-interactive flags for scripted usage
```
