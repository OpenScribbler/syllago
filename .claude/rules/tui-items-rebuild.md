# Content Refresh Pattern

The App has multiple content models (library, explorer, gallery) that display catalog items. When the catalog changes (install, remove, edit, rescan), all active views must be refreshed.

## Always Use refreshContent()

After any operation that changes catalog state, call `a.refreshContent()`. Never set items on sub-models directly from message handlers.

```go
// WRONG — only updates one model, misses gallery/explorer
a.library.SetItems(a.catalog.Items)

// RIGHT — refreshContent() updates the correct model for the active tab
a.refreshContent()
```

## What refreshContent() Does

1. Checks which tab is active (library, gallery, or explorer)
2. Calls `SetItems()` on the correct sub-model with fresh catalog data
3. Calls `SetSize()` to recalculate layout with current dimensions
4. For gallery tabs, calls `refreshGallery()` to rebuild cards

## When to Call

| Situation | Method |
|-----------|--------|
| After install/remove/uninstall completes | `a.refreshContent()` |
| After edit saved (name/description) | `a.refreshContent()` |
| After tab/group change | `a.refreshContent()` |
| After catalog rescan (R key) | Called inside `a.rescanCatalog()` |

## rescanCatalog() for Full Refresh

When the user presses `R`, `rescanCatalog()` re-reads all content from disk,
reloads config (to pick up registry changes), rebuilds the catalog, then calls
`refreshContent()` + `updateNavState()`.
