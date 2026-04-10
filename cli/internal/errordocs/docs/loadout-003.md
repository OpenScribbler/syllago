## What This Means

You are trying to apply a loadout, but another loadout is already active for the same provider. Only one loadout can be active at a time per provider.

## Common Causes

- A previously applied loadout was not removed before applying a new one
- A loadout was applied in a different session and is still active

## How to Fix

1. Check which loadout is currently active:

```
syllago loadout list
```

2. Remove the current loadout before applying the new one:

```
syllago loadout remove
```

3. Apply the new loadout:

```
syllago loadout apply <name>
```

## Example Output

```
Error LOADOUT_003: loadout "team-defaults" is already active for provider "claude-code"
  Suggestion: run 'syllago loadout remove' before applying a new loadout
```
