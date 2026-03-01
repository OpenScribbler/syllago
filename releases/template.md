# Release Notes Template

Use this format when writing release notes for a new version tag.

---

## Syntax Reference

```markdown
## vX.Y.Z

### Highlights

**Feature Name** — One sentence describing the user-facing impact.

**Another Feature** — One sentence describing the user-facing impact.

---

### New Features

#### Feature Area
- **Short label**: What it does and why it matters.
- **Another label**: What it does and why it matters.

#### Another Area
- **Label**: Description with `inline code` for CLI commands or flags.

### Improvements

#### Area
- Description of improvement.

### Security

- Description of what was hardened and what it prevents.

### Bug Fixes

- Fixed issue where [symptom] when [condition].

### Build & Infrastructure

- Description of build change.

### Breaking Changes

- **What changed**: Old behavior → new behavior. Migration: `do X`.

### Stats

- N files changed, N insertions, N deletions
```

---

## Section Rules

| Section | When to include | Format |
|---------|----------------|--------|
| **Highlights** | Always | `**Bold Name** — description` (2-3 max) |
| **New Features** | New user-facing functionality | `#### Subheading` + `- **Label**: description` |
| **Improvements** | Enhancements to existing features | Same as New Features |
| **Security** | Any security-related change | `- Plain description` |
| **Bug Fixes** | Resolved defects | `- Fixed [symptom] when [condition]` |
| **Build & Infrastructure** | Build, CI, deps, tooling | `- Plain description` |
| **Breaking Changes** | Only if something breaks | `- **What**: old → new. Migration: \`cmd\`` |
| **Stats** | Optional, for major releases | `- N files, N insertions, N deletions` |

## Guidelines

- **Audience**: Users and contributors. Write from "what changed for me?"
- **Highlights**: 2-3 items max. The skim-friendly summary.
- **Grouping**: Use `####` subheadings when a section has 5+ items.
- **Brevity**: One line per item. Link to issues/PRs for details.
- **Code references**: Use backticks for commands (`syllago scan`), flags (`--json`), files (`Makefile`), and functions.
- **Tone**: Factual and direct. No marketing language.
- **Omit empty sections**: Don't include headings with nothing under them.
- **Breaking changes**: Always explicit, even if minor.
- **Tag message**: Should match highlights. `git tag -a vX.Y.Z -m "vX.Y.Z: highlights summary"`
