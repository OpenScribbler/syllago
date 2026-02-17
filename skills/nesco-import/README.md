# Nesco Import Skill

A built-in skill that teaches LLMs how to import content into a romanesco repository using the `nesco` CLI.

## What It Does

This skill provides instructions for using `nesco add` (non-interactive import) and `nesco import --from` (provider discovery) to manage AI coding tool content. It covers:

- Content types and directory conventions
- The `nesco add` command for scripted/LLM-driven imports
- README generation behavior (auto-created when missing)
- Provider-specific vs universal content handling

## Installation

This skill is tagged `builtin` and is automatically offered during `nesco init`. You can also install it manually through the TUI or via `nesco add`.

**Type:** Skills
