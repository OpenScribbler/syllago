# PostHog Telemetry - Design Document

**Goal:** Add anonymous, opt-out usage analytics to syllago via PostHog so we can understand how people use the tool, which providers matter, and where errors occur.

**Decision Date:** 2026-04-02

---

## Problem Statement

After release, we'll have no visibility into how syllago is used — which commands people run, which providers they target, where they hit errors, or whether they prefer the TUI or CLI. Without this data, we can't prioritize provider support, find bugs at scale, or understand adoption patterns.

We need telemetry that's transparent, privacy-respecting, and trivially easy to disable. The developer community is (rightly) sensitive about CLI tools that phone home, so the implementation must earn trust through clarity, not obscurity.

## Proposed Solution

Embed anonymous usage analytics using PostHog's ingest API. Telemetry is **enabled by default** (opt-out model) with a first-run notice on every user's first invocation. Users can disable with `syllago telemetry off` or the `DO_NOT_TRACK` environment variable.

Data collection is minimal: command names, provider types, content type counts, error codes, version, and OS. No content, file paths, usernames, or anything that could identify a user or their work.

## Architecture

### Components

**1. `cli/internal/telemetry/` package**
- `telemetry.go` — `Init()`, `Track(event, properties)`, `Shutdown()` functions
- `config.go` — Read/write `~/.syllago/telemetry.json`

**2. `syllago telemetry` subcommand**
- `status` — Shows enabled/disabled, anonymous ID, tracked events, never-tracked data
- `on` — Enables telemetry
- `off` — Disables telemetry
- `reset` — Generates new anonymous ID (does not delete previously collected data)

**3. Root command integration**
- `PersistentPreRun` — calls `telemetry.Init()` (checks `DO_NOT_TRACK` first, then loads config, shows first-run notice to stderr)
- `PersistentPostRun` — calls `telemetry.Shutdown()` (waits up to 2s for in-flight POST)

### Init() Order of Operations

The order is critical for correctness:

1. Check `DO_NOT_TRACK` env var — if set to any truthy value, set disabled flag and **return immediately**. No config read, no client initialization, no network.
2. Check system-level config (`/etc/syllago/telemetry.json`) — if present and `enabled: false`, set disabled flag and return.
3. Read user config (`~/.syllago/telemetry.json`) — if unreadable or unwritable, **treat as disabled** for this session. Do not collect from users who cannot persist an opt-out.
4. If config missing, create with defaults (`enabled: true`, generate anonymous ID, `noticeSeen: false`).
5. If `noticeSeen: false`, print first-run notice to **stderr**, set `noticeSeen: true`.
6. If `enabled: false`, return without initializing HTTP client.
7. Initialize PostHog HTTP client with configured or default ingest endpoint.

### Data Flow

```
User runs command → PersistentPreRun: Init() (see order above)
  → Command executes → Track("command_executed", props)
  → Goroutine fires POST to PostHog ingest API
  → PersistentPostRun: Shutdown() waits for pending send
```

### Transport

Fire-and-forget HTTP POST to PostHog's ingest endpoint. Non-blocking goroutine, 2-second timeout. If offline or the POST fails, the event is silently dropped. No local queue, no retry, no batching.

The ingest endpoint is configurable via the `endpoint` field in `~/.syllago/telemetry.json` (defaults to `https://us.i.posthog.com/capture/`). This allows enterprises to route telemetry to a self-hosted PostHog instance or internal proxy.

## First-Run Notice

**Output target: stderr** (never stdout — must not corrupt piped output like `syllago list | jq`).

The exact notice text, locked before implementation:

```
syllago collects anonymous usage data (commands run, provider
types, error codes) to help prioritize development. Syllago is
a solo-developer project and this data is invaluable for
steering its direction.

No file contents, paths, or identifying information is collected.

  Disable:  syllago telemetry off
  Env var:  DO_NOT_TRACK=1
  Details:  https://syllago.dev/telemetry
```

**TUI path:** When the TUI launches and `noticeSeen` is false, display as an info toast. The toast text is the same content, condensed to fit the toast format:

