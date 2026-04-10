## What This Means

Syllago could not determine the user's home directory, which is needed for locating configuration and content files.

## Common Causes

- The `HOME` environment variable is not set
- Running under an unusual OS configuration or containerized environment without a home directory
- Running as a system user without a home directory assigned

## How to Fix

1. Set the `HOME` environment variable: `export HOME=/home/youruser`
2. Run syllago from a standard user account that has a home directory
3. In containers, ensure the `HOME` variable is set in the Dockerfile or entrypoint

## Example Output

```
Error SYSTEM_001: cannot determine home directory
  Suggestion: set the HOME environment variable, or run from a standard user account
```
