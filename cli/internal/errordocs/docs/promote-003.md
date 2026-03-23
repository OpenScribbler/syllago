## What This Means

A git operation (creating a branch, committing, or pushing) failed during the promote process.

## Common Causes

- No git remote configured for the repository
- Push was rejected (e.g., remote branch has diverged)
- Git authentication failed (expired token, missing SSH key)
- Network connectivity issues

## How to Fix

1. Verify a git remote is configured: `git remote -v`
2. Check that you have push access to the remote repository
3. If authentication failed, refresh your credentials (SSH key, personal access token, etc.)
4. If the remote branch has diverged, pull the latest changes first: `git pull --rebase`

## Example Output

```
Error PROMOTE_003: git push failed: authentication required
  Suggestion: ensure git remote is configured and you have push access
```
