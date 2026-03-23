## What This Means

Content from a private registry cannot be published to a public registry. The privacy gate prevents private content from leaking into public spaces.

## Common Causes

- Attempting to publish content that was imported from a private (enterprise/team) registry
- Running `syllago promote` targeting a public registry with private-origin content
- The content's source registry is marked as private in its configuration

## How to Fix

1. Move the content to a public registry first, then publish from there
2. Or publish to a private registry instead of a public one
3. If the content should be public, copy it to your local library (removing the private origin) and publish from there

## Example Output

```
Error PRIVACY_001: cannot publish private content to public registry "community-rules"
  Suggestion: move content to a public registry first, or publish to a private registry
```
