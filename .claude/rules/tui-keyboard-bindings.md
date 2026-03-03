---
paths:
  - "cli/internal/tui/keys.go"
  - "cli/internal/tui/app.go"
  - "cli/internal/tui/detail.go"
---

# Keyboard Binding Rules

## All key bindings live in keys.go

Never hardcode key strings like `"i"`, `"enter"`, or `"esc"` in Update methods. Instead:
- Define bindings in the `keyMap` struct in keys.go
- Check with `key.Matches(msg, keys.Foo)` in Update handlers
- Exception: `msg.Type == tea.KeyEnter`, `tea.KeyEsc`, `tea.KeyLeft`, `tea.KeyRight` are acceptable for bubbletea key types (these are type checks, not string comparisons)

## Binding Conventions

- Navigation keys include vim alternatives: up/k, down/j, left/h, right/l
- Action keys are single lowercase letters: i=install, u=uninstall, c=copy, s=save, e=env, p=promote
- Uppercase letters for toggles: H=toggle hidden
- Modifiers for secondary actions: shift+tab, ctrl+n/p

## Focus Priority

When `focus == focusModal`, ALL keyboard input goes to the active modal. Component Update methods must check focus state before handling keys. App.Update() enforces this by routing to modal.Update() first when a modal is active.
