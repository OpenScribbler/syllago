## What This Means

Syllago failed to clone the import source repository. The content could not be fetched from the specified location.

## Common Causes

- Invalid or mistyped repository URL
- Network connectivity issues (firewall, proxy, DNS resolution)
- The repository requires authentication (private repo)
- The repository no longer exists or has been moved

## How to Fix

1. Verify the import URL is correct and accessible in a browser
2. Check your network connectivity (`ping github.com` or equivalent)
3. For private repositories, ensure your Git credentials are configured
4. If the repository has moved, update the URL to the new location

## Example Output

```
Error IMPORT_001: failed to clone "https://github.com/org/repo": authentication required
  Suggestion: verify the import URL is accessible and check network connectivity
```
