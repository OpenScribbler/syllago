# TUI Messaging Patterns — Error, Success, Loading, and UX Guidance

How popular Go TUI apps handle user feedback: errors, success, warnings, loading states, empty states, confirmations, and contextual help. Conducted 2026-03-24.

---

## The Messaging Hierarchy

Every surveyed project uses a severity-based hierarchy. Messages escalate from least to most disruptive:

| Level | Mechanism | Disruption | Example |
|-------|-----------|-----------|---------|
| 1. Inline indicator | Status icon/spinner next to the item | None | Spinner next to "alpha-skill" during install |
| 2. Toast/flash | Brief message in status bar or overlay | Low — auto-dismisses | "Done: alpha-skill installed to Claude Code" |
| 3. Persistent error | Stays visible until dismissed or navigated away | Medium | "Error: permission denied — check directory permissions" |
| 4. Modal alert | Centered overlay, must dismiss | High | Unexpected error with diagnostic details |
| 5. Confirmation modal | Must choose before proceeding | Highest | "Uninstall alpha-skill? This removes the symlink." |
| 6. Type-to-confirm | Must type name/string to proceed | Maximum | k9s's `ShowConfirmAck` for cluster-level destructive ops |

**Key principle:** Use the least disruptive mechanism that still ensures the user gets the information they need.

---

## How Each Project Does It

### Lazygit — Four-Layer System

Lazygit has the most complete messaging model:

**Toasts** — `PopupHandler.Toast(message)` / `PopupHandler.ErrorToast(message)`
- Two severity variants: `ToastKindStatus` (neutral) and `ToastKindError` (red/bold)
- Non-blocking, auto-disappear, appear at bottom
- Used for low-importance feedback: "disabled keybinding", "copied to clipboard"

**Loading/Waiting** — `WithWaitingStatus(message, func() error)`
- Status bar shows spinner + message: "Fetching...", "Pushing..."
- Sync variant blocks UI; async doesn't
- Per-item inline indicators for individual operations (pushing, pulling, fetching)

**Errors** — `Alert(title, message)` / `ErrorHandler(err)`
- Full modal popup for serious/unexpected errors
- Red + bold text styling
- Blocks until dismissed
- Distinguishes between "show as toast" vs "show as panel" based on error type

**Confirmations** — `Confirm(ConfirmOpts)` / `ConfirmIf(condition, opts)`
- `ConfirmIf` is smart: if the condition is false, executes directly without showing UI
- Useful for "if count > N, ask; otherwise just do it"

### K9s — Channel-Based Flash with Structured Severity

**Flash messages** — channel-based, decoupled from UI:
```go
type LevelMessage struct {
    Level FlashLevel  // FlashInfo, FlashWarn, FlashErr
    Text  string
}
const DefaultFlashDelay = 6 * time.Second
```

- Auto-clear after 6 seconds
- New messages cancel previous auto-clear timer (each gets full duration)
- Color-coded: info=default, warn=yellow, err=red
- Channel architecture lets any component send messages without knowing about the view

**Dialogs** — six specialized functions:
- `ShowConfirm` — standard y/n
- `ShowConfirmAck` — type-to-confirm (dangerous actions require typing the resource name)
- `ShowDelete` — dedicated delete dialog with distinct styling
- `ShowError` — error display modal
- `ShowPrompt` — text input dialog
- `ShowSelection` — pick from a list

**Empty state bug** — k9s issue #3267: resource count shows `0` both while loading AND when truly empty. Users can't tell which. This is the #1 empty state anti-pattern to avoid.

### Superfile — Bubble Tea Command Pattern

