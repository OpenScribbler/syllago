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
- Actions: single lowercase — i=install, u=uninstall, c=copy, s=save, e=env, p=share
- Card/list pages: a=add {context}, r=remove {context}, s=sync (registries only)
- Help text must be context-specific: "a add skill", "r remove registry", not just "a add"
- Toggles: uppercase — H=toggle hidden
- Confirm/cancel: `keys.ConfirmYes` (y/Y), `keys.ConfirmNo` (n/N)

## Focus Priority

1. `focusModal` — ALL input goes to the active modal
2. Toast key handling — error toast: Esc/c; short success toast: any key dismisses (passes through); long scrollable success toast: Up/Down scroll, Esc dismisses
3. Search bar — when active, captures text input
4. `focusContent` / `focusSidebar` — normal page-level routing

## App-Level Key Interceptor Exclusions

App.Update() has global interceptors that run BEFORE screen-specific handlers (Tab toggle, sidebar Enter routing, search toggle, quit). These interceptors MUST exclude screens that handle those keys internally:

**Excluded screens (single-pane and wizard):** `screenImport`, `screenUpdate`, `screenSettings`, `screenSandbox`, `screenCreateLoadout`

When adding a new full-screen wizard or single-pane screen:
1. Add it to the Tab toggle exclusion list (~line 1869)
2. Add it to the sidebar Enter/Right routing exclusion list (~line 1887)
3. Add it to the search toggle exclusion list (~line 1755)
4. Verify with a test that Enter on the new screen doesn't trigger sidebar navigation

**Why this matters:** Without exclusions, app-level interceptors steal keys from screen-specific handlers. For example, Tab toggles sidebar focus, then Enter routes through the sidebar handler — triggering an import or navigation instead of advancing the wizard.

## Tab Behavior

Tab toggles focus between sidebar and content on ALL pages except:
- `screenDetail` — Tab switches between Files/Install tabs (Contents/Apply for loadouts) instead
- `screenImport`, `screenUpdate`, `screenSettings`, `screenSandbox`, `screenCreateLoadout` — single-pane/wizard screens, no Tab toggle

Card pages (Homepage, Library, Loadouts, Registries) ALL support Tab focus toggling.

## Create Loadout Screen

The create loadout wizard (`screenCreateLoadout`) has step-specific bindings:

| Step | Key | Action |
|------|-----|--------|
| Provider | up/down | Navigate providers |
| Provider | enter | Select provider |
| Types | up/down | Navigate type checkboxes |
| Types | space | Toggle type |
| Types | enter | Advance to items |
| Items | up/down | Navigate items |
| Items | space | Toggle item selection |
| Items | a | Toggle all items |
| Items | t | Toggle compatibility filter |
| Items | / | Search items |
| Items | h/l | Switch pane focus (split view) |
| Items | enter | Advance to next type or step |
| Name | tab | Switch between name/description fields |
| Name | enter | Advance to destination |
| Dest | up/down | Navigate destination options |
| Dest | enter | Advance to review |
| Review | left/right | Switch Back/Create buttons |
| Review | enter | Confirm selected button |
| All | esc | Go back one step (first step exits) |

## Search Availability

`/` activates search on: Homepage, Items, Library Cards, Loadout Cards, Registries, Detail.
Not available on: Import, Update, Settings, Sandbox, Create Loadout (form-based or wizard screens where `/` could conflict or is handled internally).

## Help Overlay

`?` opens help overlay on any screen. The overlay MUST have a section for every screen type — no gaps.
