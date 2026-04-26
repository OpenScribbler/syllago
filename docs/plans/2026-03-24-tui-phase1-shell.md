# TUI Phase 1: Shell + Styles + Test Infrastructure

The foundation everything else builds on. This phase produces an empty shell frame that renders correctly at all breakpoints, a complete style system, keybinding definitions, and the test infrastructure for all future phases.

**Output:** Running `syllago` shows an empty frame with topbar placeholders, a help bar with version, and "no content" guidance. All golden tests pass at 60x20, 80x30, and 120x40.

---

## Files to Create

```
cli/internal/tui/
  app.go              — root model, shell layout, message routing, WindowSizeMsg
  styles.go           — complete color palette, all reusable styles
  keys.go             — all keybinding definitions (named, not inline strings)
  helpbar.go          — context-sensitive footer with version
  doc.go              — package documentation

  testmain_test.go    — deterministic init (color profile, zone, NO_COLOR)
  testhelpers_test.go — testApp, golden helpers, key helpers, test catalogs
  app_test.go         — shell unit tests + golden tests

  testdata/
    shell-empty-60x20.golden
    shell-empty-80x30.golden
    shell-empty-120x40.golden
    shell-toosmall-50x15.golden
```

## Files to Delete

Everything currently in `cli/internal/tui/` except `doc.go` (if it exists and is still valid). The v2 attempt files (`app.go`, `topbar.go`, `dropdown.go`, `helpbar.go`, `keys.go`, `styles.go`, all `*.bak` files, test files, capture_test.go) are replaced. The `tui_v1/` package stays untouched as fallback.

---

## app.go — Root Model

### Model Struct

```go
type App struct {
    // Layout zones (added in later phases)
    helpBar helpBarModel

    // Dimensions
    width, height int

    // State
    ready bool  // false until first WindowSizeMsg
}
```

Deliberately minimal. Topbar, explorer, gallery, metadata, modal, toast, and search get added in their respective phases. The struct grows incrementally.

### Init

```go
func (a App) Init() tea.Cmd {
    return nil  // no async work at startup in Phase 1
}
```

### Update — Message Routing Skeleton

```go
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        a.width = msg.Width
        a.height = msg.Height
        a.ready = true
        a.helpBar.SetSize(msg.Width)
        return a, nil

    case tea.KeyPressMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return a, tea.Quit
        case "?":
            // Phase 7: help overlay
        }
    }
    return a, nil
}
```

The routing skeleton shows WHERE future phases plug in (comments mark the slots). No dead code — just comments indicating future expansion points.

### View — Shell Layout

```go
func (a App) View() string {
    if !a.ready {
        return ""  // no render before first WindowSizeMsg
    }
    if a.width < 60 || a.height < 20 {
        return a.renderTooSmall()
    }

    topBar := a.renderTopBarPlaceholder()  // Phase 2 replaces this
    content := a.renderEmptyContent()      // Phase 3 replaces this
    helpBar := a.helpBar.View()

    return lipgloss.JoinVertical(lipgloss.Left,
        topBar,
        content,
        helpBar,
    )
}
```

### Content Height Calculation

```go
func (a App) contentHeight() int {
    topBarHeight := 1  // Phase 2: will come from topBar.height()
    helpBarHeight := 1
    return a.height - topBarHeight - helpBarHeight
}
```

### Placeholder Renderers

```go
func (a App) renderTopBarPlaceholder() string {
    logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")
    right := mutedStyle.Render("Phase 2: navigation dropdowns")
    gap := strings.Repeat(" ", max(0, a.width-lipgloss.Width(logo)-lipgloss.Width(right)))
    return logo + gap + right
}

func (a App) renderEmptyContent() string {
    h := a.contentHeight()
    msg := lipgloss.NewStyle().
        Width(a.width).
        Height(h).
        Align(lipgloss.Center, lipgloss.Center).
        Foreground(mutedColor).
        Render("No content loaded.\n\nPhase 3 adds the explorer layout.")
    return msg
}

func (a App) renderTooSmall() string {
    return lipgloss.Place(
        a.width, a.height,
        lipgloss.Center, lipgloss.Center,
        warningStyle.Render("Terminal too small\nMinimum: 60x20"),
    )
}
```

### Constructor

```go
func NewApp(cat *catalog.Catalog, provs []provider.Provider, cfg *config.Config) App {
    return App{
        helpBar: newHelpBar(),
    }
}
```

The constructor accepts the same backend interfaces the full TUI will need. They're stored but unused until Phase 3+.

---

## styles.go — Complete Color Palette

Define ALL colors and styles upfront, even those not used until later phases. This prevents style drift and ensures consistency from the start.

### Colors

