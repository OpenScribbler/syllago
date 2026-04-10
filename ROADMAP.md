# Roadmap

## Security

| Feature | Phase | Status |
|---------|-------|--------|
| Audit logging -- wire into install/import flows, query CLI, CSV/SIEM export | Foundation | Planned |
| Script scanning -- language-specific patterns for .sh, .py, .js, .ps1, .rb | Foundation | Planned |
| Trust tiers -- trusted/verified/community with install-time gates | Foundation | Planned |
| Pluggable scanner -- chain interface, external adapters (ShellCheck, Semgrep) | Infrastructure | Planned |
| Hook signing and verification (Sigstore keyless + GPG) | Cryptographic | Planned |
| Revocation -- per-registry revocation index, sync, install-time blocking | Enforcement | Planned |
| Policy engine -- per-tier rules, allowed identities, signature requirements | Enforcement | Planned |

## Privacy and Integrity

| Feature | Status |
|---------|--------|
| Registry privacy gates -- prevent private content from leaking to public registries | Done |
| Content integrity hashes at install time | Done |
| Audit trail for content operations (add, install, remove, share) | Done |

## Distribution and Content

| Feature | Status |
|---------|--------|
| Bulk install operations (`install --all`, `install --type rules`) | Done |
| `add --from shared` for bundled content | Done |
| Provider-to-provider file conversion (`convert --from --to`) | Done |
| Conversion diff mode (`convert --diff`) | Done |
| Content update mechanism (`syllago update`) | Planned |
| Additional loadout provider emitters beyond Claude Code | Planned |

## Platform and Tooling

| Feature | Status |
|---------|--------|
| `syllago doctor` -- diagnostic command for troubleshooting | Done |
| Enhanced `syllago info` -- detected providers, paths, registries | Done |
| `syllago compat --hooks` -- provider hook capability matrix | Planned |
| Hook portability report -- warn about capability mismatches during install | Planned |
| Container image and GitHub Action for CI/CD pipelines | Planned |
| Org-level config inheritance | Planned |
| macOS sandbox support (Linux sandbox already available) | Planned |
| VHS demo GIFs for README | Planned |
| SBOM generation in release artifacts | Done |
| Dependency review in CI | Done |
| OpenSSF Best Practices badge (self-assessment) | Planned |
| GitHub Discussions for community Q&A | Planned (post-launch) |

## Canonical Format Specs

Syllago defines provider-neutral interchange formats for each content type. The hooks spec is complete; the rest are implemented in code but not yet formally specified.

| Content Type | Spec | Status |
|---|---|---|
| Hooks | [`docs/spec/hooks.md`](docs/spec/hooks.md) | Draft |
| Skills | `docs/spec/skills-v1.md` | Planned |
| Agents | `docs/spec/agents-v1.md` | Planned |
| Rules | `docs/spec/rules-v1.md` | Planned |
| MCP | `docs/spec/mcp-v1.md` | Planned |
| Commands | `docs/spec/commands-v1.md` | Planned |
| Loadouts | `docs/spec/loadouts-v1.md` | Planned |

## New Providers

| Provider | Notes |
|----------|-------|
| VS Code Copilot | Preview hooks (same 3 events as Copilot CLI), `.vscode/hooks.json` config |
| Qwen Code | Fork of Gemini CLI -- same `settings.json` format, skills, MCP. Low effort (path mapping + detection) |
| Crush | Charmbracelet's Go TUI agent (21.8k stars). LSP-aware, MCP support, multi-provider |
| Kimi CLI | Moonshot AI's agent (7.1k stars). Skills, MCP, hooks. 5,000+ community skills via ClawHub |
| Trae Agent | ByteDance's research CLI (11k stars). YAML/JSON config, modular architecture, MIT licensed |
| Droid | Factory's enterprise agent. Top terminal benchmarks, specialized agent types, YAML config |
| Pi Agent Rust | TypeScript extensions via embedded QuickJS, 20+ lifecycle events, capability-gated security |
| Aider | `--auto-lint` and `--auto-test` flags (no hook system, but content types apply) |
| Continue.dev | `config.yaml` rules and MCP integration |
| Goose | MCP-only extensions model |
