# Syllago Sandbox Wrapper Design

*Design date: 2026-02-25*
*Status: Draft (stress-tested)*

## Overview

`syllago sandbox run <provider>` wraps AI CLI tools in bubblewrap sandboxes, restricting filesystem access, network egress, and environment variables. The sandbox prevents a compromised or misbehaving AI tool from reading secrets, exfiltrating data, or causing accidental damage outside the project directory.

**Platform:** Linux-only for v1 (bubblewrap requirement). macOS support on post-v1 roadmap.

**Dependencies:** bubblewrap (bwrap) >= 0.4.0, socat >= 1.7.0

---

## Threat Model

**What the sandbox protects against:**

1. **Compromised AI tool** reading SSH keys, credentials, env files, other projects
2. **Malicious registry content** (MCP servers, hooks) reaching beyond the project
3. **Data exfiltration** to arbitrary network endpoints
4. **Accidental damage** from AI tool mistakes (rm -rf wrong directory, overwriting configs)

**What the sandbox does NOT protect against:**

- User choosing to run the AI tool without the sandbox
- Social engineering the user to weaken sandbox settings
- Kernel exploits (bubblewrap is user-namespace isolation, not VM-level)
- The AI tool's own API being used as an exfiltration channel (provider API is always allowed)

**Security model:** The sandbox makes violations *structurally impossible* rather than relying on the AI tool's cooperation. Network isolation is enforced via kernel namespaces, not advisory proxy settings. Filesystem restrictions are enforced via bind mounts, not permission checks.

---

## Architecture

```
Host (unsandboxed)
 │
 ├── syllago sandbox run claude
 │   ├── Validates directory safety
 │   ├── Copies provider config files to sandbox staging
 │   ├── Starts egress proxy (listens on UNIX socket)
 │   ├── Constructs bubblewrap arguments
 │   └── Launches bubblewrap ──────────────────────┐
 │                                                  │
 │   ┌──────────────────────────────────────────────▼──┐
 │   │  Bubblewrap Sandbox                             │
 │   │                                                 │
 │   │  socat (bridges UNIX socket → TCP localhost)    │
 │   │                                                 │
 │   │  claude (sandboxed)                             │
 │   │    ├── Sees: project dir (RW)                   │
 │   │    ├── Sees: config copy (RW)                   │
 │   │    ├── Sees: /usr, /lib (RO)                    │
 │   │    ├── Sees: git wrapper (blocks push/fetch)    │
 │   │    ├── Network: --unshare-net (namespace)       │
 │   │    ├── Egress: HTTPS_PROXY → socat → proxy      │
 │   │    └── Env: allowlisted vars only               │
 │   │                                                 │
 │   └─────────────────────────────────────────────────┘
 │
 └── On sandbox exit:
     ├── Diff provider config (staged copy vs original)
     ├── Show changes for user approval
     ├── Merge approved changes back
     └── Show session summary (blocked domains, etc.)
```

---

## CLI Interface

### Basic Usage

```bash
syllago sandbox run claude
syllago sandbox run gemini-cli
syllago sandbox run codex
```

### Override Flags

```bash
syllago sandbox run claude --allow-domain cdn.example.com
syllago sandbox run claude --allow-port 5432          # local service
syllago sandbox run claude --allow-env DATABASE_URL
syllago sandbox run claude --mount-ro ~/shared-data
syllago sandbox run claude --force-dir                # skip directory safety
syllago sandbox run claude --no-network               # block ALL egress
```

### Info and Diagnostics

```bash
syllago sandbox check              # verify bubblewrap, socat, namespaces
syllago sandbox check claude       # show Claude's mount profile + resolved paths
syllago sandbox info               # show current sandbox config
syllago sandbox info claude        # show effective settings for Claude
```

### Configuration Management

```bash
syllago sandbox allow-domain cdn.example.com
syllago sandbox deny-domain cdn.example.com
syllago sandbox allow-env DATABASE_URL
syllago sandbox deny-env DATABASE_URL
syllago sandbox allow-port 5432
syllago sandbox deny-port 5432
syllago sandbox domains            # list domain allowlist
syllago sandbox env                # list env var allowlist
syllago sandbox ports              # list port allowlist
```

