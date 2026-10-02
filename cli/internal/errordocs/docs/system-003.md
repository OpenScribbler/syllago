## What This Means

Another syllago process is installing, uninstalling, removing, pinning, or
otherwise changing install state, and it did not finish within 10 seconds.
Syllago lets one process change install state at a time so that two processes
never overwrite each other's install records or provider settings.

## Common Causes

- A second terminal is running a syllago command that changes installs
- The syllago TUI or GUI is in the middle of an install or removal
- A loadout apply or removal is still running

## How to Fix

1. Wait for the other syllago command to finish, then run the command again.
2. If no other syllago process is running, run the command again. The
   operating system releases the lock when a process exits, so a crashed
   process cannot hold it.

## Example Output

```
Error SYSTEM_003: another syllago process is changing installs
  Suggestion: Wait for it to finish, then run the command again
```
