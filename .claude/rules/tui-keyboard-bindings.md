---
paths:
  - "cli/internal/tui/**"
---

# Keyboard Binding Rules

## All key bindings live in keys.go

Define bindings in the `keyMap` struct, then check with `key.Matches`:

```go
// Right — use binding from keys.go
case key.Matches(msg, keys.Install):

// Wrong — don't hardcode key strings
case msg.String() == "i":
```

**Exception for BubbleTea key types:** `msg.Type == tea.KeyEnter`, `tea.KeyEsc`, `tea.KeyLeft`, `tea.KeyRight` are acceptable — these are type checks on BubbleTea's key enum, not string comparisons.

## Binding Conventions

- Navigation: up/k, down/j, left/h, right/l (vim alternatives)
- Actions: single lowercase — i=install, u=uninstall, c=copy, s=save, e=env, p=promote
- Card pages: a=add, d=remove, r=sync
- Toggles: uppercase — H=toggle hidden
- Confirm/cancel: `keys.ConfirmYes` (y/Y), `keys.ConfirmNo` (n/N)

## Focus Priority

1. `focusModal` — ALL input goes to the active modal
2. Toast key handling — error toast: Esc/c; success toast: any key dismisses (passes through)
3. Search bar — when active, captures text input
4. `focusContent` / `focusSidebar` — normal page-level routing

## Tab Behavior

Tab toggles focus between sidebar and content on ALL pages except:
- `screenDetail` — Tab switches between Overview/Files/Install tabs instead
- `screenImport`, `screenUpdate`, `screenSettings`, `screenSandbox` — single-pane screens, no Tab toggle

Card pages (Homepage, Library, Loadouts, Registries) ALL support Tab focus toggling.

## Search Availability

`/` activates search on: Homepage, Items, Library Cards, Loadout Cards, Registries, Detail.
Not available on: Import, Update, Settings, Sandbox (form-based screens where `/` could conflict).

## Help Overlay

`?` opens help overlay on any screen. The overlay MUST have a section for every screen type — no gaps.
