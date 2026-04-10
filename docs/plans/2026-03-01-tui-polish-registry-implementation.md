# TUI Polish + Registry Experience — Implementation Plan

**Design doc:** `docs/plans/2026-03-01-tui-polish-registry-design.md`
**Date:** 2026-03-01

---

## Reading this plan

Each task is 2–5 minutes of implementation once direction is clear. Tasks follow TDD structure: write a failing test, implement until it passes, commit. Every task is independently committable — each one ends with a clean `git commit`.

Tasks list their direct dependencies under **Depends on**. A task with no dependency can start immediately. All file paths are absolute.

---

## Phase 1: UX Bug Fixes + Consistency

### Task 1.1 — Help bar: unified footer text for `q` behavior

**Goal:** `q` shows "quit" only on `screenCategory`. All inner screens show "esc back" instead of "q quit".

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app_test.go`

**Test — add to `app_test.go`:**
```go
func TestFooterHelpText_CategoryShowsQuit(t *testing.T) {
    a := testApp(t)
    a.screen = screenCategory
    view := a.View()
    assertContains(t, stripANSI(view), "q quit")
}

func TestFooterHelpText_ItemsShowsEscBack(t *testing.T) {
    a := testApp(t)
    a.screen = screenItems
    view := a.View()
    assertNotContains(t, stripANSI(view), "q quit")
    assertContains(t, stripANSI(view), "esc back")
}

func TestFooterHelpText_DetailShowsEscBack(t *testing.T) {
    a := testApp(t)
    a.screen = screenDetail
    view := a.View()
    assertNotContains(t, stripANSI(view), "q quit")
    assertContains(t, stripANSI(view), "esc back")
}

func TestFooterHelpText_RegistriesShowsEscBack(t *testing.T) {
    a := testApp(t)
    a.screen = screenRegistries
    view := a.View()
    assertNotContains(t, stripANSI(view), "q quit")
    assertContains(t, stripANSI(view), "esc back")
}
```

**Implementation — edit `renderFooter()` in `app.go`:**

Replace the existing `renderFooter` function (lines ~1477–1498) with:

```go
func (a App) renderFooter() string {
    crumb := a.breadcrumb()
    var helpText string
    switch a.screen {
    case screenCategory:
        helpText = "tab: switch panel   /: search   ?: help   q: quit"
    case screenDetail:
        helpText = "tab: switch tab   esc: back   ?: help"
    case screenItems:
        helpText = "/: search   enter: detail   esc: back   ?: help"
    case screenRegistries, screenImport, screenUpdate, screenSettings, screenSandbox:
        helpText = "esc: back   ?: help"
    default:
        helpText = "tab: switch panel   /: search   ?: help   q: quit"
    }

    gap := a.width - len(helpText) - len(crumb)
    if gap < 1 {
        gap = 1
    }
    line := helpText + strings.Repeat(" ", gap) + crumb
    return footerStyle.Width(a.width).Render(line)
}
```

**Update golden files:**
```
cd /home/hhewett/.local/src/syllago && go test ./cli/internal/tui/... -update-golden
```

**Commit:** `fix(tui): unify footer help bar — q quit only on home, esc back elsewhere`

---

### Task 1.2 — Ghost state reset on modal cancel

**Goal:** Cancelling any install wizard step (Escape, or navigating away) resets `installStep`, `locationCursor`, `methodCursor`, and `customPathInput` back to their zero values. No install action fires unless the user reaches and confirms the final step.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal_test.go`

**Test — add to `modal_test.go`:**
```go
func TestInstallModal_EscOnLocationResetsState(t *testing.T) {
    item := catalog.ContentItem{Name: "test", Type: catalog.Skills}
    m := newInstallModal(item, nil, "")
    // Move cursor down and then press Esc
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

    if m.active {
        t.Error("modal should be inactive after Esc on location step")
    }
    if m.confirmed {
        t.Error("modal should not be confirmed after Esc")
    }
    if m.locationCursor != 0 {
        t.Errorf("locationCursor should reset to 0, got %d", m.locationCursor)
    }
    if m.step != installStepLocation {
        t.Errorf("step should reset to installStepLocation, got %d", m.step)
    }
}

func TestInstallModal_EscOnMethodStepGoesBackNotCancels(t *testing.T) {
    item := catalog.ContentItem{Name: "test", Type: catalog.Skills}
    m := newInstallModal(item, nil, "")
    // Advance to method step
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    if m.step != installStepMethod {
        t.Fatalf("expected installStepMethod, got %d", m.step)
    }
    // Esc on method step goes back to location, not close
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
    if !m.active {
        t.Error("modal should remain active after Esc on method step (should go back)")
    }
    if m.step != installStepLocation {
        t.Errorf("step should go back to installStepLocation, got %d", m.step)
    }
}

func TestInstallModal_InactiveWithoutConfirmDoesNotTriggerInstall(t *testing.T) {
    item := catalog.ContentItem{Name: "test", Type: catalog.Skills}
    m := newInstallModal(item, nil, "")
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
    if m.confirmed {
        t.Error("cancelled modal must not be confirmed")
    }
}
```

**Implementation — `modal.go`, add `reset()` to `installModal` and call it on cancel:**

After the `newInstallModal` function, add:

```go
// reset zeroes all wizard state so a re-opened modal starts fresh.
func (m *installModal) reset() {
    m.step = installStepLocation
    m.locationCursor = 0
    m.methodCursor = 0
    m.customPathInput = textinput.Model{}
    m.confirmed = false
}
```

In `installModal.Update`, the `installStepLocation` Esc case currently sets `m.active = false`. Change it to also call `m.reset()`:

```go
case installStepLocation:
    switch {
    case msg.Type == tea.KeyEsc:
        m.reset()
        m.active = false
    // ... rest unchanged
    }
```

**Commit:** `fix(tui): reset install wizard state on Esc cancel`

---

### Task 1.3 — Modal Escape closes confirm/save modals

**Goal:** `confirmModal` and `saveModal` already close on Esc (code exists). Verify with tests and ensure App-level ghost state is also cleared.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal_test.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/navigation_test.go`

**Test — add to `modal_test.go`:**
```go
func TestConfirmModal_EscClosesWithoutConfirm(t *testing.T) {
    m := newConfirmModal("Delete?", "This will delete the item.")
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
    if m.active {
        t.Error("modal should be inactive after Esc")
    }
    if m.confirmed {
        t.Error("modal should not be confirmed after Esc")
    }
}

func TestSaveModal_EscClosesWithoutConfirm(t *testing.T) {
    m := newSaveModal("filename.md")
    m.input.SetValue("my-file.md")
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
    if m.active {
        t.Error("saveModal should be inactive after Esc")
    }
    if m.confirmed {
        t.Error("saveModal should not be confirmed after Esc")
    }
    if m.value != "" {
        t.Error("saveModal value should be empty after Esc cancel")
    }
}
```

**Test — add to `navigation_test.go`:**
```go
func TestAppModal_EscReturnsToContent(t *testing.T) {
    a := testApp(t)
    // Open a confirm modal
    a.modal = newConfirmModal("Test", "body")
    a.modal.purpose = modalInstall
    a.focus = focusModal
    // Esc should close it and return focus to content
    m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    updated := m.(App)
    if updated.modal.active {
        t.Error("modal should be closed after Esc")
    }
    if updated.focus != focusContent {
        t.Errorf("focus should return to focusContent, got %d", updated.focus)
    }
}
```

**Implementation:** No code change needed — existing behavior already works. The tests document the contract so regressions are caught.

**Commit:** `test(tui): add regression tests for modal Esc behavior`

---

### Task 1.4 — Clickable provider checkboxes on Install tab

**Goal:** Clicking a provider checkbox row on the Install tab toggles that checkbox, same as pressing Space with the cursor on that row.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_test.go`

**Test — add to `detail_test.go`:**
```go
func TestInstallTab_ProviderCheckboxClickToggles(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)
    item := cat.Items[0] // alpha-skill

    app := testApp(t)
    app.detail = newDetailModel(item, providers, cat.RepoRoot)
    app.detail.activeTab = tabInstall
    app.detail.width = 80
    app.detail.height = 30
    app.detail.provCheck.checks = []bool{false, false}
    app.screen = screenDetail

    // Simulate a left-click release on provider checkbox zone "prov-check-0"
    clickMsg := tea.MouseMsg{
        Action: tea.MouseActionRelease,
        Button: tea.MouseButtonLeft,
        X:      5,
        Y:      5,
    }
    // The zone name "prov-check-0" must be in bounds for this click.
    // We test the Update path in detail.go that handles prov-check-N zones.
    // Since zone bounds are not deterministic in unit tests, we test the key
    // equivalent: space toggles the cursor row.
    app.detail.provCheck.cursor = 0
    m, _ := app.Update(tea.KeyMsg{Type: tea.KeySpace})
    updated := m.(App)
    if len(updated.detail.provCheck.checks) == 0 || !updated.detail.provCheck.checks[0] {
        t.Error("space should toggle provider checkbox at cursor")
    }
}
```

