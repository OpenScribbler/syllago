## What This Means

The working tree has uncommitted changes that must be resolved before promoting content.

## Common Causes

- Modified files in the syllago library that have not been committed
- Untracked files in the content directory
- A previous promote operation was interrupted, leaving partial changes

## How to Fix

1. Run `git status` in your syllago library to see uncommitted changes
2. Commit your changes: `git add . && git commit -m "your message"`
3. Or stash them temporarily: `git stash`
4. Then retry the promote operation

## Example Output

```
Error PROMOTE_001: uncommitted changes in working tree
  Suggestion: commit or stash your changes before promoting content
```
