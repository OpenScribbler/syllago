## What This Means

The loadout's `loadout.yaml` manifest file exists but contains invalid content and cannot be parsed.

## Common Causes

- YAML syntax errors (bad indentation, missing colons, unclosed quotes)
- Missing required fields (`name`, `provider`, `items`)
- Invalid content type references in the items list
- File encoding issues

## How to Fix

1. Open the `loadout.yaml` file and check for YAML syntax errors
2. Ensure all required fields are present:

```yaml
name: my-loadout
provider: claude-code
items:
  - type: rules
    path: my-rule.md
```

3. Validate YAML syntax with a linter or online validator
4. Ensure content type references match valid syllago content types

## Example Output

```
Error LOADOUT_002: failed to parse "my-loadout/loadout.yaml": yaml: line 3: did not find expected key
  Suggestion: check YAML syntax and ensure required fields (name, provider, items) are present
```
