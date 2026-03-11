# TUI Component Rules

This directory is the BubbleTea terminal UI for syllago. All components follow strict conventions to maintain visual and behavioral consistency.

## Architecture

Root model is `App` (app.go) — central event hub and state orchestrator. Sub-models own their state and are composed into App. Messages flow upward to App, which dispatches back down.

**File organization:**
- One model per file (sidebar.go, items.go, detail.go, modal.go, etc.)
- Rendering split from logic when large (detail.go + detail_render.go)
- All styles in styles.go — never define colors or styles inline in component files
- All key bindings in keys.go — never hardcode key strings in Update methods

## Color and Styling

**All colors are defined once in styles.go using `lipgloss.AdaptiveColor` for light/dark themes.** Never create new color values outside styles.go.

Color palette:
- `primaryColor` (mint) — titles, labels, headings
- `accentColor` (viola/purple) — selection highlight, active tabs, buttons
- `mutedColor` (stone/gray) — help text, inactive elements, separators
- `successColor` (green) — installed status, success messages
- `dangerColor` (red) — error messages
- `warningColor` (amber) — warnings, global badge, update banner

**Style rules:**
- Use existing named styles (titleStyle, labelStyle, valueStyle, helpStyle, etc.) — don't create one-off lipgloss.NewStyle() calls for colors that already have a style
- No emojis in UI — use colored symbols instead (✓, ✗, ▸, >, ─, ←, ⚠)
- Padding is applied via lipgloss styles, not manual spaces (except for list indentation with cursor prefix)

## Selection Cursor Pattern

All navigable lists use the same cursor rendering:

```go
prefix := "  "
style := itemStyle
if i == cursor {
    prefix = "> "
    style = selectedItemStyle
}
entry := fmt.Sprintf("  %s%s", prefix, style.Render(text))
```

- `"> "` for selected item, `"  "` for unselected (2-char prefix)
- `selectedItemStyle` applies bold + accent color + selected background
- Always indent with 2 leading spaces before the prefix (total 4 chars for unselected)

## Modal Conventions

All modals share these structural rules:

**Dimensions:**
- Simple dialogs (confirm, save): `modalWidth = 40`
- Complex wizards (install, env setup): `modalWidth = 56`, `modalHeight` as needed
- Use fixed dimensions per modal type to prevent jitter between steps

**Style:**
```go
modalStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(modalBorderColor).
    Background(modalBgColor).
    Padding(1, 2).
    Width(modalWidth).
    Height(modalHeight) // if fixed height needed
```

**Buttons:**
- Use `renderButtons(left, right string, cursor, contentWidth int)` for all two-button modal footers
- Active button: `▸ ` prefix + `buttonStyle` (white on viola)
- Inactive button: `  ` prefix + `buttonDisabledStyle` (muted on gray)
- Buttons are pinned to the bottom of the modal using spacer lines:

```go
contentLines := strings.Count(content, "\n")
spacer := innerHeight - contentLines - 1
if spacer < 0 { spacer = 0 }
content += strings.Repeat("\n", spacer) + buttons
```

**Overlay:**
- Every modal implements `overlayView(background string) string`
- Uses `overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)`
- Click-away dismissal: clicks outside modal-zone dismiss it; clicks inside do NOT dismiss

**Keyboard in modals:**
- `Enter` confirms (when button cursor is on confirm)
- `Esc` cancels/goes back
- `Left/Right` arrows switch between buttons
- `Up/Down` navigate options within the modal
- Confirm modal also supports `y/Y` and `n/N` shortcuts

**Multi-step wizards:**
- Track step with typed enum (e.g., `installStep`, `envSetupStep`)
- Show progress indicator: `"(N of M)"` for sequential steps
- `Esc` goes back one step (not dismiss), except on first step where it dismisses
- Each step uses the same fixed modal dimensions

## Keyboard Handling

**All key bindings are defined in keys.go** as `key.Binding` objects in the global `keys` keyMap struct. Check keys with `key.Matches(msg, keys.Foo)`.

