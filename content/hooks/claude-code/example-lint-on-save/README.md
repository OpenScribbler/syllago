# Lint on Save

A Claude Code hook that runs a linter command after file edits (Write or Edit tool use).

## How it works

This hook triggers on the `PostToolUse` event, matching `Write|Edit` tool calls. When Claude Code writes or edits a file, the configured linter command runs automatically.

## Usage

1. Install this hook via syllago
2. Replace the placeholder command in `hook.json` with your actual linter (e.g., `eslint --fix`, `gofmt -w`)

## Configuration

Edit `hook.json` to customize:
- **matcher**: Which tool calls trigger the hook (default: `Write|Edit`)
- **command**: The shell command to run
