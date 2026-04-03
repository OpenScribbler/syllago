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

- Go 1.25+
- Make

### Building and Testing

From the `cli/` directory (or use `make` targets from the repo root):

```bash
make build    # Build binary to ~/.local/bin/syllago
make test     # Run test suite (includes go vet)
make fmt      # Format code with gofmt
make vet      # Run go vet
```

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

## Getting started

1. Check [existing issues](https://github.com/OpenScribbler/syllago/issues) to see if someone has already raised your idea
2. Pick the right template when opening a new issue
3. Answer the questions — be as specific as you can
4. That's it. We'll take it from there.
