# TUI Phase 7: Edit Modal + Help Overlay

Two deliverables: upgrade the single-field rename modal to a two-field edit modal (name + description), and add a full-screen help overlay showing all keyboard shortcuts.

**Depends on:** Phases 1-6 (all complete). Install/uninstall wizard and loadout apply/remove deferred to Phase 7b.

---

## Part A: Edit Modal

### What Changes

`[r] Rename` becomes `[e] Edit` everywhere: key binding, helpbar hints, metapanel button label. The modal gains a second field for description. Both fields save to `.syllago.yaml` via the metadata package.

### Behavior

- **Key:** `e` opens the edit modal for the selected item
- **Fields:** Display name (line 1), Description (line 2, taller)
- **Pre-fill:** Load current values from `.syllago.yaml` (metadata.Load)
- **Save:** Write both fields to `.syllago.yaml` (metadata.Save), update catalog in-place
- **Focus:** Tab cycles: name field -> description field -> Cancel -> Save
- **Submit:** Enter on Save button or Ctrl+S from any field. Enter in text fields moves to next field (not submit)
- **Cancel:** Esc or Enter on Cancel button
- **Contexts:** Works from Library browse, Library detail, Explorer, Gallery drill-in

### Layout

```
╭──────────────────────────────────────────────────╮
│ Edit: my-skill-name                              │
│                                                  │
│ Display Name                                     │
│ ┌──────────────────────────────────────────────┐ │
│ │ My Skill Name█                               │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ Description                                      │
│ ┌──────────────────────────────────────────────┐ │
│ │ Reviews Go code for common patterns and      │ │
│ │ suggests improvements.                       │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│                          [Cancel]  [Save]        │
╰──────────────────────────────────────────────────╯
```

- Width: 56 chars (wider than current 50 to fit description)
- Height: 14 lines (name field 1 line, description field 2 lines)
- Description field: 2 visible lines, single-line input (no multiline editing needed for v1)

### Implementation

**Modified files:**
- `keys.go` — `keyRename` -> `keyEdit`, value `"r"` -> `"e"`
- `modal.go` — Replace `textInputModal` with `editModal` supporting two fields
- `app.go` — Update `handleRename` -> `handleEdit`, `handleModalSaved` saves both name + description
- `metapanel.go` — `[r] Rename` label -> `[e] Edit`
- `helpbar hints` in `app.go` — `"r rename"` -> `"e edit"`

**Message types:**
- `editSavedMsg{name, description, path string}` — replaces `modalSavedMsg`
- `editCancelledMsg{}` — replaces `modalCancelledMsg`

---

## Part B: Help Overlay

### What Changes

Pressing `?` shows a full-screen overlay listing all keyboard shortcuts, organized by section. The overlay dismisses with `?` or `Esc`.

### Layout

```
╭── Keyboard Shortcuts ────────────────────────────────────────────────╮
│                                                                      │
│  Navigation                        Actions                           │
│  ─────────                         ───────                           │
│  1 / 2 / 3    Switch group         a          Add content            │
│  h / l        Cycle sub-tabs       n          Create new             │
│  j / k        Move up/down         e          Edit name/description  │
│  Enter        Select / drill in    /          Search                 │
│  Esc          Back / close         s / S      Sort / reverse sort    │
│  q            Back / quit          R          Refresh catalog        │
│                                                                      │
│  File Tree (detail view)           Gallery                           │
│  ────────────────────────          ───────                           │
│  h / l        Switch pane          arrows     Navigate grid          │
│  Enter        Open file            Tab        Grid / contents        │
│  Esc          Close detail         Enter      Drill into card        │
│                                                                      │
│                                                                      │
│                       Press ? or Esc to close                        │
╰──────────────────────────────────────────────────────────────────────╯
```

### Implementation

**New file:** `help.go` — `helpOverlay` model with `active bool`, `width/height int`
- Renders a bordered box with shortcut sections in two columns
- No scrolling needed (content fits in 20 lines)
- Uses existing styles (primaryColor for headers, mutedColor for separators)

**Modified files:**
- `app.go` — Add `helpOverlay` to App struct, wire `?` key, overlay rendering
- `keys.go` — Add `keyHelp = "?"`

---

## Deferred to Phase 7b

- Install/uninstall wizard (requires installer integration)
- Loadout apply/remove confirmation
- Toast notifications for save success/failure
