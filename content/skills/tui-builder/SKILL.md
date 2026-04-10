---
name: tui-builder
description: Use when building, modifying, or debugging the syllago TUI. Provides golden rules checklist, component message contracts, and collected gotchas. TRIGGER when working on any file in cli/internal/tui/.
---

# TUI Builder — Golden Rules + Quick Reference

> **Context gate:** On first invocation in a session, run: `touch /tmp/syllago-tui-builder-${PPID}`
> This signals the tui-context-gate hook that the skill was loaded.

## Golden Rules Checklist

Before any layout or component work, verify:

1. **Mouse + keyboard parity on EVERY interactive element** — `zone.Mark()` in View, `zone.Get().InBounds()` in updateMouse. No exceptions. If it responds to a keypress, it must respond to a click.
2. Subtract 2 from height for borders, 2 from width for borders
3. Never rely on auto-wrap — truncate explicitly (`maxTextWidth = panelWidth - 4`)
4. Use `MaxWidth()` not `Width()` — Width word-wraps
5. Use `borderedPanel()` for all bordered panels
6. Parent owns child sizes — call `SetSize()`, never let children calculate
7. All async work via `tea.Cmd` — never goroutines
8. All state changes returned from `Update()` — never mutate elsewhere
9. Always propagate `Update()` to active sub-models
10. Handle `tea.WindowSizeMsg` — store and recalculate

## Component Message Contracts

| Component | Receives | Sends |
|-----------|----------|-------|
| topbar | `tea.WindowSizeMsg`, `tea.KeyMsg`, `tea.MouseMsg` | `tabChangedMsg`, `actionPressedMsg`, `helpToggleMsg` |
| items | `tea.WindowSizeMsg`, `tea.KeyMsg` | `itemSelectedMsg` |
| explorer | `tea.WindowSizeMsg`, `tea.KeyMsg`, content items | `explorerDrillMsg`, `explorerCloseMsg` |
| gallery | `tea.WindowSizeMsg`, `tea.KeyMsg`, `tea.MouseMsg` | `cardSelectedMsg`, `cardDrillMsg` |
| modal | `tea.KeyMsg`, `tea.MouseMsg` | `editSavedMsg`, `editCancelledMsg` |
| install | `tea.KeyMsg`, `tea.MouseMsg`, step data | `installResultMsg`, `installDoneMsg`, `installCloseMsg` |
| toast | `tea.KeyMsg`, `toastTickMsg` | (dismisses via `Dismiss()` cmd) |

## Gotchas

- **AdaptiveColor mutates renderer state.** Fix: warmup render in `TestMain()`.
- **bubbletea v1 uses `tea.KeyMsg`**, not `tea.KeyPressMsg` (v2 API). Check version before using spec code.
- **Golden files with raw ANSI are fragile.** Strip ANSI before storing.
- **`lipgloss.Width()` is ANSI-aware, `len()` is not.** Always use `lipgloss.Width()` for rendered strings.
- **Children that never receive WindowSizeMsg render at zero size.**
- **goimports strips "unused" imports between edits.** Add import and usage in a single Edit call.
- **Cursor initialization with Reset():** When `active = -1`, `Open()` must default cursor to 0, not -1.
- **lipgloss Width() wraps, MaxWidth() truncates.** For bordered panels, always use both Width+MaxWidth and Height+MaxHeight. Without MaxHeight, content overflow pushes the header offscreen.
- **Manual string splitting destroys zone markers.** Never split rendered strings containing zone.Mark() on newlines or truncate by rune — the invisible zero-width markers get broken. Use lipgloss styling for dimension control instead.
- **Sort indicators overflow short columns.** "Files ▲" is 7 visual chars but the Files column is 5. Use `headerCell()` which truncates the label to make room for the indicator within the column width.
- **App-level keys intercept search input.** When the library search input is active, keys like 'a' (add), 'q' (quit), '1' (group switch) must be passed through to the search handler instead of triggering app shortcuts. Check `table.searching` before handling global letter keys.
- **Help bar separator is middle dot (·)** not asterisk (*). Cleaner look.
- **Metadata bar steals table height.** When adding a fixed-height panel below a variable-height component (like table + metadata bar), always reduce the variable component's height in both `SetSize()` AND `View()`. If only one is updated, the table renders at the wrong height on resize vs initial draw.
- **Modal `lipgloss.Place()` centering.** Use `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, rendered)` for modal centering — not manual padding math. Handles terminal resize automatically.
- **YAML folded scalars add trailing newlines.** Descriptions from `loadout.yaml` using `>` have `\n` at the end. Always sanitize catalog text fields before rendering in the TUI.
- **Unified frame requires manual border construction.** `borderedPanel()` can't create shared borders between sections. Build frames manually: `╭`/`╰` for corners, `├──┤` for internal separators, `├──┬──┤`/`╰──┴──╯` for split pane junctions.
- **Metadata panel height is constant (metaBarLines=3 + 1 separator).** Type-specific line 3 is blank for simple types, not omitted. This prevents the frame from shifting when scrolling between hooks and skills.
- **Explorer needs providers for metadata.** Pass providers + repoRoot to `newExplorerModel()` so it can compute installed status via `computeMetaPanelData()`.
- **Keyboard-only components are bugs.** If a View renders interactive elements without `zone.Mark()`, mouse users can't interact with them. Every radio option, checkbox row, list item, button, and text input needs a zone mark in View and a corresponding `zone.Get().InBounds()` check in updateMouse. The `checkboxList` component automates this via `zonePrefix` — set it and call `HandleClick()` in the parent's mouse handler.