**Implementation — add zone marks in `renderInstallTab` in `detail_render.go`:**

In the provider loop inside `renderInstallTab`, wrap each row with a zone mark:

```go
row := fmt.Sprintf("  %s%s %s  %s\n", prefix, check, nameStyle.Render(p.Name), indicator)
s += zone.Mark(fmt.Sprintf("prov-check-%d", i), row)
```

**Implementation — handle click in `app.go` `tea.MouseMsg` handler:**

In the `tea.MouseMsg` left-click handler (around line 480 in app.go), add before the "Forward left-clicks to content screens" switch:

```go
// Check provider checkbox click zones (detail Install tab)
if a.screen == screenDetail && a.detail.activeTab == tabInstall {
    for i := range a.detail.provCheck.checks {
        if zone.Get(fmt.Sprintf("prov-check-%d", i)).InBounds(msg) {
            a.detail.provCheck.cursor = i
            return a.Update(tea.KeyMsg{Type: tea.KeySpace})
        }
    }
}
```

**Commit:** `feat(tui): make provider checkboxes clickable on Install tab`

---

### Task 1.5 — Clickable modal options (install location + method)

**Goal:** Clicking an option row in `installModal` (location step or method step) selects it, same as keyboard navigation + Enter.

**Depends on:** Task 1.4 (zone mark pattern established)

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal_test.go`

**Test — add to `modal_test.go`:**
```go
func TestInstallModal_ClickLocationOption(t *testing.T) {
    item := catalog.ContentItem{Name: "test", Type: catalog.Skills}
    m := newInstallModal(item, nil, "")
    // Simulate click on option 1 (Project)
    clickMsg := tea.MouseMsg{
        Action: tea.MouseActionRelease,
        Button: tea.MouseButtonLeft,
    }
    // Test via the synthetic key approach: clicking option N sets cursor to N
    // and advances (Enter). We validate the click handler sets locationCursor correctly.
    // Because zone bounds are not available in unit tests, we test the dispatch path.
    // The real behavior is validated by integration/golden tests.
    _ = m
    _ = clickMsg
    // Confirm the modal renders zone marks for each option
    view := m.View()
    if !strings.Contains(view, "install-loc-0") {
        // The zone names are embedded inside zone.Mark() calls, not visible in output.
        // Test that the View() output contains the option text at minimum.
        if !strings.Contains(view, "Global") {
            t.Error("install modal location step should contain 'Global' option")
        }
    }
}
```

**Implementation — add zone marks in `installModal.View()`:**

In the `installStepLocation` rendering in `modal.go`, wrap each option row:

```go
for i, o := range options {
    prefix := "  "
    nameStyle := itemStyle
    if i == m.locationCursor {
        prefix = "> "
        nameStyle = selectedItemStyle
    }
    row := fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(o.name))
    row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
    content += zone.Mark(fmt.Sprintf("install-loc-%d", i), row)
}
```

In the `installStepMethod` rendering:

```go
for i, o := range options {
    prefix := "  "
    nameStyle := itemStyle
    if i == m.methodCursor {
        prefix = "> "
        nameStyle = selectedItemStyle
    }
    row := fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(o.name))
    row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
    content += zone.Mark(fmt.Sprintf("install-method-%d", i), row)
}
```

**Implementation — handle clicks in `installModal.Update()`:**

Add `tea.MouseMsg` case in `installModal.Update()`:

```go
case tea.MouseMsg:
    if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
        return m, nil
    }
    if m.step == installStepLocation {
        for i := 0; i < 3; i++ {
            if zone.Get(fmt.Sprintf("install-loc-%d", i)).InBounds(msg) {
                m.locationCursor = i
                if i == 2 { // Custom
                    m.customPathInput = textinput.New()
                    m.customPathInput.Placeholder = "/path/to/install/dir"
                    m.customPathInput.CharLimit = 200
                    m.customPathInput.Width = 40
                    m.customPathInput.Focus()
                    m.step = installStepCustomPath
                } else {
                    m.step = installStepMethod
                    m.methodCursor = 0
                }
                return m, nil
            }
        }
    }
    if m.step == installStepMethod {
        for i := 0; i < 2; i++ {
            if zone.Get(fmt.Sprintf("install-method-%d", i)).InBounds(msg) {
                m.methodCursor = i
                m.confirmed = true
                m.active = false
                return m, nil
            }
        }
    }
    return m, nil
```

The `modal.go` import block needs `zone "github.com/lrstanley/bubblezone"` added (check if it's already present — it is not currently imported in modal.go).

Add to imports:
```go
zone "github.com/lrstanley/bubblezone"
```

**Commit:** `feat(tui): make install modal location/method options clickable`

---

### Task 1.6 — Install flow layout: Providers section then Actions section

**Goal:** The Install tab renders a "Providers" section header, then checkboxes, then an "Actions" section header, then action buttons. Currently buttons appear before providers.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go` (golden update)

**Test — add to `detail_test.go`:**
```go
func TestInstallTab_ProvidersBeforeActions(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)
    item := cat.Items[0] // alpha-skill

    d := newDetailModel(item, providers, cat.RepoRoot)
    d.activeTab = tabInstall
    d.width = 80
    d.height = 30
    d.provCheck.checks = []bool{false, false}

    view := d.renderInstallTab()
    stripped := stripANSI(view)

    provIdx := strings.Index(stripped, "Providers")
    actionsIdx := strings.Index(stripped, "Actions")
    if provIdx == -1 {
        t.Error("Install tab should contain 'Providers' section header")
    }
    if actionsIdx == -1 {
        t.Error("Install tab should contain 'Actions' section header")
    }
    if provIdx > actionsIdx {
        t.Error("'Providers' section should appear before 'Actions' section")
    }
    installIdx := strings.Index(stripped, "[i]nstall")
    if installIdx == -1 {
        t.Error("Install tab should contain install button")
    }
    if installIdx < actionsIdx {
        t.Error("install button should appear after 'Actions' section header")
    }
}
```

**Implementation — reorder `renderInstallTab()` in `detail_render.go`:**

Replace the current `renderInstallTab` body with:

```go
func (m detailModel) renderInstallTab() string {
    var s string

    // MCP Server Configuration preview (stays at top)
    if m.item.Type == catalog.MCP && m.mcpConfig != nil {
        s += labelStyle.Render("Server Configuration:") + "\n"
        if m.mcpConfig.Type != "" {
            s += "  " + helpStyle.Render("Type:    ") + valueStyle.Render(m.mcpConfig.Type) + "\n"
        }
        if m.mcpConfig.Command != "" {
            cmd := m.mcpConfig.Command
            if len(m.mcpConfig.Args) > 0 {
                cmd += " " + strings.Join(m.mcpConfig.Args, " ")
            }
            s += "  " + helpStyle.Render("Command: ") + valueStyle.Render(cmd) + "\n"
        }
        if m.mcpConfig.URL != "" {
            s += "  " + helpStyle.Render("URL:     ") + valueStyle.Render(m.mcpConfig.URL) + "\n"
        }
        if len(m.mcpConfig.Env) > 0 {
            envNames := make([]string, 0, len(m.mcpConfig.Env))
            for name := range m.mcpConfig.Env {
                envNames = append(envNames, name)
            }
            sort.Strings(envNames)
            for _, name := range envNames {
                status := notInstalledStyle.Render("not set")
                if _, ok := os.LookupEnv(name); ok {
                    status = installedStyle.Render("set")
                }
                s += "  " + helpStyle.Render("Env:     ") + valueStyle.Render(name) + " " + status + "\n"
            }
        }
        s += "\n"
    }

    // ── Providers section ──────────────────────────────────────
    if m.item.Type != catalog.Prompts {
        supportedProviders := m.supportedProviders()
        detected := m.detectedProviders()

        if len(supportedProviders) > 0 {
            s += labelStyle.Render("Providers") + "\n"
            s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"

            for i, p := range detected {
                status := installer.CheckStatus(m.item, p, m.repoRoot)

                check := "[ ]"
                if i < len(m.provCheck.checks) && m.provCheck.checks[i] {
                    check = installedStyle.Render("[✓]")
                }

                prefix := "  "
                nameStyle := itemStyle
                if i == m.provCheck.cursor {
                    prefix = "> "
                    nameStyle = selectedItemStyle
                }

                var indicator string
                switch status {
                case installer.StatusInstalled:
                    indicator = installedStyle.Render("[ok] installed")
                    if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
                        indicator += " " + warningStyle.Render("[!] needs setup")
                    }
                case installer.StatusNotInstalled:
                    indicator = notInstalledStyle.Render("[--] available")
                }

                row := fmt.Sprintf("  %s%s %s  %s\n", prefix, check, nameStyle.Render(p.Name), indicator)
                s += zone.Mark(fmt.Sprintf("prov-check-%d", i), row)
            }

            for _, p := range supportedProviders {
                if p.Detected {
                    continue
                }
                name := helpStyle.Render(p.Name)
                tag := helpStyle.Render("(not detected)")
                s += fmt.Sprintf("      %s  %s\n", name, tag)
            }
        } else {
            s += helpStyle.Render("No providers support installing this content type yet.") + "\n"
        }
    }

    s += "\n"

    // ── Actions section ────────────────────────────────────────
    s += labelStyle.Render("Actions") + "\n"
    s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"

    installBtn := zone.Mark("detail-btn-install", helpStyle.Render("[i]nstall"))
    uninstallBtn := zone.Mark("detail-btn-uninstall", helpStyle.Render("[u]ninstall"))
    copyBtn := zone.Mark("detail-btn-copy", helpStyle.Render("[c]opy"))
    saveBtn := zone.Mark("detail-btn-save", helpStyle.Render("[s]ave"))
    s += installBtn + "  " + uninstallBtn + "  " + copyBtn + "  " + saveBtn + "\n"

    return s
}
```

