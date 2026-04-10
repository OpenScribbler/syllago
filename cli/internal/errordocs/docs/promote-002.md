## What This Means

The content failed validation checks that run before promotion. Content must be structurally valid before it can be shared.

## Common Causes

- Missing required fields in content metadata (e.g., no `name` or `description`)
- Invalid YAML syntax in configuration files
- Content references files that do not exist
- Hook definitions with invalid event names or tool names

## How to Fix

1. Review the validation errors in the command output — each will describe the specific issue
2. Fix the structural issues in your content files
3. Run `syllago inspect <item>` to verify the content is valid
4. Retry the promote operation

## Example Output

```
Error PROMOTE_002: content validation failed: missing required field "description" in rules/my-rule
  Suggestion: fix validation errors shown above, then retry
```
