## What This Means

Syllago cannot write to the target install path. The content cannot be installed because the destination is not writable.

## Common Causes

- Permission issues on the provider's configuration directory
- The target path is on a read-only filesystem
- The directory is owned by another user or root
- SELinux or other security policies restricting write access

## How to Fix

1. Check permissions on the provider's config directory: `ls -la <path>`
2. Ensure your user owns the directory or has write access.
3. If the directory is owned by root, check if the provider was installed system-wide and adjust permissions accordingly.

## Example Output

```
Error INSTALL_001: install path not writable
  Suggestion: check permissions on the provider config directory
```