- Navigation: up/k, down/j, home/g, end/G, pgup, pgdown
- Vim-like alternatives are built into the bindings (hjkl)
- Tab switching: tab, shift+tab (cycle), 1/2/3 (jump directly)
- Actions are single letters: i=install, u=uninstall, c=copy, s=save, e=env, p=promote, a=add, d=delete, r=refresh, l=create loadout
- Global: q=quit, ctrl+c=quit, /=search, ?=help overlay, esc=back

**When focus is `focusModal`, all keyboard input goes to the active modal.** Other components must not handle keys when a modal is active.

**Active key bindings by screen:**

| Key | Category | Items | Library Cards | Loadout Cards | Registries | Detail |
|-----|----------|-------|---------------|---------------|------------|--------|
| a   | --       | add   | add           | create loadout| add registry| --    |
| d   | --       | --    | --            | --            | remove      | --    |
| r   | --       | --    | --            | --            | sync        | --    |
| l   | --       | create loadout (registry context) | -- | -- | -- | --    |
| H   | toggle hidden | toggle hidden | -- | --       | --          | --    |

## Mouse Handling

- Use `zone.Mark(id, content)` to register clickable regions
- Zone IDs follow patterns: `"tab-N"`, `"file-N"`, `"prov-check-N"`, `"detail-btn-action"`, `"crumb-home"`, `"crumb-category"`
- Modal button click detection uses coordinate math within the `"modal-zone"` bounds
- Mouse wheel scrolls content in the focused component

## Help Text

**Footer help bar** (global, rendered by App) shows context-sensitive key hints:
- Format: `"key action"` pairs joined with `" • "` separator
- Example: `"esc back • tab switch tab • up/down scroll • c copy"`
- Each component provides its help via a `renderHelp() string` method
- Help overlay (`?` key) shows full keyboard shortcuts organized by screen

## Status Messages

- Transient feedback: `message string` + `messageIsErr bool` on the model
- Rendered with `successMsgStyle` or `errorMsgStyle`
- Cleared on next keypress (not timer-based)
- Rendered outside the scrollable area so always visible

## Layout Rules

**Height calculations must account for borders:**
```go
contentHeight -= 2  // top + bottom borders
```

**Text truncation is explicit — never rely on terminal auto-wrap:**
```go
maxTextWidth := panelWidth - 4  // -2 borders, -2 padding
title = truncateString(title, maxTextWidth)
```

**Scroll handling:**
- Track `scrollOffset int` on the model
- Implement `clampScroll()` to prevent scrolling past content bounds
- Both keyboard (up/down/pgup/pgdown) and mouse wheel supported
- Reset scroll offset to 0 when switching tabs or navigating to new content

## Testing

**Golden file tests** compare rendered output against expected baselines in `testdata/`:

- Component tests: `component-*.golden`
- Full app tests: `fullapp-*.golden`
- Size variants: `-60x20`, `-120x40`, `-160x50` for responsive testing
- Run with `-update-golden` flag to regenerate baselines
- Path normalization: temp paths → `<TESTDIR>`, trailing whitespace stripped

**After any visual change, update golden files:**
```bash
go test ./cli/internal/tui/ -update-golden
```
Then verify the diff looks correct before committing.

## Message Passing

- Sub-models request actions by returning `tea.Cmd` functions that produce typed messages
- Modal requests: `openModalMsg`, `openInstallModalMsg`, `openSaveModalMsg`, `openEnvModalMsg`
- Async operations return result messages: `appInstallDoneMsg`, `promoteDoneMsg`, `importCloneDoneMsg`
- App.Update() receives all messages and routes to the appropriate handler
- Never send messages between sibling components directly — always through App

## Adding New Components

1. Create the model struct with `Update(tea.Msg)` and `View() string` methods
2. Add styles to styles.go (never inline)
3. Add key bindings to keys.go if needed
4. Add a new `screen` enum value if it's a full screen
5. Wire it into App.Update() message routing and App.View() rendering
6. Add golden file tests for visual output
7. Add `renderHelp()` method returning context-sensitive help text
