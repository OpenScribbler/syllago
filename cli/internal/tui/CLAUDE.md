# TUI Component Rules

This directory is the BubbleTea terminal UI for syllago. All components follow strict conventions to maintain visual and behavioral consistency.

**Design spec:** `docs/design/tui-spec.md` — comprehensive human-readable reference covering all pages, components, and interaction patterns.

**Claude rules:** `.claude/rules/tui-*.md` — machine-enforced rules that auto-load when touching any file in this directory. These cover cards, modals, keyboard, mouse, scroll, styles, responsive layout, text handling, pages, and testing.

## Before You Edit

Run this checklist before modifying any TUI file:

1. **Read `styles.go`** — all colors and named styles are defined there. If you need a color, it already exists or should be added there.
2. **Check `keys.go`** — all key bindings are defined as `key.Binding` objects. Use `key.Matches(msg, keys.Foo)` instead of `msg.String() == "x"`.
3. **Use shared helpers** — `pagehelpers.go` provides `renderBreadcrumb()`, `renderStatusMsg()`, `cursorPrefix()`, `renderScrollUp/Down()`, `renderDescriptionBox()`. Don't reimplement these patterns.
4. **Review the design rules** — `.claude/rules/tui-*.md` files define required patterns for cards, modals, scroll, mouse, etc.
5. **Run golden tests after visual changes** — `go test ./cli/internal/tui/ -update-golden`, then review the diff.
6. **Test with the large dataset** — use `testAppLarge(t)` (85+ items) to verify overflow, scroll, and truncation. Use `testAppEmpty(t)` for empty states.
7. **Test at multiple sizes** — verify at 60x20, 80x30, 120x40, and 160x50.

## Architecture

Root model is `App` (app.go) — central event hub and state orchestrator. Sub-models own their state and are composed into App. Messages flow upward to App, which dispatches back down.

**File organization:**
- One model per file (sidebar.go, items.go, detail.go, modal.go, etc.)
- Rendering split from logic when large (detail.go + detail_render.go)
- All styles in styles.go — never define colors or styles inline
- All key bindings in keys.go — never hardcode key strings in Update methods

## Shared Helpers (pagehelpers.go)

Every page model should use these instead of reimplementing:

```go
// Breadcrumb navigation — variadic segments, last one is title style
renderBreadcrumb(
    BreadcrumbSegment{"Home", "crumb-home"},
    BreadcrumbSegment{"Skills", "crumb-category"},
    BreadcrumbSegment{"my-skill", ""},  // empty ZoneID = final segment
)

// Cursor prefix for navigable lists
prefix, style := cursorPrefix(i == cursor)
// Returns ("> ", selectedItemStyle) or ("  ", itemStyle)

// Scroll indicators
renderScrollUp(count, isContentView)   // "(N more above)"
renderScrollDown(count, isContentView) // "(N more below)"

// Description box with fixed height (prevents jitter)
renderDescriptionBox(text, width, maxLines)
```

## Toast System (toast.go)

All transient feedback (success/error messages) goes through the centralized toast overlay. Components do not render messages inline.

**How it works:** Components set `message`/`messageIsErr` fields. After dispatching Update, App promotes these to the toast via `promoteDetailMessage()`, `promoteSettingsMessage()`, etc.

```go
// In your component's save/action method:
m.message = "Settings saved"
m.messageIsErr = false

// App handles promotion and rendering — you don't render messages in View()
```

**Error toasts** are semi-modal: `Esc` dismisses, `c` copies sanitized text to clipboard. Other keys pass through.
**Success toasts** dismiss on any keypress without consuming the key.

## Color and Styling

All colors are defined in `styles.go` using `lipgloss.AdaptiveColor` for light/dark themes.

Color palette:
- `primaryColor` (mint) — titles, labels, headings
- `accentColor` (viola/purple) — selection highlight, active tabs, buttons
- `mutedColor` (stone/gray) — help text, inactive elements, separators
- `successColor` (green) — installed status, success messages
- `dangerColor` (red) — error messages
- `warningColor` (amber) — warnings, global badge, update banner

**Adding a new color:**
```go
// In styles.go — define the adaptive color pair
var newColor = lipgloss.AdaptiveColor{Light: "#hexLight", Dark: "#hexDark"}

// Then create a named style that uses it
var newStyle = lipgloss.NewStyle().Foreground(newColor)
```

