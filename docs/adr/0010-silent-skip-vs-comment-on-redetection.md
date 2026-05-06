# 0010. silent skip vs comment-on-redetection

Date: 2026-05-06
Status: Accepted
Feature: capmon-provider-batching

## Context

The design concept explicitly calls out that comment spam adds noise without helping reviewers prioritize. The existing open issue is the signal; appending identical hash-change messages each run buries the original event in noise. The warning-issue path already uses a combined find-or-create pattern but does append on re-detection — however, warning re-detection carries state (the violation is still present), whereas a hash-change re-detection does not carry new information (the old issue documents the changed hash already). The two cases are semantically different; silencing re-detection is correct only for the hash-change issue type.

## Decision

Chose **Silent skip — make no GitHub API call when an open issue already exists for the Provider Slug** over **Append a comment to the existing issue on redetection (prior behavior via `AppendCapmonChangeEvent` in `cli/internal/capmon/report.go`)**.

## Consequences

`AppendCapmonChangeEvent` is not called from `runSourceCheck` under the new design. It remains available for the warning-issue path which legitimately appends. Any future decision to re-enable appending on hash-change redetection would require revisiting this ADR.