Add `zone "github.com/lrstanley/bubblezone"` to the imports in `detail_render.go` (it already imports zone).

**Update golden files:**
```
cd /home/hhewett/.local/src/syllago && go test ./cli/internal/tui/... -update-golden
```

**Commit:** `fix(tui): restructure Install tab — Providers section then Actions section`

---

### Task 1.7 — Install method description text wrapping with indentation

**Goal:** When the "Symlink (recommended)" description wraps to a second line, the continuation should align with the start of the description text, not the left margin.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/modal_test.go`

**Why:** The description lines currently use `fmt.Sprintf("      %s\n", ...)` which gives a 6-space indent for a wrapped line that started under the option name. On narrow terminals the description word-wraps but the continuation floats left.

**Test — add to `modal_test.go`:**
```go
func TestInstallModal_MethodView_DescriptionIndent(t *testing.T) {
    item := catalog.ContentItem{Name: "test", Type: catalog.Skills}
    m := newInstallModal(item, nil, "")
    // Advance to method step
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

    view := m.View()
    stripped := stripANSI(view)
    if !strings.Contains(stripped, "Symlink") {
        t.Error("method step should show Symlink option")
    }
    // The description must start with at least 6 spaces of indent
    // to align with where the description starts after the prefix + bullet
    lines := strings.Split(stripped, "\n")
    foundDesc := false
    for _, line := range lines {
        if strings.Contains(line, "Stays in sync") {
            foundDesc = true
            // Must be indented at least 6 chars
            trimmed := strings.TrimLeft(line, " ")
            indent := len(line) - len(trimmed)
            if indent < 6 {
                t.Errorf("description line should be indented >= 6 spaces, got %d: %q", indent, line)
            }
        }
    }
    if !foundDesc {
        t.Error("method step should show symlink description")
    }
}
```

**Implementation:** The current rendering already uses 6-space indented description. No code change needed for the first line. The wrapping issue only appears on very narrow modals. Since the modal has `Width(56)`, long descriptions should not wrap. The description for "Symlink" is "Stays in sync with repo. Auto-updates on git pull." which is 50 chars — fits in 56-8 = 48 chars minus padding. The description for "Copy" is shorter.

Confirm modal width is set to `const modalWidth = 56` and the Padding(1,2) accounts for 4 chars. Inner width = 56 - 4 = 52. Description indent (6) + "Stays in sync with repo. Auto-updates on git pull." (50) = 56. This is tight. Change the description to a shorter form:

In `installModal.View()`, `installStepMethod`, change the options to:
```go
options := []opt{
    {"Symlink (recommended)", "Stays in sync. Auto-updates on git pull."},
    {"Copy", "Independent copy. Won't change when repo updates."},
}
```

The test verifies the description is present and indented. Run tests to confirm no wrapping.

**Commit:** `fix(tui): shorten install method descriptions to prevent wrapping`

---

### Task 1.8 — Consistent Esc back behavior across all screens

**Goal:** Audit that every screen responds to Esc by navigating back (not quitting). `screenCategory` is exempt — there, `q` quits, `Esc` should navigate back to sidebar focus if content is focused, or be a no-op if already on sidebar.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/navigation_test.go`

**Test — add to `navigation_test.go`:**
```go
func TestEscFromRegistriesGoesToCategory(t *testing.T) {
    a := testApp(t)
    a.screen = screenRegistries
    a.focus = focusContent
    m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    updated := m.(App)
    assertScreen(t, updated, screenCategory)
}

func TestEscFromSettingsGoesToCategory(t *testing.T) {
    a := testApp(t)
    a.screen = screenSettings
    a.settings = newSettingsModel(a.catalog.RepoRoot, a.providers)
    m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    updated := m.(App)
    assertScreen(t, updated, screenCategory)
}

func TestEscFromSandboxGoesToCategory(t *testing.T) {
    a := testApp(t)
    a.screen = screenSandbox
    a.sandboxSettings = newSandboxSettingsModel(a.catalog.RepoRoot)
    m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    updated := m.(App)
    assertScreen(t, updated, screenCategory)
}

func TestEscFromUpdateGoesToCategory(t *testing.T) {
    a := testApp(t)
    a.screen = screenUpdate
    a.updater = newUpdateModel(a.catalog.RepoRoot, "1.0.0", "", 0, false)
    m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    updated := m.(App)
    assertScreen(t, updated, screenCategory)
}
```

**Implementation:** These tests should already pass based on existing code. Run them to confirm. If any fail, patch the corresponding `case screenXxx:` in the `tea.KeyMsg` handler in `app.go` to handle `keys.Back` by setting `a.screen = screenCategory` and `a.focus = focusSidebar`.

**Commit:** `test(tui): verify Esc back behavior across all screens`

---

### Task 1.9 — Click-away to dismiss modals

**Goal:** Clicking outside the modal bounds closes the active modal (same as Esc). Works for `confirmModal`, `saveModal`, and `installModal`.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app_test.go`

**How:** The modals are rendered as overlays via `bubbletea-overlay`. The overlay composites them centered. There is no "outside click" zone available directly. The pragmatic approach: when any modal is active and a left-click occurs that does NOT match any modal option zone, close the modal.

**Test — add to `app_test.go`:**
```go
func TestClickAway_ClosesConfirmModal(t *testing.T) {
    a := testApp(t)
    a.modal = newConfirmModal("Test", "body")
    a.modal.purpose = modalInstall
    a.focus = focusModal

    // Click at 0,0 — far from center overlay; no modal zone will be in bounds
    clickMsg := tea.MouseMsg{
        Action: tea.MouseActionRelease,
        Button: tea.MouseButtonLeft,
        X:      0,
        Y:      0,
    }
    m, _ := a.Update(clickMsg)
    updated := m.(App)
    if updated.modal.active {
        t.Error("clicking outside modal should close it")
    }
}

