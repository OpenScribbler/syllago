## What This Means

The registry URL you tried to use is blocked by the `allowedRegistries` policy in your configuration. This is a security control that restricts which registries can be used.

## Common Causes

- Your organization has configured an allowlist of approved registries
- The registry URL doesn't match any entry in the `allowedRegistries` list
- A workspace or global config restricts registry sources

## How to Fix

1. Check the allowed registries in your config: look for `allowedRegistries` in `.syllago/config.json`.
2. If you believe this registry should be allowed, contact your administrator to update the policy.
3. Use a registry that is already on the approved list.

## Example Output

```
Error REGISTRY_003: registry URL not in allowedRegistries
  Suggestion: check your allowedRegistries policy or contact your admin
```
