# Create Release Workflow

> **Trigger:** `/release <version>` (e.g., `/release 0.5.0`)

## Purpose

Execute the full nesco release workflow: version bump, release notes, commit, tag, push, GitHub release.

## Prerequisites

- Clean working tree (no unstaged changes that should be committed first)
- `gh` CLI authenticated
- Argument is a valid semver version string (e.g., `0.5.0`)

## Workflow Steps

### Step 1: Validate Version

1. Read `VERSION` file for current version
2. Parse the argument as the new version
3. Validate: new version must be greater than current version
4. Check that tag `v<version>` doesn't already exist: `git tag -l v<version>`

If validation fails, stop and explain.

### Step 2: Gather Change Context

Run these in parallel to understand what changed:

```bash
# Commits since last tag
git log --oneline v<current>..HEAD

# Diffstat since last tag
git diff --stat v<current>..HEAD
```

### Step 3: Write Release Notes

1. Read the template: `releases/template.md`
2. Read 1-2 recent release notes (e.g., `releases/v<current>.md`) for tone/style reference
3. Write `releases/v<version>.md` following the template:
   - **Highlights**: 2-3 items max, skim-friendly
   - **New Features**: Grouped by area with `####` subheadings if 5+ items
   - **Improvements**, **Security**, **Bug Fixes**: Only include sections with content
   - **Stats**: Files changed, insertions, deletions from diffstat
   - Omit empty sections entirely
4. Tone: Factual and direct. Audience is users and contributors.

### Step 4: Bump VERSION

Edit `VERSION` file to contain the new version string (no `v` prefix).

### Step 5: Commit

```bash
git add releases/v<version>.md VERSION
git commit -m "docs: add v<version> release notes and bump version

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Step 6: Tag

Create an annotated tag. The message should summarize the highlights (from the release notes).

```bash
git tag -a v<version> -m "v<version>: <highlights summary>"
```

### Step 7: Push

```bash
git push && git push origin v<version>
```

### Step 8: Create GitHub Release

```bash
gh release create v<version> --title "v<version>" --notes-file releases/v<version>.md
```

### Step 9: Confirm

Print the GitHub release URL and a summary of what was done.

## Error Handling

| Error | Action |
|-------|--------|
| Dirty working tree | Warn user, ask if they want to commit first |
| Version not greater than current | Stop, show current version |
| Tag already exists | Stop, show existing tag |
| `gh` not authenticated | Stop, suggest `gh auth login` |
| Push fails | Stop, show error (likely branch protection or auth issue) |
