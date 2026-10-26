# Agent Instructions

## Rules

- No mutation testing while the first implementation of the OpenScribbler stack is built (the `OpenScribbler/stack` repo's README defines it), which overrides the global rule to mutation-test each stage.
- Merge a PR you opened without asking once both second-opinion rounds are done, the second round's fixes are pushed, and CI passes on the commit that carries them. Merge with `gh pr merge --merge --admin`: the ruleset wants the last push approved by someone other than the pusher, which a solo maintainer can't satisfy.
