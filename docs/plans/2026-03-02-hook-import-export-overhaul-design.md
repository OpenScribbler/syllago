# Hook Import/Export Overhaul — Design Doc

## Status: Design Complete — Ready for Planning

---

## Problem Statement

Hooks are the only content type where multiple items live inside a single file (`settings.json`). The current import treats the entire file as one blob, making it impossible to manage individual hooks. Additionally, the stored format doesn't match what the installer expects, and the TUI has zero hook-specific rendering — no compatibility information, no structured display.

### Current Problems

1. **No splitting**: `syllago import --from claude-code --type hooks` imports all hooks as a single item named "settings"
2. **Format mismatch**: Import produces `{"hooks":{"EventName":[...]}}` but installer expects `{"event":"EventName","matcher":"...","hooks":[...]}`
3. **No compatibility visibility**: Users can't see which providers a hook works with or what features are lost
4. **Generic TUI**: Hooks are rendered identically to rules/commands — no event type, matcher, or compatibility info

---

## Decisions Made

### 1. Compatibility Levels

Four levels with colored symbols and detail-on-select:

| Level | Symbol | Color | Definition | Examples |
|-------|--------|-------|------------|---------|
| **Full** | ✓ | Green | All features translate, no behavioral change | Command hook, no matcher, supported event → any provider |
| **Degraded** | ~ | Yellow | Minor features lost, core behavior unchanged | `statusMessage` dropped on Kiro (hook works fine, no status text) |
| **Broken** | ! | Orange | Hook runs but behavior is fundamentally wrong — will cause real problems | Matcher dropped → fires on every tool call; LLM hook → wrapper script; async dropped on slow hook → blocks execution |
| **None** | ✗ | Red | Can't install — event doesn't exist on target | `SubagentStart` → any non-Claude provider |

**Feature → level mappings:**

| Lost Feature | Level | Why |
|-------------|-------|-----|
| `statusMessage` dropped | Degraded | Cosmetic only |
| Timeout precision (ms→sec rounding) | Degraded | Off by fractions of a second |
| Matcher dropped | **Broken** | Fires on everything instead of one tool |
| `async` dropped | **Broken** | Slow hooks will block execution |
| LLM hook → wrapper script | **Broken** | Different mechanism, requires CLI installed |
| LLM hook dropped entirely | **None** (for that entry) | No equivalent |
| Event not supported | **None** | Can't install at all |

### 2. TUI Hooks List — Compatibility Matrix

Only show the 4 hook-capable providers as columns (Claude Code, Gemini CLI, Copilot CLI, Kiro). Hookless providers (Cursor, Windsurf, Codex, etc.) are excluded.

Items alphabetized. Each cell shows the colored symbol (✓ ~ ! ✗). Selecting a cell shows detail about what's lost.

```
   Name                 Claude  Gemini  Copilot  Kiro
   ───────────────────  ──────  ──────  ───────  ────
 > bash-validator       ✓       ✓       !        ✓
   lint-on-edit         ✓       ✓       !        ✓
   llm-safety-check     ✓       ✗       ✗        ✗
   session-telemetry    ✓       ✓       ✓        ✓
```

### 3. Import Splitting

Import explodes `settings.json` into individual content items — one per matcher group per event. Each item gets the flat installer format:

```json
{"event": "PreToolUse", "matcher": "Bash", "hooks": [...]}
```

### 4. Hook Naming

Auto-derive names from content (event + matcher + command/statusMessage), then offer user a chance to rename. "Accept all" option for generated names.

Name derivation priority:
1. `statusMessage` if present → slugify (e.g., "Validating shell command..." → "validating-shell-command")
2. `matcher` + event → (e.g., "pretooluse-bash")
3. Event + first meaningful word(s) from command

### 5. Import UX

- **TUI wizard**: Multi-select which hooks to import, auto-derived names shown inline. No separate rename step.
- **CLI**: Batch import all by default. `--preview` to see what would be imported, `--exclude` to skip specific items.
- No fully-automatic/silent mode for TUI; CLI is fully automatic by default.

---

## Provider Capability Reference

### Hook-Capable Providers (4)

