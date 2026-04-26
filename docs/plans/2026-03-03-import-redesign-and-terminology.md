# Import Redesign & Terminology Audit

**Date:** 2026-03-03
**Context:** Import was broken — files written to wrong directory when `content_root` is configured. Led to broader discussion about where imported content should live and what the core operations should be called.

---

## Bug Found & Fixed

**Root cause:** The TUI import model used `catalog.RepoRoot` (content root) as the base for writing to `local/`. When `.syllago/config.json` has `"content_root": "content"`, this writes to `content/local/` instead of `local/`.

**Fix applied:** Import model now tracks `projectRoot` separately from `repoRoot`. All `local/` path construction uses `projectRoot`. Files changed:
- `cli/internal/tui/import.go` — added `projectRoot` field, updated `destinationPath()`, `batchDestForSource()`, `importSelectedHooks()`, `discoverProviderDirs()`
- `cli/internal/tui/app.go` — passes `a.projectRoot` to import model
- `cli/internal/tui/testhelpers_test.go` — test app uses `cat.RepoRoot` as `projectRoot`

**Status:** Fix is committed to working tree, tests pass. But the broader import destination question (below) may supersede this fix.

**Orphaned files:** Previous failed imports landed in `content/local/` and need manual cleanup:
```
content/local/agents/  (research-agent.md, pr-verify.md, d2-diagram-expert.md)
content/local/skills/  (building-agentic-systems)
```

---

## Decision: Import Destination

**Problem:** Users don't know where imported content goes. `<project>/local/` is confusing and project-scoped, but most imported content (agents, skills, prompts) should be available globally.

**Decision:** Import should write to `~/.syllago/content/` — the global content directory. Rationale:
- Users always have `~/.syllago/` — it's the known home for syllago
- Content is available across all projects
- Single, predictable location — no confusion about "where did my stuff go?"
- The catalog scanner (`ScanWithGlobalAndRegistries`) already scans this directory
- Promote/Publish to registry still knows where to pull from

**Open question:** How "My Tools" in the TUI should handle this. Currently filters for `Local: true` which won't match global items. Options: adjust My Tools to include globally imported content, rename the section, or rethink the sidebar categories.

---

## Decision: Terminology Overhaul

### New vocabulary (three core operations)

| Action | Direction | What It Does |
|--------|-----------|-------------|
| **Import** | IN | Bring content into syllago's catalog from any source (local path, git URL, provider, registry) |
| **Install** | OUT → provider | Make content active in a target provider. Auto-converts between provider formats. |
| **Publish** | OUT → registry | Add content to a registry for others to use |

### What changes

**Install absorbs Export:**
- Current `install` = push to same-format provider (symlink/copy)
- Current `export` = convert + push to different-format provider
- New `install` = unified action that auto-converts when targeting a different provider format
- `export` CLI command and TUI action get removed/merged

**Promote becomes Publish:**
- Current `promote` = move from local/ to shared/ (git-tracked, registry-ready)
- New `publish` = add content to a registry
- Clearer intent, familiar from npm/cargo

### What stays the same

**Import** keeps its name and meaning. The destination changes (global instead of project-local) but the user-facing concept is the same.

---

## Implementation Work Items

### 1. Import destination → global (`~/.syllago/content/`)
- Change TUI import to write to `~/.syllago/content/<type>/<name>/` (universal) or `~/.syllago/content/<type>/<provider>/<name>/` (provider-specific)
- Change CLI import similarly
- Ensure catalog scanning picks up imports correctly (already should)
- Decide on My Tools / sidebar presentation for imported global content
- Create `~/.syllago/content/` if it doesn't exist on first import

### 2. Merge Install + Export → unified Install
- Combine `install` and `export` CLI commands
- Unified TUI action that auto-detects whether conversion is needed
- Remove `export` command (or alias it to `install` for backwards compat)
- Update converter pipeline to be invoked automatically during install

### 3. Rename Promote → Publish
- Rename CLI command `promote` → `publish`
- Update TUI labels and actions
- Update internal references and documentation
- Consider keeping `promote` as an alias during transition

---

## Notes

- The `local/` directory at the project level may still be useful for project-specific content created with "Create New" in the TUI, but import should not target it.
- Hooks are already handled correctly as individual JSON snippets (not symlinks), so the global destination works for them too.
- The content_root vs project_root distinction remains important for repos with nested content directories — just not for import destinations.
