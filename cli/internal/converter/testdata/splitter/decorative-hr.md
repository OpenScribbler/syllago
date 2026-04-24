<!-- modeled after: p33m5t3r/vibecoding/conway CLAUDE.md -->
## Introduction

This document collects the working agreements for the project. It is
deliberately short. Every section below is a rule, a convention, or a
handoff pointer for where to read more.

---

Every rule begins with a one-line summary, followed by the rationale if
the rule is non-obvious. Rules without a rationale are conventions; do
not re-derive them in review.

## Style Rules

Favor clarity over cleverness. The reader outnumbers the writer.

---

Use the editor config checked into the repo. If your editor does not
respect editorconfig, install a plugin; do not override the project
defaults.

## Review Rules

Every PR needs at least one reviewer from a different sub-team. Review
latency should be under one business day.

---

Approving a PR is an assertion that the change is correct and the tests
are sufficient. If either is in doubt, request changes rather than
approving.

## Release Rules

Cut releases on the first Tuesday of each month. The release captain
runs the checklist and drives the rollout.

---

Every release passes through canary before taking full production
traffic. Health signals from canary gate promotion; a red signal blocks
promotion until an override is documented.