| Feature | Claude Code | Gemini CLI | Copilot CLI | Kiro |
|---------|:-----------:|:----------:|:-----------:|:----:|
| Events supported | 10/10 | 8/10 | 5/10 | 5/10 |
| Matchers | ✓ (group-level) | ✓ (group-level) | ✗ (dropped) | ✓ (per-entry) |
| Hook type: command | ✓ | ✓ | ✓ (bash/powershell) | ✓ |
| Hook type: prompt (LLM) | ✓ | ✗ | ✗ | ✗ |
| Hook type: agent (LLM) | ✓ | ✗ | ✗ | ✗ |
| Async | ✓ | ✓ | ✗ | ✗ |
| Timeout unit | ms | ms | seconds | ms |
| Status message | ✓ | ✓ | ✓ (as `comment`) | ✗ |
| PowerShell support | ✗ | ✗ | ✓ | ✗ |

### Event Support Matrix

| Canonical Event | Claude Code | Gemini CLI | Copilot CLI | Kiro |
|----------------|:-----------:|:----------:|:-----------:|:----:|
| PreToolUse | ✓ | ✓ (BeforeTool) | ✓ (preToolUse) | ✓ (preToolUse) |
| PostToolUse | ✓ | ✓ (AfterTool) | ✓ (postToolUse) | ✓ (postToolUse) |
| UserPromptSubmit | ✓ | ✓ (BeforeAgent) | ✓ (userPromptSubmitted) | ✓ (userPromptSubmit) |
| Stop | ✓ | ✓ (AfterAgent) | ✗ | ✓ (stop) |
| SessionStart | ✓ | ✓ | ✓ (sessionStart) | ✓ (agentSpawn) |
| SessionEnd | ✓ | ✓ | ✓ (sessionEnd) | ✗ |
| PreCompact | ✓ | ✓ (PreCompress) | ✗ | ✗ |
| Notification | ✓ | ✓ | ✗ | ✗ |
| SubagentStart | ✓ | ✗ | ✗ | ✗ |
| SubagentCompleted | ✓ | ✗ | ✗ | ✗ |

### Hookless Providers (7) — excluded from matrix

OpenCode, Zed, Cline, Roo Code, Cursor, Windsurf, Codex

---

## Designed Sections

### 6. TUI: Hooks Item List

**Responsive column layout** replacing single Provider column with 4-column compatibility matrix:

**Breakpoints by terminal width:**

| Terminal Width | Panel Width | Matrix Headers | Columns Shown |
|---------------|-------------|----------------|---------------|
| ≥120 | ≥101 | Full (Claude, Gemini, Copilot, Kiro) | Name + Matrix + Description |
| 80–119 | 61–100 | Abbreviated (CC, GC, Cp, Ki) | Name + Matrix + Description |
| <80 | <61 | Abbreviated (CC, GC, Cp, Ki) | Name + Matrix only |

**Matrix cell rendering:**
- Each cell is a single colored symbol: ✓ (green), ~ (yellow), ! (orange), ✗ (red)
- Column width = max(header text width, 1 char)
- Inner gap between matrix columns: 1 space (not the standard 2-char colGap)
- Matrix block width: ~11 chars abbreviated, ~26 chars full

**Width calculation:**
```
cursorWidth(4) + nameW + colGap(2) + matrixW + [colGap(2) + descW] = panelWidth
```

**Sort:** Alphabetical by name (consistent with design decision).

**Name truncation:** Standard `truncate()` with `...` suffix when name exceeds budget.

**Implementation:** Hook list is a variant of the existing `itemsModel.View()` — detected by `contentType == catalog.Hooks`, which switches to the matrix column layout instead of the single Provider column.

---

### 7. TUI: Hook Detail View — 4-Tab Layout

Hooks use 4 tabs instead of the standard 3: **Overview | Compatibility | Files | Install**

**Overview tab** — hook-specific metadata before risks/README:
```
Event:    PreToolUse
Matcher:  Bash
Type:     command
Command:  go vet ./...
Timeout:  5000ms
Async:    false
```
Then standard risk indicators and README (if available).

**Compatibility tab** — new tab, reusable across all content types:

