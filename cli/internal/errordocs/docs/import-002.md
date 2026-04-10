## What This Means

The content you are importing conflicts with content that already exists in your library. An item with the same name is already present.

## Common Causes

- An item with the same name was previously imported from a different source
- The same content was imported twice
- Locally created content has the same name as the import

## How to Fix

1. Check existing items with `syllago list` to find the conflict
2. Rename the existing item if you want to keep both
3. Remove the existing item first with `syllago remove` if you want to replace it
4. Use the `--force` flag if available to overwrite the existing item

## Example Output

```
Error IMPORT_002: import conflicts with existing item "team-rules" in library
  Suggestion: rename or remove the existing item, or use --force to overwrite
```
