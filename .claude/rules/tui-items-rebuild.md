# Items Model Rebuild Pattern

The `itemsModel` has two kinds of state:
- **Data state** (items, providers, repoRoot) — changes on refresh
- **Navigation context** (`itemsContext`) — must survive refreshes

When refreshing item data (after install, remove, toggle-hidden, search cancel), **always use `rebuildItems()` or `rebuildItemsFiltered(query)`**. Never construct a new `itemsModel` directly in app.go handlers.

## The `itemsContext` Struct

```go
type itemsContext struct {
    sourceRegistry   string // set when browsing items from a specific registry
    sourceProvider   string // provider slug when drilled in from loadout cards
    parentLabel      string // intermediate breadcrumb (e.g. "Library", "Loadouts")
    hiddenCount      int    // number of hidden items filtered out
    hideLibraryBadge bool   // suppress [LIBRARY] badge (when already in Library view)
}
```

These fields control breadcrumb rendering, provider filtering, and display flags. Losing them causes broken breadcrumbs and incorrect item lists.

## When to Use Each Method

| Situation | Method |
|-----------|--------|
| Refresh data (install/remove/toggle-hidden done) | `a.rebuildItems()` |
| Cancel search (restore full list) | `a.rebuildItems()` |
| Apply search filter | `a.rebuildItemsFiltered(query)` |
| Live search typing | `a.rebuildItemsFiltered(query)` |
| Initial navigation (first time entering items) | `newItemsModel()` + set `ctx` fields |

## What the Methods Do

Both methods:
1. Save the current `itemsContext`
2. Fetch fresh items from the catalog
3. Apply the provider filter if `sourceProvider` is set
4. Apply search filter if query is provided
5. Create a new `itemsModel` with fresh data
6. Restore the saved context
7. Clamp the cursor to valid range

## Initial Navigation (newItemsModel is OK)

When navigating to a NEW items view (e.g., clicking a card, entering from sidebar), you create a fresh `itemsModel` and set context fields explicitly:

```go
items := newItemsModel(catalog.Loadouts, filtered, a.providers, a.catalog.RepoRoot)
items.ctx.sourceProvider = prov
items.ctx.parentLabel = "Loadouts"
items.ctx.hiddenCount = countHidden(a.catalog.ByType(catalog.Loadouts))
items.width = a.width - sidebarWidth - 1
items.height = a.panelHeight()
a.items = items
```

## Never Do This in Rebuild Handlers

```go
// WRONG — loses navigation context
items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
items.width = a.width
items.height = a.panelHeight()
a.items = items

// RIGHT — use rebuildItems
a.rebuildItems()
```