For hooks (richest version):
```
Compatibility

  Provider       Level      Notes
  ─────────────  ─────────  ──────────────────────────────
  Claude Code    ✓ Full     Native format
  Gemini CLI     ✓ Full     Event → BeforeTool
  Copilot CLI    ! Broken   Matcher dropped
  Kiro           ✓ Full     Matcher → per-entry

  Feature Impact
  ─────────────────────────────────────────────────────────
                      CC  GC  Cp  Ki
  Matcher ("Bash")    ✓   ✓   ✗   ✓
  Status Message      ✓   ✓   ✓   ✗
  Async               ✓   ✓   ✗   ✗

  ✗ Copilot CLI: Matcher not supported.
    Hook will fire on ALL tool calls.
  ✗ Kiro: Status message not supported.
    No user-visible status during execution.
```

For other content types (simpler version):
```
Compatibility

  Provider       Support
  ─────────────  ──────────────────────────────
  Claude Code    ✓ Native
  Gemini CLI     ✓ Converted (.gemini/rules/)
  Cursor         ✓ Converted (.cursorrules)
  Windsurf       ✗ Not supported
```

**Visual treatment:**
- Non-detected providers rendered with `helpStyle` (grayed out)
- Detected providers use `valueStyle` (normal text)
- Compatibility symbols use their assigned colors (green ✓, yellow ~, orange !, red ✗)

**Files tab** — no change from standard (raw JSON viewer).

