---
name: Kitchen Sink App
description: Example app that populates every available frontmatter field for testing and reference
providers:
  - claude-code
  - gemini-cli
  - copilot-cli
---

# Kitchen Sink App

This is an example app demonstrating every available frontmatter field in the app format.

## What It Does

Nothing practical. It serves as a living reference for the complete set of metadata fields an app can declare, plus an example install script.

## Installation

This app includes an `install.sh` script that would typically set up dependencies, configure environment variables, or perform other setup tasks.

## Requirements

- Node.js 18+
- A supported AI coding provider (Claude Code, Gemini CLI, or Copilot CLI)

## Fields Demonstrated

- **name**: Display name for the app
- **description**: Human-readable summary
- **providers**: List of provider slugs this app supports
