---
description: Systematic code review checklist
alwaysApply: false
---

# Code Review

Perform a systematic code review across four dimensions:

## Correctness

- Does the logic handle edge cases (empty input, boundary values, concurrent access)?
- Are error paths handled and propagated correctly?
- Do tests cover the happy path and at least one failure path?
- Does the implementation match the spec or PR description?

## Clarity

- Are names accurate and self-documenting?
- Is each function focused on one responsibility?
- Would a new contributor understand this in 5 minutes without explanation?

## Safety

- Are there injection risks at system boundaries (SQL, shell, HTML output)?
- Is untrusted input validated before use?
- Are secrets or credentials accidentally exposed in code or logs?

## Completeness

- Are there missing test cases for documented behavior?
- Is documentation updated if behavior changed?
- Are breaking changes backwards-compatible or flagged?

## Output format

Report findings as:
- **Must Fix** — correctness or security issues that block merge
- **Should Fix** — meaningful improvements worth addressing now
- **Consider** — observations that may improve the code but aren't blocking
