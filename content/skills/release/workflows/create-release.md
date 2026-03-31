# Create Release Workflow

> **Trigger:** `/release`

## Purpose

Interactive two-phase release: prepare (analyze, write notes, commit) then ship (tag, push, publish). Human approval at every decision point via AskUserQuestion.

## Prerequisites

- Clean working tree (no unstaged changes)
- `gh` CLI authenticated
- On the `main` branch

## Phase 1: Prepare

### Step 1: Analyze Changes

1. Read `VERSION` file for current version
2. Check that tag `v<current>` exists (otherwise use `git log --oneline -20`)
3. Gather context:

```bash
# Commits since last release
git log --oneline v<current>..HEAD

# Diffstat
git diff --stat v<current>..HEAD
```

4. Categorize changes: features, improvements, bug fixes, breaking changes
5. Count by category for the recommendation

### Step 2: Recommend Release Type

Based on the change analysis and the semver guidelines in SKILL.md, recommend a release type.

**Use AskUserQuestion** with these options, filling in actual reasoning from the changes:

| Option | Label | Description |
|--------|-------|-------------|
| 1 | `Major (<calculated>)` | "Breaking changes detected: [list them]. Bumps to <X.0.0>" — Only offer if breaking changes exist |
| 2 | `Minor (<calculated>)` | "New features: [list key ones]. No breaking changes. Bumps to <0.X.0>" — Recommend if features exist |
| 3 | `Patch (<calculated>)` | "Bug fixes and minor improvements only. Bumps to <0.0.X>" — Recommend if only fixes |

Mark the recommended option with "(Recommended)" based on the actual changes. Calculate the version for each option and include it in the label.

If the user provides a custom version via "Other", validate it's greater than current and that the tag doesn't exist.

### Step 3: Write Release Notes

1. Read the template: `releases/template.md`
2. Read 1-2 recent release notes for tone reference
3. Write `releases/v<version>.md` following the template:
   - **Highlights**: 2-3 items max, user-facing impact only
   - **New Features**: Grouped by area with `####` subheadings if 5+ items
   - **Improvements**, **Security**, **Bug Fixes**: Only include sections with content
   - **Stats**: Files changed, insertions, deletions
   - Omit empty sections entirely
4. Show the draft to the user

### Step 4: Approve Notes

**Use AskUserQuestion:**

| Option | Label | Description |
|--------|-------|-------------|
| 1 | `Looks good` | "Proceed with these release notes" |
| 2 | `Edit first` | "I want to make changes before continuing" |

If "Edit first": wait for the user to describe changes, apply them, and re-show. Loop until approved.

### Step 5: Commit and Prepare

1. Bump `VERSION` file to the new version
2. Commit:

```bash
git add releases/v<version>.md VERSION
git commit -m "release: prepare v<version>

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

3. Create `.release-pending.yml` in the repo root:

```yaml
version: "<version>"
type: major|minor|patch
status: prepared
created_at: "<ISO 8601 timestamp>"
notes_file: "releases/v<version>.md"
```

4. Tell the user: "Release v<version> is prepared. The release guard hook will now allow tagging."

## Phase 2: Ship

### Step 6: Confirm Ship

**Use AskUserQuestion:**

| Option | Label | Description |
|--------|-------|-------------|
| 1 | `Ship v<version>` | "Tag, push, and create the GitHub Release now" |
| 2 | `Wait` | "I want to review more before shipping" |

If "Wait": pause and let the user review. They can say "ship it" or similar when ready.

### Step 7: Tag and Push

```bash
git tag -a v<version> -m "v<version>: <highlights summary from release notes>"
git push && git push origin v<version>
```

### Step 8: Create GitHub Release

```bash
gh release create v<version> --title "v<version>" --notes-file releases/v<version>.md
```

### Step 9: Clean Up and Confirm

1. Delete `.release-pending.yml`
2. Print the GitHub release URL
3. Summarize: version, highlights, what happens next (GitHub Actions builds binaries, updates Homebrew)

## Error Handling

| Error | Action |
|-------|--------|
| Dirty working tree | Warn user, ask if they want to commit first |
| Not on main branch | Warn user, ask if they want to continue anyway |
| Version not greater than current | Stop, show current version |
| Tag already exists | Stop, show existing tag |
| `gh` not authenticated | Stop, suggest `gh auth login` |
| Push fails | Stop, show error, leave `.release-pending.yml` so user can retry |
| `.release-pending.yml` already exists | Show its contents, ask if user wants to resume or start fresh |

## Resuming a Prepared Release

If `/release` is invoked and `.release-pending.yml` exists with `status: prepared`:

**Use AskUserQuestion:**

| Option | Label | Description |
|--------|-------|-------------|
| 1 | `Resume` | "Continue shipping v<version> (already prepared)" |
| 2 | `Start fresh` | "Discard the pending release and start over" |

If resume: skip to Phase 2, Step 6.
If start fresh: delete `.release-pending.yml` and begin from Step 1.