```
Syllago collects anonymous usage data to help prioritize
development. No file contents or identifying info is collected.
Run "syllago telemetry off" to disable.
syllago.dev/telemetry
```

**No data is sent until after the notice is displayed.** The first `Track()` call in a session happens after `Init()` completes, which means the notice is already printed. The notice and the first event are sequenced — never reversed.

## Data Model

### Config File: `~/.syllago/telemetry.json`

```json
{
  "enabled": true,
  "anonymousId": "syl_a1b2c3d4e5f6",
  "noticeSeen": false,
  "endpoint": "",
  "createdAt": "2026-04-02T12:00:00Z"
}
```

- `syl_` prefix on the anonymous ID makes it identifiable in config
- Generated via `crypto/rand`, not tied to any machine or user identifier
- `noticeSeen` prevents the first-run notice from appearing again
- `endpoint` is empty by default (uses PostHog cloud); enterprises can override to a self-hosted instance

### System-Level Config: `/etc/syllago/telemetry.json` (optional)

```json
{
  "enabled": false
}
```

Checked before user config. Allows IT/platform teams to enforce telemetry state org-wide via MDM, Ansible, Chef, or group policy. Only the `enabled` field is read from this file — all other settings come from the user config.

### Event Payload (sent to PostHog)

```json
{
  "event": "command_executed",
  "distinct_id": "syl_a1b2c3d4e5f6",
  "properties": {
    "command": "install",
    "success": true,
    "provider": "claude-code",
    "content_type": "rules",
    "content_count": 3,
    "error_code": "",
    "version": "0.8.0",
    "os": "linux",
    "arch": "amd64"
  }
}
```

## Privacy Guarantees

| Always tracked | Never tracked |
|---|---|
| Command name | File contents or rule text |
| Provider type (slug) | File paths or directory names |
| Content type + count | Usernames or hostnames |
| Success/failure + error code | IP addresses (see PostHog IP Stripping below) |
| syllago version + OS/arch | Registry URLs or git remote names |
| Pseudonymous random ID | Hook commands or MCP configs |

**Note on the anonymous ID:** The `syl_` ID is a persistent pseudonymous identifier — not truly anonymous. It correlates all events from a single machine into one behavioral timeline. It is not derived from any machine or user information (generated via `crypto/rand`), but over time it constitutes a behavioral fingerprint. Users can rotate it with `syllago telemetry reset`, but this does not delete previously collected data from PostHog (see Data Deletion below).

### PostHog IP Stripping

PostHog Cloud supports server-side IP stripping via the project setting **"Discard client IP data"** (Settings > Project > IP Data). This must be enabled on the syllago PostHog project before launch. The `/telemetry` docs page must link to PostHog's documentation on this setting for verifiability: https://posthog.com/docs/privacy

If PostHog's IP stripping behavior changes in the future, the syllago project will update the docs page and evaluate alternatives.

### `DO_NOT_TRACK` Priority Chain

1. `DO_NOT_TRACK` env var set to any truthy value (`1`, `true`, `yes`, `on`, case-insensitive) — telemetry disabled, `Init()` returns immediately (highest priority)
2. `/etc/syllago/telemetry.json` with `enabled: false` — telemetry disabled (fleet management)
3. `~/.syllago/telemetry.json` unreadable or unwritable — telemetry disabled for session (safe default)
4. `~/.syllago/telemetry.json` with `enabled: false` — telemetry disabled
5. Config missing or `enabled: true` — telemetry enabled

CI environments can set `DO_NOT_TRACK=1` globally without any syllago-specific config. Enterprise IT teams can deploy `/etc/syllago/telemetry.json` to disable org-wide.

### Data Deletion

Users and organizations can request deletion of data associated with their anonymous IDs:

1. Run `syllago telemetry status` to find your anonymous ID
2. Email `privacy@syllago.dev` with the ID(s) to request deletion
3. Deletion will be processed via PostHog's API within 30 days