Migrated from goroutine channels to Bubble Tea commands (PR #979). Key insight: **notifications are `tea.Msg` values, not channel events.** File operation completes → sends a message → `Update()` handles it → notify model renders the result.

Progress bars for file operations (copy, paste) render inline in the footer panel.

### huh — Two-Layer Validation

- **Inline indicator**: red `*` appended to the field title when validation fails
- **Footer error messages**: raw `err.Error()` strings at the group footer, replacing help text
- No field-level error text — only the indicator symbol inline, full message in footer
- Validation triggers on blur (leaving a field) or submit, not as-you-type

### bubbles/list — Built-In Status Messaging

- `NewStatusMessage(s string)` — transient notification in the **title area** (not status bar)
- Auto-clears after `StatusMessageLifetime` (default 1 second)
- Empty state: `"No [itemNamePlural]."` using `Styles.NoItems`
- Filtered empty: `"Nothing matched"` using `Styles.StatusEmpty`
- Built-in spinner via `StartSpinner()` / `StopSpinner()`

### gh-dash — Minimal, Footer-Based

- Status messages in the list pager area at the bottom
- `SelectedBackground` fill on the pager bar
- Shows: `"Updated ~2m ago * PR 3/42 (fetched 20)"`
- No visible error modal system — errors likely surface as status text

---

## Error Message Quality

From clig.dev and actual implementations:

### The Two-Part Rule

Every error should answer: **what happened** + **what to do about it**

| Bad | Good |
|-----|------|
| `"permission denied"` | `"Can't write to ~/.claude/rules/alpha-skill — check directory permissions"` |
| `"network error"` | `"Can't reach github.com/team/rules — check your network connection or try again"` |
| `"invalid config"` | `"Config file has invalid JSON at line 12 — run 'syllago config validate' to see details"` |

### Error Codes

Tools like Xcode use artificial codes (e.g., `HE0030`) so users can search for help. Syllago already has structured error codes in `cli/internal/output/errors.go`. **Include the code in the visible error text**, not just logs — users need it to search.

### Actionable vs Informational

| Type | Pattern | Example |
|------|---------|---------|
| **Actionable** | Include the fix | `"Registry not found. Run 'syllago registry add <url>' to add it."` |
| **Informational** | Include context | `"Hook has partial compatibility with Gemini CLI — matcher syntax differs"` |
| **Unexpected** | Include report path | `"Unexpected error (PRIV-001). Report at github.com/syllago/issues"` |

### Group Repeated Errors

Don't repeat the same error 20 times. Say `"20 items failed: permission denied"` once.

---

## Loading States

### The Three-State Pattern (REQUIRED)

Every data-driven view must distinguish three states explicitly:

```go
type model struct {
    loading bool
    items   []Item
    err     error
}

func (m model) View() string {
    switch {
    case m.err != nil:
        return renderError(m.err)
    case m.loading:
        return m.spinner.View() + " Loading..."
    case len(m.items) == 0:
        return renderEmptyState()
    default:
        return renderList(m.items)
    }
}
```

**Why:** Without the `loading` flag, `len(items) == 0` is ambiguous — is it still loading or truly empty? K9s has this exact bug (issue #3267).

### Spinner Mechanics

- Use `bubbles/spinner` with descriptive text
- Text carries the meaning, spinner is just a liveness indicator
- Stop forwarding `spinner.TickMsg` once `loading = false` to prevent unnecessary redraws
- For long operations, consider progress text: `"Installing 3 of 7 items..."`

### Inline Loading per Item

For per-item operations (install, uninstall), show a spinner next to the item in the list rather than a global loading state. Lazygit does this with `ItemOperation` tracking.

---

## Empty States

Four types, each with different messaging:

| Type | When | Pattern |
|------|------|---------|
| **First use** | Nothing configured yet | Explain + guide: `"No skills yet. Press 'a' to add your first one."` |
| **User cleared** | Deleted everything | Confirm the result: `"All items removed."` |
| **Search empty** | Filter matched nothing | Suggest: `"No matches for 'xyz'. Press Esc to clear search."` |
| **Error empty** | Fetch failed | Show the error, NEVER show as empty: `"Failed to load: [reason]"` |

**Empty states are valuable real estate.** Never leave them blank. Always include guidance on what the user can do.

### First-Run Experience

When no registries are configured and the library is empty:
```
No content yet.

Get started:
  'a'  Add content from a provider, file, or URL
  '2'  Browse registries to find shared content

Or run: syllago registry add <url>
```

---

## Confirmation Patterns

### Severity Tiers

| Severity | Pattern | Example |
|----------|---------|---------|
| **Mild** | Optional y/n, default=cancel | Remove a single item |
| **Moderate** | Required confirmation | Uninstall from provider, remove registry |
| **Severe** | Type-to-confirm | Delete loadout with installed dependencies |

### Design Rules

- **Default focus on the safe option** (Cancel), not the destructive one
- **Descriptive button labels**: `"Uninstall"` and `"Cancel"`, not `"OK"` and `"Cancel"`
- **Escape always dismisses** (same as cancel)
- **Include consequences**: `"This will remove the symlink from Claude Code. The content stays in your library."`
- **`ConfirmIf` pattern**: Skip the dialog when the operation is low-risk. If there's only one provider to uninstall from, just do it. If there are multiple, ask.

### Undo as Alternative

For reversible actions, undo is better UX than a blocking confirm:
```
Uninstalled alpha-skill from Claude Code. Press 'z' to undo.
```
Reduces interruption while maintaining safety.

---

## Contextual Help / Guidance

### Three Levels of Help

**Level 1 — Footer hints (always visible)**

Context-sensitive, changes per screen and focus state:
```
j/k navigate * h/l pane * / search * i install * ? help          syllago v1.2.0
```

When hints are too long, drop lower-priority ones. `?` is always shown.

**Level 2 — Help overlay (? key)**

Full keyboard reference for the current screen. Replaces content area (sidebar stays). Scrollable. Uses `bubbles/help` pattern:
- `ShortHelp()` — 3-4 most important keys for the footer
- `FullHelp()` — all keys grouped by category for the overlay

Disabled bindings are automatically excluded — context-sensitive without conditional logic.

**Level 3 — In-context guidance**

Empty states include what to do. Modals include what will happen. Error messages include how to fix. This is not a separate component — it's a quality standard for every message in the app.

### Progressive Disclosure

- Footer shows the top 3-4 keys
- `?` reveals all keys
- Hovering/selecting an item reveals item-specific actions in the metadata bar
- Opening a modal explains what the modal does

---

## Toast Design Specifics

### Three Toast Types

| Type | Color | Prefix | Auto-dismiss | Other keys |
|------|-------|--------|-------------|------------|
| **Success** | Green | `Done:` | 3 seconds | Any key dismisses + passes through |
| **Warning** | Amber | `Warn:` | 5 seconds | Any key dismisses + passes through |
| **Error** | Red | `Error:` | Never | Pass through (toast stays visible) |

### Interaction Rules

- `c` copies toast message to clipboard (all types)
- `Esc` dismisses (all types)
- Success/warning: any keypress dismisses AND the key passes through to the underlying UI
- Error: persists until `Esc` or `c` — other keys pass through while toast stays
- Multi-line toasts (batch operations): scrollable with Up/Down, show `(N more below)`

### Message Quality

- Include the item name: `"Done: alpha-skill installed to Claude Code"` not just `"Install complete"`
- Include the target: `"to Claude Code"` not just `"installed"`
- For batch operations: `"Done: Imported 5 items from team-rules"` with details below
- For errors: include the specific error + what to do: `"Error: Permission denied — check ~/.claude/rules/ permissions"`

---

## Summary: Syllago Messaging Design Rules

1. **Use the least disruptive mechanism.** Inline > toast > persistent > modal > confirmation.
2. **Every error answers two questions:** what happened and what to do about it.
3. **Distinguish loading from empty.** Use the three-state pattern (loading/empty/results).
4. **Empty states are guidance, not void.** Always tell the user what they can do.
5. **Default to the safe option** in confirmations. Cancel is the default.
6. **Include specifics in messages.** Item name, target provider, operation performed.
7. **Group repeated messages.** Don't repeat the same error N times.
8. **Toasts auto-dismiss for success, persist for errors.** Users need time to read errors.
9. **Everything is copyable.** `c` copies any visible message to clipboard.
10. **Help is progressive.** Footer hints → ? overlay → in-context guidance.

---

## Sources

### Projects
- [lazygit](https://github.com/jesseduffield/lazygit) — PopupHandler, Toast, WithWaitingStatus
- [k9s](https://github.com/derailed/k9s) — Flash model, dialog system, ShowConfirmAck
- [superfile](https://github.com/yorukot/superfile) — notification model migration (PR #979)
- [huh](https://github.com/charmbracelet/huh) — two-layer validation, ErrorIndicator
- [bubbles/list](https://github.com/charmbracelet/bubbles) — NewStatusMessage, empty states
- [bubbles/help](https://github.com/charmbracelet/bubbles) — ShortHelp/FullHelp progressive disclosure

### Articles
- [CLI Guidelines (clig.dev)](https://clig.dev/) — error message quality, confirmation patterns
- [UX Patterns for CLI Tools (Lucas Costa)](https://lucasfcosta.com/2022/06/01/ux-patterns-cli-tools.html)
- [Atlassian CLI Design Principles](https://www.atlassian.com/blog/it-teams/10-design-principles-for-delightful-clis)
- [Charm blog: 100k](https://charm.land/blog/100k/) — developer delight philosophy
- [k9s empty state issue #3267](https://github.com/derailed/k9s/issues/3267)
