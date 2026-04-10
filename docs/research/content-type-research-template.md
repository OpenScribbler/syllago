# [Content Type]: Cross-Provider Deep Dive

> Research compiled [DATE]. Covers [content type] systems across [N] AI coding tools.
> Template derived from docs/research/hook-research.md structure.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview: Who Supports This](#overview)
2. [Per-Provider Deep Dive](#per-provider)
3. [Cross-Platform Normalization Problem](#normalization)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#feature-matrix)
6. [Compat Scoring Implications](#compat-scoring)
7. [Converter Coverage Audit](#converter-audit)

---

## Overview

### Summary Table

Which providers support this content type, and at what level:

| Tool | Supports [Type] | Format | Scoping | Key Differentiator |
|------|-----------------|--------|---------|-------------------|
| Claude Code | | | | |
| Cursor | | | | |
| Gemini CLI | | | | |
| Copilot CLI | | | | |
| VS Code Copilot | | | | |
| Windsurf | | | | |
| Kiro | | | | |
| Codex CLI | | | | |
| Cline | | | | |
| OpenCode | | | | |

**No support:** [List tools that don't support this content type at all]

---

## Per-Provider Deep Dive

### [Provider Name]

For each provider that supports this content type, document:

**Format and structure:**
- File format (MD, MDC, TOML, JSON, YAML, etc.)
- File naming conventions
- Directory structure expectations
- Single-file vs multi-file

**Features and semantics:**
- [Content-type-specific features — e.g., for rules: alwaysApply, globs, description]
- [What metadata fields exist and what they mean]
- [How the provider processes/applies this content]

**Configuration locations:**
- Global scope path
- Project scope path
- Precedence rules

**Example:**
```
[Provide a real-world example of this content type in this provider's native format]
```

**Unique capabilities:**
- [Features only this provider has]

---

## Cross-Platform Normalization Problem

### What Differs

| Concept | Claude Code | Cursor | Gemini CLI | [Others] |
|---------|-------------|--------|------------|----------|
| [Feature A] | | | | |
| [Feature B] | | | | |

### The Asymmetry Problem

[Document which conversions are lossy and in which direction. Which provider has the richest feature set? What gets lost converting "down" to simpler providers?]

### What's Universal

[Document features that exist identically across all providers — these are safe conversions]

---

## Canonical Mapping

Based on overlapping capabilities, syllago's canonical format for this content type:

```
[Show the canonical format structure with annotations for which fields map to which providers]
```

### Field Mapping Table

| Canonical Field | Claude Code | Cursor | Gemini CLI | [Others] | Loss if Missing |
|----------------|-------------|--------|------------|----------|-----------------|
| [field] | [equivalent] | [equivalent] | [equivalent] | | [impact] |

---

## Feature/Capability Matrix

This directly feeds into `converter/compat.go` for the unified compat scorer.

### Feature Definitions

| Feature | Description | Impact When Missing |
|---------|-------------|-------------------|
| [FeatureA] | [What it does] | [CompatDegraded/CompatBroken/CompatNone] |

### Provider Support Matrix

| Feature | Claude Code | Cursor | Gemini CLI | Copilot CLI | Windsurf | Kiro | Codex |
|---------|-------------|--------|------------|-------------|----------|------|-------|
| [FeatureA] | [supported?] | | | | | | |

---

## Compat Scoring Implications

### Conversion Paths — Expected Compat Levels

| Source -> Target | Expected Level | Key Losses |
|-----------------|---------------|------------|
| Claude Code -> Cursor | | |
| Claude Code -> Gemini CLI | | |
| Cursor -> Claude Code | | |
| [etc.] | | |

### Recommendations for syllago

- [What features to include in canonical format]
- [What conversion warnings to generate]
- [What should be CompatNone vs CompatDegraded vs CompatBroken]

---

## Converter Coverage Audit

Current state of syllago's converter for this content type:

| Function | Implemented? | Providers Covered | Gaps |
|----------|-------------|-------------------|------|
| Canonicalize() | | | |
| Render() | | | |

[Note any provider combinations where Render() would produce incorrect output or isn't implemented]