### TUI Integration

Full sandbox management under the config sidebar:
- Domain allowlist management
- Env var allowlist management
- Port allowlist management
- Provider mount profile display
- Launch sandbox from provider view

---

## Filesystem Isolation

### Base Mounts (All Providers)

```
Read-only (system):
  /usr                    # System binaries, libraries
  /lib                    # Shared libraries (glibc, etc.)
  /lib64                  # 64-bit loader
  /bin → symlink to usr/bin
  /sbin → symlink to usr/sbin
  /etc/resolv.conf        # DNS resolution
  /etc/hosts              # Hostname resolution
  /etc/ssl                # TLS certificates
  /etc/ca-certificates    # CA bundle
  /etc/nsswitch.conf      # Name service switch (glibc DNS)

Read-write:
  <project-dir>           # The current project
  /tmp (private tmpfs)    # Sandbox-local temp directory

Minimal devices:
  /dev (bwrap --dev)      # null, zero, urandom, tty only

Hidden (not mounted):
  $HOME (except project + config copies)
  ~/.ssh/
  ~/.aws/, ~/.config/gcloud/, ~/.azure/
  ~/.gnupg/
  Other projects
  /var, /opt, /srv
```

### Provider Config Handling (Copy-Diff-Approve)

Provider config files are **copied** into a sandbox staging area on entry. The sandbox operates on the copies. On exit, syllago diffs the copies against the originals and presents changes for user approval.

**Why:** Config files like `~/.claude.json` define MCP servers and hooks that execute outside the sandbox. A compromised tool could inject a malicious MCP server that runs with full privileges on the next normal session. The copy-diff-approve pattern prevents this.

**On sandbox entry:**
1. Copy provider config files to `/tmp/syllago-sandbox-<id>/config/`
2. Mount the copies at the original paths inside the sandbox
3. Store hashes of the originals

**On sandbox exit:**
1. Compare copies against originals
2. If unchanged: clean up silently
3. If changed: show diff, categorize changes:
   - **High risk:** New MCP servers, new hooks, modified commands → require explicit approval
   - **Low risk:** Setting changes, preference updates → show diff, auto-approve with notification
4. User approves/rejects each change category
5. Approved changes are merged back to originals

### Provider Mount Profiles

Each provider has a curated mount profile defining which config files to copy-stage:

**Claude Code:**
- `~/.claude.json` (MCP servers, settings)
- `~/.claude/` (settings, agents, skills)
- `.claude/` (project config)

**Gemini CLI:**
- `~/.gemini/settings.json` (MCP servers, hooks)
- `.gemini/` (project config)

**Codex:**
- `~/.codex/` (settings, config)
- `.codex/` (project config)

**Copilot CLI:**
- `~/.config/github-copilot/` (settings)

**Cursor:**
- `~/.cursor/` (settings)

### Ecosystem Cache Mounts (Auto-Detected)

Based on project markers, mount relevant package caches **read-only** so cached dependencies work without network:

| Project Marker | Cache Path | Mount |
|---------------|------------|-------|
| `package.json` | `~/.npm/`, `~/.cache/npm/` | RO |
| `go.mod` | `~/go/pkg/mod/`, `~/.cache/go-build/` | RO |
| `Cargo.toml` | `~/.cargo/registry/`, `~/.cargo/git/` | RO |
| `pyproject.toml`, `requirements.txt` | `~/.cache/pip/` | RO |
| `pnpm-lock.yaml` | `~/.local/share/pnpm/store/` | RO |
| `bun.lockb` | `~/.bun/install/cache/` | RO |

**Trade-off:** First-time installs (nothing cached) will fail or be slow since writes to the cache are blocked. This is documented and expected — the sandbox prioritizes security over convenience for uncached packages. Users can run `npm install` outside the sandbox first, then enter the sandbox with warm caches.

---

## Network Isolation

### Enforcement: Kernel Namespace (Not Advisory)

Network isolation uses `--unshare-net` to create a new network namespace. Inside it, the process has **no network access** except through the proxy bridge. This is enforced by the kernel — no process can bypass it regardless of proxy environment variables.

