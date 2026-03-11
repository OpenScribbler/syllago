---
paths:
  - "cli/internal/tui/keys.go"
  - "cli/internal/tui/app.go"
  - "cli/internal/tui/detail.go"
---

# Keyboard Binding Rules

## All key bindings live in keys.go

Define bindings in the `keyMap` struct, then check with `key.Matches`:

```go
// Right way — use binding from keys.go
case key.Matches(msg, keys.Install):

// Wrong way — don't hardcode key strings
case msg.String() == "i":
```

**Exception for BubbleTea key types:** `msg.Type == tea.KeyEnter`, `tea.KeyEsc`, `tea.KeyLeft`, `tea.KeyRight` are acceptable — these are type checks on BubbleTea's key enum, not string comparisons. They don't have `key.Binding` equivalents that work as well.

## Binding Conventions

- Navigation keys include vim alternatives: up/k, down/j, left/h, right/l
- Action keys are single lowercase letters: i=install, u=uninstall, c=copy, s=save, e=env, p=promote
- Uppercase letters for toggles: H=toggle hidden
- Confirm/cancel shortcuts: `keys.ConfirmYes` (y/Y), `keys.ConfirmNo` (n/N)

## Focus Priority

When `focus == focusModal`, all keyboard input goes to the active modal. App.Update() routes to modal.Update() first when a modal is active. Toast key handling (error toast: Esc/c) sits between modal and component routing.
