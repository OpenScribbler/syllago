# Research: CLI Best Practices (2024-2026)

**Date:** 2026-03-11
**Scope:** User feedback, error design, progressive disclosure, exit codes, interactive vs non-interactive, Go CLI conventions
**Note:** Research agent was terminated early due to rate limiting. Synthesized from partial results + overlapping findings from other research agents.

---

## Core Philosophy

### Human-First Design
CLI tools should be designed for humans first while maintaining compatibility with automated systems. Key principles (from clig.dev):
- "Simple parts that work together" -- modularity and composability
- Consistency across programs to build user intuition
- Be generous with output for humans, parseable for machines

### POSIX Conventions
- Options vs. operands: "Arguments that consist of hyphens and single letters or digits"
- Short flags (`-v`), long flags (`--verbose`), combined short flags (`-vf`)
- `--` terminates option parsing
- Flags should have sensible defaults; required args should be positional

---

## Error Message Design

### Actionable Errors (Effective Go + CLI Guidelines)
Errors should answer three questions:
1. **What happened?** -- specific description
2. **Why?** -- cause or context
3. **What can the user do?** -- constructive suggestion

**Good:**
```
ERROR: Port 8080 already in use
       Run with --port=8081 or stop the process on 8080
```

**Bad:**
```
Error: invalid input
```

### Go Error Wrapping (Go errors package)
```go
wrapsErr := fmt.Errorf("doing X with %s: %w", thing, err)
```
- Wrap with context at each call site
- Use `errors.Is()` and `errors.As()` for programmatic error checking
- Error messages should be lowercase, no trailing punctuation (Go convention)

### Error Hierarchy
- **Fatal errors:** Print to stderr, exit non-zero. User cannot continue.
- **Warnings:** Print to stderr, continue execution. User should know but isn't blocked.
- **Info:** Print to stdout. Normal operational feedback.

### Prefix Convention
```
error: cannot connect to registry
warning: registry has no manifest
info: syncing 3 registries
```
Use text prefixes, not just color. Enables grep, piping, accessibility.

---

## User Feedback Patterns

### Status Communication
- **Before operation:** "Cloning repository..." (what's about to happen)
- **During operation:** Spinner or dots (something is happening)
- **After operation:** "Added registry: my-repo" (what happened)
- **On failure:** "Clone failed: repository not found" (what went wrong + why)

### Progressive Disclosure
1. **Default output:** Essential information only
2. **`-v` / `--verbose`:** Detailed progress and context
3. **`-q` / `--quiet`:** Suppress non-error output
4. **`--debug`:** Internal diagnostics for troubleshooting

### Transient vs Persistent Feedback
- **Transient:** Status messages that clear on next action (TUI pattern)
- **Persistent:** Warnings/errors that stay visible until acknowledged
- **Log-style:** Appending to output stream (CLI pattern)

---

## Handling Wrong Input Gracefully

### Smart Suggestions
When input is close but not right, suggest the correct form:
```
error: unknown command "registy"
       Did you mean "registry"?
```

### Type Redirection
When input is valid but wrong type for the current context:
```
This URL contains Claude Code content, not a syllago registry.
Would you like to browse and import individual items instead?
```

### Cobra Pattern
Cobra implements automatic:
- Unknown command suggestions (Levenshtein distance)
- Required flag validation with clear messages
- Type coercion with descriptive errors
- Help text injection on error

---

## Exit Codes

### Standard Conventions (POSIX/sysexits.h)
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (wrong args) |
| 64 | Command line usage error (EX_USAGE) |
| 65 | Data format error (EX_DATAERR) |
| 66 | Input not found (EX_NOINPUT) |
| 69 | Service unavailable (EX_UNAVAILABLE) |
| 70 | Internal error (EX_SOFTWARE) |
| 73 | Can't create output (EX_CANTCREAT) |
| 74 | I/O error (EX_IOERR) |
| 75 | Temporary failure (EX_TEMPFAIL) |
| 77 | Permission denied (EX_NOPERM) |
| 78 | Configuration error (EX_CONFIG) |
| 126 | Command not executable |
| 127 | Command not found |
| 128+N | Killed by signal N |

### Practical Guidance
- Use 0 for success, 1 for general errors
- Use 2 for usage/argument errors (Cobra default)
- Be consistent within your tool
- Document non-standard exit codes

---

## Interactive vs Non-Interactive

### Detection
```go
if term.IsTerminal(os.Stdout.Fd()) {
    // Interactive: show TUI, colors, spinners
} else {
    // Non-interactive: plain text, no colors, no spinners
}
```

### Rules
- **Never prompt in non-interactive mode** -- fail with clear error instead
- **Support `--yes` / `-y` flag** for scripted confirmation bypass
- **Machine-readable output** via `--json` or `--format` flags
- **stdin piping** should work for all input that accepts text

---

## Logging Pattern (12 Factor)

"Treat logs as event streams"
- Applications should write to stdout/stderr, not manage log files
- stderr for diagnostic/error output
- stdout for primary output (data, results)
- Let the environment handle log routing

---

## Terminal Output Styling

### Chalk/Lipgloss Pattern
- Chainable styling API for composing text appearance
- Automatic color detection and degradation
- Respect terminal capabilities (16, 256, truecolor)
- NO_COLOR support as first-class concern

### Width Awareness
```go
width, _, _ := term.GetSize(int(os.Stdout.Fd()))
if width < 80 {
    // Compact layout
}
```
- Detect terminal width; adapt layout
- Default to 80 columns when detection fails
- Never assume infinite width

---

## Go-Specific CLI Conventions

### Cobra Command Structure
Pattern: `APPNAME COMMAND ARG --FLAG`
- Commands are verbs: `add`, `remove`, `sync`
- Subcommands for namespacing: `registry add`, `registry remove`
- Flags modify behavior, args specify targets
- Help auto-generated from annotations

### Error Handling in Cobra
```go
RunE: func(cmd *cobra.Command, args []string) error {
    if err := doThing(); err != nil {
        return fmt.Errorf("doing thing: %w", err)
    }
    return nil
}
```
- Use `RunE` (not `Run`) to propagate errors
- Cobra handles exit codes from returned errors
- Wrap errors with context at each level

---

## Sources (Partial -- Agent Terminated Early)
1. clig.dev -- Command Line Interface Guidelines (comprehensive CLI design)
2. Effective Go -- go.dev/doc/effective_go (error handling, logging)
3. Go errors package -- pkg.go.dev/errors (wrapping, Is/As)
4. Cobra -- github.com/spf13/cobra (command patterns, error handling)
5. POSIX Utility Conventions -- IEEE Std 1003.1 (argument syntax)
6. 12 Factor App -- 12factor.net (logs as event streams)
7. Chalk -- github.com/chalk/chalk (terminal styling patterns)
8. sysexits.h -- BSD exit code conventions