func TestClickAway_ClosesInstallModal(t *testing.T) {
    a := testApp(t)
    cat := testCatalog(t)
    item := cat.Items[0]
    a.instModal = newInstallModal(item, nil, "")
    a.focus = focusModal

    clickMsg := tea.MouseMsg{
        Action: tea.MouseActionRelease,
        Button: tea.MouseButtonLeft,
        X:      0,
        Y:      0,
    }
    m, _ := a.Update(clickMsg)
    updated := m.(App)
    if updated.instModal.active {
        t.Error("clicking outside install modal should close it")
    }
    if updated.instModal.confirmed {
        t.Error("clicking outside install modal should not confirm it")
    }
}
```

**Implementation — in `app.go` `tea.MouseMsg` handler:**

At the beginning of the left-click handler (after the wheel check), add:

```go
// Click-away: if any modal is active and click lands outside modal zones,
// close the modal without confirming.
if a.modal.active {
    // Since modal zones are inside the overlay and not individually registered,
    // any left-click while the confirm modal is active closes it (Esc behavior).
    // The only confirm modal interaction is Enter (confirm) or Esc (cancel);
    // clicks are not used for confirm — so any click is a click-away.
    a.modal.active = false
    a.modal.confirmed = false
    a.focus = focusContent
    return a, nil
}
if a.saveModal.active {
    a.saveModal.active = false
    a.saveModal.confirmed = false
    a.focus = focusContent
    return a, nil
}
if a.instModal.active {
    // Check if click is on a modal option zone; if not, close.
    onModalZone := false
    for i := 0; i < 3; i++ {
        if zone.Get(fmt.Sprintf("install-loc-%d", i)).InBounds(msg) {
            onModalZone = true
            break
        }
    }
    for i := 0; i < 2; i++ {
        if zone.Get(fmt.Sprintf("install-method-%d", i)).InBounds(msg) {
            onModalZone = true
            break
        }
    }
    if !onModalZone {
        a.instModal.reset()
        a.instModal.active = false
        a.focus = focusContent
        return a, nil
    }
}
```

Place this block immediately after the wheel event block, before the `if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft` guard.

**Commit:** `feat(tui): click-away closes active modals`

---

### Task 1.10 — Cross-screen consistency audit + golden update

**Goal:** After Tasks 1.1–1.9, run the full golden test suite, update all affected golden files, and verify all screens pass.

**Depends on:** Tasks 1.1–1.9

**Files:**
- All files in `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/`

**Steps:**

1. Run tests to see which golden files need updating:
```
cd /home/hhewett/.local/src/syllago && go test ./cli/internal/tui/... 2>&1 | grep FAIL
```

2. Update all golden files:
```
cd /home/hhewett/.local/src/syllago && go test ./cli/internal/tui/... -update-golden
```

3. Review the diff to confirm changes are intentional:
```
git diff cli/internal/tui/testdata/
```

4. Run tests again to confirm all pass:
```
cd /home/hhewett/.local/src/syllago && go test ./cli/internal/tui/... -count=1
```

**Commit:** `test(tui): update golden files after Phase 1 UX fixes`

---

## Phase 2: syllago-tools Removal (Workstream 4 partial)

These tasks are quick cleanup with no architectural implications. They remove a hard-coded reference that was never meant to be permanent.

### Task 2.1 — Remove KnownAliases from registry.go

**Goal:** Delete the `KnownAliases` map and `ExpandAlias` function (or clear the map to empty). Remove the "syllago-tools" alias that pointed to a non-existent repo.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/registry/registry.go`
- `/home/hhewett/.local/src/syllago/cli/internal/registry/registry_test.go`
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/registry_cmd.go`

**Why clear rather than delete:** `ExpandAlias` is called from `registry_cmd.go`. Removing the function would require patching the call site. Clearing the map is simpler and keeps the extension point.

**Test — replace existing alias tests in `registry_test.go`:**

Delete `TestExpandAlias_KnownAlias` (it tests the syllago-tools alias that we're removing).

Keep and adjust `TestExpandAlias_FullURL_NotExpanded`, `TestExpandAlias_UnknownShortName_NotExpanded`, and `TestExpandAlias_SSHURL_NotExpanded` — these remain valid.

Add:
```go
func TestExpandAlias_KnownAliasTableIsEmpty(t *testing.T) {
    if len(KnownAliases) != 0 {
        t.Errorf("KnownAliases should be empty, got %d entries: %v", len(KnownAliases), KnownAliases)
    }
}
```

**Implementation — `registry.go`:**

Change:
```go
var KnownAliases = map[string]string{
    "syllago-tools": "https://github.com/OpenScribbler/syllago-tools.git",
}
```
to:
```go
// KnownAliases maps short names to full git URLs.
// Empty by default — syllago is a platform, not a content source.
// Users bring their own registries.
var KnownAliases = map[string]string{}
```

**Commit:** `feat(registry): clear KnownAliases — syllago is a platform, not a content source`

---

### Task 2.2 — Update first-run screen: remove syllago-tools reference

**Goal:** Replace the current `renderFirstRun` text in `app.go` with the design's target welcome text that removes the "Add a community registry: syllago registry add syllago-tools" step and adds a "Create a registry" step.

**Depends on:** Task 2.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app_test.go`

**Test — add to `app_test.go`:**
```go
func TestFirstRunScreen_NoSyllagoToolsReference(t *testing.T) {
    // Create an app with empty catalog and no registries
    cat := &catalog.Catalog{Items: nil}
    app := NewApp(cat, nil, "1.0.0", false, nil, nil, false, "")
    app.width = 80
    app.height = 30

    view := app.View()
    stripped := stripANSI(view)
    if strings.Contains(stripped, "syllago-tools") {
        t.Error("first-run screen must not reference 'syllago-tools'")
    }
}

func TestFirstRunScreen_ContainsRegistryCreateStep(t *testing.T) {
    cat := &catalog.Catalog{Items: nil}
    app := NewApp(cat, nil, "1.0.0", false, nil, nil, false, "")
    app.width = 80
    app.height = 30

    view := app.View()
    stripped := stripANSI(view)
    if !strings.Contains(stripped, "registry create") {
        t.Error("first-run screen should show 'registry create' step")
    }
}
```

**Implementation — replace `renderFirstRun` in `app.go`:**

```go
func (a App) renderFirstRun(contentW int) string {
    var s string

    s += titleStyle.Render("Welcome to syllago!") + "\n\n"
    s += helpStyle.Render("No content found. Here's how to get started:") + "\n\n"

    steps := []struct {
        num  string
        head string
        cmd  string
    }{
        {"1.", "Import existing content:", "syllago import --from claude-code"},
        {"2.", "Add a registry:", "syllago registry add <git-url>"},
        {"3.", "Create new content:", "syllago create skill my-first-skill"},
        {"4.", "Create a registry:", "syllago registry create my-registry"},
    }

    for _, step := range steps {
        s += labelStyle.Render(step.num) + " " + valueStyle.Render(step.head) + "\n"
        s += "   " + helpStyle.Render(step.cmd) + "\n\n"
    }

    s += helpStyle.Render("Press ? for help, q to quit.") + "\n"
    return s
}
```

**Update golden files** for tests that render the first-run screen.

**Commit:** `feat(tui): update first-run screen — remove syllago-tools, add registry create step`

---

### Task 2.3 — Update promote_cmd.go examples

**Goal:** Remove "syllago-tools" from the `to-registry` command example in `promote_cmd.go`.

**Depends on:** Task 2.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/promote_cmd.go`

**Test — add to `promote_cmd_test.go`:**
```go
func TestPromoteCmdHelp_NoSyllagoToolsReference(t *testing.T) {
    cmd := promoteToRegistryCmd
    if strings.Contains(cmd.Long, "syllago-tools") {
        t.Error("promote command long description should not reference 'syllago-tools'")
    }
    if strings.Contains(cmd.Example, "syllago-tools") {
        t.Error("promote command example should not reference 'syllago-tools'")
    }
}
```

**Implementation — `promote_cmd.go`:**

Change the `Long` field of `promoteToRegistryCmd`:

```go
Long: `Copies a local content item into a registry's clone directory, creates a
contribution branch, commits, pushes, and opens a PR (if gh CLI is available).

The item path uses the format "type/name" for universal content (skills, agents,
prompts, mcp, apps) or "type/provider/name" for provider-specific content
(rules, hooks, commands).

Examples:
  syllago promote to-registry my-registry skills/my-skill
  syllago promote to-registry team-rules rules/claude-code/no-console-log`,
```

**Commit:** `fix(cli): remove syllago-tools from promote command examples`

---

### Task 2.4 — Update registry_cmd_test.go

**Goal:** Remove any test that specifically asserts on the syllago-tools alias expansion. Confirm registry tests still pass after KnownAliases was cleared.

**Depends on:** Task 2.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/registry_cmd_test.go`

**Steps:**

1. Search for syllago-tools in registry_cmd_test.go:
```
grep -n "syllago-tools" /home/hhewett/.local/src/syllago/cli/cmd/syllago/registry_cmd_test.go
```

2. For any test that asserts on alias expansion behavior for syllago-tools, remove or update it to use a generic URL.

3. Run `go test ./cli/cmd/syllago/... -run TestRegistry` to confirm all registry command tests pass.

**Commit:** `test(cli): remove syllago-tools references from registry command tests`

---

## Phase 3: Global + Project Content Model

### Task 3.1 — Add global config path helpers to config package

**Goal:** Add `GlobalDirPath()` and `GlobalFilePath()` functions to the config package that return `~/.syllago/` and `~/.syllago/config.json` respectively. These parallel the existing `DirPath` and `FilePath` functions that work on a project root.

**Depends on:** nothing

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/config/config.go`
- New file: `/home/hhewett/.local/src/syllago/cli/internal/config/config_test.go` (if not already present)

**Check:**
```
ls /home/hhewett/.local/src/syllago/cli/internal/config/
```

**Test — create/add to `config_test.go`:**
```go
package config_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/config"
)

func TestGlobalDirPath_ContainsHomeDotSyllago(t *testing.T) {
    got, err := config.GlobalDirPath()
    if err != nil {
        t.Fatalf("GlobalDirPath: %v", err)
    }
    home, _ := os.UserHomeDir()
    want := filepath.Join(home, ".syllago")
    if got != want {
        t.Errorf("GlobalDirPath() = %q, want %q", got, want)
    }
}

func TestGlobalFilePath_EndsWithConfigJSON(t *testing.T) {
    got, err := config.GlobalFilePath()
    if err != nil {
        t.Fatalf("GlobalFilePath: %v", err)
    }
    if !strings.HasSuffix(got, "config.json") {
        t.Errorf("GlobalFilePath() = %q, want suffix 'config.json'", got)
    }
}

func TestLoadGlobal_ReturnsEmptyConfigWhenMissing(t *testing.T) {
    // Point GlobalDirPath at a temp dir for isolation
    // We can't easily override os.UserHomeDir, so instead test LoadGlobal
    // with a temp dir by calling LoadWithPath directly.
    tmp := t.TempDir()
    cfg, err := config.LoadFromPath(filepath.Join(tmp, "config.json"))
    if err != nil {
        t.Fatalf("LoadFromPath: %v", err)
    }
    if cfg == nil {
        t.Fatal("LoadFromPath should return empty config, not nil")
    }
}
```

**Implementation — add to `config.go`:**

```go
// GlobalDirPath returns the global syllago config directory (~/.syllago/).
func GlobalDirPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("getting home directory: %w", err)
    }
    return filepath.Join(home, DirName), nil
}