**Style rules:**
- Use existing named styles (titleStyle, labelStyle, valueStyle, helpStyle, etc.)
- No emojis in UI — use colored symbols instead (checkmark, X, >, dash, arrow, warning)
- Padding is applied via lipgloss styles, not manual spaces (except for list indentation with cursor prefix)

## Selection Cursor Pattern

All navigable lists use the shared `cursorPrefix()` helper:

```go
prefix, style := cursorPrefix(i == cursor)
entry := fmt.Sprintf("  %s%s", prefix, style.Render(text))
```

- `"> "` for selected, `"  "` for unselected (2-char prefix)
- `selectedItemStyle` applies bold + accent color + selected background
- Always indent with 2 leading spaces before the prefix

## Modal Conventions

**Dimensions:**
- **All modals use `modalWidth = 56`** — one standard size, no exceptions
- Use fixed height when buttons are pinned to bottom (prevents jitter between steps)

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
- Active button: `buttonStyle` (white on viola). Inactive: `buttonDisabledStyle` (muted on gray)
- Buttons are pinned to bottom using spacer lines

**Overlay:**
- Every modal implements `overlayView(background string) string`
- Uses `overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)`
- Click-away dismissal: clicks outside modal-zone dismiss; clicks inside do not

**Keyboard in modals:**
- `Enter` confirms, `Esc` cancels/goes back
- `Left/Right` switch between buttons, `Up/Down` navigate options
- Confirm modal supports `y/Y` and `n/N` shortcuts via `keys.ConfirmYes`/`keys.ConfirmNo`
- When `focusModal` is active, modals take priority over all other key handling

## Keyboard Handling

All key bindings are defined in `keys.go` as `key.Binding` objects:

```go
// Right way — use the binding from keys.go
case key.Matches(msg, keys.Up):

// Wrong way — don't hardcode key strings
case msg.String() == "k":
```

**Exception:** `msg.Type == tea.KeyEnter` and `msg.Type == tea.KeyEsc` are acceptable for BubbleTea special key types that don't map cleanly to key.Binding.

**Active key bindings by screen:**

| Key | Category | Items | Library Cards | Loadout Cards | Registries | Detail |
|-----|----------|-------|---------------|---------------|------------|--------|
| a   | --       | add {type} | add content | create loadout | add registry | -- |
| r   | --       | remove {type} (library only) | -- | -- | remove registry | remove {type} (library only) |
| s   | --       | --    | --            | --            | sync registry | --    |
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
- Each component provides its help via a `helpText()` method
- Help overlay (`?` key) shows full keyboard shortcuts organized by screen

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

## Accessibility

- `NO_COLOR=1` is supported automatically via the Charm stack
- All status indicators use text+symbol alongside color: "Done: ..." with green, "Error: ..." with red
- Meaning is never color-only — symbols and text prefixes carry the semantics

## Testing

**Golden file tests** compare rendered output against baselines in `testdata/`:
- Component tests: `component-*.golden`
- Full app tests: `fullapp-*.golden`
- Size variants: `-60x20`, `-120x40`, `-160x50` for responsive testing
- Overflow tests: `fullapp-*-overflow*.golden` (large dataset)
- Empty tests: `fullapp-*-empty*.golden` (empty catalog)

**After any visual change:**
```bash
go test ./cli/internal/tui/ -update-golden
# Then: git diff cli/internal/tui/testdata/ to verify changes are intentional
```

**Boundary condition checklist:**
- What happens with 50+ items? Use `testAppLarge(t)`.
- What happens with 0 items? Use `testAppEmpty(t)`.
- What happens at 60x20 terminal? Use `testAppLargeSize(t, 60, 20)`.
- What happens with very long text (200+ chars)?

**Test structure:**
- Table-driven tests with `t.Run()` subtests
- Test both keyboard and mouse interactions
- Modal tests: open, navigate, confirm/cancel, state after close
- Always test Esc dismissal and click-away

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
6. Add golden file tests for visual output, including overflow and empty states
7. Add `helpText()` method returning context-sensitive help text
8. Use shared helpers from pagehelpers.go for breadcrumbs, cursors, scroll indicators
