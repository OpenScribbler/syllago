# Roadmap

Status reflects shipped state as of `VERSION` 0.9.0. For the running list of recent changes see [`CHANGELOG.md`](CHANGELOG.md). For active design work see [`docs/plans/`](docs/plans/).

## Trust and Supply Chain (MOAT)

Syllago is the reference implementation of [MOAT](https://github.com/OpenScribbler/moat). The package lives at `cli/internal/moat/`.

| Feature | Status |
|---|---|
| Sigstore keyless signature verification (cosign bundle) | Done |
| Rekor inclusion proof verification | Done |
| GitHub OIDC numeric-ID pinning (repository_id, repository_owner_id) | Done |
| Three trust tiers: `DUAL-ATTESTED`, `SIGNED`, `UNSIGNED` | Done |
| Bundled allowlist for well-known signing identities | Done |
| `--signing-identity` / `--signing-repository-*` flags on `registry add` | Done |
| Bundled trusted root with 90/180/365-day staleness cliff | Done |
| Revocation: archival + live, four reasons | Done |
| Per-call trust policy floor (`--min-trust`, `MOAT_009`) | Done |
| TUI trust surfacing (Library, Registries, Loadouts) | Done |
| `MOAT_001` – `MOAT_009` structured error codes with `syllago explain` | Done |
| Custom OIDC issuers (GitLab, Buildkite, generic OIDC) | Planned |
| `syllago moat sign` for self-publishing registries | Done |
| Org-policy distribution (signing identity bundles for enterprises) | Planned |

## Security (non-MOAT)

| Feature | Status |
|---|---|
| Bubblewrap sandbox (Linux) — env allowlist, network proxy, configdiff-and-approve | Done |
| `gosec` static analysis in CI | Done |
| `govulncheck` dependency scanning in CI | Done |
| Race detector in CI | Done |
| Pinned GitHub Actions (full SHAs) | Done |
| SBOM generation in release artifacts | Done |
| Dependency review in CI | Done |
| Pluggable script scanner — chain interface, external adapters (ShellCheck, Semgrep) | Planned |
| OpenSSF Best Practices badge (self-assessment) | Planned |
| macOS sandbox support | Planned |

## Privacy

| Feature | Status |
|---|---|
| Registry privacy gates — prevent private content from leaking to public registries | Done |
| Audit trail for content operations (add, install, remove, share) | Done |
| Content integrity hashes at install time | Done |

## Distribution and Content

| Feature | Status |
|---|---|
| Bulk install operations (`install --all`, `install --type rules`) | Done |
| Provider-to-provider file conversion (`convert --from --to`) | Done |
| Conversion diff mode (`convert --diff`) | Done |
| `syllago update` self-update from GitHub Releases | Done |
| `syllago refresh` library refresh from disk | Done |
| Loadout emitters for all detected providers | Done |
| `syllago-starter` built-in loadout (12 providers) | Done |

## Platform and Tooling

| Feature | Status |
|---|---|
| `syllago doctor` — diagnostic command | Done |
| `syllago info` — detected providers, paths, registries | Done |
| `syllago compat --hooks` — provider hook capability matrix | Done |
| Capability monitor (`capmon`) — provider doc drift detection pipeline | Done |
| Provider drift detection (`provmon`) — manifest + source-hash checker | Done |
| Telemetry catalog with drift-detection test (`gentelemetry`) | Done |
| Container image and GitHub Action for CI/CD pipelines | Planned |
| Org-level config inheritance | Planned |
| VHS demo GIFs for README | Planned |
| GitHub Discussions for community Q&A | Planned |

## Canonical Format Specs

Provider-neutral interchange formats for each content type. Specs live under [`docs/spec/`](docs/spec/).

| Content Type | Spec | Status |
|---|---|---|
| Hooks | [`docs/spec/hooks/`](docs/spec/hooks/) | Stable (modularized into events, tools, schema, blocking matrix, test vectors, security considerations) |
| MOAT | [`docs/spec/moat/`](docs/spec/moat/) | Stable (community-owned at github.com/OpenScribbler/moat) |
| Skills | [`docs/spec/skills/`](docs/spec/skills/) | Drafting (provenance, adversarial review notes) |
| ACP (Agent Configuration Protocol) | [`docs/spec/acp/`](docs/spec/acp/) | Draft |
| Canonical keys | [`docs/spec/canonical-keys.yaml`](docs/spec/canonical-keys.yaml) | Stable |
| Agents | `docs/spec/agents/` | Planned |
| Rules | `docs/spec/rules/` | Planned |
| MCP | `docs/spec/mcp/` | Planned |
| Commands | `docs/spec/commands/` | Planned |
| Loadouts | `docs/spec/loadouts/` | Planned |

## New Providers

Currently shipping support for **15 providers**: Claude Code, Cursor, Windsurf, Codex, Gemini CLI, Copilot CLI, Cline, Roo Code, Zed, OpenCode, Kiro, Amp, Factory Droid, Pi, Crush.

| Provider | Notes |
|----------|-------|
| VS Code Copilot | Preview hooks (same 3 events as Copilot CLI), `.vscode/hooks.json` config |
| Qwen Code | Fork of Gemini CLI — same `settings.json` format, skills, MCP. Low effort (path mapping + detection) |
| Kimi CLI | Moonshot AI's agent (7.1k stars). Skills, MCP, hooks. 5,000+ community skills via ClawHub |
| Trae Agent | ByteDance's research CLI (11k stars). YAML/JSON config, modular architecture, MIT licensed |
| Aider | `--auto-lint` and `--auto-test` flags (no hook system, but content types apply) |
| Continue.dev | `config.yaml` rules and MCP integration |
| Goose | MCP-only extensions model |