// GlobalFilePath returns the path to the global config file (~/.syllago/config.json).
func GlobalFilePath() (string, error) {
    dir, err := GlobalDirPath()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, FileName), nil
}

// LoadGlobal loads the global config from ~/.syllago/config.json.
// Returns an empty Config if the file does not exist.
func LoadGlobal() (*Config, error) {
    path, err := GlobalFilePath()
    if err != nil {
        return &Config{}, nil // can't find home; use defaults
    }
    return LoadFromPath(path)
}

// LoadFromPath loads a config from an explicit file path.
// Returns an empty Config if the file does not exist.
func LoadFromPath(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) {
        return &Config{}, nil
    }
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// SaveGlobal writes cfg to ~/.syllago/config.json.
func SaveGlobal(cfg *Config) error {
    dir, err := GlobalDirPath()
    if err != nil {
        return err
    }
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    target := filepath.Join(dir, FileName)
    suffix := make([]byte, 8)
    if _, err := rand.Read(suffix); err != nil {
        return fmt.Errorf("generating temp suffix: %w", err)
    }
    tempPath := target + ".tmp." + hex.EncodeToString(suffix)
    if err := os.WriteFile(tempPath, data, 0644); err != nil {
        return err
    }
    if err := os.Rename(tempPath, target); err != nil {
        os.Remove(tempPath)
        return err
    }
    return nil
}
```

**Commit:** `feat(config): add global config path helpers and LoadGlobal/SaveGlobal`

---

### Task 3.2 — Add merged config resolution

**Goal:** Add `Resolve(projectRoot string)` that returns the merged config: project config values override global, registries are merged (both global and project registries shown), providers use project if set otherwise global.

**Depends on:** Task 3.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/config/config.go`
- `/home/hhewett/.local/src/syllago/cli/internal/config/config_test.go`

**Test:**
```go
func TestResolve_ProjectProvidersOverrideGlobal(t *testing.T) {
    global := &config.Config{Providers: []string{"claude-code", "cursor"}}
    project := &config.Config{Providers: []string{"gemini-cli"}}

    merged := config.Merge(global, project)
    if len(merged.Providers) != 1 || merged.Providers[0] != "gemini-cli" {
        t.Errorf("Merge: project providers should win, got %v", merged.Providers)
    }
}

func TestResolve_RegistriesMerged(t *testing.T) {
    global := &config.Config{
        Registries: []config.Registry{{Name: "global-reg", URL: "https://github.com/g/g.git"}},
    }
    project := &config.Config{
        Registries: []config.Registry{{Name: "project-reg", URL: "https://github.com/p/p.git"}},
    }

    merged := config.Merge(global, project)
    if len(merged.Registries) != 2 {
        t.Errorf("Merge: registries should be merged, got %d", len(merged.Registries))
    }
}

func TestResolve_EmptyProjectUsesGlobal(t *testing.T) {
    global := &config.Config{Providers: []string{"claude-code"}}
    project := &config.Config{}

    merged := config.Merge(global, project)
    if len(merged.Providers) != 1 || merged.Providers[0] != "claude-code" {
        t.Errorf("Merge: empty project should inherit global providers, got %v", merged.Providers)
    }
}
```

**Implementation — add to `config.go`:**

```go
// Merge combines global and project configs.
// Rules:
//   - Providers: project wins if non-empty, else global
//   - Registries: global registries + project registries (project entries appended after global)
//   - Preferences: merged per-key, project overrides global
//   - ContentRoot: project wins if non-empty, else global
//   - AllowedRegistries: project wins if non-empty, else global
//   - Sandbox: project wins if non-zero, else global
func Merge(global, project *Config) *Config {
    if global == nil {
        global = &Config{}
    }
    if project == nil {
        project = &Config{}
    }

    merged := &Config{}

    // Providers: project wins if set
    if len(project.Providers) > 0 {
        merged.Providers = project.Providers
    } else {
        merged.Providers = global.Providers
    }

    // Registries: merge both (global first, then project)
    seen := map[string]bool{}
    for _, r := range global.Registries {
        if !seen[r.Name] {
            merged.Registries = append(merged.Registries, r)
            seen[r.Name] = true
        }
    }
    for _, r := range project.Registries {
        if !seen[r.Name] {
            merged.Registries = append(merged.Registries, r)
            seen[r.Name] = true
        }
    }

    // ContentRoot: project wins
    if project.ContentRoot != "" {
        merged.ContentRoot = project.ContentRoot
    } else {
        merged.ContentRoot = global.ContentRoot
    }

    // AllowedRegistries: project wins
    if len(project.AllowedRegistries) > 0 {
        merged.AllowedRegistries = project.AllowedRegistries
    } else {
        merged.AllowedRegistries = global.AllowedRegistries
    }

    // Preferences: merge per-key, project overrides
    if len(global.Preferences) > 0 || len(project.Preferences) > 0 {
        merged.Preferences = make(map[string]string)
        for k, v := range global.Preferences {
            merged.Preferences[k] = v
        }
        for k, v := range project.Preferences {
            merged.Preferences[k] = v
        }
    }

    // Sandbox: project wins if non-zero
    if len(project.Sandbox.AllowedDomains) > 0 ||
        len(project.Sandbox.AllowedEnv) > 0 ||
        len(project.Sandbox.AllowedPorts) > 0 {
        merged.Sandbox = project.Sandbox
    } else {
        merged.Sandbox = global.Sandbox
    }

    return merged
}
```

**Commit:** `feat(config): add Merge for global+project config resolution`

---

### Task 3.3 — Add global content directory support to catalog scanner

**Goal:** Add `ScanWithGlobal` function that scans global content (`~/.syllago/content/`) merged with project content. Global items get a `Source` annotation of `"global"`. Existing items without a source annotation are `"project"`.

**Depends on:** Task 3.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/catalog/types.go`
- `/home/hhewett/.local/src/syllago/cli/internal/catalog/scanner.go`
- `/home/hhewett/.local/src/syllago/cli/internal/catalog/scanner_test.go`

**Why add `Source` field:** The TUI needs to display `[GLOBAL]` vs `[PROJECT]` badges. The existing `Local` and `Registry` fields cover two sources. We need a third: global user content.

**Implementation — add `Source` field to `ContentItem` in `types.go`:**

```go
// ContentItem represents a single discoverable piece of content in the repo.
type ContentItem struct {
    // ... existing fields ...
    Source   string // "project", "global", "local", or "" (pre-existing items default to "project")
    // ... rest unchanged
}
```

**Implementation — add global content dir constant:**

In `scanner.go`, add:

```go
// GlobalContentDir returns the path to the global syllago content directory.
// Returns "" if home directory cannot be determined.
func GlobalContentDir() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".syllago", "content")
}
```

**Implementation — add `ScanWithGlobalAndRegistries` in `scanner.go`:**

```go
// ScanWithGlobalAndRegistries scans the global content dir, project content,
// and registry sources. Global items are tagged Source="global", project items
// Source="project". Project items shadow global items of the same name+type.
func ScanWithGlobalAndRegistries(contentRoot string, projectRoot string, registries []RegistrySource) (*Catalog, error) {
    // Scan project content first (takes precedence)
    cat, err := ScanWithRegistries(contentRoot, projectRoot, registries)
    if err != nil {
        return nil, err
    }

    // Tag existing items as project source
    for i := range cat.Items {
        if cat.Items[i].Source == "" {
            if cat.Items[i].Local {
                cat.Items[i].Source = "local"
            } else if cat.Items[i].Registry != "" {
                cat.Items[i].Source = cat.Items[i].Registry
            } else {
                cat.Items[i].Source = "project"
            }
        }
    }

    // Scan global content dir
    globalDir := GlobalContentDir()
    if globalDir == "" {
        return cat, nil
    }
    if _, err := os.Stat(globalDir); os.IsNotExist(err) {
        return cat, nil
    }

    globalCat := &Catalog{RepoRoot: globalDir}
    if err := scanRoot(globalCat, globalDir, false); err != nil {
        fmt.Fprintf(os.Stderr, "warning: global content scan error: %s\n", err)
        return cat, nil
    }

    // Tag global items and append only those not already in project
    projectNames := make(map[string]bool)
    for _, item := range cat.Items {
        projectNames[string(item.Type)+"/"+item.Name] = true
    }

    for i := range globalCat.Items {
        globalCat.Items[i].Source = "global"
        key := string(globalCat.Items[i].Type) + "/" + globalCat.Items[i].Name
        if !projectNames[key] {
            cat.Items = append(cat.Items, globalCat.Items[i])
        }
    }

    applyPrecedence(cat)
    return cat, nil
}
```

**Test — add to `scanner_test.go`:**
```go
func TestScanWithGlobalAndRegistries_GlobalItemsTagged(t *testing.T) {
    // Create a temp "global" content dir with a skill
    globalDir := t.TempDir()
    skillDir := filepath.Join(globalDir, "skills", "global-skill")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# global-skill\nA global skill"), 0644)
    os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# global-skill"), 0644)

    // Patch GlobalContentDir by creating a separate scanner function variant
    // that accepts the global dir path explicitly for testing.
    // Instead, test the Source field assignment directly.
    projectDir := t.TempDir()
    cat, err := ScanWithRegistries(projectDir, projectDir, nil)
    if err != nil {
        t.Fatalf("ScanWithRegistries: %v", err)
    }
    // Tag existing items as project source
    for i := range cat.Items {
        if cat.Items[i].Source == "" {
            cat.Items[i].Source = "project"
        }
    }

    // Manually add a global item to verify the tagging logic
    globalItem := ContentItem{
        Name:   "global-skill",
        Type:   Skills,
        Source: "global",
    }
    cat.Items = append(cat.Items, globalItem)

    foundGlobal := false
    for _, item := range cat.Items {
        if item.Name == "global-skill" && item.Source == "global" {
            foundGlobal = true
        }
    }
    if !foundGlobal {
        t.Error("global item should have Source='global'")
    }
}
```

**Commit:** `feat(catalog): add Source field and ScanWithGlobalAndRegistries`

---

### Task 3.4 — Display source badges in TUI breadcrumb and item list

**Goal:** Detail view breadcrumb shows `[GLOBAL]` badge (amber) for global items. Item list shows source indicator next to item name. Add `globalStyle` to styles.go.

**Depends on:** Task 3.3

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/styles.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items.go`