The `syllago telemetry reset` command generates a new anonymous ID but **does not delete previously collected data**. The `reset` subcommand output explicitly states this.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Analytics service | PostHog (cloud, self-host supported) | 1M events/mo free tier, Go-friendly ingest API, open source with self-hosted option for enterprise |
| Default state | Enabled (opt-out) | Industry standard (Next.js, Homebrew, Astro). First-run notice provides transparency. Opt-in gets ~2% participation — not enough data to be useful |
| Transport | Fire-and-forget POST | Zero impact on CLI speed. No local state to manage. Dropped events are acceptable — we need trends, not perfect counts |
| Config location | `~/.syllago/telemetry.json` | Separate from project config (per-machine, not per-project). Clean separation of concerns |
| Fleet config | `/etc/syllago/telemetry.json` | System-level override for enterprise IT. Checked before user config. Deployable via MDM/Ansible/Chef |
| First-run notice | On first invocation (any command), to stderr | Most transparent option. Even `syllago --help` shows it once. stderr prevents piped output corruption |
| TUI notice | Toast notification | Non-blocking, consistent with existing TUI notification system. Auto-dismisses after ~10s |
| Anonymous ID format | `syl_` + crypto/rand hex | Identifiable prefix, cryptographically random, not derived from machine/user info. Documented as pseudonymous, not anonymous |
| Events at launch | command, error, version/OS, TUI session | Covers the core questions (what, where it breaks, who, which interface). Can add more later |
| `DO_NOT_TRACK` | Any truthy value (1, true, yes, on) | Broader than strict `=1` spec. Prevents gotcha when users copy CI configs from other tools |
| Ingest endpoint | Configurable (default: PostHog cloud) | Enterprise users can route to self-hosted PostHog or internal proxy. Enables air-gapped environments |

## Events & Integration Points

| Location | Event | Properties |
|---|---|---|
| `PersistentPostRun` in `main.go` | `command_executed` | command, provider, content_type, success, error_code |
| `install_cmd.go` RunE | `command_executed` | provider slug, content types installed, count |
| `add_cmd.go` RunE | `command_executed` | source type (path/git/registry), content type |
| `convert_cmd.go` RunE | `command_executed` | from_provider, to_provider, content type |
| `export_cmd.go` RunE | `command_executed` | provider, content types |
| TUI `App.Init()` | `tui_session_started` | version, os, arch |
| Any command error path | `error_occurred` | error_code, command |

`Track()` calls are one-liners at the end of each command's `RunE`. No structural changes to existing code. Events only fire from top-level command invocations — internal function calls between packages do not trigger events (no double-counting).

## `syllago telemetry` Subcommand

### Command Tree

```
syllago telemetry           → shows status (alias for 'status')
syllago telemetry status    → detailed view
syllago telemetry on        → enable
syllago telemetry off       → disable
syllago telemetry reset     → new anonymous ID (does not delete old data)
```

### `syllago telemetry status` Output

```
Telemetry: enabled
Anonymous ID: syl_a1b2c3d4e5f6

Events tracked:
  command_executed    Command name, provider, content type, success/failure
  error_occurred      Structured error code on failure
  tui_session_started TUI opened (no interaction details)
  cli_info            syllago version, OS, architecture

Never tracked:
  File contents, paths, usernames, hostnames, registry URLs,
  hook commands, MCP configs, or any content you manage.

Disable:  syllago telemetry off
Reset ID: syllago telemetry reset
Docs:     https://syllago.dev/telemetry
```

### `syllago telemetry reset` Output

```
Anonymous ID rotated: syl_x9y8z7w6v5u4

Note: Previously collected data under your old ID is not deleted.
To request deletion, email privacy@syllago.dev with your old ID.
```

All subcommands respect `--json` for scripting/automation.

## Documentation (Required — blocking for launch)

