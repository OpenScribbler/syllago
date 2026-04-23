## What This Means

Content from a private registry cannot be shared to a public registry. The privacy gate prevents private content from leaking into public spaces.

## Common Causes

- Running `syllago share <name> --registry <registry-name>` when the target registry is public and the content originates from a private (enterprise/team) registry
- The content's source registry is marked as private in its configuration

## How to Fix

1. Share to a private registry instead
2. Or move the content to your local library first (which strips the private-origin metadata), then share from there
3. If the content should be public, copy it to your local library (removing the private origin) and share from there

## Example Output

```
Error PRIVACY_001: cannot share private content to public registry "community-rules"
  Suggestion: move content to a public registry first, or share to a private registry
```
