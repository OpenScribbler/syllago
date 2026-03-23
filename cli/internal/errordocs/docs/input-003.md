## What This Means

A flag or argument value has an invalid format or is out of the expected range.

## Common Causes

- Invalid provider slug (e.g., `--to=not-a-provider`)
- Malformed argument value (e.g., a path with invalid characters)
- Numeric value out of expected range

## How to Fix

1. Check the expected format in the command help: `syllago <command> --help`
2. For provider slugs, run `syllago info providers` to see valid values
3. Verify the argument value matches the expected format

## Example Output

```
Error INPUT_003: invalid provider slug "not-a-provider"
  Suggestion: run 'syllago info providers' for valid provider slugs
```