```go
var (
    // Semantic colors — adaptive for light/dark terminals
    primaryColor = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"}  // mint
    accentColor  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}  // viola
    mutedColor   = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"}  // stone
    successColor = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}  // green
    dangerColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}  // red
    warningColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"}  // amber

    // Structural colors
    borderColor    = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
    selectedBG     = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#1A3A2A"}
    modalBG        = lipgloss.AdaptiveColor{Light: "#F4F4F5", Dark: "#27272A"}
    primaryText    = lipgloss.AdaptiveColor{Light: "#1C1917", Dark: "#FAFAF9"}
)
```

### Component Styles

```go
var (
    // Logo
    logoStyle       = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
    accentLogoStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)

    // Buttons (background-color blocks, huh/superfile pattern)
    activeButtonStyle = lipgloss.NewStyle().
        Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
        Background(accentColor).
        Padding(0, 2).
        MarginRight(1)

    inactiveButtonStyle = lipgloss.NewStyle().
        Foreground(mutedColor).
        Background(lipgloss.AdaptiveColor{Light: "#E4E4E7", Dark: "#3F3F46"}).
        Padding(0, 2).
        MarginRight(1)

    // Selected row (full-width background, gh-dash pattern)
    selectedRowStyle = lipgloss.NewStyle().
        Background(selectedBG).
        Bold(true)

    // Panel borders (border-color only, superfile pattern)
    focusedPanelStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(accentColor)

    unfocusedPanelStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(borderColor)

    // Tabs (bold+bg active, faint inactive, gh-dash pattern)
    activeTabStyle = lipgloss.NewStyle().
        Bold(true).
        Background(selectedBG).
        Foreground(primaryText).
        Padding(0, 2)

    inactiveTabStyle = lipgloss.NewStyle().
        Faint(true).
        Padding(0, 2)

    // Dropdown
    dropdownBorderStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(accentColor)

    dropdownItemStyle = lipgloss.NewStyle().
        PaddingLeft(2).
        PaddingRight(2)

    dropdownSelectedStyle = lipgloss.NewStyle().
        Background(selectedBG).
        Bold(true).
        PaddingLeft(2).
        PaddingRight(2)

    // Modal
    modalStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(accentColor).
        Background(modalBG).
        Padding(1, 2).
        Width(56)

    // Toast
    successToastStyle = lipgloss.NewStyle().Foreground(successColor)
    warningToastStyle = lipgloss.NewStyle().Foreground(warningColor)
    errorToastStyle   = lipgloss.NewStyle().Foreground(dangerColor).Bold(true)

    // Help bar
    helpBarStyle = lipgloss.NewStyle().
        Foreground(mutedColor)

    helpKeyStyle = lipgloss.NewStyle().
        Foreground(primaryText).
        Bold(true)

    versionStyle = lipgloss.NewStyle().
        Foreground(mutedColor).
        Faint(true)

    // Warning
    warningStyle = lipgloss.NewStyle().
        Foreground(warningColor).
        Bold(true).
        Align(lipgloss.Center)

    // General text
    mutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)
    primaryStyle = lipgloss.NewStyle().Foreground(primaryColor)
    boldStyle    = lipgloss.NewStyle().Bold(true).Foreground(primaryText)

    // Inline section title: ──Title (N)──────────────────
    sectionTitleStyle = lipgloss.NewStyle().Foreground(primaryColor)
    sectionRuleStyle  = lipgloss.NewStyle().Foreground(mutedColor)
)
```

### Helper: Inline Section Title

```go
func renderSectionTitle(title string, width int) string {
    prefix := sectionRuleStyle.Render("──")
    label := sectionTitleStyle.Render(title)
    used := 2 + lipgloss.Width(title)  // "──" + title
    remaining := max(0, width-used-1)
    suffix := sectionRuleStyle.Render("─" + strings.Repeat("─", remaining))
    return prefix + label + suffix
}
```

---

## keys.go — Keybinding Definitions

All bindings defined here, referenced by name elsewhere. Never hardcode key strings in Update handlers.

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    // Global
    Quit    key.Binding
    Help    key.Binding
    Search  key.Binding

    // Navigation
    Up      key.Binding
    Down    key.Binding
    Left    key.Binding
    Right   key.Binding
    Enter   key.Binding
    Escape  key.Binding
    Tab     key.Binding
    Home    key.Binding
    End     key.Binding
    PgUp    key.Binding
    PgDown  key.Binding

    // Dropdowns (Phase 2)
    Dropdown1 key.Binding  // Content
    Dropdown2 key.Binding  // Collection
    Dropdown3 key.Binding  // Config

    // Actions (Phase 4+)
    Install   key.Binding
    Uninstall key.Binding
    Copy      key.Binding
    Share     key.Binding
    Add       key.Binding
    Remove    key.Binding
    Sync      key.Binding
    ToggleHidden key.Binding
}

