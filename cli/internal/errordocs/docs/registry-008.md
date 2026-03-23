## What This Means

Syllago could not save changes to the registry configuration file. The operation completed in memory but failed to persist to disk.

## Common Causes

- Permission issues on the config file or its parent directory
- Disk is full
- The config file is locked by another process
- The `.syllago` directory was deleted mid-operation

## How to Fix

1. Check write permissions on the config file: `ls -la .syllago/config.json`
2. Verify available disk space: `df -h .`
3. Ensure no other process has the file locked.
4. Check that the `.syllago` directory still exists.

## Example Output

```
Error REGISTRY_008: failed to save registry config
  Suggestion: check write permissions on .syllago/config.json
```
