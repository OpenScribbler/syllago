## What This Means

Content from a private registry cannot be shared to a public repository. The privacy gate prevents accidental exposure of private content.

## Common Causes

- Running `syllago share` targeting a public git repository with content that originates from a private registry
- The target repository is publicly accessible but the content's source registry is private

## How to Fix

1. Share to a private repository instead
2. Or move the content to your local library first (which strips the private-origin metadata), then share from there
3. Verify the target repository's visibility matches your intent

## Example Output

```
Error PRIVACY_002: cannot share private content to public repo "github.com/org/public-rules"
  Suggestion: share to a private repository, or move content to your local library first
```
