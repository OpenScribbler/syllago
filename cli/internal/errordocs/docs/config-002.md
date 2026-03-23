## What This Means

Syllago could not find a configuration file. The command requires a syllago project to be initialized in the current directory or a parent directory.

## Common Causes

- Running a syllago command outside of a syllago project directory
- The `.syllago` directory was deleted or never created
- Running from a directory that is not within a syllago project tree

## How to Fix

Initialize a syllago project in the current directory:

```
syllago init
```

Or navigate to an existing syllago project directory before running the command.

## Example Output

```
Error CONFIG_002: configuration file not found
  Suggestion: run 'syllago init' to create project configuration
```
