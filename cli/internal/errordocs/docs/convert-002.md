## What This Means

The source content could not be parsed during conversion. The file exists but its contents are not valid for the expected format.

## Common Causes

- Malformed source file with syntax errors
- YAML or JSON syntax errors (missing quotes, unclosed brackets, invalid indentation)
- File encoding issues (non-UTF-8 content)
- File is empty or contains only whitespace

## How to Fix

1. Open the source file and check for syntax errors
2. For YAML files, validate with a YAML linter or online validator
3. For JSON files, validate with `jq . < file.json` or an online JSON validator
4. Ensure the file uses UTF-8 encoding

## Example Output

```
Error CONVERT_002: failed to parse "rules/my-rule.yaml": yaml: line 5: mapping values are not allowed here
  Suggestion: check the source file for syntax errors
```
