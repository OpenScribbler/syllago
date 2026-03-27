---
paths:
  - "cli/internal/tui/**"
---

# Elm Architecture Enforcement

## Never Do These

1. **No goroutines** — all async work via `tea.Cmd`. Bubbletea manages goroutines internally.
   ```go
   // WRONG — race condition, silent stale renders
   go func() { m.data = fetchData() }()

   // RIGHT — return a command from Update()
   return m, func() tea.Msg { return dataMsg{fetchData()} }
   ```

2. **No blocking I/O in Update() or View()** — freezes the entire event loop.
   ```go
   // WRONG
   func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       data, _ := os.ReadFile(path)  // blocks UI

   // RIGHT
   return m, func() tea.Msg {
       data, _ := os.ReadFile(path)
       return fileReadMsg{data}
   }
   ```

3. **No state mutation outside Update()** — model changes must be returned from Update().
   ```go
   // WRONG — mutating model from a goroutine or View()
   m.items = newItems

   // RIGHT — return updated model from Update()
   func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       m.items = msg.(itemsMsg).items
       return m, nil
   }
   ```

4. **No assuming command order** — use `tea.Sequence()` when order matters.
   ```go
   // WRONG — tea.Batch() runs concurrently, no ordering
   return m, tea.Batch(saveCmd, refreshCmd)

   // RIGHT — tea.Sequence() guarantees order
   return m, tea.Sequence(saveCmd, refreshCmd)
   ```

5. **Always propagate Update() to active sub-models** — bubbletea has no auto-routing.
   ```go
   // WRONG — sub-model never sees messages
   return m, nil

   // RIGHT
   m.child, cmd = m.child.Update(msg)
   cmds = append(cmds, cmd)
   ```

6. **Always handle tea.WindowSizeMsg** — store dimensions, recalculate all dependents.
   ```go
   // WRONG — ignoring resize
   case tea.WindowSizeMsg:
       return m, nil

   // RIGHT — propagate to all sub-models
   case tea.WindowSizeMsg:
       m.width = msg.Width
       m.height = msg.Height
       m.child.SetSize(msg.Width, childHeight)
       return m, nil
   ```

7. **No business logic in the TUI** — the TUI is a presentation layer. File I/O, content parsing, and disk mutations belong in CLI packages (`catalog`, `installer`, `provider`, etc.).
   ```go
   // WRONG — TUI reads and parses content files directly
   data, _ := os.ReadFile(filepath.Join(item.Path, "hook.json"))
   event := gjson.GetBytes(data, "event").String()

   // RIGHT — delegate to catalog package
   summary := catalog.HookSummary(item)
   ```

   ```go
   // WRONG — TUI removes files directly
   os.RemoveAll(item.Path)

   // RIGHT — delegate to catalog package
   catalog.RemoveLibraryItem(item.Path)
   ```
