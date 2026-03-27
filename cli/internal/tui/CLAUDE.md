# TUI Component Rules

This directory is the BubbleTea terminal UI for syllago (v3). Components follow strict conventions for visual and behavioral consistency.

**Skill reference:** Run `/tui-builder` before editing TUI files to load the golden rules checklist and component message contracts.

## Before You Edit

1. Read `styles.go` — all colors (Flexoki palette) and named styles defined there
2. Check `keys.go` — all key bindings defined there
3. Run golden tests after visual changes: `go test ./internal/tui/ -update-golden`
4. Test at multiple sizes: 60x20, 80x30, and 120x40

## Architecture

Root model is `App` (defined in `app.go`). Sub-models own their state and are composed into App. Messages flow up to App, which dispatches back down.

**Message routing priority:** global keys -> modal/wizard -> toast -> focused panel

### Delegation Principle

The TUI is a **presentation layer only**. All business logic lives in CLI packages:

| Concern | Package |
|---------|---------|
| Content discovery, metadata, removal | `internal/catalog` |
| Install/uninstall operations | `internal/installer` |
| Provider detection, config paths | `internal/provider` |
| Loadout parsing and application | `internal/loadout` |
| Format conversion | `internal/converter` |
| Registry operations | `internal/registry` |
| User configuration | `internal/config` |

The TUI may call these packages from `tea.Cmd` functions (async) but must never:
- Read or parse content files directly (use `catalog.HookSummary`, `catalog.MCPSummary`, `catalog.ReadFileContent`)
- Delete files directly (use `catalog.RemoveLibraryItem`)
- Install/uninstall without going through the `installer` package

When adding features, ask: "Could a CLI command need this same logic?" If yes, it belongs in a shared package.

## File Organization (post-split)

| File | Contents |
|------|----------|
| `app.go` | `App` struct, `NewApp()`, `Init()`, nav state, helpers |
| `app_update.go` | `Update()`, key/mouse routing, message handlers |
| `app_view.go` | `View()`, `overlayModal()`, render helpers |
| `install.go` | `installWizardModel` struct, step enum, constructor, `validateStep()` |
| `install_update.go` | Install wizard `Update()`, key/mouse handlers |
| `install_view.go` | Install wizard `View()`, per-step rendering |
| `styles.go` | All colors, styles, theme constants |
| `keys.go` | All key bindings as `key.Binding` |
| Other `*.go` | One model per file (topbar, helpbar, library, explorer, gallery, etc.) |

## Color Palette (Flexoki)

All theme colors come from the Flexoki palette. Logo uses separate syllago brand colors.

| Role | Variable | Usage |
|------|----------|-------|
| Primary (cyan) | `primaryColor` | Active tabs, headings |
| Accent (purple) | `accentColor` | Focus borders, buttons |
| Muted | `mutedColor` | Help text, inactive elements |
| Success (green) | `successColor` | Installed status, success toasts |
| Danger (red) | `dangerColor` | Errors, risk indicators |
| Warning (orange) | `warningColor` | Warnings, update badge |
| Logo mint | `logoMint` | `syl` in logo ONLY |
| Logo viola | `logoViola` | `lago` in logo ONLY |

**Rules:** No raw hex values — define named variables in `styles.go`. No emojis in UI — use colored text symbols. New colors must come from the Flexoki extended palette.

## Keyboard Handling

All bindings defined in `keys.go` as `key.Binding`. Use `key.Matches(msg, keys.Foo)` not `msg.String() == "x"`. Topbar keys are routed directly in `app_update.go` via `msg.String()` comparisons (topbar doesn't own key handling).

All hotkey labels use **square brackets**: `[1]`, `[a]`, `[esc]`. Never parentheses.

## Navigation

Two-tier tab bar: groups (`[1]`/`[2]`/`[3]`) + sub-tabs (Tab/Shift+Tab).

- Collections is `[1]` (default landing page = Library)
- Content is `[2]`, Config is `[3]`
- `q` backs out (only quits from Collections > Library browse)
- `R` refreshes catalog from disk via `rescanCatalog()`

## Message Passing

Sub-models return `tea.Cmd` producing typed messages. `App.Update()` in `app_update.go` receives and routes all messages. Never send messages between sibling components directly.

Message types end in `Msg` suffix (e.g., `editSavedMsg`, `tabChangedMsg`). Model types end in `Model` suffix.

## Testing

Golden files in `testdata/`. Naming: `{component}-{variant}-{width}x{height}.golden`.

After any visual change:
```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/   # review every change
```

Test helpers in `testhelpers_test.go`: `testApp(t)`, `testAppSize(t, w, h)`, `keyRune(r)`, `keyPress(k)`, `pressN(m, key, n)`, `assertContains`, `assertNotContains`, `requireGolden`, `snapshotApp`.

Deterministic output configured in `testmain_test.go` — do not modify without understanding the AdaptiveColor race condition fix.