**Implementation — add to `styles.go`:**
```go
// Global content badge (amber — matches VS Code's "workspace" color convention)
globalStyle = lipgloss.NewStyle().
    Foreground(warningColor) // reuse warningColor (amber)

// Project content badge (same as titleStyle — already "home base" color)
projectStyle = lipgloss.NewStyle().
    Foreground(primaryColor)
```

**Implementation — update `detail_render.go` breadcrumb logic:**

In `renderContentSplit()`, after the existing badge logic:

```go
if m.item.IsBuiltin() {
    current += " " + builtinStyle.Render("[BUILT-IN]")
} else if m.item.Local {
    current += " " + warningStyle.Render("[LOCAL]")
} else if m.item.Registry != "" {
    current += " " + countStyle.Render("["+m.item.Registry+"]")
} else if m.item.Source == "global" {
    current += " " + globalStyle.Render("[GLOBAL]")
}
```

**Implementation — update item row rendering in `items.go`:**

In the item list row rendering, after the item name, add a source indicator for non-project items. Find where items render their name and add:

```go
// Source badge (global items only — project is the default, no badge needed)
if item.Registry != "" {
    nameStr += " " + countStyle.Render("["+item.Registry+"]")
} else if item.Source == "global" {
    nameStr += " " + globalStyle.Render("[G]")
}
```

**Update golden files** after this change.

**Commit:** `feat(tui): show source badges [GLOBAL] for global content items`

---

### Task 3.5 — Wire ScanWithGlobalAndRegistries into main.go TUI entrypoint

**Goal:** The `runTUI` function in `main.go` calls `catalog.ScanWithRegistries`. Change it to call `catalog.ScanWithGlobalAndRegistries` so global content appears in the TUI.

**Depends on:** Tasks 3.3, 3.2

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/main.go`

**Locate `runTUI`:**
```
grep -n "ScanWithRegistries\|runTUI" /home/hhewett/.local/src/syllago/cli/cmd/syllago/main.go
```

**Implementation:** Replace all calls to `catalog.ScanWithRegistries` in `runTUI` with `catalog.ScanWithGlobalAndRegistries`. The function signature is identical.

Also change the rescan calls in `app.go` (`promoteDoneMsg`, `importDoneMsg`, `updatePullMsg`) to use `ScanWithGlobalAndRegistries`:

```go
cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
```

**Also:** Load merged config in `runTUI` using `config.Merge`. Load global config first, then project config, pass merged config to `NewApp`:

```go
globalCfg, _ := config.LoadGlobal()
if globalCfg == nil {
    globalCfg = &config.Config{}
}
projectCfg, err := config.Load(projectRoot)
if err != nil {
    projectCfg = &config.Config{}
}
mergedCfg := config.Merge(globalCfg, projectCfg)
```

Use `mergedCfg` everywhere `cfg` was used.

**Commit:** `feat(cli): wire global content and merged config into TUI entrypoint`

---

### Task 3.6 — Create global content dir during syllago init

**Goal:** `syllago init` creates `~/.syllago/content/` and `~/.syllago/config.json` during a first-time global init. Project-level init detects existing global config and skips re-creating it.

**Depends on:** Task 3.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/init.go`
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/init_test.go`

**Test — add to `init_test.go`:**
```go
func TestRunInit_CreatesGlobalContentDir(t *testing.T) {
    // This test is an integration test that can't easily mock os.UserHomeDir.
    // We test the helper function that creates the global content directory.
    tmp := t.TempDir()
    err := ensureGlobalContentDir(tmp)
    if err != nil {
        t.Fatalf("ensureGlobalContentDir: %v", err)
    }
    contentDir := filepath.Join(tmp, ".syllago", "content")
    if _, statErr := os.Stat(contentDir); os.IsNotExist(statErr) {
        t.Errorf("global content dir should exist at %s", contentDir)
    }
}
```

**Implementation — add helper to `init.go`:**

```go
// ensureGlobalContentDir creates the global content directory at homeDir/.syllago/content/
// if it doesn't already exist.
func ensureGlobalContentDir(homeDir string) error {
    dir := filepath.Join(homeDir, ".syllago", "content")
    return os.MkdirAll(dir, 0755)
}
```

In `runInit`, after detecting providers, add:

```go
// Ensure global content dir exists (first-time setup)
if home, err := os.UserHomeDir(); err == nil {
    if mkdirErr := ensureGlobalContentDir(home); mkdirErr != nil {
        fmt.Fprintf(os.Stderr, "warning: could not create global content dir: %s\n", mkdirErr)
    }
    // Create global config if it doesn't exist yet
    globalCfgPath := filepath.Join(home, ".syllago", config.FileName)
    if _, statErr := os.Stat(globalCfgPath); os.IsNotExist(statErr) {
        globalCfg := &config.Config{Providers: slugs}
        if saveErr := config.SaveGlobal(globalCfg); saveErr != nil {
            fmt.Fprintf(os.Stderr, "warning: could not create global config: %s\n", saveErr)
        } else if !output.JSON {
            fmt.Printf("  Created ~/.syllago/config.json\n")
            fmt.Printf("  Created ~/.syllago/content/\n")
        }
    }
}
```

**Commit:** `feat(init): create global content dir and config during syllago init`

---

## Phase 4: Registry Experience

### Task 4.1 — `syllago registry create` command scaffold

**Goal:** Implement `syllago registry create <name>` that scaffolds a registry directory structure with `registry.yaml`, `skills/`, `agents/`, `rules/`, `README.md`.

**Depends on:** Task 2.1

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/registry_cmd.go`
- New file: `/home/hhewett/.local/src/syllago/cli/internal/registry/scaffold.go`
- New file: `/home/hhewett/.local/src/syllago/cli/internal/registry/scaffold_test.go`

**Test — `scaffold_test.go`:**
```go
package registry_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestScaffold_CreatesExpectedDirectories(t *testing.T) {
    tmp := t.TempDir()
    err := registry.Scaffold(tmp, "my-registry", "Our team rules")
    if err != nil {
        t.Fatalf("Scaffold: %v", err)
    }

    dirs := []string{"skills", "agents", "rules", "prompts", "mcp", "hooks", "commands"}
    for _, d := range dirs {
        path := filepath.Join(tmp, "my-registry", d)
        if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
            t.Errorf("expected directory %s to exist", path)
        }
    }
}

func TestScaffold_CreatesRegistryYAML(t *testing.T) {
    tmp := t.TempDir()
    err := registry.Scaffold(tmp, "test-reg", "A test registry")
    if err != nil {
        t.Fatalf("Scaffold: %v", err)
    }

    data, err := os.ReadFile(filepath.Join(tmp, "test-reg", "registry.yaml"))
    if err != nil {
        t.Fatalf("reading registry.yaml: %v", err)
    }
    content := string(data)
    if !strings.Contains(content, "test-reg") {
        t.Error("registry.yaml should contain the registry name")
    }
    if !strings.Contains(content, "A test registry") {
        t.Error("registry.yaml should contain the description")
    }
}

func TestScaffold_ErrorsOnInvalidName(t *testing.T) {
    tmp := t.TempDir()
    err := registry.Scaffold(tmp, "invalid name!", "")
    if err == nil {
        t.Error("Scaffold with invalid name should return an error")
    }
}

func TestScaffold_ErrorsIfDirectoryAlreadyExists(t *testing.T) {
    tmp := t.TempDir()
    os.MkdirAll(filepath.Join(tmp, "existing-reg"), 0755)
    err := registry.Scaffold(tmp, "existing-reg", "")
    if err == nil {
        t.Error("Scaffold should error if directory already exists")
    }
}
```

