# Telemetry

Syllago collects anonymous usage data to help prioritize development. This page explains exactly what is collected, why, and how to opt out.

## What we collect

Two event types, all fire-and-forget:

| Event | Fired when | Properties |
|-------|-----------|-----------|
| `command_executed` | `install`, `add`, or `convert` completes successfully | `command`, `provider`/`from`/`to_provider`, `content_type`, `content_count`, `success`, `dry_run` |
| `tui_session_started` | TUI exits normally | `success` |

Every event also includes these standard properties: `version`, `os`, `arch`.

### Per-command properties

**install:**

| Property | Type | Example | Description |
|----------|------|---------|-------------|
| `command` | string | `"install"` | Always `"install"` |
| `provider` | string | `"claude-code"` | Target provider slug |
| `content_type` | string | `"rules"` | Content type filter (empty = all) |
| `content_count` | int | `3` | Number of items installed |
| `success` | bool | `true` | Always true (only fires on success) |
| `dry_run` | bool | `false` | Whether `--dry-run` was used |

**add:**

| Property | Type | Example | Description |
|----------|------|---------|-------------|
| `command` | string | `"add"` | Always `"add"` |
| `from` | string | `"cursor"` | Source provider slug |
| `content_type` | string | `"hooks"` | Content type (empty = all/mixed) |
| `content_count` | int | `5` | Number of items added |
| `success` | bool | `true` | Always true (only fires on success) |
| `dry_run` | bool | `false` | Whether `--dry-run` was used |

**convert:**

| Property | Type | Example | Description |
|----------|------|---------|-------------|
| `command` | string | `"convert"` | Always `"convert"` |
| `from_provider` | string | `"cursor"` | Source provider (may be empty for library items) |
| `to_provider` | string | `"claude-code"` | Target provider slug |
| `content_type` | string | `"rules"` | Content type being converted |
| `success` | bool | `true` | Always true (only fires on success) |

**tui_session_started:**

| Property | Type | Example | Description |
|----------|------|---------|-------------|
| `success` | bool | `true` | TUI exited normally |

### Example payload

```json
{
  "api_key": "phc_...",
  "event": "command_executed",
  "distinct_id": "syl_a1b2c3d4e5f6",
  "properties": {
    "command": "install",
    "provider": "claude-code",
    "content_type": "rules",
    "content_count": 3,
    "success": true,
    "dry_run": false,
    "version": "0.7.0",
    "os": "linux",
    "arch": "amd64"
  }
}
```

## What we never collect

| Category | Examples | Collected? |
|----------|----------|-----------|
| File contents | Rule text, skill prompts, hook commands, MCP configs | Never |
| File paths | `/home/user/.claude/rules/my-secret-rule` | Never |
| User identity | Usernames, hostnames, IP addresses, email | Never |
| Registry URLs | Git clone URLs, registry names | Never |
| Content names | Names of rules, skills, agents you manage | Never |
| Interaction details | Keystrokes, mouse clicks, TUI navigation | Never |

## How to disable

**Per-user (command):**

```bash
syllago telemetry off
```

**Per-user (environment variable):**

```bash
export DO_NOT_TRACK=1
```

Accepts any truthy value: `1`, `true`, `yes`, `on` (case-insensitive).

**Fleet-wide (system config):**

Create `/etc/syllago/telemetry.json`:

```json
{
  "enabled": false
}
```

This overrides all user-level settings. Useful for enterprise deployments.

**Re-enable:**

```bash
syllago telemetry on
```

## Why we collect it

Syllago is a solo-developer project. Without usage data, development priorities are based on guesses. Telemetry answers concrete questions:

- Which providers should get the most attention?
- Which commands are actually used vs theoretical?
- What errors do users hit that they never report?
- Is the TUI or CLI the primary interface?

This data directly steers what gets built next.

## How it works

- Events are sent as HTTP POST requests to PostHog's ingest endpoint
- Each request has a 2-second timeout
- All network errors are silently dropped — telemetry never affects command execution
- No local event storage or retry queue — if the POST fails, the event is lost
- Events fire asynchronously after the command completes
- `Shutdown()` waits up to 2 seconds for in-flight events before the process exits

Dev builds (compiled without `SYLLAGO_POSTHOG_KEY`) have telemetry compiled out entirely — `Init()` returns immediately when the API key is empty.

## Data retention

- Events are retained for **1 year** (PostHog Cloud free tier default)
- After 1 year, events are automatically deleted
- No backups or secondary storage

## Data deletion

To request deletion of your data:

1. Run `syllago telemetry status` to see your anonymous ID
2. Email `privacy@syllago.dev` with the ID(s) you want deleted
3. Deletion is processed within 30 days

After deletion, rotate your ID to prevent future correlation:

```bash
syllago telemetry reset
```

Note: `reset` generates a new ID but does not delete previously collected data — you must request deletion separately.

## PostHog compliance

Syllago uses [PostHog Cloud](https://posthog.com) (US region) for event ingestion and storage.

- **SOC 2 Type II certified** — [PostHog security](https://posthog.com/handbook/company/security)
- **GDPR compliant** — [PostHog DPA](https://posthog.com/privacy)
- **IP stripping enabled** — "Discard client IP data" is enabled on the syllago PostHog project. PostHog does not store IP addresses for syllago events.
- **Data residency** — US-hosted (PostHog Cloud US region)
- **Privacy documentation** — [posthog.com/docs/privacy](https://posthog.com/docs/privacy)

## Enterprise deployment

**Fleet-wide opt-out:**

Deploy `/etc/syllago/telemetry.json` with `{"enabled": false}` to all machines. This takes precedence over user-level settings.

**Self-hosted PostHog endpoint:**

Users can configure a custom endpoint in `~/.syllago/telemetry.json`:

```json
{
  "enabled": true,
  "endpoint": "https://posthog.internal.company.com/capture/"
}
```

This routes events to your own PostHog instance instead of PostHog Cloud.

**CI/CD environments:**

Set `DO_NOT_TRACK=1` in your CI environment to suppress telemetry and the first-run notice.

**Verification at scale:**

```bash
syllago telemetry status --json
```

Returns machine-readable status for fleet management scripts:

```json
{
  "enabled": false,
  "anonymousId": "syl_a1b2c3d4e5f6",
  "endpoint": "https://us.i.posthog.com/capture/"
}
```