**Proxy bridge architecture:**
1. Host: Egress proxy listens on UNIX domain socket (`/tmp/syllago-sandbox-<id>/proxy.sock`)
2. bwrap: Socket bind-mounted into sandbox
3. Sandbox: socat bridges UNIX socket → TCP localhost:3128
4. Sandbox: `HTTPS_PROXY=http://127.0.0.1:3128` and `HTTP_PROXY=http://127.0.0.1:3128`

### Domain Allowlist

**Per-provider defaults (auto-included):**

| Provider | Allowed Domains |
|----------|----------------|
| Claude Code | `api.anthropic.com`, `sentry.io` |
| Gemini CLI | `generativelanguage.googleapis.com` |
| Codex | `api.openai.com` |
| Copilot CLI | `api.githubcopilot.com` |
| Cursor | `api2.cursor.sh` |

**Per-ecosystem defaults (auto-detected from project markers):**

| Ecosystem | Domains |
|-----------|---------|
| npm | `registry.npmjs.org`, `*.npmjs.org`, `objects.githubusercontent.com` |
| Go | `proxy.golang.org`, `sum.golang.org`, `storage.googleapis.com` |
| Rust | `crates.io`, `static.crates.io` |
| Python | `pypi.org`, `files.pythonhosted.org` |

**Registry-derived (prompted on `syllago registry add`):**

```
$ syllago registry add https://github.com/team/ai-rules
  Security: Syllago does not verify registry content...
  Sandbox: Add github.com to sandbox network allowlist? [Y/n]
```

Note: Registry domains are only for registry sync operations outside the sandbox. Adding github.com to the sandbox allowlist is optional and has security implications (enables git push — see Git Isolation section).

**User-configured (persistent):**

```json
// .syllago/config.json
{
  "sandbox": {
    "allowed_domains": ["internal-api.company.com"]
  }
}
```

**Session overrides (non-persistent):**

```bash
syllago sandbox run claude --allow-domain cdn.example.com
```

**Hot-add during session:**

When the proxy blocks a domain, it prints:
```
[sandbox] Blocked connection to cdn.example.com (not in allowlist)
[sandbox] To allow: syllago sandbox allow-domain cdn.example.com
```

### Localhost Port Allowlist

By default, **only the proxy's own port** is reachable on localhost inside the sandbox. This prevents access to local databases, Docker, Kubernetes API, etc.

Users can explicitly allow specific ports:

```bash
syllago sandbox run claude --allow-port 5432    # PostgreSQL
syllago sandbox run claude --allow-port 6379    # Redis
```

Or in config:
```json
{
  "sandbox": {
    "allowed_ports": [5432, 6379]
  }
}
```

---

## Git Isolation

### Git Wrapper (Subcommand Allowlist)

The sandbox mounts a **git wrapper script** at a higher PATH priority than the real git binary. The wrapper allowlists safe subcommands and blocks network operations:

**Allowed (local operations):**
- `git log`, `git blame`, `git diff`, `git show`, `git status`
- `git add`, `git commit`, `git reset`, `git stash`
- `git branch`, `git checkout`, `git switch`, `git merge`, `git rebase`
- `git config` (read-only — blocks `--global` writes)
- `git rev-parse`, `git describe`, `git tag` (read-only)

**Blocked (network operations):**
- `git push`, `git fetch`, `git pull`
- `git clone`, `git submodule update --remote`
- `git remote` (all subcommands)
- `git ls-remote`

**Why a wrapper instead of relying on network isolation:** Even with the egress proxy, `git push` over HTTPS could reach any allowlisted domain. If `github.com` is on the allowlist (common for npm packages), push works. The wrapper blocks it at the command level regardless of network policy.

### Git Credential Isolation

Inside the sandbox:
- `GIT_CONFIG_NOSYSTEM=1` (ignore system gitconfig)
- `GIT_CONFIG_GLOBAL=/tmp/syllago-sandbox-<id>/gitconfig` (minimal: user.name, user.email only)
- `GIT_TERMINAL_PROMPT=0` (no credential prompts)
- `~/.git-credentials` not mounted
- Credential helper sockets not mounted

This prevents extraction of stored git credentials while preserving local commit functionality.

---

## Environment Variable Handling

