---
name: release
description: Automate the full release workflow for nesco. USE WHEN creating a release OR tagging a version OR bumping VERSION OR publishing to GitHub releases. Takes version as argument.
---

# release

Creates a tagged release for nesco: bumps the `VERSION` file, writes release notes from the template, commits, tags, pushes, and creates a GitHub release.

## Arguments

| Argument | Required | Example | Description |
|----------|----------|---------|-------------|
| `<version>` | Yes | `0.5.0` | The semver version to release (without `v` prefix) |

## Workflow Routing

| Workflow | Trigger | File |
|----------|---------|------|
| **create-release** | `/release 0.5.0` | `workflows/create-release.md` |

## Examples

**Example 1: Create a minor release**
```
User: "/release 0.5.0"
→ Invokes create-release workflow
→ Reads VERSION (current: 0.4.0), validates 0.5.0 > 0.4.0
→ Analyzes git log + diffstat since v0.4.0
→ Writes releases/v0.5.0.md from template
→ Bumps VERSION to 0.5.0
→ Commits, tags v0.5.0, pushes, creates GitHub release
```

**Example 2: Create a patch release**
```
User: "/release 0.4.1"
→ Invokes create-release workflow
→ Smaller changeset, fewer sections in release notes
→ Same full workflow: commit → tag → push → GitHub release
```
