# 0018. Accepted-drift list vs redefining the Coverage Drift comparison

Date: 2026-07-13
Status: Accepted
Feature: capmon-pull

## Context

The Coverage Drift gate compares two deliberately different questions: the Capability Feed's content-type `supported` answers "does the provider have this concept?" (capmon auto-flips a content type to supported when any sub-capability is mapped), while Go's `SupportsType` answers "can syllago install this content type there?". For most provider/content-type pairs the answers coincide, and a mismatch means someone is wrong — capmon's data is stale or syllago is missing real support — which the finding exists to surface.

zed/agents is the first pair where both answers are correct and permanently differ: Zed Agent Profiles carry tool restrictions and per-profile MCP scoping (concept: supported), but profiles are `settings.json` entries with no prompt and no definition file, so there is nothing an agent content item — fundamentally a prompt with metadata — could be installed into. A name+tools-only install would silently drop the prompt, which is worse than declining. Verified against Zed's live docs (agent-profiles page, 2026-07-13), not just capmon's cache.

Without a recorded verdict the non-required gate stays red on every roll of the mirror PR, and a permanently red check trains readers to ignore it — the next real finding hides behind a familiar X. Alternatives considered: narrow the comparison itself so feed `supported: true` only counts when a file-authoring key (e.g. `agents.definition_format`) is also mapped — more principled, but changes behavior for all providers at once and silently depends on capmon mapping those keys consistently; or accept the permanent red — zero code, maximal signal erosion.

## Decision

Chose **an in-code accepted-drift list** (`acceptedFeedDrift` in `cli/internal/provider/coverage_feed.go`) over **redefining the comparison** or **living with a permanently red gate**.

- Entries are keyed `provider/content-type` and MUST carry a reason string; the reason states what would invalidate the entry (for zed/agents: Zed shipping an installable agent form → delete the entry and implement support).
- The exemption applies only in the missed-capability direction (feed `true` / Go `false`). Go claiming support the feed denies is always a real finding, even for a listed pair.
- The CI gate reports accepted entries as log lines (`Coverage Drift (accepted): …`) instead of failures, so the verdict stays visible in run output.

## Consequences

The gate stays meaningful: green means "no unexplained divergence," and every future real finding is unmissable. The concept-vs-install distinction now has a canonical home a future reader will hit before "fixing" either side. The direction guard plus mandatory reason keep the list from quietly becoming a suppression dump, but the list is still a curated artifact: each new entry is a reviewed judgment, and a growing list is itself a signal to revisit the narrower-comparison alternative. The factual basis of the zed/agents entry depends on capmon actually watching Zed's agents docs — tracked as capmon-sc6 (zed agents sources were unmonitored when this was decided).
