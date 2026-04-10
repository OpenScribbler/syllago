# Syllago TUI: Adaptive Layout Specification

Syllago uses a **Standardized Shell** system. The Top Bar and Metadata Bar remain fixed in their vertical stack, while the Content Stage below adapts to the specific data type.

---

## 1. Global Shell Structure (The "Tri-Layer")

```text
┌ Syllago ───────────────────────────────────────────────────────────────────────────────────┐
│ LAYER 1: GLOBAL TOP BAR (Nav & Dropdowns)                                                  │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ LAYER 2: ITEM METADATA BAR (Identity, Providers, & Primary Actions)                        │
├──────────────────────┬─────────────────────────────────────────────────────────────────────┤
│                      │                                                                     │
│ LAYER 3:             │ LAYER 3:                                                            │
│ PRIMARY NAV          │ CONTENT PREVIEW                                                     │
│ (Left Pane)          │ (Right Pane)                                                        │
│                      │                                                                     │
└──────────────────────┴─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Shell A: The "Project IDE"
**Content Types:** Skills, Hooks
*Focus: Full-height Explorer and Preview panes.*

```text
┌ Syllago ───────────────────────────────────────────────────────────────────────────────────┐
│ SYL ◈ AI Content: [Skills ▾] | Collection: [Library ▾] | Config: [Select ▾]  [+ ADD] [+ NEW] │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ 🛠️  SKILL: Refactor-Python | 🏠 Library | 📂 3 Files | [G][C][Z]    [ UNINSTALL ] [ EXPORT ] │
├──────────────────────┬─────────────────────────────────────────────────────────────────────┤
│ 📂 EXPLORER          │ 📄 FILE PREVIEW: rule.md                                            │
│                      │                                                                     │
│ ▼ Local Library      │ # Refactor Python Rule                                              │
│   ▼ Refactor-Python  │                                                                     │
│   [====] rule.md     │ You are an expert Python developer.                                 │
│     📄 hook.py       │ When reviewing code, look for:                                      │
│     ⚙️ config.json   │                                                                     │
│   ▶ Test-Gen         │ 1. Missing type hints in function signatures.                       │
│                      │ 2. Opportunities to use list comprehensions.                        │
└──────────────────────┴─────────────────────────────────────────────────────────────────────┘
  [TAB] Switch Pane  [↑/↓] Select File  [j/k] Navigate Tree  [/] Search Skills
```

---

## 3. Shell B: The "Catalog Inspector"
**Content Types:** Agents, Rules, MCP Servers, Commands
*Focus: Balanced list-detail view for single-file content.*

```text
┌ Syllago ───────────────────────────────────────────────────────────────────────────────────┐
│ SYL ◈ AI Content: [Agents ▾] | Collection: [Library ▾] | Config: [Select ▾]  [+ ADD] [+ NEW] │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ 👤 AGENT: Auditor | 🏠 Library | Type: Gemini-CLI | [G]            [ UNINSTALL ] [ EXPORT ] │
├───────────────────────────────────┬────────────────────────────────────────────────────────┤
│ 👤 AGENTS                         │ 📄 PREVIEW                                             │
│ [Search Agents...]                │                                                        │
│                                   │ **Description:**                                       │
│ 🟢 Auditor (Gemini)               │ Specializes in finding SQLi and XSS in Node.js.        │
│                                   │                                                        │
│ ⚪ Architect (Claude)             │ **System Prompt:**                                     │
│                                   │ You are a security auditor. Analyze the following code │
│ 🔵 Coder (OpenCode)               │ for common vulnerabilities. Focus on the `auth/` and   │
│                                   │ `api/` directories specifically.                       │
└───────────────────────────────────┴────────────────────────────────────────────────────────┘
  [TAB] Switch Pane  [Enter] Edit Prompt  [u] Uninstall  [/] Filter List
