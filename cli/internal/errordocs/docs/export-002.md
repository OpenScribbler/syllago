## What This Means

The export operation failed while writing content to the target location.

## Common Causes

- Write permission denied on the target directory
- The target directory does not exist
- Disk is full or has insufficient space
- The target path is on a read-only filesystem

## How to Fix

1. Ensure the target directory exists: `mkdir -p /path/to/target`
2. Check write permissions on the target directory: `ls -la /path/to/target`
3. Verify available disk space: `df -h`
4. If permissions are the issue, adjust with `chmod` or run from a writable location

## Example Output

```
Error EXPORT_002: failed to export "my-skill" to /home/user/.config/provider/skills/: permission denied
  Suggestion: ensure the target directory exists and is writable
```
