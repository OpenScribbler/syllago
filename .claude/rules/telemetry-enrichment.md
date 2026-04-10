# Telemetry Enrichment Rule

**Scope:** `cli/cmd/syllago/*_cmd.go`, `cli/cmd/syllago/*.go` files with `RunE` functions

---

## When adding or modifying a CLI command

1. **Add `telemetry.Enrich()` calls for relevant properties** in the `RunE` function, before it returns. Track:
   - Provider slugs (`provider`, `from`, `from_provider`, `to_provider`)
   - Content type string (`content_type`)
   - Counts (`content_count`, `item_count`, `action_count`, `registry_count`)
   - Boolean flags (`dry_run`)
   - Mode strings (`mode`, `source_filter`)

   Never enrich with: file contents, file paths, content names, registry URLs, usernames, or any PII.

2. **If the property key is new** (not already in `EventCatalog()` in `cli/internal/telemetry/catalog.go`):
   - Add a `PropertyDef` entry to the relevant event in `EventCatalog()`
   - The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` will fail CI if you forget this step

3. **Regenerate `telemetry.json`** after any catalog change:
   ```
   cd cli && make gendocs
   ```
   The pre-push hook blocks pushes with stale `telemetry.json`.

## Quick reference

| Property key     | Type   | When to use                                          |
|------------------|--------|------------------------------------------------------|
| `provider`       | string | Target provider slug (install, uninstall, apply)     |
| `from`           | string | Source provider slug for add/import commands         |
| `from_provider`  | string | Source provider for convert                          |
| `to_provider`    | string | Target provider for convert                          |
| `content_type`   | string | Content type filter or selected type                 |
| `content_count`  | int    | Number of items installed/added                      |
| `item_count`     | int    | Number of items in a list result                     |
| `action_count`   | int    | Number of actions in a loadout result                |
| `registry_count` | int    | Number of registries involved                        |
| `dry_run`        | bool   | Whether --dry-run was used                           |
| `mode`           | string | Operational mode (e.g. "try" for loadout)            |
| `source_filter`  | string | Source filter (library, shared, registry)            |

## Example

```go
func runMyCommand(cmd *cobra.Command, args []string) error {
    // ... command logic ...
    telemetry.Enrich("provider", providerSlug)
    telemetry.Enrich("content_type", typeStr)
    telemetry.Enrich("content_count", len(installed))
    return nil
}
```

`TrackCommand()` fires automatically from `PersistentPostRun` — you do not call it yourself.
