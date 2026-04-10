## What This Means

Syllago could not find a repository or library to work with. The catalog requires a `.syllago` directory to discover and manage content.

## Common Causes

- Running a syllago command outside of a project directory
- The `.syllago` directory is missing or was deleted
- You haven't initialized syllago in this project yet

## How to Fix

1. Navigate to your project directory and run `syllago init` to create a new syllago repository.
2. If you already have a syllago project, make sure you're running commands from within that directory (or a subdirectory of it).

## Example Output

```
Error CATALOG_001: no syllago repository found
  Suggestion: run 'syllago init' to create one, or navigate to an existing syllago project
```
