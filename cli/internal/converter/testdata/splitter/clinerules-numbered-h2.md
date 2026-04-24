<!-- modeled after: nammayatri/nammayatri .clinerules -->
## 1. Project Overview

The project is a distributed service with a strongly-typed domain
model. Code is organized by bounded context; each context owns its
own persistence and exposes a narrow API to its peers.

Reading this file gives you enough context to make non-trivial
changes. For architectural questions, read the ADRs in docs/adr/.

## 2. Coding Style

Favor plain functions over classes. Keep cyclomatic complexity under
ten per function. Prefer early returns over deeply nested ifs.

Names are spelled out. Avoid three-letter abbreviations unless they
are well-known domain acronyms.

Every exported function has a one-line summary. If the behavior is
non-obvious, add an example in a code fence.

## 3. Testing

Table-driven tests are the default. Each row covers one behavior.
Names read as predicates: it_returns_zero_for_empty.

Unit tests live alongside the code. Integration tests sit under
tests/integration. End-to-end tests run nightly in CI only.

Every bug fix lands with a regression test that would have caught
the bug. If writing the test is hard, refactor first.

## 4. Error Handling

Propagate errors up to a boundary with enough context to decide what
to do. Do not swallow errors with empty catch or except blocks.

Wrap errors with context at the boundary so the stack trace points at
the originating call, not the last hop.

Panics are for unrecoverable programmer errors only. Every panic has
a comment explaining why recovery is impossible.

## 5. Release

Releases cut on the first Tuesday of each month. The release captain
runs the checklist in docs/release.md and drives promotion.

Every release goes through canary for at least one hour. Promotion is
gated on health signals from canary. Rollback is a one-command
operation; the prior artifact stays hot for twenty-four hours after
rollout.

Changelogs are generated from commit messages. Conventional Commits
format is required so the generation is automatic.
