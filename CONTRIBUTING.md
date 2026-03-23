# Contributing to syllago

Thank you for wanting to contribute. syllago has a contribution model built around one core idea: **your thinking is the contribution**.

## How contributions work

syllago accepts **ideas, not code**. You don't need to write a single line to make a meaningful contribution.

When you open an issue, you'll be guided through a structured set of questions depending on what you're bringing:

- **Bug Report** — Something isn't working the way it should
- **Feature Idea** — Something that should exist but doesn't yet
- **Improvement** — Something that works but could be better
- **Content Request** — A suggestion for new built-in AI content (skills, agents, rules, hooks, etc.) that helps people use or understand syllago

Each template walks you through the right questions. Just follow the prompts.

## What we're looking for

- **Describe the what and why.** What's the problem, gap, or opportunity? Why does it matter?
- **Explain the how in your own words.** If you have ideas about how something should work or change, walk us through your thinking. Explain it like you're talking to a teammate.
- **No code.** Don't paste snippets, diffs, or implementations. Your description is the contribution — we'll handle the build.

## About syllago's built-in content

syllago ships with AI content — skills, agents, rules, hooks, and more — that help users work with and understand the CLI itself. These are meta-tools: they exist to make syllago better, not for general-purpose distribution. When suggesting new content, think about what would make syllago easier to learn, use, or extend.

## Getting started

1. Check [existing issues](https://github.com/OpenScribbler/syllago/issues) to see if someone has already raised your idea
2. Pick the right template when opening a new issue
3. Answer the questions — be as specific as you can
4. That's it. We'll take it from there.

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

### Code Organization

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full package map and data flow.

### Testing Patterns

- Table-driven tests with `t.Run()`
- `t.TempDir()` for filesystem fixtures -- never hardcode paths
- No mocking library -- hand-crafted provider stubs and function overrides
- TUI components: golden file visual regression tests
  - Regenerate baselines after visual changes: `cd cli && go test ./internal/tui/ -update-golden`
  - Run individual golden tests when updating, not the full suite (known test isolation issue with golden updates)

### Why No External PRs?

Syllago is maintained using an AI-augmented development workflow where a small team handles design and implementation together. External code contributions create coordination overhead that doesn't fit this model.

We genuinely welcome issues. Bug reports, feature requests, and use case descriptions help us prioritize what to build next -- your thinking is the real contribution.