var keys = keyMap{
    Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
    Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
    Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),

    Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("j/k", "navigate")),
    Down:    key.NewBinding(key.WithKeys("down", "j")),
    Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("h/l", "pane")),
    Right:   key.NewBinding(key.WithKeys("right", "l")),
    Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
    Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
    Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
    Home:    key.NewBinding(key.WithKeys("home", "g")),
    End:     key.NewBinding(key.WithKeys("end", "G")),
    PgUp:    key.NewBinding(key.WithKeys("pgup")),
    PgDown:  key.NewBinding(key.WithKeys("pgdown")),

    Dropdown1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "content")),
    Dropdown2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "collection")),
    Dropdown3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "config")),

    Install:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install")),
    Uninstall:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "uninstall")),
    Copy:         key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
    Share:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "share")),
    Add:          key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
    Remove:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "remove")),
    Sync:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
    ToggleHidden: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "hidden")),
}
```

---

## helpbar.go — Context-Sensitive Footer

```go
type helpBarModel struct {
    width   int
    hints   []string  // set by parent based on current screen/focus
    version string
}

func newHelpBar() helpBarModel {
    return helpBarModel{
        version: "syllago v" + build.Version(),
        hints:   []string{"? help", "ctrl+c quit"},
    }
}

func (h *helpBarModel) SetSize(width int) {
    h.width = width
}

func (h *helpBarModel) SetHints(hints []string) {
    h.hints = hints
}

func (h helpBarModel) View() string {
    left := h.renderHints()
    right := versionStyle.Render(h.version)

    rightW := lipgloss.Width(right)
    leftW := lipgloss.Width(left)
    gap := max(0, h.width-leftW-rightW-1)

    return helpBarStyle.Render(left) + strings.Repeat(" ", gap) + right
}

func (h helpBarModel) renderHints() string {
    if len(h.hints) == 0 {
        return ""
    }

    sep := mutedStyle.Render(" * ")
    var parts []string
    totalWidth := 0
    maxWidth := h.width - lipgloss.Width(h.version) - 4  // reserve space for version

    for _, hint := range h.hints {
        rendered := helpBarStyle.Render(hint)
        w := lipgloss.Width(rendered)
        if totalWidth+w+3 > maxWidth && len(parts) > 0 {
            break  // drop lower-priority hints that don't fit
        }
        parts = append(parts, rendered)
        totalWidth += w + 3
    }
    return strings.Join(parts, sep)
}
```

---

## Test Infrastructure

### testmain_test.go

See `references/testing-setup.md` in the tui-builder skill for the exact code. Critical elements:
- `lipgloss.SetColorProfile(termenv.Ascii)`
- `lipgloss.SetHasDarkBackground(true)`
- `zone.NewGlobal()`
- `os.Setenv("NO_COLOR", "1")` / `os.Setenv("TERM", "dumb")`
- Warmup render for AdaptiveColor stability

### testhelpers_test.go

See `references/testing-setup.md` for the exact code. Provides:
- `keyRune()`, `keyPress()`, `pressN()` helpers
- `testApp()`, `testAppSize()` constructors
- `requireGolden()`, `normalizeSnapshot()`, `snapshotApp()`, `diffStrings()`
- `assertContains()`, `assertNotContains()`

### Test Catalog Stubs

```go
func testCatalog(t *testing.T) *catalog.Catalog {
    // Minimal catalog for Phase 1 — grows in later phases
    return catalog.NewEmpty()
}

func testProviders() []provider.Provider {
    return nil  // no providers needed for shell-only tests
}

func testConfig() *config.Config {
    return config.DefaultConfig()
}
```

### app_test.go — Phase 1 Tests

```go
// --- Unit tests ---

func TestApp_WindowSizeMsg(t *testing.T) {
    app := NewApp(testCatalog(t), testProviders(), testConfig())
    m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
    a := m.(App)
    if a.width != 120 || a.height != 40 {
        t.Errorf("expected 120x40, got %dx%d", a.width, a.height)
    }
    if !a.ready {
        t.Error("app should be ready after WindowSizeMsg")
    }
}

func TestApp_TooSmall(t *testing.T) {
    app := testAppSize(t, 50, 15)
    view := app.View()
    assertContains(t, view, "Terminal too small")
    assertContains(t, view, "60x20")
}

func TestApp_QuitOnCtrlC(t *testing.T) {
    app := testApp(t)
    _, cmd := app.Update(tea.KeyPressMsg(tea.KeyMsg{Type: tea.KeyCtrlC}))
    if cmd == nil {
        t.Fatal("ctrl+c should produce a quit command")
    }
}

func TestApp_EmptyContentGuidance(t *testing.T) {
    app := testApp(t)
    view := app.View()
    assertContains(t, view, "No content")
}