**Install tab** — provider checkboxes with inline compatibility level:
```
Providers
────────────────────
  > [✓] Claude Code   ✓ Full        [ok] installed
    [ ] Gemini CLI     ✓ Full        [--] available
    [ ] Copilot CLI    ! Broken      [--] available
        Kiro           ✓ Full        (not detected)
```
- Broken-level providers: show warning modal when toggled on for install
- None-level providers: checkbox disabled entirely (can't install)

**Note:** The Compatibility tab design replaces the "TUI: Compatibility Detail on Select" section — compatibility is always visible, no click-to-reveal interaction needed.

---

### 8. Compatibility Computation Engine

**Location:** `cli/internal/converter/` package (new file `compat.go`)

**Rationale:** The converter already has event/tool translation tables, LLM hook mode handling, and per-provider rendering logic. Adding the capability registry keeps all provider-specific knowledge in one package.

**Single source of truth** — one data structure defines all provider capabilities:

```go
// converter/compat.go

type HookFeature int
const (
    FeatureMatcher HookFeature = iota
    FeatureAsync
    FeatureStatusMessage
    FeatureLLMHook
    FeatureTimeout
)

type FeatureSupport struct {
    Supported bool
    Notes     string           // e.g., "mapped to 'comment' field"
    Impact    CompatLevel      // what level when this feature is lost
}

type ProviderCapability struct {
    Events   map[string]string              // canonical → provider event name
    Features map[HookFeature]FeatureSupport // per-feature support
}

// HookCapabilities is the single source of truth for provider hook support.
// The TUI, the computation engine, and tests all read from this.
var HookCapabilities = map[string]ProviderCapability{
    "claude-code": { ... },
    "gemini-cli":  { ... },
    "copilot-cli": { ... },
    "kiro":        { ... },
}
```

**Computation function:**
```go
type CompatResult struct {
    Provider string
    Level    CompatLevel      // Full, Degraded, Broken, None
    Notes    string           // summary note
    Features []FeatureResult  // per-feature breakdown
}

// Analyze computes compatibility for a single hook against a target provider.
// Checks: (1) event support, (2) per-feature support, (3) aggregates to worst level.
func AnalyzeHookCompat(hook HookData, targetProvider string) CompatResult
```

**Caching:** Compute on demand (called when detail view opens or list renders). No persistent cache — the computation is cheap (table lookups, no I/O).

**Testing:** Tests validate the capability registry against known behavior. Test cases for each provider+feature combination, plus integration tests that verify AnalyzeHookCompat produces correct levels for known hooks.

---

### 9. Canonical Storage Format

**File structure per hook item:**
```
hooks/<provider>/<hook-name>/
  hook.json          # flat format
  .syllago.yaml      # metadata
  README.md          # optional documentation
```

**`hook.json` format (flat, one event+matcher per file):**
```json
{
  "event": "PreToolUse",
  "matcher": "Bash",
  "hooks": [
    {
      "type": "command",
      "command": "go vet ./...",
      "timeout": 5000,
      "statusMessage": "Validating shell command...",
      "async": false
    }
  ]
}
```

**Why flat:** The installer already expects this format. The import splits the nested `settings.json` format into individual flat items. One item = one event + one matcher group.

**Scanner changes:** Minimal — `scanProviderSpecific()` already handles directory-per-item. The scanner calls `describeHookJSON()` which already parses hook.json to auto-generate descriptions. Add event/matcher extraction for the Compatibility tab and list view.

**Converter changes:** The converter currently handles batch (nested) format for import/export. After the overhaul:
- **Import path:** Read nested `{"hooks":{"EventName":[...]}}` from settings.json → split into individual flat items
- **Canonicalization:** Takes a flat `{"event":"...","hooks":[...]}` → translates event/tool names between providers
- **Export/render:** Takes flat format → merges back into provider's nested settings format

**Existing content migration:**
- `example-kitchen-sink-hooks/` → split into 3 individual items (PreToolUse, PostToolUse, Notification)
- `example-lint-on-save/` → convert from nested to flat format (single event+matcher, no split needed)
- Template (`templates/hooks/hook.json`) → already in flat format, no change

---

### 10. TUI: Import Wizard Flow

Hook-specific steps inserted after file/git selection in the existing import wizard. Simplified from 3 steps to 1 — no separate naming step.

**Step: Multi-select with auto-derived names**
```
Found 4 hooks in settings.json:

  [✓] validating-shell-command   (PreToolUse/Bash)
  [✓] checking-file-permissions  (PreToolUse/Write)
  [✓] post-tool-cleanup          (PostToolUse/*)
  [ ] session-start-echo         (SessionStart/*)

  ▸ Import Selected (3)        Deselect All
```

**Interactions:**
- Space: toggle individual item
- Enter: import selected items
- `a`: select all
- `n`: deselect all
- Esc: go back

**Name derivation** (auto-applied, no rename step):
1. `statusMessage` if present → slugify (e.g., "Validating shell command..." → "validating-shell-command")
2. `matcher` + event → (e.g., "pretooluse-bash")
3. Event + first meaningful word(s) from command

Users rename later from the content directory if needed.

**After import:** Show summary and navigate to the imported hooks in the content list.

---

### 11. CLI: Import Flow

Batch import — no interactive selection. Power-user oriented.

**Default behavior:**
```bash
$ syllago import --from claude-code --type hooks

Discovered 4 hooks in ~/.claude/settings.json:
  validating-shell-command   (PreToolUse/Bash)
  checking-file-permissions  (PreToolUse/Write)
  post-tool-cleanup          (PostToolUse/*)
  session-start-echo         (SessionStart/*)

Imported 4 hooks to local/hooks/claude-code/
```

**Flags:**
- `--preview` / `--dry-run`: Show what would be imported without writing anything
- `--exclude <name>`: Skip specific hooks by auto-derived name (repeatable)
- `--force`: Overwrite existing items with the same name

**Output format:** One line per imported hook with auto-derived name and event/matcher info.

---

### 12. Export/Install Integration

**Install tab behavior by compatibility level:**

| Level | Checkbox State | Behavior |
|-------|---------------|----------|
| Full (✓) | Enabled | Normal install, no warnings |
| Degraded (~) | Enabled | Install with inline note about what's lost (cosmetic only) |
| Broken (!) | Enabled | Confirm modal required before install (see below) |
| None (✗) | **Disabled** | Can't install — event doesn't exist on target |

**Broken install confirm modal:**
```
┌────────────────────────────────────────┐
│  ! Compatibility Warning               │
│                                        │
│  Installing to Copilot CLI:            │
│                                        │
│  • Matcher dropped                     │
│    Hook fires on ALL tool calls        │
│    instead of only "Bash"              │
│                                        │
│  • Async not supported                 │
│    Hook will block execution           │
│                                        │
│  ▸ Cancel              Install Anyway  │
└────────────────────────────────────────┘
```

- Default cursor on **Cancel** (destructive action pattern per modal rules)
- Modal style: standard `modalWidth = 40`, `lipgloss.RoundedBorder()`, `modalBorderColor`
- Content from `AnalyzeHookCompat()` — same single source of truth as the Compatibility tab
- Uses `warningStyle` for the `!` header

**Uninstall:** Uses hash-based matching (see section 15) and snapshot safety (see section 14).

**Export:** Converter's `Render()` takes flat `{"event":"...","hooks":[...]}` and merges into the target provider's settings format. No compatibility check on export — that's the user's intent. Auto-detects both global and project-level settings for the target provider (see section 16).

---

### 13. Migration

**No migration needed.** Syllago is pre-v1 and the hook format hasn't shipped to any external users. The existing example hooks in `content/hooks/` will be updated to the flat format during implementation:

- `example-kitchen-sink-hooks/` → split into 3 individual hook directories (one per event+matcher)
- `example-lint-on-save/` → convert from nested to flat format in-place
- Template (`templates/hooks/hook.json`) → already in flat format, no change

No auto-migration logic, no format detection, no backwards compatibility. Just update the files.

---

### 14. Snapshot Safety for settings.json

**Problem:** The current `.bak` overwrite approach loses history — each install/uninstall overwrites the same `.bak` file. If you install A, then install B, you can't roll back to the original state.

**Solution:** Use the existing `snapshot` package (already built for loadouts). Before any settings.json modification (install or uninstall), take a timestamped snapshot.

**Integration:**
```go
// Before modifying settings.json:
snapshot.Create(projectRoot, "hook-install:"+item.Name, "keep",
    []string{settingsPath}, nil, nil)

// If install/uninstall fails, auto-rollback:
manifest, snapshotDir, _ := snapshot.Load(projectRoot)
snapshot.Restore(snapshotDir, manifest)
```

**Snapshot manifest generalization:** The current manifest has `LoadoutName` and `Mode` fields. Generalize to support hook operations:
- `LoadoutName` → rename to `Source` or add a `Source` field: `"hook-install:bash-validator"`, `"hook-uninstall:bash-validator"`, `"loadout:my-loadout"`
- `Mode` stays as-is (`"keep"` for permanent changes)

**Snapshot lifecycle:**
- Created before each settings.json modification
- On success: snapshot preserved for manual rollback (`syllago restore`)
- On failure: auto-rollback via `snapshot.Restore()`
- Cleanup: older snapshots can be pruned (keep last N, or by age)

---

### 15. Hash-Based Uninstall Matching

**Problem:** Current uninstall matches by command string, which is fragile — two hooks with the same command, or a manually-edited hook, cause wrong deletions or missed matches.

**Solution:** Hash-based matching. On install, compute a SHA256 hash of the full matcher group JSON. On uninstall, hash each entry in `settings.json` and match against the stored hash.

```go
type InstalledHook struct {
    Name        string    `json:"name"`
    Event       string    `json:"event"`
    GroupHash   string    `json:"groupHash"` // SHA256 of matcher group JSON
    Command     string    `json:"command"`   // kept for display/debugging
    Source      string    `json:"source"`
    InstalledAt time.Time `json:"installedAt"`
}
```

**On install:**
```go
hash := sha256.Sum256(matcherGroup)
groupHash := hex.EncodeToString(hash[:])
```

**On uninstall:**
```go
for i, entry := range hooksArray.Array() {
    h := sha256.Sum256([]byte(entry.Raw))
    if hex.EncodeToString(h[:]) == inst.Hooks[instIdx].GroupHash {
        found = i
        break
    }
}
```

**Fail-safe behavior:**
- If hash doesn't match any entry (user manually edited the hook), uninstall fails with a clear message: "Hook was modified since installation. Use snapshot restore to revert."
- Won't delete the wrong entry — exact identification or nothing
- Snapshot system (section 14) provides the escape hatch

---

### 16. Scope Detection (Global vs Project)

**Problem:** The current installer only handles global `settings.json` (`~/<ConfigDir>/settings.json`). Claude Code and other providers support both global and project-level settings.

**Solution:** Auto-detect both scopes on import AND export/install.

**Import auto-detection:**
```
Global: ~/<ConfigDir>/settings.json
Project: <cwd>/<ConfigDir>/settings.json (or nearest git root)
```

If both have hooks:
- **TUI:** Show both as import sources, user picks which (or both)
- **CLI:** Default to global. `--scope project` or `--scope all` to include project-level.

**Install scope:**
- Auto-detect which scope(s) are available for the target provider
- If both global and project settings exist, prompt user for which scope to install to
- If only one exists, use that scope without prompting
- Store scope in `installed.json` so uninstall targets the right file

```go
type InstalledHook struct {
    // ... existing fields
    Scope string `json:"scope"` // "global" or "project"
}
```

**Export scope:** Same auto-detection applies. If hooks exist at both global and project level, the import discovers and splits from both sources.
