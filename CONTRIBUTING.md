# Contributing to syllago

Thank you for wanting to contribute. syllago has a contribution model built around one core idea: **your thinking is the contribution**.

## How contributions work

The most valuable contributions are **ideas, not code**. You don't need to write a single line to make a meaningful impact.

When you open an issue, you'll be guided through a structured set of questions depending on what you're bringing:

- **Bug Report** — Something isn't working the way it should
- **Feature Idea** — Something that should exist but doesn't yet
- **Improvement** — Something that works but could be better
- **Content Request** — A suggestion for new built-in AI content (skills, agents, rules, hooks, etc.) that helps people use or understand syllago

Each template walks you through the right questions. Just follow the prompts.

## What we're looking for

- **Describe the what and why.** What's the problem, gap, or opportunity? Why does it matter?
- **Explain the how in your own words.** If you have ideas about how something should work or change, walk us through your thinking. Explain it like you're talking to a teammate.
- **No code required.** Your description is the contribution — we'll handle the build.

## Code contributions

syllago accepts pull requests from **vouched contributors**. We use [Vouch](https://github.com/mitchellh/vouch) to manage contributor trust — PRs from unvouched users are automatically closed.

### How to get vouched

1. **Start with an issue.** Open a bug report, feature idea, or improvement. Show us what you're thinking.
2. **Engage with the project.** Participate in discussions, help reproduce bugs, or provide feedback.
3. **Get vouched.** Once a maintainer is familiar with your work, they can vouch for you by commenting `!vouch` on one of your issues.

After you're vouched, your pull requests will be accepted for review.

### Why this model?

syllago is maintained using an AI-augmented development workflow where a small team handles design and implementation together. The vouch system lets us welcome contributions from people we trust while keeping the signal-to-noise ratio high. It's not about gatekeeping — it's about building relationships before building code.

## About syllago's built-in content

syllago ships with AI content — skills, agents, rules, hooks, and more — that help users work with and understand the CLI itself. These are meta-tools: they exist to make syllago better, not for general-purpose distribution. When suggesting new content, think about what would make syllago easier to learn, use, or extend.

## Development

### Requirements

- Go 1.26+
- Make
- `golangci-lint` v2.x (only required if you want to run lint locally; CI runs it on every push)

### One-time setup

After cloning, install the git hooks:

```bash
make setup
```

This wires up `.githooks/pre-commit` (gofmt enforcement) and `.githooks/pre-push` (golangci-lint + freshness checks for `commands.json` and `providers.json`). Skipping this step means CI will reject your push.

### Building and Testing

From the repo root (the root `Makefile` delegates into `cli/`):

```bash
make build    # Compiles the binary to cli/syllago (does NOT install to PATH)
make test     # Runs the full test suite
make fmt      # Formats Go code with gofmt
make vet      # Runs go vet
```

After `make build`, the binary lives at `cli/syllago`. To test against your `syllago` command on PATH, copy it explicitly:

```bash
cp cli/syllago ~/.local/bin/syllago   # or wherever `which syllago` resolves
```

This is intentionally separate from `make build` so the build target is non-destructive.

### Linting

`cli/.golangci.yml` configures golangci-lint v2 with `gofmt` formatting enforcement. CI runs the same config:

```bash
cd cli && golangci-lint run ./...
```

The pre-push hook runs this automatically.

### Test coverage

Every code change should ship with tests. The internal target is **80% coverage minimum per package, 95%+ aspirational**. Bug fixes must include a regression test that would have caught the bug. New CLI commands need integration tests using cobra's `RunE` pattern.

To check coverage locally:

```bash
cd cli && go test ./your/package/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
```

### Regenerating derived files

Some files in the repo are generated and CI fails if they're stale:

- After adding or changing CLI flags: `cd cli && make gendocs` regenerates `commands.json`.
- After provider changes: the same target also regenerates `providers.json`.
- After adding telemetry properties: `cd cli && make gendocs` updates `telemetry.json`. The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` will fail if you forget.

### Architectural decisions

Significant architectural choices are recorded as ADRs in [`docs/adr/`](docs/adr/). [`docs/adr/INDEX.md`](docs/adr/INDEX.md) is the index. Strict-enforcement ADRs block commits that touch their scoped files; advisory ADRs warn. Read the relevant ADR before modifying files in its scope.

### Code of Conduct

Contributors are expected to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

### Telemetry

Dev builds have telemetry compiled out — the PostHog API key is only embedded in release binaries via `SYLLAGO_POSTHOG_KEY` ldflags. You will never send telemetry data from a local build unless you explicitly set the environment variable.

### Code Organization

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full package map and data flow.

### Testing Patterns

- Table-driven tests with `t.Run()`
- `t.TempDir()` for filesystem fixtures -- never hardcode paths
- No mocking library -- hand-crafted provider stubs and function overrides
- TUI components: golden file visual regression tests
  - Regenerate baselines after visual changes: `cd cli && go test ./internal/tui/ -update-golden`

### capmon (capability monitor)

capmon is the pipeline that extracts and tracks AI provider capability drift. It runs automatically on CI via `.github/workflows/capmon.yml`.

**Pausing the pipeline:**

Create a `.capmon-pause` file in the repo root to prevent Stage 4 from opening PRs or issues:

```bash
touch .capmon-pause   # pause
rm .capmon-pause      # resume
```

The pipeline still runs Stages 1-3 (fetch, extract, diff) when paused. Only the GitHub PR/issue step is skipped.

**Updating test fixtures:**

Static extraction fixtures live in `cli/internal/capmon/testdata/fixtures/`. When a provider changes its docs format, update the fixture and re-run the tests:

```bash
# Update a fixture manually, then verify:
cd cli && go test ./internal/capmon/ -run TestFixtures
```

Live network tests are gated behind `SYLLAGO_TEST_NETWORK=1` and require external access.

**Manual audit workflow:**

To review capability drift without CI:

1. Run `syllago capmon run --dry-run` to see what would change
2. Review `docs/provider-capabilities/<slug>.yaml` for the affected provider
3. Apply updates manually if the change is correct
4. Run `syllago capmon generate` to regenerate derived views and spec tables

## Getting started

1. Check [existing issues](https://github.com/OpenScribbler/syllago/issues) to see if someone has already raised your idea
2. Pick the right template when opening a new issue
3. Answer the questions — be as specific as you can
4. That's it. We'll take it from there.
