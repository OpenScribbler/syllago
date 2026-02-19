## v0.2.1

### Highlights

**Import Conflict Resolution** — Importing content that already exists now shows a `git diff`-style unified diff and lets you overwrite or skip, instead of failing silently.

**Page Up/Down Navigation** — All scrollable areas now support `PgUp`/`PgDn` for jumping a full viewport at a time.

---

### New Features

#### Import Conflict Resolution
- **Conflict detection**: Importing content to an existing destination triggers a conflict screen instead of a bare "destination already exists" error.
- **Unified diff view**: Shows colored line-by-line diff (green for additions, red for removals, `@@` hunk headers) matching `git diff` output.
- **Single import**: Overwrite with `y` or cancel with `Esc` to go back and rename.
- **Batch import**: Step through each conflict one at a time with `y` to overwrite or `n` to skip, then the batch proceeds.
- **Horizontal scroll**: `←`/`→` arrows scroll wide diff lines in 8-column steps.

#### Navigation
- **Page Up/Down**: `PgUp`/`PgDn` keys jump a full viewport in all scrollable areas: detail overview, file viewer, file browser preview, update preview, and conflict diff view.
- **Clickable breadcrumbs**: "Home >" link at the top of Import, Update, and Settings screens navigates back to the sidebar.

### Improvements

#### Mouse Support
- Import screen options (source, type, provider, browse start, git pick, validation items) are now clickable.
- Update screen menu options are now clickable.
- Settings screen rows and sub-picker items are now clickable.

#### TUI Polish
- Refined sidebar layout and navigation.
- Updated item list rendering and detail view layout.
- Expanded color palette with adaptive light/dark terminal support.

### Stats

- 16 files changed, 1,775 insertions, 218 deletions
