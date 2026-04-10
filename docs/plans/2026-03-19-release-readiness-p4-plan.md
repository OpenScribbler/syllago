# Release Readiness Phase 4: Repo Files + README — Implementation Plan

*Plan date: 2026-03-20*
*Design doc: docs/plans/2026-03-19-release-readiness-p4-design.md*

## Current State

- SECURITY.md, VERSIONING.md, CHANGELOG.md, ARCHITECTURE.md: Do not exist
- CONTRIBUTING.md: Exists, needs Development section
- README.md: Exists, ~40% accurate (6 providers, deprecated commands, no loadouts)
- `.github/workflows/pr-policy.yml`: Does not exist
- Version history: releases/v0.5.0.md, v0.6.0.md, v0.7.0.md exist
- Current version: 0.6.1 in VERSION file; unreleased v0.7.0 + P1-P3 work

## Tasks (all independent, can parallelize)

### Task 1: SECURITY.md
- File: `/SECURITY.md`
- Threat surface, trust model, contact info, disclosure policy
- Source: design doc section 1

### Task 2: VERSIONING.md
- File: `/VERSIONING.md`
- Semver, pre/post-1.0, release process, checklist
- Source: design doc section 2, .github/workflows/release.yml

### Task 3: CHANGELOG.md
- File: `/CHANGELOG.md`
- Keep a Changelog format, backfill from v0.5.0
- Versions: [Unreleased], 0.6.1, 0.6.0, 0.5.0
- Source: releases/*.md, git log, git tags for dates

### Task 4: .github/workflows/pr-policy.yml
- File: `/.github/workflows/pr-policy.yml`
- Auto-close external PRs, allowlist 3 users
- SHA-pin actions/github-script

### Task 5: ARCHITECTURE.md
- File: `/ARCHITECTURE.md`
- Package map (18 packages + cmd), data flow, conversion model, conventions
- Source: cli/internal/ exploration, cli/CLAUDE.md

### Task 6: CONTRIBUTING.md Update
- Append Development section (requirements, build, test, code org, why no PRs)
- Do not modify existing content

### Task 7: README.md Full Rewrite
- 16 sections per design doc outline
- All 11 providers, current v0.7.0 verbs, conversion table
- Placeholders for logo and VHS demos
- Source: commands.json/help, releases/*.md, design doc

## Pre-work
1. `git log -1 --format="%ad" --date=short v0.6.1` for CHANGELOG date
2. Verify actions/github-script@v7 SHA from existing workflows
