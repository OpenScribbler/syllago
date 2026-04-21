# Cerebrum

> OpenWolf's learning memory. Updated automatically as the AI learns from interactions.
> Do not edit manually unless correcting an error.
> Last updated: 2026-04-15

## User Preferences

<!-- How the user likes things done. Code style, tools, patterns, communication. -->

- **Release notes are strictly user-facing.** Never include internal labels: beads issue codes (`D1`, `D13`, `D14-delta`), task IDs (`T7-T16`, `T27`), epic slugs (`syllago-xxx`, `beads-xxx`), version markers (`V4`, `V5`), delta/phase tokens. Describe what changed for the user, not the work item that produced it. Before showing a notes draft, grep for these patterns and scrub each one. Rule also captured in `.claude/skills/release/workflows/create-release.md` Step 3.

## Key Learnings

- **Project:** syllago
- **Description:** <div align="center">
- **Release pipeline is CI-driven.** `.github/workflows/release.yml` fires on `v*` tag push and does the full pipeline itself: builds six binaries (linux/darwin/windows × amd64/arm64), generates `commands.json`/`providers.json`/`telemetry.json`/`capabilities.json`, produces an SPDX SBOM via syft, signs checksums with cosign, creates the GitHub Release with all artifacts attached, and updates the Homebrew tap. Do NOT run `gh release create` manually — the `$PAI_DIR/hooks/release-guard.sh` hook will block it, and duplicating the work conflicts with the CI run. Flow: prep commit → push tag → wait for CI. The `release` skill's Step 8 still says to run `gh release create` manually; that guidance is stale for this repo.
- **Release workflow does not auto-detect pre-release tags.** `.github/workflows/release.yml` calls `gh release create` without `--prerelease`, so tags like `v0.7.0-beta.1` come out as stable releases. Workaround: `gh release edit <tag> --prerelease` after the run completes. Follow-up: teach the workflow to detect pre-release suffixes (`-alpha`, `-beta`, `-rc`, `-pre`) and pass the flag automatically.
- **`gh api /repos/…/actions/runs` paginates silently.** Default page size is 30, max is 100. A single call never reveals the real population — a workflow with 190 runs will return only the first 30 and give no hint more exist. Always check `.total_count` first (use `per_page=1` for a cheap probe), then paginate explicitly. For duration math across all runs, loop `page=1..N` until results are exhausted. Applies to `gh api /repos/{owner}/{repo}/actions/workflows/{id}/runs` too.
- **Actions billing attribution gotcha.** GitHub's billing UI uses the workflow's `name:` field, not the filename. CodeQL's default name is "CodeQL" and it's categorized under "Code scanning" in some views — easy to confuse with a Claude review workflow when both contain "code" in the name. When investigating cost, map filename → `name:` → billing label before drawing conclusions.

## Do-Not-Repeat

<!-- Mistakes made and corrected. Each entry prevents the same mistake recurring. -->
<!-- Format: [YYYY-MM-DD] Description of what went wrong and what to do instead. -->

- **[2026-04-15] Internal labels in release notes.** Copied `D1/D13/D14-delta/D16` straight from a commit message into the 0.7.0-beta.1 release notes. The user called them "garbage" — they mean nothing to a reader who doesn't have the bead tracker open. Fix: describe what the user can now do (e.g., "Provider detail view: `syllago info providers <slug>`") instead of naming the work item.
- **[2026-04-15] Claimed local `go build` success = release-ready.** CleanStale in `cli/internal/sandbox/staging.go` used `syscall.Stat_t` and `os.Getuid` inline. This compiles fine on Linux but the Windows cross-compile in the release workflow failed with "undefined: syscall.Stat_t". For any release that ships Windows binaries, assume local `go build` is insufficient signal — either cross-compile for each target (`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/syllago`) before tagging, or accept that the release workflow is the real validator and be ready to fix-and-retag.
- **[2026-04-17] When repo goes public, re-add `push: main` to scorecard.yml.** Trigger was removed while private because OSSF Scorecard's `publish_results: true` is a no-op on private repos — the SARIF gets uploaded to code-scanning but nothing reaches the public OpenSSF database. After the public flip, restore the `push: main` trigger so Scorecard updates the badge on every commit (current weekly cron becomes insufficient signal for a public-facing security score).
- **[2026-04-20] Don't recommend architectural changes to syllago without reading `cli/internal/{loadout,installer,catalog,snapshot}/` first.** Wrote a research doc comparing MO2 to syllago and recommended "split install into manifest + reconcile with snapshot rollback" — which `loadout/apply.go:52` already does (Resolve → Validate → Preview → Snapshot → apply, with snapshot.Restore on failure). User had to ask for a second pass grounded in the actual code. **Rule:** before proposing any apply/install/conflict/catalog change, grep for the feature in the existing packages. `installer/orphans.go`, `catalog/precedence.go` (with `Overridden`), `snapshot/` atomic rollback, and `loadout/preview.go` cover more than their package names suggest.

## Decision Log

<!-- Significant technical decisions with rationale. Why X was chosen over Y. -->

- **[2026-04-15] Force-moved `v0.7.0-beta.1` tag instead of burning version.** First tag push pointed at a commit that failed CI (Windows build). Options were: (a) burn the tag and use `v0.7.0-beta.2`, (b) force-update the tag to the fix commit. Chose (b): the tag had been public ~5 minutes, CI failed before creating the GitHub Release, no Homebrew push happened, no one could have pulled it. Force-moving is acceptable for a tag that never corresponded to a real release. Would not have done this if a release had already been created or if the tag had been public longer.
- **[2026-04-15] Windows `isOwnedByCurrentUser` returns true (no-op) rather than implementing Windows ACL checks.** The function gates stale-staging-dir cleanup on ownership to avoid deleting other users' dirs in shared `/tmp`. On Windows, temp paths are per-user (`%LOCALAPPDATA%\Temp`, `C:\Users\<user>\AppData\...`) so cross-user contention doesn't arise the same way. Implementing real Windows ACL introspection (via `golang.org/x/sys/windows`) would add a dependency and complexity for no observable benefit. Revisit only if a real Windows sandbox use case emerges.