### Allowlist Model

Only explicitly allowed environment variables are forwarded into the sandbox. Everything else is stripped.

**Always allowed:**
```
HOME, USER, SHELL, TERM, LANG, LC_ALL, LC_CTYPE
XDG_RUNTIME_DIR, XDG_DATA_HOME, XDG_CONFIG_HOME, XDG_CACHE_HOME
COLORTERM, TERM_PROGRAM, FORCE_COLOR, NO_COLOR
EDITOR, VISUAL
TZ
```

**Set by sandbox:**
```
PATH=<curated — sandbox binaries, /usr/bin, /usr/local/bin>
HTTP_PROXY=http://127.0.0.1:3128
HTTPS_PROXY=http://127.0.0.1:3128
NO_PROXY=
GIT_CONFIG_NOSYSTEM=1
GIT_CONFIG_GLOBAL=/tmp/syllago-sandbox-<id>/gitconfig
GIT_TERMINAL_PROMPT=0
```

**Provider-specific (auto-included):**
- Claude Code: whatever auth mechanism it uses (typically not env-var based)
- Gemini CLI: `GOOGLE_API_KEY` or `GEMINI_API_KEY` if set
- Codex: `OPENAI_API_KEY` if set

**User-configurable allowlist:**
```json
// .syllago/config.json
{
  "sandbox": {
    "allowed_env": ["DATABASE_URL", "REDIS_URL", "GOPATH"]
  }
}
```

Or per-session:
```bash
syllago sandbox run claude --allow-env DATABASE_URL
```

### Transparency

On sandbox start, syllago reports:
```
Sandbox environment:
  Forwarded: HOME, USER, SHELL, TERM, LANG, ANTHROPIC_API_KEY
  Stripped: 12 env vars (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, SSH_AUTH_SOCK, ...)
  To allow a stripped var: syllago sandbox allow-env <VAR>
```

---

## Directory Safety Validation

### Requirements (All Must Pass)

1. **Depth check:** CWD must be at least 2 directory levels deep from `$HOME`
   - `~/projects/syllago` — passes (2 levels)
   - `~/projects` — fails (1 level)
   - `~` — fails (0 levels)

2. **Sensitive path blocklist:** Reject these explicitly:
   - `/`, `/tmp`, `/etc`, `/var`, `/opt`
   - `$HOME` itself
   - `$HOME/.ssh`, `$HOME/.config`, `$HOME/.gnupg`, `$HOME/.aws`

3. **Project marker requirement:** CWD must contain at least one of:
   - `.git` (must be a directory, not a file)
   - `.syllago`
   - `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`
   - `Makefile`, `CMakeLists.txt`
   - `.project-root` (zero-byte escape hatch)

4. **Symlink resolution:** Resolve all symlinks in CWD **before** checking depth and blocklist. Prevents `~/code/project → /` bypass.

### Override

`--force-dir` bypasses validation with a warning:
```
WARNING: Directory safety checks skipped. The entire directory
/home/user/large-dir will be writable inside the sandbox.
```

---

## Bubblewrap Configuration

### Base Arguments

```
bwrap \
  --new-session \
  --die-with-parent \
  --unshare-net \
  --unshare-pid \
  --unshare-ipc \
  --cap-drop ALL \
  --dev /dev \
  --proc /proc \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --symlink usr/bin /bin \
  --symlink usr/sbin /sbin \
  --ro-bind /etc/resolv.conf /etc/resolv.conf \
  --ro-bind /etc/hosts /etc/hosts \
  --ro-bind /etc/ssl /etc/ssl \
  --ro-bind /etc/ca-certificates /etc/ca-certificates \
  --ro-bind /etc/nsswitch.conf /etc/nsswitch.conf \
  --tmpfs /tmp \
  --bind <project-dir> <project-dir> \
  --bind <proxy-socket> <proxy-socket> \
  --ro-bind <git-wrapper> /usr/local/bin/git \
  <per-provider config mounts> \
  <per-ecosystem cache mounts> \
  --setenv HOME <home> \
  --setenv PATH /usr/local/bin:/usr/bin:/bin \
  --setenv HTTPS_PROXY http://127.0.0.1:3128 \
  --setenv HTTP_PROXY http://127.0.0.1:3128 \
  -- <wrapper-script>
```

