# 0017. Separate main package vs hidden `_`-prefixed subcommand

Date: 2026-07-12
Status: Accepted
Feature: capmon-pull

## Context

The signed-off concept explicitly chose a separate maintainer tool over re-adding a `syllago capmon` surface, which would partially reverse the `ef6fdd2b` extraction that removed the in-repo capmon family. The hidden-subcommand pattern is for pure generators that dump JSON from the binary's own in-memory state (`release.yml:58-66`); Capmon Pull carries network fetching, sigstore verification, and git-tree mutation — capabilities that do not belong compiled into the user-facing CLI even hidden, and that syllago-sign's "not shipped to end users, `go run` from a workflow" precedent fits exactly.

## Decision

Chose **Separate main package under `cli/cmd/`, following syllago-sign (`cli/cmd/syllago-sign/main.go:1-5`)** over **Hidden cobra subcommand on the user-facing binary, following `_gencapabilities` (`cli/cmd/syllago/gencapabilities.go:198-206`)**.

## Consequences

The tool gets its own `main` + `main_test.go` like syllago-sign, shares `cli/internal/` packages (provider registry, moat trusted-root loader), and is invoked as `go run ./cmd/<tool>` from the cron workflow and locally. It never appears in `commands.json`, release artifacts, or the user-facing help. The cost is one more entry point to keep compiling, covered by the module-wide `go build ./...`/`go vet` CI.
