## What This Means

Syllago encountered an error while scanning the catalog for content items. The scan could not complete successfully.

## Common Causes

- Corrupted or malformed content files (invalid YAML/JSON syntax)
- Permission issues on content directories or files
- Broken symlinks in the content tree
- Disk I/O errors

## How to Fix

1. Check file permissions on your `.syllago` directory and its contents: `ls -la .syllago/`
2. Validate the syntax of recently modified content files.
3. Look for broken symlinks: `find .syllago -xtype l`
4. If the issue persists, try removing and re-importing the problematic content.

## Example Output

```
Error CATALOG_002: failed to scan catalog
  Suggestion: check file permissions and content file syntax in .syllago/
```
