## What This Means

Syllago was unable to write the configuration file to disk.

## Common Causes

- Permission denied on the `.syllago/` directory or config file
- Disk is full or has insufficient space
- The `.syllago/` directory was deleted while syllago was running
- The filesystem is mounted as read-only

## How to Fix

1. Check write permissions on the `.syllago/` directory: `ls -la .syllago/`
2. Verify available disk space: `df -h`
3. If the directory is missing, re-initialize: `syllago init`
4. If permissions are wrong, fix with: `chmod u+w .syllago/`

## Example Output

```
Error CONFIG_004: failed to save configuration: permission denied
  Suggestion: check write permissions on the .syllago/ directory
```
