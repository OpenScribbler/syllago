# 0012. silent skip vs comment-on-redetection

Date: 2026-05-06
Status: Proposed
Feature: capmon-provider-batching

## Context

The design concept explicitly calls out that comment spam adds noise without helping reviewers prioritize. The existing open issue is the signal; appending identical hash-change messages each run buries the original event in noise. The warning-issue path (check.go:140–155) already uses a combined find-or-create pattern but does append on re-detection — however, warning re-detection carries state (the violation is still present), whereas a hash-change re-detection does not carry new information (the old issue documents the changed hash already). The two cases are semantically different; silencing re-detection is correct only for the hash-change issue type.

## Decision

Chose **Silent skip — make no GitHub API call when an open issue already exists for the Provider Slug** over **Append a comment to the existing issue on redetection (current behavior for the per-content-type path via `AppendCapmonChangeEvent` at `cli/internal/capmon/check.go:228`)**.

## Consequences

The `AppendCapmonChangeEvent` function is not called from `runSourceCheck` under the new design. It remains available for the warning-issue path (check.go:147) which legitimately appends. Any future decision to re-enable appending on hash-change redetection would require revisiting this ADR. Tests that currently stub `AppendCapmonChangeEvent` behavior need to be updated to assert it is NOT called.