func TestApp_HelpBarVersion(t *testing.T) {
    app := testApp(t)
    view := app.View()
    assertContains(t, view, "syllago v")
}

func TestApp_Branding(t *testing.T) {
    app := testApp(t)
    view := app.View()
    assertContains(t, view, "syl")
    assertContains(t, view, "lago")
}

// --- Golden tests ---

func TestGolden_Shell_60x20(t *testing.T) {
    app := testAppSize(t, 60, 20)
    requireGolden(t, "shell-empty-60x20", snapshotApp(t, app))
}

func TestGolden_Shell_80x30(t *testing.T) {
    app := testAppSize(t, 80, 30)
    requireGolden(t, "shell-empty-80x30", snapshotApp(t, app))
}

func TestGolden_Shell_120x40(t *testing.T) {
    app := testAppSize(t, 120, 40)
    requireGolden(t, "shell-empty-120x40", snapshotApp(t, app))
}

func TestGolden_Shell_TooSmall(t *testing.T) {
    app := testAppSize(t, 50, 15)
    requireGolden(t, "shell-toosmall-50x15", snapshotApp(t, app))
}
```

---

## What the Shell Looks Like

### 80x30

```
syllago                                     Phase 2: navigation dropdowns

                          No content loaded.

                    Phase 3 adds the explorer layout.



? help * ctrl+c quit                                        syllago v1.2.0
```

### 120x40

Same structure, more space. The content area fills the available height. The topbar placeholder and helpbar are the only visible elements.

### 50x15 (too small)

```
       Terminal too small
         Minimum: 60x20
```

---

## Build Order Within Phase 1

1. **styles.go** — all colors and styles (no dependencies)
2. **keys.go** — all keybindings (no dependencies)
3. **helpbar.go** — footer component (depends on styles)
4. **app.go** — root model + shell layout (depends on styles, keys, helpbar)
5. **doc.go** — package doc
6. **testmain_test.go** — deterministic test setup
7. **testhelpers_test.go** — test utilities
8. **app_test.go** — unit + golden tests
9. Run `go test ./internal/tui/` — all pass
10. Run `go test ./internal/tui/ -update-golden` — create baseline golden files
11. Run `make build` — binary builds and runs

---

## Interconnections to Plan For

These don't get built in Phase 1, but the shell must accommodate them:

| Future Phase | What It Needs from the Shell |
|-------------|------------------------------|
| Phase 2 (Topbar) | `renderTopBarPlaceholder()` replaced by real `topBar.View()`. Topbar height may grow when dropdown is open. |
| Phase 3 (Explorer) | `renderEmptyContent()` replaced by explorer layout. `contentHeight()` used for explorer sizing. |
| Phase 4 (Metadata) | Metadata bar inserted between topbar and content. `contentHeight()` updated. |
| Phase 5 (Split) | Content zone internals change. No shell impact. |
| Phase 6 (Gallery) | Content area switches between explorer and gallery based on active dropdown. |
| Phase 7 (Modal/Toast) | Modal renders via `lipgloss.Place()` over everything. Toast inserts between topbar and content. |

**The shell's `View()` will evolve** from `JoinVertical(topbar, content, helpbar)` to `JoinVertical(topbar, toast?, metadata?, content, helpbar)` with a modal overlay when active. Each phase adds one layer.

---

## Done Criteria

- [ ] `make build` succeeds
- [ ] `syllago` launches and shows the empty shell
- [ ] Shell renders correctly at 60x20, 80x30, 120x40
- [ ] "Terminal too small" shows at 50x15
- [ ] Ctrl+C quits, q quits
- [ ] All unit tests pass
- [ ] All golden tests pass
- [ ] `make fmt` produces no changes
- [ ] No dead code, no unused imports
- [ ] tui-builder skill updated (Phase Log + any new Gotchas)

---

## Skill Updates (Post-Phase)

After Phase 1 is complete, update `.claude/skills/tui-builder/SKILL.md`:

### Phase Log Entry

Add to the `## Phase Log` section:

```markdown
### Phase 1 (Shell + Styles + Tests) — YYYY-MM-DD
- [Record what was learned: any style adjustments, test gotchas, build issues]
- [Record any pattern changes from what the spec prescribed]
- [Record anything that would help future phases]
```

### Gotchas to Add

If any of these are encountered during implementation, add to the `## Gotchas` section:
- Build issues with specific lipgloss/bubbletea versions
- Golden file surprises (normalization edge cases, platform differences)
- Style rendering quirks discovered during testing
- Any deviation from the spec and why

### Reference Updates

If the testing setup code in `references/testing-setup.md` needs adjustment based on what actually works (e.g., different import paths, additional normalization), update it so Phase 2 starts from correct boilerplate.
