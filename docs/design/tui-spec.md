# TUI Design Specification

This document defines the visual and interaction design for the Syllago terminal UI. It serves as the canonical reference for contributors — every page, component, and interaction pattern is documented here.

The TUI is built with BubbleTea (Go) and uses the Charm stack (lipgloss for styling, bubblezone for mouse support, bubbletea-overlay for modals).

## Table of Contents

- [Layout Structure](#layout-structure)
- [Color Palette](#color-palette)
- [Shared Components](#shared-components)
  - [Cards](#cards)
  - [Modals](#modals)
  - [Buttons](#buttons)
  - [Breadcrumbs](#breadcrumbs)
  - [Toast Notifications](#toast-notifications)
  - [Sidebar](#sidebar)
  - [Footer](#footer)
  - [Search Bar](#search-bar)
  - [Help Overlay](#help-overlay)
  - [Scroll Indicators](#scroll-indicators)
- [Pages](#pages)
  - [Homepage](#homepage)
  - [Items List](#items-list)
  - [Detail View](#detail-view)
  - [Library Cards](#library-cards)
  - [Loadout Cards](#loadout-cards)
  - [Registries](#registries)
  - [Import/Add](#importadd)
  - [Update](#update)
  - [Settings](#settings)
  - [Sandbox](#sandbox)
- [Interaction Model](#interaction-model)
  - [Keyboard Navigation](#keyboard-navigation)
  - [Mouse Support](#mouse-support)
  - [Focus System](#focus-system)
- [Responsive Design](#responsive-design)
- [Text Handling](#text-handling)
- [Accessibility](#accessibility)

---

## Layout Structure

The TUI uses a fixed two-panel layout with a persistent footer:

```
+------------------+----------------------------------------------+
|                  |                                              |
|     Sidebar      |              Content Area                    |
|   (fixed width)  |           (fills remaining)                  |
|                  |                                              |
|                  |                                              |
+------------------+----------------------------------------------+
| Footer: help text (left)              breadcrumb trail (right)  |
+------------------+----------------------------------------------+
```

- **Sidebar:** Always visible on the left. Fixed width. Right border.
- **Content Area:** Renders the active page (cards, list, detail, form, etc.).
- **Footer:** Context-sensitive help text on the left, breadcrumb location on the right. Replaced by the search bar when search is active.
- **Minimum terminal size:** 60 columns x 20 rows. Below this, a warning is shown.

---

## Color Palette

All colors use adaptive pairs for light and dark terminal themes.

| Name | Role | Light | Dark |
|------|------|-------|------|
| Primary (mint) | Titles, labels, headings | `#047857` | `#6EE7B7` |
| Accent (viola) | Selection, active elements, buttons | `#6D28D9` | `#C4B5FD` |
| Muted (stone) | Help text, inactive elements, separators | `#57534E` | `#A8A29E` |
| Success (green) | Installed status, success messages | `#15803D` | `#4ADE80` |
| Danger (red) | Error messages, error borders | `#B91C1C` | `#FCA5A5` |
| Warning (amber) | Warnings, global badge, update banner | `#B45309` | `#FCD34D` |
| Border | Panel borders, card borders (default) | `#D4D4D8` | `#3F3F46` |
| Selected BG | Background for selected items | `#D1FAE5` | `#1A3A2A` |
| Modal BG | Modal background fill | `#F4F4F5` | `#27272A` |
| Modal Border | Modal border (same as accent) | `#6D28D9` | `#C4B5FD` |

**Rules:**
- Never use raw hex colors in code — define named variables in `styles.go`.
- Check the existing palette before adding a new color. Reuse when possible.
- No emojis in the UI. Use colored text symbols instead.

---

## Shared Components

### Cards

Cards display items in a responsive grid layout. Used on Homepage, Library, Loadouts, and Registries pages.

**Layout:**
- Two columns when content width >= 42 characters, single column below
- Card width: `(contentWidth - 5) / 2` in two-column mode, `contentWidth - 2` in single-column
- Minimum card width: 18 characters
- **Fixed height in two-column mode** (set via `cardStyle.Height()`), dynamic in single-column. This is critical for correct click zones — bubblezone calculates `zone.Mark()` regions from rendered string positions, so variable-height cards in the same row cause click targets to misalign. Single-column mode doesn't need fixed height because cards stack vertically with no side-by-side alignment.
- 1-character gap between columns

**Visual:**
- Rounded border
- Unselected: muted border color
- Selected: accent border color, bold text
- Title in label style (bold primary), description in help style (muted)
- Counts in parentheses: `"(5)"`

**Interaction:**
- Arrow keys navigate the grid (Up/Down skip by column count, Left/Right move by 1)
- Enter drills into the selected card
- Home/End jump to first/last card
- Click on any card selects and drills in
- Scroll support when cards exceed viewport height

---

### Modals

Modals are centered overlay dialogs for confirmations, text input, and multi-step wizards.

**Visual:**
- Rounded border in accent color
- Background fill in modal background color
- Padding: 1 line top/bottom, 2 characters left/right
- **Standard width: 56 characters for all modals** (one size)
- Fixed height when buttons are pinned to bottom (prevents jitter between wizard steps)

**Modal Types:**

| Modal | Purpose | Height | Has Buttons |
|-------|---------|--------|-------------|
| Confirm | Yes/No decisions | 10 fixed | Yes |
| Save | Text input (filename) | Dynamic | Yes |
| Install | Multi-step: location, method | 18 fixed | Yes |
| Env Setup | Multi-step: env var configuration | 14 fixed | Yes |
| Registry Add | URL + name text inputs | 14 fixed | Yes |
| Loadout Create | Name + description inputs | 14 fixed | Yes |

**Keyboard:**
- Enter: activates current button
- Esc: cancels or goes back one step (Esc on first step dismisses, Esc on later steps goes back)
- Left/Right: switch between buttons
- Up/Down: navigate options within the modal
- y/Y: confirm (on confirm modals), n/N: cancel (on confirm modals)

**Mouse:**
- Click on a button activates it
- Click outside the modal dismisses it
- Clickable options (radio items) respond to click

---

### Buttons

Action buttons appear in modal footers.

**Visual:**
- Active button: white text on accent background, prefixed with `">`
- Inactive button: muted text on gray background, prefixed with spaces
- Both buttons centered within the available width

**Behavior:**
- All modals with confirm/cancel actions render buttons using the shared `renderButtons()` helper
- No inline text hints like `"[Enter] Save [Esc] Cancel"` — always use styled buttons
- Both buttons are mouse-clickable
- Default cursor position: Cancel for destructive actions, Confirm for safe actions

---

### Breadcrumbs

Clickable navigation trail at the top of the content area.

**Visual:**
- Segments separated by `" > "` in muted style
- Non-final segments: muted style, clickable (navigates on click)
- Final segment: primary/bold style, not clickable (current location)

**Rules:**
- Every page except Homepage renders a breadcrumb
- Pattern: `Home > [Section] > [Item]`
- Click on any segment navigates to that level

---

### Toast Notifications

Transient feedback messages overlaid at the bottom of the content area.

**Success Toast:**
- Green border, `"Done: "` prefix
- Short messages (1-2 lines): dismiss on any keypress (the key passes through to the underlying page)
- Long messages (e.g., bulk operations with warnings): scrollable with Up/Down, Esc dismisses. Fixed 5 visible lines.

**Error Toast:**
- Red border, `"Error: "` prefix
- Semi-modal: Esc dismisses, `c` copies sanitized text to clipboard
- Other keys pass through
- Fixed 5 visible lines, scrollable with Up/Down for long errors
- Sensitive data (paths, URLs, secrets) sanitized before clipboard copy

**Message Flow:**
- Components set `message` / `messageIsErr` fields
- App promotes these to the toast after each Update cycle
- Components do NOT render messages inline — the toast system handles all display

---

### Sidebar

Persistent navigation panel on the left side of the screen.

**Contents:**
- Content types section (Skills, Agents, Rules, etc.) with item counts
- Collections section (Library, Loadouts, Registries)
- Configuration section (Add, Update, Settings, Sandbox)

**Interaction:**
- Up/Down navigate the list
- Enter or Right drills into the selected section
- `H` toggles hidden content visibility
- Click on any item navigates to it
- Tab toggles focus between sidebar and content

---

### Footer

Context-sensitive help bar at the bottom of the screen.

**Layout:**
- Left: key hints for the current screen (e.g., `"arrows navigate . enter select . esc back"`)
- Right: breadcrumb showing current location
- Top border in default border color

**Behavior:**
- Replaced by the search bar when search is active
- Each page provides its own help text via `helpText()` method

---

### Search Bar

Inline text search that replaces the footer.

**Visual:**
- `"Search: "` prompt in primary bold style
- Shows match count: `"(N matches)"`

**Behavior:**
- Activated by `/` key
- Available on: Homepage, Items, Library Cards, Loadout Cards, Registries, Detail
- NOT available on: Import, Update, Settings, Sandbox (form-based screens)
- Filters by name, description, or provider (case-insensitive substring)
- Enter confirms, Esc cancels and clears

---

### Help Overlay

Full keyboard shortcut reference.

**Visual:**
- Replaces the content area entirely (sidebar remains visible)
- Global shortcuts section + context-sensitive section for current screen

**Behavior:**
- Activated by `?` key
- Dismissed by `?` or Esc
- Must include a section for EVERY screen type — no gaps
- Scrollable with Up/Down/PgUp/PgDown on small terminals where content overflows

---

### Scroll Indicators

Visual cues that content extends beyond the visible viewport.

**Visual:**
- `"(N more above)"` / `"(N more below)"` for lists and card grids
- `"(N lines above)"` / `"(N lines below)"` for content views (detail text)
- Rendered in muted style

**Rules:**
- Any area where content can exceed the visible viewport MUST implement scrolling
- Never let content silently disappear off the bottom of the screen
- Keyboard: Up/Down for line-by-line, PgUp/PgDown for page jumps, Home/End for bounds
- Mouse wheel support on all scrollable areas
- Reset scroll offset to 0 when navigating away or switching context

---

## Pages

### Homepage

**Purpose:** Landing page and entry point to all content.

**Layout:** Three sections of card grids — Content (category cards), Collections (Library/Loadouts/Registries), Configuration (Add/Update/Settings/Sandbox). ASCII art title when terminal is large enough.

| Property | Value |
|----------|-------|
| Screen enum | `screenCategory` |
| Breadcrumb | None (this is home) |
| Tab focus | Yes — Tab focuses card grid for arrow navigation |
| Search | Yes |
| Cards | Standard card grid with cursor navigation |
| Special | ASCII art when h>=48 and w>=75; compact text list fallback when h<35 |
| Empty state | First-run guide when no content and no registries |

---

### Items List

**Purpose:** Tabular list of content items within a category, registry, or search result.

| Property | Value |
|----------|-------|
| Screen enum | `screenItems` |
| Breadcrumb | `Home > [Category]` or `Home > Registries > [Registry]` or `Home > Search` |
| Tab focus | Yes |
| Search | Yes |
| Scroll | Yes — Up/Down/PgUp/PgDown/Home/End + mouse wheel |
| Badges | Inline: `[EXAMPLE]`, `[LIBRARY]`, `[REGISTRY]`, `[GLOBAL]`, `[BUILT-IN]` |
| Description box | Fixed-height box below list shows selected item's description |
| Action keys | `a` add {type}, `r` remove {type} (library items only), `l` create loadout (registry context only), `H` toggle hidden |

---

### Detail View

**Purpose:** Full view of a single content item with tabbed sections.

| Property | Value |
|----------|-------|
| Screen enum | `screenDetail` |
| Breadcrumb | `Home > [Category] > [Item Name] [badge]` |
| Tabs | Overview, Files, Install — navigated by Tab/Shift-Tab/1-2-3 |
| Tab focus | Tab switches between tabs (NOT sidebar) |
| Search | Yes (not available when text input is active) |
| Action keys | `i` install, `u` uninstall, `c` copy, `s` save, `e` env setup, `p` share |
| Mouse | Click tabs, breadcrumbs, provider checkboxes, action buttons |

---

### Library Cards

**Purpose:** Card grid grouping library items by content type.

| Property | Value |
|----------|-------|
| Screen enum | `screenLibraryCards` |
| Breadcrumb | `Home > Library` |
| Tab focus | Yes |
| Search | Yes |
| Cards | One card per content type with item count |
| Action keys | `a` opens import |

---

### Loadout Cards

**Purpose:** Card grid grouping loadouts by provider.

| Property | Value |
|----------|-------|
| Screen enum | `screenLoadoutCards` |
| Breadcrumb | `Home > Loadouts` |
| Tab focus | Yes |
| Search | Yes |
| Cards | One card per provider with loadout count |
| Action keys | `a` creates loadout |

---

### Registries

**Purpose:** Card grid of configured git registries.

| Property | Value |
|----------|-------|
| Screen enum | `screenRegistries` |
| Breadcrumb | `Home > Registries` |
| Tab focus | Yes |
| Search | Yes |
| Cards | Dynamic sizing (NOT hardcoded), fixed height in two-col mode for click zone alignment, shows name/status/version/URL/description |
| Action keys | `a` add registry, `r` remove registry, `s` sync registry |

---

### Import/Add

**Purpose:** Multi-step wizard for adding content from providers, files, or git repos.

| Property | Value |
|----------|-------|
| Screen enum | `screenImport` |
| Breadcrumb | Custom with step indicator |
| Tab focus | No (single-pane wizard) |
| Search | No |
| Navigation | Up/Down options, Enter/Space select, Esc back |

---

### Update

**Purpose:** Check for updates and apply them.

| Property | Value |
|----------|-------|
| Screen enum | `screenUpdate` |
| Breadcrumb | `Home > Update` |
| Tab focus | No |
| Search | No |

---

### Settings

**Purpose:** Configuration form for paths and providers.

| Property | Value |
|----------|-------|
| Screen enum | `screenSettings` |
| Breadcrumb | `Home > Settings` |
| Tab focus | No |
| Search | No |
| Navigation | Up/Down fields, Enter edit, `s` save |
| Scroll | Yes — when fields exceed viewport |

---

### Sandbox

**Purpose:** Sandbox configuration for isolated content testing.

| Property | Value |
|----------|-------|
| Screen enum | `screenSandbox` |
| Breadcrumb | `Home > Sandbox` |
| Tab focus | No |
| Search | No |

---

## Interaction Model

### Keyboard Navigation

All key bindings are defined in `keys.go` as named bindings. Never hardcode key strings in event handlers.

**Global keys** (available on all screens):
- `?` — Help overlay
- `Ctrl+C` — Quit
- `Esc` — Back / cancel / dismiss
- `/` — Search (on supported screens)

**Navigation keys** (include vim alternatives):
- `Up/k`, `Down/j` — Move cursor
- `Left/h`, `Right/l` — Navigate horizontally (cards, tabs)
- `Enter` — Select / drill in
- `Home/g`, `End/G` — Jump to first/last item
- `PgUp`, `PgDown` — Page scroll
- `Tab` — Toggle sidebar/content focus (or switch tabs on Detail)

**Action keys:**
- `i` install, `u` uninstall, `c` copy, `e` env setup, `p` share
- `a` add {context}, `r` remove {context} (library items and registries), `s` sync (registries only)
- `l` create loadout (Items page, registry context only)
- `H` toggle hidden content
- `y/Y` confirm, `n/N` cancel (in confirm modals)

---

### Mouse Support

Every interactive element supports both keyboard AND mouse. No keyboard-only or mouse-only elements.

**Click targets:**
- Cards: select and drill in
- List items: click selects and drills in (same as Enter)
- Breadcrumb segments: navigate to that level
- Tabs: switch to that tab
- Modal buttons: activate the button
- Modal background: dismiss the modal
- Checkboxes: toggle
- Sidebar items: navigate to that section

**Scroll wheel:** Scrolls the focused component (lists, detail content, card grids, sidebar).

---

### Focus System

Three focus targets:
1. **`focusModal`** — Active modal captures ALL input. Highest priority.
2. **`focusContent`** — Content area handles input (cards, lists, detail tabs).
3. **`focusSidebar`** — Sidebar handles input (navigation list).

Toast sits between modal and content focus:
- **Error toast:** Esc dismisses, `c` copies to clipboard. Other keys pass through.
- **Short success toast:** Any keypress dismisses (the key passes through to the underlying page).
- **Long scrollable success toast:** Up/Down scroll content, Esc dismisses. Other keys pass through.

---

## Responsive Design

### Breakpoints

| Size | Dimensions | Behavior |
|------|-----------|----------|
| Below minimum | < 60x20 | "Terminal too small" warning |
| Minimum | 60x20 | Single-column cards, compact layout |
| Default | 80x30 | Two-column cards, standard layout |
| Medium | 120x40 | Full card layout, more visible content |
| Large | 160x50 | ASCII art title on homepage |

### Adaptive Rules

- **Cards:** Two-column when `contentWidth >= 42`, single-column below
- **Homepage:** Card grid when `height >= 35`, text list fallback below
- **ASCII art:** Only when `height >= 48` AND `contentWidth >= 75`
- **Modals:** Standard 56w width. Max height: terminal height - 2
- **On resize:** Recalculate dimensions, clamp cursor and scroll positions

### Golden File Testing

Every visual component is tested at four sizes: 60x20, 80x30, 120x40, 160x50. Also tested with large datasets (85+ items) and empty catalogs at each size.

---

## Text Handling

### When to Truncate

Single-line labels in constrained spaces: card titles, card descriptions, list item names, URLs, breadcrumb segments.

- Truncate to available width minus border/padding
- Suffix: `"..."` (3 characters)
- Minimum 10 characters before truncating
- **OSC 8 hyperlinks for truncated URLs:** When a URL is truncated with `"..."`, wrap the display text in an OSC 8 hyperlink (`\x1b]8;;FULL_URL\x1b\\DISPLAY\x1b]8;;\x1b\\`) so terminals that support it link to the full URL, not the truncated text

### When to Word-Wrap

Multi-line prose in reading contexts: detail view content, modal body text, toast messages, error messages.

- Use `wordwrap.String(text, maxWidth)` from `muesli/reflow/wordwrap`

### Width Calculations

Always subtract border and padding from container width:
- Card inner text: `cardWidth - 4` (2 border + 2 padding)
- Modal inner text: `56 - 6 = 50` (2 border + 4 padding)
- Content pane: `terminalWidth - sidebarWidth - 1` (1 for sidebar border)

**Never rely on terminal auto-wrap.** All text must be explicitly truncated or word-wrapped.

---

## Accessibility

- `NO_COLOR=1` is supported automatically via the Charm stack
- All status indicators use text + symbol alongside color — meaning is never color-only
- Symbols: checkmark (success), X (error), > (selected), dash (separator), arrow (navigation), warning triangle (caution)
