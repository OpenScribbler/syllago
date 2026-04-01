---
id: "0004"
title: Executable Content Always Requires Confirmation
status: accepted
date: 2026-04-01
enforcement: strict
files: ["cli/internal/analyzer/analyzer.go"]
tags: [security, hooks, mcp, confirmation, trust]
---

# ADR 0004: Executable Content Always Requires Confirmation

## Status

Accepted

## Context

The analyzer partitions detected items into two categories based on confidence scores:
- **Auto** (confidence > 0.80): included in the manifest without user interaction
- **Confirm** (confidence 0.50–0.80): presented to the user for review

Hooks and MCP server configurations are executable content — they run code on the user's machine. A hook at 0.95 confidence is still running arbitrary shell commands. An MCP server at 0.90 confidence is still spawning a process with network access.

Confidence scoring measures "how sure are we this file is a hook?" — it does NOT measure "is this hook safe to run?" These are fundamentally different questions. A perfectly-detected hook that installs a cryptominer is 0.95 confidence and 0.0 safety.

## Decision

Hooks (`catalog.Hooks`) and MCP configs (`catalog.MCP`) always route to the `Confirm` bucket regardless of their confidence score. The confidence-based Auto/Confirm partition only applies to non-executable content types (Skills, Agents, Rules, Commands, Loadouts).

This is implemented in `analyzer.go` lines 100-103:
```go
if item.Type == catalog.Hooks || item.Type == catalog.MCP {
    result.Confirm = append(result.Confirm, item)
    continue
}
```

This is a security invariant, not a UX preference. It cannot be overridden by raising a detector's confidence score.

## Consequences

**What becomes easier:**
- Security reasoning is simple: "did the user confirm all executable content?" is a single check.
- No risk of auto-installing hooks from an untrusted registry. Even if a registry.yaml declares hooks at maximum confidence, the user must acknowledge them.

**What becomes harder:**
- Trusted registries with many hooks will always prompt for confirmation. There is no "trust this registry's hooks" shortcut (intentionally — trust decisions should be explicit per-item, not blanket).

**What's deferred:**
- A future "trusted registries" feature could skip confirmation for registries the user has explicitly trusted. This ADR does not prevent that — it prevents *implicit* skipping based on confidence scores alone.
