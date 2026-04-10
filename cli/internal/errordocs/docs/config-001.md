## What This Means

The syllago configuration file exists but contains invalid or malformed content and cannot be parsed.

## Common Causes

- Syntax error in `.syllago/config.json` from manual editing
- Incomplete JSON (missing closing braces, trailing commas)
- File was corrupted or partially written

## How to Fix

1. Open `.syllago/config.json` and check for JSON syntax errors
2. Validate with `jq . < .syllago/config.json` to pinpoint the issue
3. Fix the syntax error, or if the file is beyond repair, delete it and re-initialize:

```
rm .syllago/config.json
syllago init
```

## Example Output

```
Error CONFIG_001: invalid configuration file ".syllago/config.json": unexpected end of JSON input
  Suggestion: fix JSON syntax or re-initialize with 'syllago init'
```
