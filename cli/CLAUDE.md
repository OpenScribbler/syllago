# CLI Rules

The `cli/` directory is the Go codebase for the syllago terminal application.

## Build and Test

```bash
# From cli/ directory:
make build          # Build binary with version/commit ldflags
make test           # Run all tests
make fmt            # Format with gofmt
make vet            # Run go vet

# TUI-specific:
go test ./internal/tui/ -update-golden   # Regenerate golden files after visual changes
```

**Always run `make test` after changes to verify nothing broke.** Golden file tests will fail if visual output changed without updating baselines.

## Package Structure

| Package | Purpose |
|---------|---------|
| `cmd/syllago` | Entry point, cobra command wiring |
| `internal/tui` | BubbleTea terminal UI (see tui/CLAUDE.md for detailed conventions) |
| `internal/catalog` | Content discovery, loading, risk indicators |
| `internal/provider` | Provider detection and configuration |
| `internal/installer` | Install/uninstall operations per provider |
| `internal/converter` | Content format conversion between providers |
| `internal/config` | User configuration management |
| `internal/registry` | Remote registry client |
| `internal/loadout` | Loadout apply/remove/preview logic |
| `internal/promote` | Local-to-shared content promotion |
| `internal/gitutil` | Git operations (clone, pull, status) |
| `internal/metadata` | Content metadata parsing |
| `internal/model` | Shared data types |
| `internal/output` | CLI output formatting (non-TUI) |
| `internal/parse` | File parsing utilities |
| `internal/readme` | README rendering |
| `internal/sandbox` | Sandbox configuration |
| `internal/snapshot` | Snapshot management |
| `internal/updater` | Self-update logic |

## Go Conventions

- **Error handling:** Return errors, don't panic. Use `fmt.Errorf("context: %w", err)` for wrapping.
- **Testing:** Table-driven tests with `t.Run()` subtests. Golden files for visual output.
- **Naming:** Follow standard Go conventions. Exported types for public API, unexported for internal.
- **Dependencies:** Charm stack for TUI (bubbletea, lipgloss, bubbles). Cobra for CLI commands. YAML for config files.

## Content Types

Syllago manages these AI tool content types:
- Rules, Skills, Agents, Commands, Hooks, MCP configs, Prompts, Apps, Loadouts

Content is provider-agnostic at the catalog level. Provider-specific handling (install paths, format conversion) lives in the `provider` and `installer` packages.
