## What This Means

A filesystem operation (reading, writing, or creating directories) failed unexpectedly.

## Common Causes

- Permission denied on the target path
- Disk is full
- Path is too long for the filesystem
- Corrupted filesystem or inaccessible mount
- Another process holds a lock on the file

## How to Fix

1. Check available disk space: `df -h`
2. Verify permissions on the target path: `ls -la <path>`
3. If permission denied, adjust ownership or permissions: `chmod` / `chown`
4. If the path is on a network mount, verify the mount is accessible

## Example Output

```
Error SYSTEM_002: failed to create directory "/home/user/.syllago/library": permission denied
  Suggestion: check disk space and verify permissions on the target path
```
