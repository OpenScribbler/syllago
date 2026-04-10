## Summary

<!-- 1-2 sentences: what changed and why. -->

## Bead

<!-- Link to the bead that tracked this work, if applicable. -->
<!-- Example: syllago-d5c1 -->

N/A

## Checklist

- [ ] `make build` (binary rebuilt)
- [ ] `make test` (all tests pass)
- [ ] Golden files regenerated (`cd cli && go test ./internal/tui/ -update-golden`) — if TUI changed
- [ ] `commands.json` regenerated (`./syllago _gendocs > commands.json`) — if CLI flags changed
- [ ] Tested manually — if TUI interaction feel, visual polish, or real provider installs changed