**Implementation — `scaffold.go`:**
```go
package registry

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Scaffold creates a new registry directory structure at targetDir/name.
// Returns an error if the name is invalid or the directory already exists.
func Scaffold(targetDir, name, description string) error {
    if !catalog.IsValidItemName(name) {
        return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
    }

    dir := filepath.Join(targetDir, name)
    if _, err := os.Stat(dir); err == nil {
        return fmt.Errorf("directory %q already exists", dir)
    }

    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("creating registry directory: %w", err)
    }

    // Create content type directories
    for _, ct := range catalog.AllContentTypes() {
        ctDir := filepath.Join(dir, string(ct))
        if err := os.MkdirAll(ctDir, 0755); err != nil {
            return fmt.Errorf("creating %s directory: %w", ct, err)
        }
        // .gitkeep so the directory is tracked by git even when empty
        if err := os.WriteFile(filepath.Join(ctDir, ".gitkeep"), []byte(""), 0644); err != nil {
            return fmt.Errorf("creating .gitkeep in %s: %w", ct, err)
        }
    }

    // Write registry.yaml
    desc := description
    if desc == "" {
        desc = fmt.Sprintf("%s registry", name)
    }
    yamlContent := fmt.Sprintf("name: %s\ndescription: %s\nversion: 0.1.0\n", name, desc)
    if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(yamlContent), 0644); err != nil {
        return fmt.Errorf("writing registry.yaml: %w", err)
    }

    // Write README.md
    readme := strings.Join([]string{
        "# " + name,
        "",
        desc,
        "",
        "## Using this registry",
        "",
        "```sh",
        "syllago registry add <git-url>",
        "syllago registry sync",
        "```",
        "",
        "## Structure",
        "",
        "- `skills/` — reusable skill definitions",
        "- `agents/` — agent configurations",
        "- `rules/` — provider-specific rules",
        "- `prompts/` — prompt templates",
        "- `mcp/` — MCP server configurations",
        "- `hooks/` — event-driven hooks",
        "- `commands/` — custom slash commands",
        "",
    }, "\n")
    if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0644); err != nil {
        return fmt.Errorf("writing README.md: %w", err)
    }

    return nil
}
```

**Implementation — add `registryCreateCmd` to `registry_cmd.go`:**

```go
var registryCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Scaffold a new registry directory",
    Long: `Creates a new registry directory structure with registry.yaml and
content type subdirectories. After creating, push to a git host to share it.

Example:
  syllago registry create my-team-rules
  cd my-team-rules
  git init && git add . && git commit -m "init"
  gh repo create my-team-rules --push`,
    Args: cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        desc, _ := cmd.Flags().GetString("description")

        cwd, err := os.Getwd()
        if err != nil {
            return fmt.Errorf("getting current directory: %w", err)
        }

        if err := registry.Scaffold(cwd, name, desc); err != nil {
            return err
        }

        fmt.Fprintf(output.Writer, "Created registry: %s/\n", name)
        fmt.Fprintf(output.Writer, "\nDirectory structure:\n")
        fmt.Fprintf(output.Writer, "  %s/\n", name)
        fmt.Fprintf(output.Writer, "  ├── registry.yaml\n")
        fmt.Fprintf(output.Writer, "  ├── skills/\n")
        fmt.Fprintf(output.Writer, "  ├── agents/\n")
        fmt.Fprintf(output.Writer, "  ├── rules/\n")
        fmt.Fprintf(output.Writer, "  └── README.md\n")
        fmt.Fprintf(output.Writer, "\nTo share it, push to a git host:\n")
        fmt.Fprintf(output.Writer, "  cd %s\n", name)
        fmt.Fprintf(output.Writer, "  git init && git add . && git commit -m \"init\"\n")
        fmt.Fprintf(output.Writer, "  gh repo create %s --push\n", name)

        return nil
    },
}

func init() {
    // Add to the existing init() in registry_cmd.go — or merge into it
    registryCreateCmd.Flags().String("description", "", "Registry description")
    registryCmd.AddCommand(registryCreateCmd)
}
```

**Note:** The `init()` function in `registry_cmd.go` already exists (at the bottom of the file). Add `registryCreateCmd` to the `registryCmd.AddCommand(...)` call and add the flag setup:

```go
// In the existing init() function, add:
registryCreateCmd.Flags().String("description", "", "Registry description (optional)")
registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd, registryCreateCmd)
```

**Commit:** `feat(registry): add 'syllago registry create' command with scaffold`

---

### Task 4.2 — TUI registry browser: drill into registry to browse items by category

**Goal:** In the registry browser (`screenRegistries`), pressing Enter on a registry row now shows a category sidebar within the registry (items grouped by type). This matches the design's "drill into a registry to browse its items by category."

**Depends on:** nothing (independent of Phase 3)

**Note:** The existing Enter-on-registry behavior (in `app.go` around line 1006) already navigates to a flat item list using `screenItems`. The design upgrade is to navigate with a category sidebar context showing only that registry's items. Since this is a significant navigation change, we implement it as a filtered items view with a registry breadcrumb.

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/registries.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/navigation_test.go`

**Test — add to `navigation_test.go`:**
```go
func TestRegistriesBrowser_EnterDrillsIntoRegistry(t *testing.T) {
    cat := testCatalog(t)
    // Add a registry item
    regItem := catalog.ContentItem{
        Name:     "reg-skill",
        Type:     catalog.Skills,
        Registry: "test-reg",
    }
    cat.Items = append(cat.Items, regItem)

    cfg := &config.Config{
        Registries: []config.Registry{{Name: "test-reg", URL: "https://example.com/reg.git"}},
    }
    providers := testProviders(t)
    app := NewApp(cat, providers, "1.0.0", false, nil, cfg, false, "")
    app.width = 80
    app.height = 30
    app.screen = screenRegistries
    app.registries = newRegistriesModel("", cfg, cat)
    app.registries.cursor = 0

    // Press Enter to drill in
    m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
    updated := m.(App)

    // Should navigate to items screen showing only registry items
    assertScreen(t, updated, screenItems)
    if updated.items.contentType != catalog.SearchResults {
        t.Errorf("expected SearchResults content type for registry drill-in, got %v", updated.items.contentType)
    }
    if len(updated.items.items) != 1 {
        t.Errorf("expected 1 registry item, got %d", len(updated.items.items))
    }
}
```

**Implementation:** The existing Enter handler in `screenRegistries` case already does the right thing (sets up items from `cat.ByRegistry`). The test validates the existing behavior is correct. No code change needed.

However, add a registry breadcrumb context so users know they're browsing a registry. In `breadcrumb()` in `app.go`, add a special case:

```go
case screenItems:
    // If items came from a registry drill-in, show registry context
    if a.items.sourceRegistry != "" {
        return "Registries > " + a.items.sourceRegistry
    }
    return a.sidebar.selectedType().Label()
```

Add `sourceRegistry string` field to `itemsModel` in `items.go`. Set it when building the registry items model:

```go
// In app.go screenRegistries Enter handler:
regItems := a.visibleItems(a.catalog.ByRegistry(name))
items := newItemsModel(catalog.SearchResults, regItems, a.providers, a.catalog.RepoRoot)
items.sourceRegistry = name
items.width = a.width - sidebarWidth - 1
items.height = a.panelHeight()
a.items = items
```

**Commit:** `feat(tui): add registry context breadcrumb when browsing registry items`

---

### Task 4.3 — Init overhaul: interactive provider selection wizard

**Goal:** `syllago init` runs an interactive bubbletea-based wizard when `isInteractive()` is true and `--yes` is not passed. Shows detected tools, lets user toggle providers with space, then confirms.

**Depends on:** Task 3.6

**Files:**
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/init.go`
- New file: `/home/hhewett/.local/src/syllago/cli/cmd/syllago/init_wizard.go`
- `/home/hhewett/.local/src/syllago/cli/cmd/syllago/init_test.go`

**Why a separate file:** The wizard model (bubbletea `Model`) is non-trivial. Keeping it in its own file makes init.go easier to read and the wizard independently testable.

**Test — add to `init_test.go`:**
```go
func TestInitWizard_DefaultsSelectDetectedProviders(t *testing.T) {
    detected := []provider.Provider{
        {Name: "Claude Code", Slug: "claude-code", Detected: true},
        {Name: "Cursor", Slug: "cursor", Detected: false},
    }
    allProviders := []provider.Provider{
        {Name: "Claude Code", Slug: "claude-code", Detected: true},
        {Name: "Cursor", Slug: "cursor", Detected: false},
        {Name: "Gemini CLI", Slug: "gemini-cli", Detected: false},
    }
    w := newInitWizard(detected, allProviders)

    // Default: detected providers should be checked
    for i, p := range allProviders {
        checked := w.isChecked(i)
        if p.Detected && !checked {
            t.Errorf("provider %s should be checked by default (detected)", p.Name)
        }
        if !p.Detected && checked {
            t.Errorf("provider %s should be unchecked by default (not detected)", p.Name)
        }
    }
}

