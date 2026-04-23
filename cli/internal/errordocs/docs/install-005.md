## What This Means

The item you are trying to uninstall is not currently installed in your system.

## Common Causes

- Trying to uninstall an item that was never installed
- The item was already removed in a previous operation
- Typo in the item name or identifier

## How to Fix

Check what is currently installed by running:

```
syllago list --source library
```

Verify the exact name of the item you want to remove and try again.

## Example Output

```
Error INSTALL_005: item "my-rule" is not installed
  Suggestion: run 'syllago list --source library' to see installed items
```
