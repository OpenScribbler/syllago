## What This Means

The install cannot proceed because of a conflict with an active loadout or an already-installed item.

## Common Causes

- An active loadout already manages this content, and installing directly would conflict
- A different version of the same item is already installed
- Two items would write to the same destination path

## How to Fix

1. If a loadout is managing this content, remove the loadout first: `syllago loadout remove <name>`
2. If a conflicting item is already installed, uninstall it before installing the new one.
3. Check which loadouts are active: `syllago loadout list`

## Example Output

```
Error INSTALL_004: conflict with active loadout "dev-setup"
  Suggestion: run 'syllago loadout remove dev-setup' first, then retry the install
```
