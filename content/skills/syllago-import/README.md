# Syllago Import Skill

A built-in skill that teaches LLMs how to import content into a syllago repository using the `syllago` CLI.

## What It Does

This skill provides instructions for using `syllago add` (non-interactive import) and `syllago import --from` (provider discovery) to manage AI coding tool content. It covers:

- Content types and directory conventions
- The `syllago add` command for scripted/LLM-driven imports
- README generation behavior (auto-created when missing)
- Provider-specific vs universal content handling

## Installation

This skill is tagged `builtin` and is automatically offered during `syllago init`. You can also install it manually through the TUI or via `syllago add`.

**Type:** Skills
