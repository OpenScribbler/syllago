## What This Means

The project has already been initialized with syllago. A `.syllago` directory already exists in the current directory.

## Common Causes

- Running `syllago init` in a directory that was previously initialized
- A `.syllago` directory was manually created or left behind from a previous setup

## How to Fix

1. If the project is already initialized, no action is needed — you can use syllago normally
2. If you want to reinitialize from scratch, remove the `.syllago` directory first: `rm -rf .syllago`
3. Then run `syllago init` again

## Example Output

```
Error INIT_001: project already initialized
  Suggestion: the project is already initialized; to reinitialize, remove the .syllago directory first
```