A dedicated `/telemetry` page on the docs site (https://syllago.dev/telemetry) covering:

1. **What we collect** — exact event list with example payloads, and what question each event answers (e.g., "command_executed tells us which providers to prioritize support for")
2. **What we never collect** — privacy guarantees table
3. **How to disable** — `syllago telemetry off`, `DO_NOT_TRACK=1`, config file location, system-level config for fleet management
4. **Why we collect it** — honest explanation: solo developer project, telemetry helps steer development priorities, find bugs, understand which providers and features matter most
5. **How it works** — fire-and-forget POST, no local storage, fails silently
6. **Data retention** — 1 year (PostHog Cloud free tier default), then automatically deleted
7. **Data deletion** — how to request deletion of data associated with your anonymous ID
8. **PostHog compliance** — link to PostHog's SOC2 report, DPA, data residency options (US/EU Cloud), GDPR compliance posture, and IP stripping documentation
9. **Enterprise deployment** — fleet-wide opt-out via `/etc/syllago/telemetry.json`, self-hosted endpoint configuration, `DO_NOT_TRACK` for CI, how to verify telemetry state at scale via `syllago telemetry status --json`

This page is linked from:
- `syllago telemetry status` output
- The first-run notice
- The project README (one line near installation instructions)
- `syllago --help` root command output

### Public Dashboard (Post-Launch)

After collecting initial data, publish a public PostHog dashboard showing aggregate usage (top commands, OS distribution, provider breakdown, error rates). Links from the `/telemetry` docs page. This follows the Astro/Next.js model and completely defuses "what are you doing with this data?" concerns by letting anyone see the aggregate data themselves.

### Contributor Note

Dev builds (built without the PostHog API key via `make build`) have telemetry silently compiled out. Contributors running local builds are never sending data. This is documented in CONTRIBUTING.md.

## Error Handling

- PostHog API unreachable → silently drop event, no user-visible error
- `~/.syllago/telemetry.json` unreadable or unwritable → **telemetry disabled for this session**. Do not collect from users who cannot persist an opt-out. Warn on `telemetry on/off` that the config couldn't be saved.
- Home directory doesn't exist (containerized/CI) → telemetry disabled for session
- `DO_NOT_TRACK` set to any truthy value → telemetry disabled, `Init()` returns before any config read or client initialization
- PostHog API key missing from build → telemetry silently disabled (dev builds)

## Success Criteria

- [ ] `syllago telemetry status` shows accurate state
- [ ] `syllago telemetry off` stops all data collection immediately
- [ ] `DO_NOT_TRACK=1` prevents any network calls to PostHog
- [ ] `DO_NOT_TRACK=true` also prevents any network calls (truthy value support)
- [ ] First-run notice appears exactly once per machine (CLI to stderr, TUI as toast)
- [ ] First-run notice does not corrupt piped output (`syllago list | jq` works on first run)
- [ ] No data is sent before the first-run notice is displayed
- [ ] Zero measurable impact on CLI command latency
- [ ] All events match the documented payload schema
- [ ] `/etc/syllago/telemetry.json` with `enabled: false` disables telemetry for all users on the machine
- [ ] Unreadable/unwritable config defaults to disabled, not enabled
- [ ] Docs site has a complete `/telemetry` page (blocking for launch)
- [ ] `--json` works for all telemetry subcommands
- [ ] `syllago telemetry reset` output states that old data is not deleted
- [ ] PostHog project has "Discard client IP data" enabled
- [ ] Data retention documented as 1 year (PostHog Cloud free tier default)
- [ ] README mentions telemetry with link to docs page
- [ ] CONTRIBUTING.md notes that dev builds have telemetry compiled out

## Panel Review (2026-04-02)

Design reviewed by three personas: privacy-conscious OSS developer, enterprise platform engineering lead, experienced OSS maintainer. All panel findings have been incorporated:

- Broadened `DO_NOT_TRACK` to any truthy value
- Changed unreadable/unwritable config fallback to disabled
- Locked first-run notice wording (stderr, exact text)
- Documented anonymous ID as pseudonymous, not anonymous
- Added data deletion process
- Added system-level config for fleet management
- Made ingest endpoint configurable for self-hosted/enterprise
- Added PostHog compliance links requirement for docs page
- Added enterprise deployment section to docs requirements
- Specified concrete data retention (1 year, PostHog Cloud default)
- Added public dashboard as post-launch goal
- Added contributor note about dev builds

---

## Next Steps

Ready for implementation planning with `/develop`.
