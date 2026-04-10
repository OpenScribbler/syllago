# Research: TUI Efficiency & Code Organization (2024-2026)

**Date:** 2026-03-11
**Scope:** State management, rendering optimization, TEA architecture, status message lifecycle, async patterns for BubbleTea/Go

---

## The Elm Architecture (TEA) Foundation

**Core Pattern:** Three pure functions on a model struct:
- `Init() tea.Cmd` -- Returns initial async commands
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)` -- Processes events, updates state, returns new commands
- `View() string` -- Pure function rendering current state

**Why:** Complete separation of state management (Update) from rendering (View). No stale state -- rendering always reflects current model. Commands decouple I/O from the main update loop.

**Source:** BubbleTea official documentation

---

## State Management

### Model Composition
- Single responsibility: each sub-component has its own model struct
- Delegation: parent model embeds sub-components, delegates Update calls
- Focus management: track which component owns focus; delegate only that component's input

### Avoiding Stale State
- **Core Rule:** Only Update modifies state, View reads current state
- **Anti-Pattern:** Caching computed values in the model (use deferred computation in View instead)
- Don't cache `visibleItems` in model -- calculate on each View() call from source data

---

## Rendering Optimization

### Efficient String Building
Use `strings.Builder` for rendering, not string concatenation:
```go
var b strings.Builder
for _, item := range m.items {
    b.WriteString(item.render())
}
return b.String()
```
String concatenation creates new allocations on each `+`; Builder reuses underlying buffer.

### Viewport Component (Large Content)
- Only render visible lines based on scroll position
- Content split by lines internally; rendering only includes visible portion

### Lip Gloss Style Reuse
Reuse styled components rather than recreating:
```go
var (
    headerStyle = lipgloss.NewStyle().Bold(true).Foreground(color)
    cellStyle   = lipgloss.NewStyle().Padding(0, 1)
)
```
Style objects are immutable; create once, reuse many times.

### Minimize View() Allocations
- Pre-allocate slices if length is known
- Avoid `append()` in loops without pre-allocation
- Hot path (View called every frame) should have minimal allocation
- Avoid `fmt.Sprintf` in hot paths; creates allocations

---

## Status Message Lifecycle

### BubbleTea Pattern
- **Show:** Set status in model from done message handler
- **Display:** Render conditionally in View()
- **Clear Options:**
  - Timer-based: `tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return statusClearedMsg{} })`
  - Keypress-based: Clear on next user interaction
- **Duration:** 2-3 seconds for transient; 5-10s for warnings
- **Placement:** Separate footer/status line, not overlaid on content

### Progress Indicators
- **Spinner:** For unknown duration (`spinner.Dot`, `spinner.Line`, `spinner.Dots`)
- **Progress Bar:** For known percentage with spring physics animation
- Both use Tick message pattern for animation

---

## Async Operation Patterns

### Command System
```go
func fetchData(url string) tea.Cmd {
    return func() tea.Msg {
        data, err := http.Get(url)
        if err != nil {
            return errMsg{err}
        }
        return dataMsg{data}
    }
}
```
- Non-blocking: command returns immediately
- Work happens in returned function (separate goroutine)
- Results come back as typed messages

### Concurrency Safety
- Never share mutable state between goroutines
- Commands execute in separate goroutines; communicate via messages
- No mutexes in model -- model only accessed from main goroutine

### Fan-out/Fan-in
- `tea.Batch()` for concurrent operations
- `tea.Sequence()` for sequential
- Cancellation via `context.WithCancel()`

---

## Code Organization

### Recommended File Structure
```
main.go              # Entry point, NewProgram()
model.go             # Model struct definition
update.go            # Update() implementation
view.go              # View() implementation
messages.go          # All message types
commands.go          # Command functions
components/          # Sub-components
```

### Message Organization
- Single file for all message types
- Unexported wrapper types to avoid collisions
- Consistent `TypeMsg` naming convention

### Update() Organization
- Type switch on message
- Delegate to sub-functions for complex handling

---

## Anti-Patterns to Avoid

| Anti-Pattern | Correct Approach |
|---|---|
| Storing computed state in model | Calculate in View() as needed |
| String concatenation in hot path | Use `strings.Builder` |
| Broadcasting all input to all components | Only delegate to focused component |
| Async work without commands | Use commands + typed messages |
| Imperative terminal manipulation | Build entire View(), let framework diff |
| Storing io.Writer in model | Perform I/O in commands only |
| Long-running operations in Update() | Use commands for all async work |
| Mutexes protecting model | Use message-based state updates |
| Caching viewport line splits | Let viewport recalculate on resize |

---

## Performance Notes

- BubbleTea uses cell-based renderer with diffing -- only changed cells sent to terminal
- Default 60 FPS; configurable via `tea.WithFPS()`
- No throttling needed -- framework handles frame rate
- View() should be fast; avoid expensive computations
- Use `sync.Pool` for buffer reuse in repeated rendering
- `strings.SplitSeq()` (Go 1.24+) for iteration without allocation

---

## Sources
1. BubbleTea -- pkg.go.dev/github.com/charmbracelet/bubbletea
2. Bubbles Components -- pkg.go.dev/github.com/charmbracelet/bubbles
3. Lip Gloss -- pkg.go.dev/github.com/charmbracelet/lipgloss
4. Go Effective Code -- go.dev/doc/effective_go
5. Go Concurrency Patterns -- go.dev/blog/pipelines, go.dev/blog/context
6. Go Sync Package -- pkg.go.dev/sync
7. Dave Cheney, "Ice Cream Makers and Data Races"