### Namespace Isolation

- `--unshare-net`: Network namespace (enforced egress proxy)
- `--unshare-pid`: PID namespace (can't see/signal host processes)
- `--unshare-ipc`: IPC namespace (can't access host shared memory)
- `--cap-drop ALL`: Drop all Linux capabilities
- `--new-session`: New terminal session (prevents /dev/tty attacks)
- `--die-with-parent`: Kill sandbox if syllago exits

### Wrapper Script

The bubblewrap command runs a wrapper script that:
1. Starts socat to bridge UNIX socket → TCP localhost:3128
2. Execs the provider binary

```bash
#!/bin/sh
# Bridge proxy socket to TCP
socat TCP-LISTEN:3128,fork,reuseaddr \
  UNIX-CONNECT:/tmp/syllago-sandbox-<id>/proxy.sock &
SOCAT_PID=$!

# Exec the provider
exec "$@"
```

PID namespace cleanup handles socat automatically — when the provider exits (PID 1 in namespace), all processes in the namespace are killed.

---

## Provider Binary Resolution

### Strategy

Each provider has a `ResolveBinary` function that:
1. Uses `which` / `exec.LookPath` to find the binary
2. Follows symlinks to the real path (`filepath.EvalSymlinks`)
3. Determines runtime dependencies (Node.js for Gemini, self-contained for Claude)
4. Returns the list of paths to mount

### Provider-Specific Handling

**Claude Code:** Self-contained Deno-compiled binary (~227MB). Only needs the binary itself + libc. Mount the resolved binary path read-only.

**Gemini CLI:** Node.js script via mise/nvm/fnm. Needs the entire Node.js runtime tree + the gemini-cli package's node_modules (~517MB). Detect the runtime manager, mount its install directory read-only.

**Codex, Copilot CLI:** TBD — resolve at implementation time based on installed versions.

### Validation Command

```bash
$ syllago sandbox check claude
  Binary: /home/user/.local/bin/claude (self-contained, 227MB)
  Runtime: none (Deno-compiled)
  Mounts needed: 1 path
  Config files: ~/.claude.json, ~/.claude/, .claude/
  Status: Ready for sandboxing
```

---

## Session Lifecycle

### Startup

1. Validate directory safety (depth, markers, symlinks)
2. Validate prerequisites (`bwrap`, `socat`, provider binary)
3. Copy provider config files to staging area
4. Detect project ecosystem (package.json → npm, go.mod → Go, etc.)
5. Build domain allowlist (provider + ecosystem + user config)
6. Build env var allowlist
7. Start egress proxy on UNIX socket
8. Construct bubblewrap arguments
9. Print sandbox summary (forwarded env vars, stripped vars, domain allowlist)
10. Launch bubblewrap

### During Session

- Proxy logs blocked domains to stderr: `[sandbox] Blocked: evil.com`
- `syllago sandbox allow-domain <domain>` hot-adds to session (if available — may require out-of-sandbox command)

### Shutdown (Normal Exit)

1. Provider process exits → PID namespace kills socat
2. bubblewrap exits → `--die-with-parent` is satisfied
3. syllago detects exit, stops proxy
4. Diff staged config copies against originals
5. If changes detected:
   - Categorize: high-risk (MCP/hooks) vs low-risk (settings)
   - Show diff to user
   - Prompt for approval per category
   - Merge approved changes
6. Print session summary:
   ```
   Sandbox session ended.
     Duration: 45m
     Blocked domains: cdn.example.com (3x), api.stripe.com (1x)
     Config changes: 2 approved, 1 rejected
     To allow blocked domains: syllago sandbox allow-domain cdn.example.com
   ```
7. Clean up staging area and socket files

### Crash Recovery

- `--die-with-parent` kills sandbox if syllago crashes
- Proxy runs as goroutine in syllago process — dies with syllago
- On next `syllago sandbox run`: check for stale staging areas, clean up
- Socket file cleanup via defer + signal handler

---

## Configuration Schema

```json
// .syllago/config.json additions
{
  "sandbox": {
    "allowed_domains": [
      "internal-api.company.com"
    ],
    "allowed_env": [
      "DATABASE_URL",
      "REDIS_URL"
    ],
    "allowed_ports": [
      5432,
      6379
    ]
  }
}
```

This config is project-level and git-tracked, so teams share the same sandbox settings.

---

## Stress Test Findings (Incorporated)

The following issues were identified during adversarial review and are addressed in this design:

| ID | Severity | Finding | Mitigation |
|----|----------|---------|------------|
| SAND-001 | Critical | Provider config RW enables deferred code execution via MCP/hook injection | Copy-diff-approve pattern |
| SAND-002 | Critical | Git push works through HTTPS egress proxy | Git subcommand allowlist wrapper |
| SAND-003 | High | Egress allowlist poisoning via malicious registry content | Registry domains require explicit approval |
| SAND-004 | High | localhost blanket-allow exposes local services | Port-specific allowlist, default deny |
| SAND-005 | High | Network isolation must be mandatory via --unshare-net | UNIX socket bridge architecture |
| SAND-006 | High | Symlinks in project dir can escape mounts | Resolved by /usr RO + limited writable paths |
| SAND-007 | High | Env var denylist is fragile | Inverted to allowlist model |
| SAND-009 | Medium | Directory safety bypassed via symlinks | filepath.EvalSymlinks before all checks |
| SAND-010 | Medium | Git credential helpers accessible | GIT_CONFIG overrides + no credential mount |
| SAND-011 | Medium | No PID namespace isolation | --unshare-pid added |
| SAND-012 | Medium | No IPC namespace isolation | --unshare-ipc added |
| SAND-013 | Medium | Bubblewrap is not VM-level isolation | Documented in threat model |
| SAND-014 | Medium | Social engineering to weaken allowlist | Session summary shows effective config |
| SAND-015 | Low | Stale state on crash | Cleanup on next run + signal handlers |
| SAND-016 | Low | /dev access | bwrap --dev (minimal devices) |
| SAND-017 | Low | Shared /tmp | Private tmpfs mount |
| SAND-018 | Low | .syllago/config.json writable | Acceptable risk — validated on exit |

---

## Implementation Estimate

| Package | Purpose | ~Lines |
|---------|---------|--------|
| `internal/sandbox/bwrap.go` | Bubblewrap argument construction | 300 |
| `internal/sandbox/proxy.go` | HTTP CONNECT proxy with domain allowlist | 200 |
| `internal/sandbox/bridge.go` | socat bridge setup, UNIX socket lifecycle | 150 |
| `internal/sandbox/runner.go` | Orchestrator: proxy → bridge → bwrap → cleanup | 200 |
| `internal/sandbox/profile.go` | Per-provider mount profiles | 150 |
| `internal/sandbox/envfilter.go` | Env var allowlist + reporting | 80 |
| `internal/sandbox/dirsafety.go` | CWD validation + symlink resolution | 100 |
| `internal/sandbox/configdiff.go` | Config copy-diff-approve on exit | 150 |
| `internal/sandbox/gitwrapper.go` | Git subcommand allowlist wrapper generation | 80 |
| `internal/sandbox/check.go` | Pre-flight validation | 100 |
| `commands/sandbox_cmd.go` | CLI subcommands | 200 |
| `internal/tui/sandbox_settings.go` | TUI config management | 300 |
| Tests | Unit + integration | 800 |
| **Total** | | **~2,810** |

---

## Explicitly NOT in V1

| Feature | Reason |
|---------|--------|
| macOS support | No bubblewrap on macOS. Post-v1 via sandbox-exec or containers. |
| SOCKS5 proxy | HTTP CONNECT covers most cases. Add if git/other tools need it. |
| Seccomp BPF filters | Additional hardening, but complex. Add post-v1. |
| Syllago binary inside sandbox | Install content before the session, not during. |
| Sandbox enforcement for teams | v1 is opt-in. Team enforcement (wrapper scripts, audit logs) post-v1. |
| Per-file write permissions | Project dir is all-or-nothing RW. Fine-grained scoping post-v1. |
| Container backend | Bubblewrap only for v1. Docker/OCI backend post-v1 for macOS. |
