---
name: release
description: Automate the full release workflow for syllago. USE WHEN creating a release OR tagging a version OR bumping VERSION OR publishing to GitHub releases.
---

# release

Interactive two-phase release workflow for syllago. Analyzes changes, recommends a version bump, writes release notes, and ships — with human approval at every decision point.

## Arguments

No arguments required. The skill determines the version interactively.

## Semver Guidelines

Use these criteria when recommending a release type. Present these in the AskUserQuestion options so the user sees the reasoning.

### Major (X.0.0)

- Breaking changes to CLI commands (renamed/removed subcommands, changed flags, changed output format)
- Breaking changes to `.syllago.yaml` schema that require user migration
- Breaking changes to registry protocol or content format
- Dropping support for a provider
- Any change that would break existing scripts or workflows using syllago

### Minor (0.X.0)

- New features (commands, subcommands, TUI screens, content types)
- New provider support
- Non-breaking additions to config format or `.syllago.yaml`
- Significant improvements to existing features (new options, expanded behavior)
- New built-in content (skills, agents, etc.)

### Patch (0.0.X)

- Bug fixes
- Performance improvements with no behavior change
- Documentation corrections that ship with the binary (help text, etc.)
- Dependency updates with no user-facing impact
- Minor UX polish (typo fixes, better error messages)

### Pre-1.0 Note

While syllago is pre-1.0, minor versions may include small breaking changes if needed. After 1.0, semver is strict — breaking changes require a major bump.

## Workflow Routing

| Workflow | Trigger | File |
|----------|---------|------|
| **create-release** | `/release` | `workflows/create-release.md` |

## Flow Overview

```
/release
  1. Analyze changes since last release (git log + diffstat)
  2. AskUserQuestion: release type (major/minor/patch) with reasoning
  3. Write release notes from template
  4. AskUserQuestion: approve release notes
  5. Bump VERSION, commit, create .release-pending.yml (status: prepared)
  6. AskUserQuestion: ship it?
  7. Tag, push, create GitHub Release, delete .release-pending.yml
```

A safety hook blocks `git tag` and `git push` of version tags unless `.release-pending.yml` exists with `status: prepared`. This prevents accidental releases.

## Examples

```
User: /release
Claude: Analyzes 12 commits since v0.4.1...
        "I see 3 new features and 1 bug fix. I'd recommend Minor (0.5.0)."
        [AskUserQuestion with Major/Minor/Patch options]
User: picks Minor
Claude: Writes release notes, shows draft
        [AskUserQuestion: Notes look good?]
User: "Looks good"
Claude: Commits notes + VERSION bump, creates .release-pending.yml
        [AskUserQuestion: Ship v0.5.0?]
User: "Ship it"
Claude: Tags v0.5.0, pushes, creates GitHub Release
        "Released! https://github.com/OpenScribbler/syllago/releases/tag/v0.5.0"
```