```

---

## 4. Shell C: The "Gallery Grid"
**Collections:** Loadouts, Registries
*Focus: High-level overview of collections and their contents.*

```text
┌ Syllago ───────────────────────────────────────────────────────────────────────────────────┐
│ SYL ◈ AI Content: [All ▾] | Collection: [Loadouts ▾] | Config: [Select ▾]    [+ ADD] [+ NEW] │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ 📦 LOADOUT: Python-Web | 🏠 Local | 📦 7 Items | [G][C]            [ APPLY ] [ EDIT ] [ DEL ] │
├──────────────────────────────────────────────────────────────────────┬─────────────────────┤
│ 📦 AVAILABLE LOADOUTS                                                │ 📝 CONTENTS         │
│                                                                      │                     │
│ ┌────────────────┐  ┌────────────────┐  ┌────────────────┐           │ • 🛠️  Refactor-Py   │
│ │ 🐍 Python-Web  │  │ ⚛️  React-FE    │  │ 🛡️  Sec-Audit   │           │ • 🛠️  Py-Doc-Gen    │
│ │ ────────────── │  │ ────────────── │  │ ────────────── │           │ • 📜 Strict-Types   │
│ │ 4 Skills       │  │ 6 Skills       │  │ 2 Agents       │           │ • 📜 PEP8-Check     │
│ │ 2 Rules        │  │ 1 Agent        │  │ 5 Rules        │           │ • 👤 Auditor        │
│ └────────────────┘  └────────────────┘  └────────────────┘           │ • 👤 Coder          │
└──────────────────────────────────────────────────────────────────────┴─────────────────────┘
  [↑/↓/←/→] Navigate Grid  [Enter] Select  [A] Apply  [Tab] Switch to Details
```

---

## 5. Overlays: Modals & Wizards
**Context:** Confirmations, Warnings, Step-by-Step creation.
*Focus: Centered, high-contrast focus. Content is copyable via `Ctrl+C`.*

```text
┌──────────────────────────────────────────────────────────────────────────┐
│ ⚠️  CONFIRM UNINSTALL                                                     │
│ ──────────────────────────────────────────────────────────────────────── │
│                                                                          │
│ Are you sure you want to uninstall "Refactor-Python"?                    │
│ This will remove all local files and provider-specific configurations.   │
│                                                                          │
│ [C] Copy Text content                                                    │
│                                                                          │
│                      [ CONFIRM (Enter) ]      [ CANCEL (Esc) ]           │
└──────────────────────────────────────────────────────────────────────────┘
```

```text
┌──────────────────────────────────────────────────────────────────────────┐
│ 🛠️  WIZARD: Create New Skill (Step 2 of 4)                                │
│ ──────────────────────────────────────────────────────────────────────── │
│                                                                          │
│ Select Target Providers:                                                 │
│ [X] Claude Code                                                          │
│ [X] Gemini CLI                                                           │
│ [ ] Cursor                                                               │
│                                                                          │
│ [C] Copy Summary                                                         │
│                                                                          │
│            [ BACK ]      [ NEXT (Enter) ]      [ CANCEL (Esc) ]          │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 6. System Notifications: Toasts
**Context:** Success, Warnings, Errors.
*Focus: Bottom-right corner. Auto-dismisses, or press `Space/Esc` to clear. `Ctrl+C` to copy.*

```text
                                         ┌──────────────────────────────────┐
                                         │ ✅ SUCCESS: Skill Installed      │
                                         │ Refactor-Python is now ready.    │
                                         │ [C] Copy    [Space] Dismiss      │
                                         └──────────────────────────────────┘

                                         ┌──────────────────────────────────┐
                                         │ ❌ ERROR: Sync Failed            │
                                         │ Registry "Main" is unreachable.  │
                                         │ [C] Copy    [Space] Dismiss      │
                                         └──────────────────────────────────┘
```

---

## Focus & Global Interaction Map

| Context | Action | Behavior |
| :--- | :--- | :--- |
| **Any Overlay** | `Ctrl+C` | Copy message/content to system clipboard. |
| **Any Overlay** | `Esc` | Close/Cancel the current overlay. |
| **Modal** | `Enter` | Trigger the Primary (Right) action. |
| **Toast** | `Space` | Immediately dismiss the toast. |
| **Primary Nav** | `Tab` | Switch to Content Pane. |
| **Content Pane**| `Tab` | Switch to Top Bar. |
| **Top Bar** | `Tab` | Switch to Primary Nav. |
