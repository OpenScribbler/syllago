# Mouse Support Gap Fixes

Audit results and implementation plan for bringing all TUI interactive elements into
compliance with the mouse parity rule: **if you can reach it with the keyboard, you
can click it with the mouse.**

Audit performed 2026-03-12. Source: `.claude/rules/tui-mouse.md` (parity principle).

---

## What's Already Good

No gaps found in these files — all interactive elements are properly zone-marked:

- `app.go` — homepage cards, library cards, loadout cards, breadcrumbs
- `sidebar.go` — all sidebar nav items
- `items.go` — all list items
- `registries.go` — all registry cards
- `settings.go` — toggle rows
- `sandbox_settings.go` — all editable rows
- `update.go` — menu options
- `help_overlay.go` — read-only, nothing to click
- `detail_render.go` — tabs, file list, back link, provider checkboxes, action buttons

---

## Implementation Phases

### Phase 1 — `modal.go`: Text Input Fields

**Bead:** `romanesco-p84r`

Text input fields inside modals that are Tab-navigable but cannot be clicked to focus.

| Modal | Fields Missing Zone Marks |
|-------|--------------------------|
| `registryAddModal` | `urlInput`, `nameInput` |
| `saveModal` | `input` |
| `envSetupModal` | value input, location input, source input |
| `installModal` | `customPathInput` |

**Pattern:**
```go
zone.Mark("modal-field-url", inputStyle.Render(m.urlInput.View()))
zone.Mark("modal-field-name", inputStyle.Render(m.nameInput.View()))
```

Each modal's `Update()` must also handle `tea.MouseLeft` clicks on these zones to
call `Focus()` on the correct input model.

---

### Phase 2 — `modal.go`: Option Lists

**Bead:** `romanesco-7bdf`

**Depends on:** Phase 1 (same file — do in the same session)

Keyboard-navigable radio/option lists that have no zone marks and cannot be clicked.

| Modal | Options Missing Zone Marks |
|-------|--------------------------|
| `envSetupModal` | 2 radio options (new value / already configured) |
| `installModal` | 3 location options (Global / Project / Custom), 2 method options (Symlink / Copy) |

**Pattern:**
```go
zone.Mark(fmt.Sprintf("modal-opt-%d", i), row)
```

Each option click in `Update()` should move the cursor and select — same effect
as Up/Down + Enter.

> **Note:** Phases 1 and 2 are both in `modal.go` and should be implemented in the
> same session. Phase 2 is listed separately because it's a different element type,
> but there's no reason to do one without the other.

---

### Phase 3 — `loadout_create.go`: All Steps

**Bead:** `romanesco-x65a`

The entire loadout create wizard is missing mouse support for interactive elements.
Every step has gaps:

| Step | Missing |
|------|---------|
| `clStepProvider` | All provider list items |
| `clStepItems` | `searchInput` field, all checkbox items |
| `clStepName` | `nameInput`, `descInput` |
| `clStepDest` | All destination options |

**Patterns:**
```go
// Text inputs
zone.Mark("modal-field-name", inputStyle.Render(m.nameInput.View()))
zone.Mark("modal-field-desc", inputStyle.Render(m.descInput.View()))

// Option/list items
zone.Mark(fmt.Sprintf("modal-opt-%d", i), row)
```

Click handling in `Update()`:
- Text input click → `Focus()` the clicked input
- List item click → move cursor to clicked index
- Checkbox item click → move cursor + toggle checked state

---

### Phase 4 — `import.go`: Text Input Fields

**Bead:** `romanesco-ikma`

The import wizard's list options and action buttons are already zone-marked. Only
these three text input fields are missing:

| Step | Field |
|------|-------|
| `stepPath` | `pathInput` |
| `stepGitURL` | `urlInput` |
| `stepName` | `nameInput` |

**Pattern:**
```go
zone.Mark("import-field-path", m.pathInput.View())
zone.Mark("import-field-url", m.urlInput.View())
zone.Mark("import-field-name", m.nameInput.View())
```

Handle `tea.MouseLeft` in `Update()` to focus the clicked input.

---

### Phase 5 — `detail_render.go`: Loadout Mode Selector

**Bead:** `romanesco-hxmw`

The three loadout apply mode options (Preview / Try / Keep) in the Install tab are
keyboard-navigable via Up/Down/Enter but have zero zone marks.

**Pattern (in `detail_render.go`):**
```go
zone.Mark(fmt.Sprintf("detail-mode-%d", i), row)
```

**Click handler (in `detail.go` `Update()`):**
```go
for i := 0; i < numModes; i++ {
    if zone.Get(fmt.Sprintf("detail-mode-%d", i)).InBounds(msg) {
        m.loadoutModeCursor = i
    }
}
```

---

## Dependencies

```
Phase 1 (modal.go inputs)
    └── Phase 2 (modal.go options)   ← same file, same session

Phase 3 (loadout_create.go)         ← independent
Phase 4 (import.go)                 ← independent
Phase 5 (detail)                    ← independent
```

Phases 3, 4, and 5 are fully independent and can be picked up in any order.
Phases 1 and 2 share a file and should be done together.

---

## Testing Checklist

After each phase, verify in the running TUI:

- [ ] Clicking a text input field focuses it (cursor appears, typing works)
- [ ] Clicking a radio/option item selects it (cursor moves to that row)
- [ ] Clicking a checkbox item toggles it
- [ ] Tab still works as before (mouse didn't break keyboard nav)
- [ ] Golden tests pass (run `go test ./cli/internal/tui/ -update-golden` if visual output changed)