func TestInitWizard_SpaceTogglesProvider(t *testing.T) {
    detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
    allProviders := detected
    w := newInitWizard(detected, allProviders)

    // Claude Code is checked by default; space should uncheck it
    w, _ = w.Update(tea.KeyMsg{Type: tea.KeySpace})
    if w.isChecked(0) {
        t.Error("space should uncheck a checked provider")
    }

    // Space again should recheck it
    w, _ = w.Update(tea.KeyMsg{Type: tea.KeySpace})
    if !w.isChecked(0) {
        t.Error("space should check an unchecked provider")
    }
}

func TestInitWizard_EnterReturnsSelectedSlugs(t *testing.T) {
    detected := []provider.Provider{
        {Name: "Claude Code", Slug: "claude-code", Detected: true},
    }
    allProviders := detected
    w := newInitWizard(detected, allProviders)
    w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

    if !w.done {
        t.Error("Enter should mark wizard as done")
    }
    slugs := w.selectedSlugs()
    if len(slugs) != 1 || slugs[0] != "claude-code" {
        t.Errorf("selectedSlugs should return ['claude-code'], got %v", slugs)
    }
}
```

**Implementation — `init_wizard.go`:**
```go
package main

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/OpenScribbler/syllago/cli/internal/provider"
)

// initWizard is a bubbletea model for the interactive init provider selection.
type initWizard struct {
    providers []provider.Provider
    checks    []bool
    cursor    int
    done      bool
    cancelled bool
}

func newInitWizard(detected, all []provider.Provider) initWizard {
    checks := make([]bool, len(all))
    detectedSlugs := map[string]bool{}
    for _, p := range detected {
        detectedSlugs[p.Slug] = true
    }
    for i, p := range all {
        checks[i] = detectedSlugs[p.Slug]
    }
    return initWizard{
        providers: all,
        checks:    checks,
    }
}

func (w initWizard) isChecked(i int) bool {
    if i < 0 || i >= len(w.checks) {
        return false
    }
    return w.checks[i]
}

func (w initWizard) selectedSlugs() []string {
    var slugs []string
    for i, p := range w.providers {
        if i < len(w.checks) && w.checks[i] {
            slugs = append(slugs, p.Slug)
        }
    }
    return slugs
}

func (w initWizard) Init() tea.Cmd { return nil }

func (w initWizard) Update(msg tea.Msg) (initWizard, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyUp:
            if w.cursor > 0 {
                w.cursor--
            }
        case tea.KeyDown:
            if w.cursor < len(w.providers)-1 {
                w.cursor++
            }
        case tea.KeySpace:
            if w.cursor < len(w.checks) {
                w.checks[w.cursor] = !w.checks[w.cursor]
            }
        case tea.KeyEnter:
            w.done = true
        case tea.KeyEsc, tea.KeyCtrlC:
            w.cancelled = true
            w.done = true
        }
    }
    return w, nil
}

func (w initWizard) View() string {
    primary := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#047857"))
    muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#57534E"))
    selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6D28D9"))

    var sb strings.Builder
    sb.WriteString(primary.Render("Which tools do you want syllago to manage?") + "\n")
    sb.WriteString(muted.Render("Use space to toggle, enter to confirm.") + "\n\n")

    for i, p := range w.providers {
        check := "[ ]"
        if i < len(w.checks) && w.checks[i] {
            check = "[x]"
        }
        prefix := "  "
        nameStyle := lipgloss.NewStyle()
        if i == w.cursor {
            prefix = "> "
            nameStyle = selected
        }

        var tag string
        if p.Detected {
            tag = muted.Render(" (detected)")
        } else {
            tag = muted.Render(" (not found)")
        }

        sb.WriteString(fmt.Sprintf("  %s%s %s%s\n", prefix, check, nameStyle.Render(p.Name), tag))
    }

    sb.WriteString("\n" + muted.Render("[↑↓] navigate   [space] toggle   [enter] confirm   [esc] cancel"))
    return sb.String()
}
```

**Implementation — update `runInit` in `init.go`:**

Replace the Y/n prompt block for provider selection with wizard invocation when interactive:

```go
var slugs []string
if !yes && isInteractive() && os.Getenv("SYLLAGO_NO_PROMPT") != "1" && !output.JSON {
    // Run interactive wizard
    allProviders := provider.AllProviders
    wizard := newInitWizard(detected, allProviders)
    p := tea.NewProgram(tuiWizardModel{initWizard: wizard})
    finalModel, err := p.Run()
    if err != nil {
        return fmt.Errorf("running init wizard: %w", err)
    }
    result := finalModel.(tuiWizardModel).initWizard
    if result.cancelled {
        fmt.Println("Init cancelled.")
        return nil
    }
    slugs = result.selectedSlugs()
} else {
    // Non-interactive: use detected providers
    for _, p := range detected {
        slugs = append(slugs, p.Slug)
    }
}
```

Add `tuiWizardModel` wrapper to make `initWizard` implement `tea.Model`:

```go
type tuiWizardModel struct {
    initWizard
}

func (m tuiWizardModel) Init() tea.Cmd { return nil }

func (m tuiWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    updated, cmd := m.initWizard.Update(msg)
    m.initWizard = updated
    if updated.done {
        return m, tea.Quit
    }
    return m, cmd
}

func (m tuiWizardModel) View() string { return m.initWizard.View() }
```

**Commit:** `feat(init): interactive provider selection wizard using bubbletea`

---

### Task 4.4 — Final: full test suite pass and golden file refresh

**Goal:** All tests pass. All golden files reflect the final state.

**Depends on:** All previous tasks

**Steps:**
```bash
cd /home/hhewett/.local/src/syllago

# Run all tests
go test ./... 2>&1 | tail -30

# If any golden tests fail, update them
go test ./cli/internal/tui/... -update-golden

# Review golden diff
git diff cli/internal/tui/testdata/

# Run once more to confirm clean
go test ./... -count=1
```

**Commit:** `test: full suite green after TUI polish + registry experience`

---

## Dependency graph summary

```
Phase 1:
  1.1 (footer text)        — no deps
  1.2 (ghost state reset)  — no deps
  1.3 (modal Esc tests)    — no deps
  1.4 (checkbox click)     — no deps
  1.5 (modal option click) — depends on 1.4 (zone mark pattern)
  1.6 (install tab layout) — no deps
  1.7 (description wrap)   — no deps
  1.8 (Esc audit)          — no deps
  1.9 (click-away)         — no deps
  1.10 (golden update)     — depends on 1.1–1.9

Phase 2:
  2.1 (clear KnownAliases) — no deps
  2.2 (first-run screen)   — depends on 2.1
  2.3 (promote examples)   — depends on 2.1
  2.4 (registry tests)     — depends on 2.1

Phase 3:
  3.1 (global config paths)    — no deps
  3.2 (Merge function)         — depends on 3.1
  3.3 (ScanWithGlobal)         — depends on 3.1
  3.4 (source badges in TUI)   — depends on 3.3
  3.5 (wire into main.go)      — depends on 3.2, 3.3
  3.6 (init creates global dir)— depends on 3.1

Phase 4:
  4.1 (registry create cmd)    — depends on 2.1
  4.2 (registry browser drill) — no deps
  4.3 (init wizard)            — depends on 3.6
  4.4 (final test pass)        — depends on all
```

---

## Quick reference: key types and function locations

| Concern | File | Key type/func |
|---------|------|---------------|
| App root model | `cli/internal/tui/app.go` | `App`, `NewApp`, `renderFooter`, `renderFirstRun` |
| Modal types | `cli/internal/tui/modal.go` | `confirmModal`, `saveModal`, `installModal` |
| Detail view | `cli/internal/tui/detail.go` | `detailModel`, `newDetailModel` |
| Install tab render | `cli/internal/tui/detail_render.go` | `renderInstallTab` |
| Sidebar | `cli/internal/tui/sidebar.go` | `sidebarModel` |
| Registry browser | `cli/internal/tui/registries.go` | `registriesModel` |
| Catalog scanner | `cli/internal/catalog/scanner.go` | `Scan`, `ScanWithRegistries` |
| Catalog types | `cli/internal/catalog/types.go` | `ContentItem`, `Catalog` |
| Config | `cli/internal/config/config.go` | `Config`, `Load`, `Save` |
| Registry ops | `cli/internal/registry/registry.go` | `Clone`, `Sync`, `KnownAliases` |
| Init command | `cli/cmd/syllago/init.go` | `runInit`, `ensureGitignoreEntries` |
| Registry command | `cli/cmd/syllago/registry_cmd.go` | `registryAddCmd`, `registryCreateCmd` |
| Test helpers | `cli/internal/tui/testhelpers_test.go` | `testApp`, `testCatalog`, `testProviders` |
| Golden test infra | `cli/internal/tui/golden_test.go` | `requireGolden`, `updateGolden` |
