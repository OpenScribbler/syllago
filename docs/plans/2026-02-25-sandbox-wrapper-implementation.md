# Sandbox Wrapper — Implementation Plan

**Goal:** Wrap AI CLI tools in bubblewrap sandboxes to restrict filesystem access, network egress, and environment variables, preventing a compromised or misbehaving AI tool from reading secrets, exfiltrating data, or causing accidental damage outside the project directory.

**Architecture:** `syllago sandbox run <provider>` validates the CWD → copies provider config files to a staging area → starts an HTTP CONNECT egress proxy on a UNIX socket → constructs bubblewrap arguments with `--unshare-net` + a socat bridge → launches the sandboxed provider → on exit diffs and approves any config changes. A git wrapper script is mounted at a higher PATH priority to block `push`/`fetch`/`clone` regardless of network policy.

**Tech stack:** Go, Cobra (CLI), Bubbletea/Lipgloss (TUI settings screen), `os/exec` (bwrap/socat shell-out), standard `net/http` (CONNECT proxy), `crypto/sha256` (config hashing). No new Go dependencies.

**Platform:** Linux only (bubblewrap requirement). All sandbox code is in `cli/internal/sandbox/`. CLI commands are in `cli/cmd/syllago/sandbox_cmd.go`. TUI settings screen is `cli/internal/tui/sandbox_settings.go`.

**Design doc:** `docs/plans/2026-02-25-sandbox-wrapper-design.md`

**Build:** `make build` | **Test:** `make test`

---

## Phase 1 — Foundation: Safety, Env Filter, Config Schema

---

### Task 1.1 — Directory safety validation

**Creates:** `cli/internal/sandbox/dirsafety.go`, `cli/internal/sandbox/dirsafety_test.go`

**Dependencies:** None

This is the first gate before any sandbox operation. All four rules must pass: symlink resolution prevents `~/code/proj → /` bypass; depth check prevents mounting `$HOME` directly; blocklist catches obvious mistakes; project marker ensures we're actually in a project. The `--force-dir` flag bypasses this with a warning for power users.

```go
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sensitiveBlocklist contains paths that must never be used as the sandbox project root.
var sensitiveBlocklist = []string{
	"/",
	"/tmp",
	"/etc",
	"/var",
	"/opt",
}

// projectMarkers are files/dirs that indicate a valid project root.
var projectMarkers = []string{
	".git",
	".syllago",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"Makefile",
	"CMakeLists.txt",
	".project-root",
}

// DirSafetyError is returned when the CWD fails a safety check.
type DirSafetyError struct {
	Reason string
}

func (e *DirSafetyError) Error() string {
	return fmt.Sprintf("directory safety check failed: %s", e.Reason)
}

// ValidateDir checks whether dir is safe to use as a sandbox project root.
// Pass forceDir=true to skip validation (--force-dir flag).
func ValidateDir(dir string, forceDir bool) error {
	if forceDir {
		return nil
	}

	// Resolve symlinks first — prevents ~/code/proj → / bypass.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return &DirSafetyError{Reason: fmt.Sprintf("cannot resolve path: %s", err)}
	}
	dir = resolved

	// Get $HOME for relative checks.
	home, err := os.UserHomeDir()
	if err != nil {
		return &DirSafetyError{Reason: "cannot determine home directory"}
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return &DirSafetyError{Reason: "cannot resolve home directory"}
	}

	// Rule 1: block sensitive explicit paths (including $HOME itself).
	blocked := append(sensitiveBlocklist, home,
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".config"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".aws"),
	)
	for _, b := range blocked {
		if dir == b {
			return &DirSafetyError{Reason: fmt.Sprintf("path %q is explicitly blocked for sandbox use", dir)}
		}
	}

	// Rule 2: depth check — must be at least 2 levels below $HOME.
	rel, err := filepath.Rel(home, dir)
	if err != nil {
		return &DirSafetyError{Reason: "cannot compute path depth relative to home"}
	}
	// rel = "projects/syllago" → depth 2 (OK), "projects" → depth 1 (fail)
	if !strings.Contains(rel, string(filepath.Separator)) {
		return &DirSafetyError{Reason: fmt.Sprintf("directory must be at least 2 levels below $HOME (e.g. ~/projects/syllago). Got: %s", rel)}
	}

	// Rule 3: project marker.
	if !hasProjectMarker(dir) {
		return &DirSafetyError{Reason: "no project marker found (.git, go.mod, package.json, etc.). Use --force-dir to override"}
	}

	return nil
}

// hasProjectMarker returns true if any project marker exists in dir.
func hasProjectMarker(dir string) bool {
	for _, m := range projectMarkers {
		target := filepath.Join(dir, m)
		if m == ".git" {
			// Must be a directory, not a file (gitdir worktrees use a file).
			info, err := os.Stat(target)
			if err == nil && info.IsDir() {
				return true
			}
			continue
		}
		if _, err := os.Stat(target); err == nil {
			return true
		}
	}
	return false
}
```

**Tests to write first (`dirsafety_test.go`):**
```go
// TestValidateDir_BlocksSensitivePaths: "/" and "$HOME" return DirSafetyError
// TestValidateDir_DepthCheck: "~/projects" (1 level) fails; "~/projects/myapp" (2 levels) passes
// TestValidateDir_RequiresMarker: temp dir with no markers fails; adding go.mod makes it pass
// TestValidateDir_SymlinkResolution: symlink to / fails even if the link itself is in ~/projects/
// TestValidateDir_ForceDir: forceDir=true always returns nil
```

**Success criteria:**
- [ ] All five test cases pass
- [ ] `make test` passes

---

### Task 1.2 — Environment variable filter

**Creates:** `cli/internal/sandbox/envfilter.go`, `cli/internal/sandbox/envfilter_test.go`

**Dependencies:** None

Allowlist model: only explicitly named vars pass through. Provider-specific vars (API keys) are added by the profile layer. The `Report` return value is used by the runner to print the transparency summary on sandbox start.

```go
package sandbox

import (
	"os"
	"strings"
)

// EnvReport describes which variables were forwarded vs stripped.
type EnvReport struct {
	Forwarded []string // variable names that will be passed into the sandbox
	Stripped  []string // variable names that were present but removed
}

// baseAllowlist is always forwarded regardless of provider or user config.
var baseAllowlist = []string{
	"HOME", "USER", "SHELL", "TERM", "LANG", "LC_ALL", "LC_CTYPE",
	"XDG_RUNTIME_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME",
	"COLORTERM", "TERM_PROGRAM", "FORCE_COLOR", "NO_COLOR",
	"EDITOR", "VISUAL",
	"TZ",
}

// FilterEnv returns (allowedPairs, report).
// allowedPairs is a slice of "KEY=VALUE" strings for the sandbox environment.
// extra is a list of additional variable names to allow (provider-specific + user config).
func FilterEnv(environ []string, extra []string) ([]string, EnvReport) {
	allowed := make(map[string]bool)
	for _, k := range baseAllowlist {
		allowed[k] = true
	}
	for _, k := range extra {
		allowed[k] = true
	}

	var pairs []string
	var report EnvReport

	present := make(map[string]string)
	for _, e := range environ {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		k, v := e[:idx], e[idx+1:]
		present[k] = v
	}

	for k, v := range present {
		if allowed[k] {
			pairs = append(pairs, k+"="+v)
			report.Forwarded = append(report.Forwarded, k)
		} else {
			report.Stripped = append(report.Stripped, k)
		}
	}

	return pairs, report
}
```

**Tests to write first (`envfilter_test.go`):**
```go
// TestFilterEnv_BaseAllowlist: HOME/USER/TERM pass through without explicit extras
// TestFilterEnv_StripsSecrets: AWS_ACCESS_KEY_ID is in Stripped
// TestFilterEnv_ExtraAllowlist: DATABASE_URL passes when included in extra
// TestFilterEnv_EmptyEnviron: no panic, returns empty pairs
// TestFilterEnv_ReportAccuracy: Forwarded + Stripped covers every input variable name
```

**Success criteria:**
- [ ] All five tests pass
- [ ] `make test` passes

---

### Task 1.3 — Extend config schema with sandbox settings

**Modifies:** `cli/internal/config/config.go`

**Dependencies:** None

The `SandboxConfig` struct is project-level and git-tracked so teams share sandbox policy. The `omitempty` on `Sandbox` preserves backward compatibility — existing configs without the key load cleanly.

```go
// SandboxConfig holds project-level sandbox policy.
type SandboxConfig struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	AllowedEnv     []string `json:"allowed_env,omitempty"`
	AllowedPorts   []int    `json:"allowed_ports,omitempty"`
}

type Config struct {
	Providers   []string          `json:"providers"`
	Registries  []Registry        `json:"registries,omitempty"`
	Preferences map[string]string `json:"preferences,omitempty"`
	Sandbox     SandboxConfig     `json:"sandbox,omitempty"`
}
```

**Success criteria:**
- [ ] `config.Load()` on a config without `sandbox` key returns zero-value `SandboxConfig` (not error)
- [ ] `config.Save()` round-trips `AllowedDomains`, `AllowedEnv`, `AllowedPorts` correctly
- [ ] Existing config tests still pass
- [ ] `make build` passes

---

## Phase 2 — Network Layer: Proxy and Socket Bridge

---

### Task 2.1 — HTTP CONNECT egress proxy

**Creates:** `cli/internal/sandbox/proxy.go`, `cli/internal/sandbox/proxy_test.go`

**Dependencies:** Task 1.2 (EnvReport types exist; proxy is independent of env filter but shares the package)

The proxy listens on a UNIX domain socket. When the sandbox's socat bridge sends an HTTP CONNECT request, the proxy checks the target host against the allowlist. If allowed, it dials the real TCP connection and tunnels bytes. If denied, it returns HTTP 403 and logs to `blockedLog`. No goroutine leak: `Shutdown()` closes the listener which unblocks all `Accept()` calls.

```go
package sandbox

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Proxy is an HTTP CONNECT proxy with a domain allowlist.
// It listens on a UNIX socket and tunnels allowed connections.
type Proxy struct {
	socketPath     string
	allowedDomains map[string]bool // host → allowed (no port)
	allowedPorts   map[int]bool    // localhost ports allowed
	blockedLog     []string        // blocked domain names (for session summary)
	mu             sync.Mutex
	listener       net.Listener
}

// NewProxy creates a Proxy. socketPath must not exist yet.
func NewProxy(socketPath string, allowedDomains []string, allowedPorts []int) *Proxy {
	dm := make(map[string]bool)
	for _, d := range allowedDomains {
		dm[d] = true
	}
	pm := make(map[int]bool)
	for _, p := range allowedPorts {
		pm[p] = true
	}
	return &Proxy{
		socketPath:     socketPath,
		allowedDomains: dm,
		allowedPorts:   pm,
	}
}

// Start begins accepting connections. Returns an error if the socket cannot be created.
// Runs the accept loop in a new goroutine; returns immediately.
func (p *Proxy) Start() error {
	ln, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}
	p.listener = ln
	go p.accept(ln)
	return nil
}

// Shutdown closes the listener, causing the accept loop to exit.
func (p *Proxy) Shutdown() {
	if p.listener != nil {
		p.listener.Close()
	}
}

// BlockedDomains returns the list of domains that were blocked during the session.
func (p *Proxy) BlockedDomains() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.blockedLog))
	copy(out, p.blockedLog)
	return out
}

func (p *Proxy) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		go p.handleConn(conn)
	}
}

func (p *Proxy) handleConn(client net.Conn) {
	defer client.Close()
	br := bufio.NewReader(client)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		fmt.Fprintf(client, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}

	if !p.isAllowed(host) {
		p.mu.Lock()
		p.blockedLog = append(p.blockedLog, host)
		p.mu.Unlock()
		log.Printf("[sandbox] Blocked connection to %s (not in allowlist)", host)
		fmt.Fprintf(client, "HTTP/1.1 403 Forbidden\r\n\r\n")
		return
	}

	upstream, err := net.Dial("tcp", req.Host)
	if err != nil {
		fmt.Fprintf(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer upstream.Close()

	fmt.Fprintf(client, "HTTP/1.1 200 Connection Established\r\n\r\n")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(upstream, br) }()
	go func() { defer wg.Done(); io.Copy(client, upstream) }()
	wg.Wait()
}

// isAllowed returns true if the host is on the allowlist.
// Handles wildcard prefixes like "*.npmjs.org".
func (p *Proxy) isAllowed(host string) bool {
	host = strings.ToLower(host)
	if p.allowedDomains[host] {
		return true
	}
	// Wildcard match: *.foo.com matches bar.foo.com
	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		wildcard := "*." + parts[1]
		if p.allowedDomains[wildcard] {
			return true
		}
	}
	return false
}
```

**Tests to write first (`proxy_test.go`):**
```go
// TestProxy_AllowedDomain: CONNECT to api.anthropic.com → 200 + bytes tunnel
// TestProxy_BlockedDomain: CONNECT to evil.com → 403 + domain in BlockedDomains()
// TestProxy_WildcardAllowlist: *.npmjs.org allows registry.npmjs.org
// TestProxy_NonConnectMethod: GET returns 405
// TestProxy_Shutdown: Shutdown() causes Start goroutine to exit cleanly
```

**Success criteria:**
- [ ] All five tests pass
- [ ] No goroutine leaks (use `goleak` or manual `Shutdown()` → conn refused check)
- [ ] `make test` passes

---

### Task 2.2 — Socat bridge: UNIX socket ↔ TCP localhost

**Creates:** `cli/internal/sandbox/bridge.go`, `cli/internal/sandbox/bridge_test.go`

**Dependencies:** Task 2.1

The bridge generates a shell wrapper script (written into the sandbox staging dir) that socat runs inside the sandbox. The script is `sh`-compatible (no bash-isms) and handles socat startup before exec-ing the provider. It is generated as a file and mounted into the sandbox read-only. The staging dir path is `/tmp/syllago-sandbox-<id>/`.

```go
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WrapperScript generates the content of the in-sandbox wrapper shell script.
// The script:
//  1. Starts socat bridging the UNIX socket to TCP localhost:3128.
//  2. Execs the provider binary with its arguments.
//
// socketPath is the UNIX socket path as seen inside the sandbox.
// providerBin is the absolute path of the provider binary inside the sandbox.
// providerArgs are additional arguments to pass to the provider.
func WrapperScript(socketPath, providerBin string, providerArgs []string) string {
	args := ""
	for _, a := range providerArgs {
		args += " " + shellescape(a)
	}
	return fmt.Sprintf(`#!/bin/sh
socat TCP-LISTEN:3128,fork,reuseaddr UNIX-CONNECT:%s &
exec %s%s
`, socketPath, shellescape(providerBin), args)
}

// WriteWrapperScript writes the wrapper script to stagingDir/wrapper.sh
// and makes it executable.
func WriteWrapperScript(stagingDir, socketPath, providerBin string, providerArgs []string) (string, error) {
	content := WrapperScript(socketPath, providerBin, providerArgs)
	path := filepath.Join(stagingDir, "wrapper.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("writing wrapper script: %w", err)
	}
	return path, nil
}

// shellescape wraps a string in single quotes for safe shell interpolation.
// Single quotes within the string are handled via the '"'"' idiom.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
```

**Note:** `shellescape` is defined in `bridge.go` and is available to all files in `package sandbox`. Do **not** redefine it in `gitwrapper.go` or any other file in the package — it will cause a "already declared" compile error.

**Tests to write first (`bridge_test.go`):**
```go
// TestWrapperScript_ContainsSocat: output contains "socat TCP-LISTEN:3128"
// TestWrapperScript_ContainsExec: output contains "exec <providerBin>"
// TestWrapperScript_ShebangFirstLine: first line is "#!/bin/sh"
// TestWrapperScript_ProviderArgs: extra args appear quoted after the binary
// TestWriteWrapperScript_FileIsExecutable: written file has mode 0755
// TestShellescape_SingleQuote: string with single quote is escaped correctly
```

**Success criteria:**
- [ ] All six tests pass
- [ ] Written script passes `sh -n` (syntax check)
- [ ] `make test` passes

---

## Phase 3 — Provider Profiles and Git Wrapper

---

### Task 3.1 — Provider mount profiles

**Creates:** `cli/internal/sandbox/profile.go`, `cli/internal/sandbox/profile_test.go`

**Dependencies:** Task 1.3 (config schema), `cli/internal/provider` package (for slugs)

Each provider has a curated list of config paths to copy-stage, plus the binary resolution logic. `ConfigPaths` returns `(globalPaths, projectPaths)` — global paths are copied to staging; project paths are mounted RW from CWD. `ResolveBinary` follows symlinks to the real executable and returns the paths the sandbox must mount read-only for the binary to work.

```go
package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// MountProfile describes the filesystem mounts for one provider's sandbox.
type MountProfile struct {
	// GlobalConfigPaths are provider config files to copy-stage (e.g. ~/.claude.json).
	GlobalConfigPaths []string
	// ProjectConfigDirs are project-local config dirs to mount RW (e.g. .claude/).
	ProjectConfigDirs []string
	// BinaryPaths are paths to mount RO to make the binary runnable.
	BinaryPaths []string
	// BinaryExec is the resolved absolute path to the provider binary.
	BinaryExec string
	// ProviderEnvVars are provider-specific env var names to forward.
	ProviderEnvVars []string
	// AllowedDomains are the provider's own API domains.
	AllowedDomains []string
}

// ProfileFor builds a MountProfile for the given provider slug and home dir.
// Returns an error if the provider is unknown or its binary cannot be found.
func ProfileFor(slug, homeDir, projectDir string) (*MountProfile, error) {
	switch slug {
	case "claude-code":
		return claudeProfile(homeDir, projectDir)
	case "gemini-cli":
		return geminiProfile(homeDir, projectDir)
	case "codex":
		return codexProfile(homeDir, projectDir)
	case "copilot-cli":
		return copilotProfile(homeDir, projectDir)
	case "cursor":
		return cursorProfile(homeDir, projectDir)
	case "windsurf":
		return nil, fmt.Errorf("windsurf does not support sandbox in v1")
	default:
		return nil, fmt.Errorf("unknown provider %q — supported: claude-code, gemini-cli, codex, copilot-cli, cursor", slug)
	}
}

func claudeProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, err := resolveBinary("claude")
	if err != nil {
		return nil, fmt.Errorf("claude binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			filepath.Join(homeDir, ".claude.json"),
			filepath.Join(homeDir, ".claude"),
		},
		ProjectConfigDirs: []string{
			filepath.Join(projectDir, ".claude"),
		},
		BinaryPaths:     binPaths,
		BinaryExec:      bin,
		ProviderEnvVars: []string{}, // Claude Code uses its own auth, not env vars
		AllowedDomains:  []string{"api.anthropic.com", "sentry.io"},
	}, nil
}

func geminiProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, err := resolveBinary("gemini")
	if err != nil {
		return nil, fmt.Errorf("gemini binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			// Stage the entire ~/.gemini/ dir (contains settings.json, hooks, etc.)
			// settings.json is the primary config but other files in the dir may hold hooks.
			filepath.Join(homeDir, ".gemini"),
		},
		ProjectConfigDirs: []string{
			filepath.Join(projectDir, ".gemini"),
		},
		BinaryPaths:     binPaths,
		BinaryExec:      bin,
		ProviderEnvVars: []string{"GOOGLE_API_KEY", "GEMINI_API_KEY"},
		AllowedDomains:  []string{"generativelanguage.googleapis.com"},
	}, nil
}

func codexProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, err := resolveBinary("codex")
	if err != nil {
		return nil, fmt.Errorf("codex binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			filepath.Join(homeDir, ".codex"),
		},
		ProjectConfigDirs: []string{
			filepath.Join(projectDir, ".codex"),
		},
		BinaryPaths:     binPaths,
		BinaryExec:      bin,
		ProviderEnvVars: []string{"OPENAI_API_KEY"},
		AllowedDomains:  []string{"api.openai.com"},
	}, nil
}

func copilotProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, err := resolveBinary("gh")
	if err != nil {
		return nil, fmt.Errorf("gh (GitHub CLI) binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			filepath.Join(homeDir, ".config", "github-copilot"),
		},
		ProjectConfigDirs: []string{},
		BinaryPaths:       binPaths,
		BinaryExec:        bin,
		ProviderEnvVars:   []string{},
		AllowedDomains:    []string{"api.githubcopilot.com"},
	}, nil
}

func cursorProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, err := resolveBinary("cursor")
	if err != nil {
		return nil, fmt.Errorf("cursor binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			filepath.Join(homeDir, ".cursor"),
		},
		ProjectConfigDirs: []string{},
		BinaryPaths:       binPaths,
		BinaryExec:        bin,
		ProviderEnvVars:   []string{},
		AllowedDomains:    []string{"api2.cursor.sh"},
	}, nil
}

// resolveBinary finds the binary on PATH, resolves symlinks, and returns
// (resolvedPath, mountPaths, error). mountPaths is the list of paths to mount
// read-only in the sandbox to make the binary accessible.
func resolveBinary(name string) (string, []string, error) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", nil, fmt.Errorf("%q not found on PATH", name)
	}
	resolved, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return "", nil, fmt.Errorf("resolving symlinks for %q: %w", bin, err)
	}
	// Mount the resolved binary's directory so relative-path interpreters work.
	return resolved, []string{resolved}, nil
}

// EcosystemDomains returns the package registry domains for detected ecosystems.
// Detects by presence of project marker files in projectDir.
func EcosystemDomains(projectDir string) []string {
	type marker struct {
		file    string
		domains []string
	}
	ecosystems := []marker{
		{"package.json", []string{"registry.npmjs.org", "*.npmjs.org", "objects.githubusercontent.com"}},
		{"pnpm-lock.yaml", []string{"registry.npmjs.org", "*.npmjs.org"}},
		{"bun.lockb", []string{"registry.npmjs.org"}},
		{"go.mod", []string{"proxy.golang.org", "sum.golang.org", "storage.googleapis.com"}},
		{"Cargo.toml", []string{"crates.io", "static.crates.io"}},
		{"pyproject.toml", []string{"pypi.org", "files.pythonhosted.org"}},
		{"requirements.txt", []string{"pypi.org", "files.pythonhosted.org"}},
	}

	seen := make(map[string]bool)
	var domains []string
	for _, e := range ecosystems {
		if _, err := os.Stat(filepath.Join(projectDir, e.file)); err == nil {
			for _, d := range e.domains {
				if !seen[d] {
					seen[d] = true
					domains = append(domains, d)
				}
			}
		}
	}
	return domains
}

// EcosystemCacheMounts returns RO bind-mount paths for detected ecosystem caches.
// Only returns paths that actually exist.
func EcosystemCacheMounts(projectDir, homeDir string) []string {
	type marker struct {
		file      string
		cachePath string
	}
	ecosystems := []marker{
		{"package.json", filepath.Join(homeDir, ".npm")},
		{"package.json", filepath.Join(homeDir, ".cache", "npm")},
		{"go.mod", filepath.Join(homeDir, "go", "pkg", "mod")},
		{"go.mod", filepath.Join(homeDir, ".cache", "go-build")},
		{"Cargo.toml", filepath.Join(homeDir, ".cargo", "registry")},
		{"Cargo.toml", filepath.Join(homeDir, ".cargo", "git")},
		{"pyproject.toml", filepath.Join(homeDir, ".cache", "pip")},
		{"requirements.txt", filepath.Join(homeDir, ".cache", "pip")},
		{"pnpm-lock.yaml", filepath.Join(homeDir, ".local", "share", "pnpm", "store")},
		{"bun.lockb", filepath.Join(homeDir, ".bun", "install", "cache")},
	}

	seen := make(map[string]bool)
	var mounts []string
	for _, e := range ecosystems {
		if _, err := os.Stat(filepath.Join(projectDir, e.file)); err != nil {
			continue
		}
		if seen[e.cachePath] {
			continue
		}
		if _, err := os.Stat(e.cachePath); err == nil {
			seen[e.cachePath] = true
			mounts = append(mounts, e.cachePath)
		}
	}
	return mounts
}
```

**Tests to write first (`profile_test.go`):**
```go
// TestProfileFor_UnknownProvider: returns error for "bad-provider"
// TestEcosystemDomains_GoMod: detects go.mod → includes proxy.golang.org
// TestEcosystemDomains_PackageJSON: detects package.json → includes *.npmjs.org
// TestEcosystemDomains_MultipleMarkers: go.mod + Cargo.toml → combined (no duplicates)
// TestEcosystemDomains_NoMarkers: returns empty slice
// TestEcosystemCacheMounts_OnlyExisting: cache path in result only if it exists on disk
```

**Success criteria:**
- [ ] All six tests pass
- [ ] `make test` passes

---

### Task 3.2 — Git subcommand allowlist wrapper generator

**Creates:** `cli/internal/sandbox/gitwrapper.go`, `cli/internal/sandbox/gitwrapper_test.go`

**Dependencies:** Task 2.2 (`bridge.go` must exist because `gitwrapper.go` uses the `shellescape` helper defined there — both files are in `package sandbox`)

Generates a shell script that wraps `git`. The wrapper is mounted at `/usr/local/bin/git` (higher PATH priority than the real `/usr/bin/git`). When the sandboxed tool calls `git push`, it hits the wrapper, which prints a clear error and exits non-zero. When it calls `git commit`, the wrapper passes through to the real git.

```go
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
)

// blockedGitSubcommands is the list of git subcommands that are always blocked.
var blockedGitSubcommands = []string{
	"push", "fetch", "pull", "clone",
	"remote", "ls-remote", "submodule",
}

// GitWrapperScript returns the content of the git wrapper shell script.
// realGit is the path to the real git binary (e.g. /usr/bin/git).
func GitWrapperScript(realGit string) string {
	blocked := ""
	for _, cmd := range blockedGitSubcommands {
		blocked += fmt.Sprintf(`    %s)
      echo "[sandbox] git %s is blocked in the sandbox." >&2
      exit 1
      ;;
`, cmd, cmd)
	}

	return fmt.Sprintf(`#!/bin/sh
# Syllago sandbox git wrapper — blocks network operations.
SUBCMD="${1:-}"
case "$SUBCMD" in
%s    config)
      # Block global config writes.
      for arg in "$@"; do
        case "$arg" in --global|--system) echo "[sandbox] git config --global/--system is blocked." >&2; exit 1 ;; esac
      done
      exec %s "$@"
      ;;
    *)
      exec %s "$@"
      ;;
esac
`, blocked, shellescape(realGit), shellescape(realGit))
}

// WriteGitWrapper writes the git wrapper script to stagingDir/git and makes it executable.
// Returns the path to the written script.
func WriteGitWrapper(stagingDir, realGit string) (string, error) {
	content := GitWrapperScript(realGit)
	path := filepath.Join(stagingDir, "git")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("writing git wrapper: %w", err)
	}
	return path, nil
}
```

**Tests to write first (`gitwrapper_test.go`):**
```go
// TestGitWrapperScript_ContainsBlockedCommands: "push)", "fetch)", "clone)" all present
// TestGitWrapperScript_AllowsCommit: "commit" routes to exec real git, not blocked
// TestGitWrapperScript_BlocksGlobalConfig: --global case present in config branch
// TestGitWrapperScript_ShebangFirstLine: first line is "#!/bin/sh"
// TestWriteGitWrapper_FileIsExecutable: written file mode is 0755
```

**Success criteria:**
- [ ] All five tests pass
- [ ] Written script passes `sh -n`
- [ ] `make test` passes

---

## Phase 4 — Config Diff and Approval

---

### Task 4.1 — Config staging: copy and hash on entry

**Creates:** `cli/internal/sandbox/configdiff.go`, `cli/internal/sandbox/configdiff_test.go`

**Dependencies:** None

On sandbox entry we copy each config file/dir into the staging area and record its SHA-256 hash. On exit we compare the current hash to the original. This is the first half of the copy-diff-approve pattern.

```go
package sandbox

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ConfigSnapshot records the pre-sandbox state of a config file.
type ConfigSnapshot struct {
	OriginalPath string // absolute path to the original file/dir
	StagedPath   string // absolute path to the copy in staging
	OriginalHash []byte // SHA-256 of original content (file) or merkle of dir
}

// StageConfigs copies globalConfigPaths into stagingDir/config/ and records hashes.
// Paths that do not exist are skipped (provider may not have created them yet).
func StageConfigs(stagingDir string, globalConfigPaths []string) ([]ConfigSnapshot, error) {
	destBase := filepath.Join(stagingDir, "config")
	if err := os.MkdirAll(destBase, 0700); err != nil {
		return nil, fmt.Errorf("creating config staging dir: %w", err)
	}

	var snapshots []ConfigSnapshot
	for _, src := range globalConfigPaths {
		info, err := os.Stat(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", src, err)
		}

		// Derive a unique dest path (preserve base name to avoid collisions).
		dest := filepath.Join(destBase, filepath.Base(src))

		if info.IsDir() {
			if err := copyDir(src, dest); err != nil {
				return nil, fmt.Errorf("copying dir %s: %w", src, err)
			}
		} else {
			if err := copyFile(src, dest); err != nil {
				return nil, fmt.Errorf("copying file %s: %w", src, err)
			}
		}

		hash, err := hashPath(src)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", src, err)
		}

		snapshots = append(snapshots, ConfigSnapshot{
			OriginalPath: src,
			StagedPath:   dest,
			OriginalHash: hash,
		})
	}
	return snapshots, nil
}

// DiffResult describes changes to one config path after the sandbox session.
type DiffResult struct {
	Snapshot ConfigSnapshot
	Changed  bool
	IsHighRisk bool   // true if diff contains MCP server or hook changes
	DiffText   string // human-readable unified diff
}

// ComputeDiffs compares staged copies against their recorded original hashes.
// Returns one DiffResult per snapshot that was changed.
func ComputeDiffs(snapshots []ConfigSnapshot) ([]DiffResult, error) {
	var results []DiffResult
	for _, snap := range snapshots {
		currentHash, err := hashPath(snap.StagedPath)
		if err != nil {
			// Staged file deleted: treat as high-risk change (config removed).
			results = append(results, DiffResult{
				Snapshot:   snap,
				Changed:    true,
				IsHighRisk: true,
				DiffText:   "(config file was deleted inside sandbox)",
			})
			continue
		}

		if string(currentHash) == string(snap.OriginalHash) {
			continue // unchanged
		}

		diff, highRisk := buildDiff(snap.OriginalPath, snap.StagedPath)
		results = append(results, DiffResult{
			Snapshot:   snap,
			Changed:    true,
			IsHighRisk: highRisk,
			DiffText:   diff,
		})
	}
	return results, nil
}

// ApplyDiff copies the staged version back to the original path.
// Call this only after user approval.
func ApplyDiff(result DiffResult) error {
	info, err := os.Stat(result.Snapshot.StagedPath)
	if err != nil {
		return fmt.Errorf("staged path gone: %w", err)
	}
	if info.IsDir() {
		return copyDir(result.Snapshot.StagedPath, result.Snapshot.OriginalPath)
	}
	return copyFile(result.Snapshot.StagedPath, result.Snapshot.OriginalPath)
}

// hashPath returns SHA-256 of a file, or a deterministic hash of a directory tree.
func hashPath(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	if info.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(path, p)
			fmt.Fprintf(h, "%s\x00", rel)
			if !d.IsDir() {
				f, err := os.Open(p)
				if err != nil {
					return err
				}
				defer f.Close()
				if _, err := io.Copy(h, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// buildDiff returns a human-readable diff and whether it's high-risk.
// High-risk: any JSON file in the path contains new "mcpServers" or "hooks" keys.
// Handles both file and directory paths — for directories it walks all JSON files.
func buildDiff(orig, staged string) (string, bool) {
	info, err := os.Stat(staged)
	if err != nil {
		return "(staged path unreadable)", false
	}

	if info.IsDir() {
		// Walk the staged directory, collect diffs and high-risk status from JSON files.
		var sb strings.Builder
		highRisk := false
		_ = filepath.WalkDir(staged, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(staged, p)
			origFile := filepath.Join(orig, rel)
			origData, _ := os.ReadFile(origFile)
			stagedData, _ := os.ReadFile(p)
			if string(origData) != string(stagedData) {
				fmt.Fprintf(&sb, "--- %s\n+++ %s\n%s", origFile, p, unifiedDiff(origData, stagedData))
				if containsHighRiskChange(stagedData) {
					highRisk = true
				}
			}
			return nil
		})
		return sb.String(), highRisk
	}

	// File path.
	origData, _ := os.ReadFile(orig)
	stagedData, _ := os.ReadFile(staged)
	diff := fmt.Sprintf("--- %s\n+++ %s\n%s",
		orig, staged,
		unifiedDiff(origData, stagedData),
	)
	highRisk := containsHighRiskChange(stagedData)
	return diff, highRisk
}

// containsHighRiskChange detects new MCP servers or hooks in JSON config.
func containsHighRiskChange(data []byte) bool {
	s := string(data)
	return strings.Contains(s, `"mcpServers"`) || strings.Contains(s, `"hooks"`)
}

// unifiedDiff produces a simple line-diff between two byte slices.
func unifiedDiff(a, b []byte) string {
	aLines := strings.Split(string(a), "\n")
	bLines := strings.Split(string(b), "\n")
	var out strings.Builder
	for _, line := range aLines {
		out.WriteString("-" + line + "\n")
	}
	for _, line := range bLines {
		out.WriteString("+" + line + "\n")
	}
	return out.String()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		return copyFile(path, target)
	})
}
```

**Tests to write first (`configdiff_test.go`):**
```go
// TestStageConfigs_CopiesFiles: file content in staged path matches original
// TestStageConfigs_SkipsNonExistent: missing path silently skipped (no error)
// TestStageConfigs_CopiesDir: directory is recursively copied
// TestComputeDiffs_UnchangedFile: returns no DiffResults
// TestComputeDiffs_ChangedFile: returns DiffResult with Changed=true
// TestComputeDiffs_HighRiskMCP: JSON with mcpServers → IsHighRisk=true
// TestComputeDiffs_HighRiskHooks: JSON with hooks → IsHighRisk=true
// TestComputeDiffs_HighRiskMCPInDir: JSON file inside a staged dir with mcpServers → IsHighRisk=true
// TestComputeDiffs_DirDiff_ShowsChangedFiles: changed file inside staged dir appears in DiffText
// TestApplyDiff_CopiesBack: original updated to staged content
```

**Success criteria:**
- [ ] All eight tests pass
- [ ] `make test` passes

---

## Phase 5 — Bubblewrap Builder and Pre-flight Check

---

### Task 5.1 — Bubblewrap argument construction

**Creates:** `cli/internal/sandbox/bwrap.go`, `cli/internal/sandbox/bwrap_test.go`

**Dependencies:** Tasks 3.1, 3.2, 4.1 (profile, git wrapper paths, staged config paths)

This is the core of the sandbox: assembles all bwrap arguments from a structured `BwrapConfig`. By keeping args in a struct and building the slice in one function, tests can verify exact flags without parsing shell commands. The `--die-with-parent` flag ensures sandbox cleanup if syllago crashes.

```go
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BwrapConfig holds all inputs needed to construct bubblewrap arguments.
type BwrapConfig struct {
	ProjectDir   string         // bind-mounted RW
	HomeDir      string         // used for --setenv HOME
	StagingDir   string         // contains wrapper.sh, git wrapper, proxy socket
	SocketPath   string         // UNIX proxy socket path (inside sandbox)
	GitWrapperPath string       // path to git wrapper script (host side, mounted RO)
	WrapperScript  string       // path to wrapper.sh (host side, mounted RO)
	Profile        *MountProfile
	Snapshots      []ConfigSnapshot // for mounting staged config copies
	EcosystemCacheRO []string       // RO bind mounts for package caches
	AdditionalMountsRO []string     // user-supplied --mount-ro paths
	EnvPairs       []string         // KEY=VALUE pairs from FilterEnv
	SandboxEnvOverrides map[string]string // set by sandbox (PATH, HTTP_PROXY, etc.)
}

// BuildArgs returns the full slice of arguments to pass to bwrap.
func BuildArgs(cfg BwrapConfig) []string {
	var args []string

	// Security/namespace flags
	args = append(args,
		"--new-session",
		"--die-with-parent",
		"--unshare-net",
		"--unshare-pid",
		"--unshare-ipc",
		"--cap-drop", "ALL",
	)

	// Minimal device filesystem
	args = append(args, "--dev", "/dev", "--proc", "/proc")

	// Read-only system mounts
	for _, ro := range []string{"/usr", "/lib", "/lib64"} {
		args = append(args, "--ro-bind-try", ro, ro)
	}

	// Symlinks for /bin and /sbin (most distros have these as symlinks anyway)
	args = append(args, "--symlink", "usr/bin", "/bin")
	args = append(args, "--symlink", "usr/sbin", "/sbin")

	// Essential /etc files for DNS and TLS
	for _, f := range []string{
		"/etc/resolv.conf",
		"/etc/hosts",
		"/etc/nsswitch.conf",
	} {
		args = append(args, "--ro-bind-try", f, f)
	}
	for _, d := range []string{"/etc/ssl", "/etc/ca-certificates"} {
		args = append(args, "--ro-bind-try", d, d)
	}

	// Private /tmp
	args = append(args, "--tmpfs", "/tmp")

	// Project directory: read-write
	args = append(args, "--bind", cfg.ProjectDir, cfg.ProjectDir)

	// Proxy socket: bind into sandbox
	args = append(args, "--bind", cfg.SocketPath, cfg.SocketPath)

	// Git wrapper: mounted RO at higher PATH priority
	args = append(args, "--ro-bind", cfg.GitWrapperPath, "/usr/local/bin/git")

	// Wrapper script: mounted RO
	args = append(args, "--ro-bind", cfg.WrapperScript, cfg.WrapperScript)

	// Provider binary
	if cfg.Profile != nil {
		for _, bp := range cfg.Profile.BinaryPaths {
			args = append(args, "--ro-bind-try", bp, bp)
		}
	}

	// Staged config copies (RW — the sandbox can modify them)
	for _, snap := range cfg.Snapshots {
		info, err := os.Stat(snap.StagedPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			// Mount the staged dir at the original path
			args = append(args, "--bind", snap.StagedPath, snap.OriginalPath)
		} else {
			// Ensure the parent dir exists inside the sandbox
			args = append(args, "--bind", snap.StagedPath, snap.OriginalPath)
		}
	}

	// Project-local config dirs (RW from actual CWD)
	if cfg.Profile != nil {
		for _, pd := range cfg.Profile.ProjectConfigDirs {
			if _, err := os.Stat(pd); err == nil {
				args = append(args, "--bind", pd, pd)
			}
		}
	}

	// Ecosystem caches (RO)
	for _, cache := range cfg.EcosystemCacheRO {
		args = append(args, "--ro-bind", cache, cache)
	}

	// User-supplied extra RO mounts (--mount-ro flag)
	for _, m := range cfg.AdditionalMountsRO {
		args = append(args, "--ro-bind", m, m)
	}

	// Environment: forwarded vars from FilterEnv
	for _, pair := range cfg.EnvPairs {
		idx := strings.IndexByte(pair, '=')
		if idx >= 0 {
			args = append(args, "--setenv", pair[:idx], pair[idx+1:])
		}
	}

	// Sandbox-set overrides (always applied, override anything from FilterEnv)
	for k, v := range cfg.SandboxEnvOverrides {
		args = append(args, "--setenv", k, v)
	}

	// The wrapper script is the entry point
	args = append(args, "--", cfg.WrapperScript)

	return args
}
```

**Tests to write first (`bwrap_test.go`):**
```go
// TestBuildArgs_ContainsUnshareNet: "--unshare-net" present
// TestBuildArgs_ContainsDieWithParent: "--die-with-parent" present
// TestBuildArgs_ContainsCapDropAll: "--cap-drop", "ALL" pair present
// TestBuildArgs_ProjectDirBind: "--bind", projectDir, projectDir all present
// TestBuildArgs_GitWrapperMount: "--ro-bind", gitWrapperPath, "/usr/local/bin/git" present
// TestBuildArgs_ProxySocketBound: "--bind", socketPath, socketPath present
// TestBuildArgs_EnvVarSet: "--setenv", "HOME", <value> present for forwarded env
// TestBuildArgs_SandboxOverridesEnv: HTTP_PROXY override appears in args
```

**Success criteria:**
- [ ] All eight tests pass
- [ ] `make test` passes

---

### Task 5.2 — Pre-flight check (bwrap, socat, binary availability)

**Creates:** `cli/internal/sandbox/check.go`, `cli/internal/sandbox/check_test.go`

**Dependencies:** Task 3.1 (ProfileFor)

`syllago sandbox check` and `syllago sandbox check <provider>` both use this. The version check (`bwrap --version`) guards against the rare case where an old bwrap without user namespace support is installed. The output format matches the design doc's check example.

```go
package sandbox

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckResult holds the outcome of a pre-flight check.
type CheckResult struct {
	BwrapOK   bool
	BwrapVersion string
	SocatOK   bool
	ProviderOK bool
	ProviderPath string
	Errors    []string
}

// Check performs the pre-flight check for the system (no provider) or for a specific provider.
// providerSlug may be empty to skip provider-specific checks.
func Check(providerSlug, homeDir, projectDir string) CheckResult {
	var r CheckResult

	// Check bwrap
	out, err := exec.Command("bwrap", "--version").Output()
	if err != nil {
		r.Errors = append(r.Errors, "bwrap not found — install bubblewrap >= 0.4.0")
	} else {
		r.BwrapOK = true
		r.BwrapVersion = strings.TrimSpace(string(out))
	}

	// Check socat
	if _, err := exec.LookPath("socat"); err != nil {
		r.Errors = append(r.Errors, "socat not found — install socat >= 1.7.0")
	} else {
		r.SocatOK = true
	}

	// Provider-specific check
	if providerSlug != "" {
		profile, err := ProfileFor(providerSlug, homeDir, projectDir)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("provider: %s", err))
		} else {
			r.ProviderOK = true
			r.ProviderPath = profile.BinaryExec
		}
	}

	return r
}

// FormatCheckResult formats a CheckResult for human display.
func FormatCheckResult(r CheckResult, providerSlug string) string {
	var sb strings.Builder

	status := func(ok bool) string {
		if ok {
			return "OK"
		}
		return "MISSING"
	}

	fmt.Fprintf(&sb, "  bwrap:  %s", status(r.BwrapOK))
	if r.BwrapVersion != "" {
		fmt.Fprintf(&sb, " (%s)", r.BwrapVersion)
	}
	sb.WriteByte('\n')

	fmt.Fprintf(&sb, "  socat:  %s\n", status(r.SocatOK))

	if providerSlug != "" {
		fmt.Fprintf(&sb, "  %s: %s", providerSlug, status(r.ProviderOK))
		if r.ProviderPath != "" {
			fmt.Fprintf(&sb, " (%s)", r.ProviderPath)
		}
		sb.WriteByte('\n')
	}

	for _, e := range r.Errors {
		fmt.Fprintf(&sb, "  ERROR: %s\n", e)
	}

	if len(r.Errors) == 0 {
		sb.WriteString("  Status: Ready for sandboxing\n")
	} else {
		sb.WriteString("  Status: Not ready\n")
	}

	return sb.String()
}
```

**Tests to write first (`check_test.go`):**
```go
// TestCheck_BwrapMissing: when bwrap not on PATH, BwrapOK=false, error message present
// TestCheck_SocatMissing: when socat not on PATH, SocatOK=false, error message present
// TestCheck_UnknownProvider: providerSlug="bad" → ProviderOK=false, error present
// TestFormatCheckResult_AllOK: output contains "Status: Ready for sandboxing"
// TestFormatCheckResult_WithErrors: output contains "Status: Not ready" and error text
```

**Success criteria:**
- [ ] All five tests pass
- [ ] `make test` passes

---

## Phase 6 — Runner/Orchestrator

---

### Task 6.1 — Staging directory lifecycle

**Creates:** `cli/internal/sandbox/staging.go`, `cli/internal/sandbox/staging_test.go`

**Dependencies:** None

The staging dir holds everything the sandbox session needs: wrapper script, git wrapper, proxy socket, staged config copies, and the minimal gitconfig. A random ID makes concurrent sessions possible. Cleanup is done via a `defer` in the runner and also on next-run stale detection.

```go
package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StagingDir manages the per-session temporary directory.
type StagingDir struct {
	ID   string // random hex ID
	Path string // absolute path: /tmp/syllago-sandbox-<id>
}

// NewStagingDir creates a new staging directory with a random ID.
func NewStagingDir() (*StagingDir, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generating staging ID: %w", err)
	}
	id := hex.EncodeToString(idBytes)
	path := filepath.Join("/tmp", "syllago-sandbox-"+id)
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, fmt.Errorf("creating staging dir: %w", err)
	}
	return &StagingDir{ID: id, Path: path}, nil
}

// SocketPath returns the path for the proxy UNIX socket.
func (s *StagingDir) SocketPath() string {
	return filepath.Join(s.Path, "proxy.sock")
}

// GitconfigPath returns the path for the sandbox-local gitconfig.
func (s *StagingDir) GitconfigPath() string {
	return filepath.Join(s.Path, "gitconfig")
}

// WriteGitconfig writes a minimal gitconfig (user.name, user.email only).
func (s *StagingDir) WriteGitconfig(name, email string) error {
	content := fmt.Sprintf("[user]\n\tname = %s\n\temail = %s\n", name, email)
	return os.WriteFile(s.GitconfigPath(), []byte(content), 0600)
}

// Cleanup removes the staging directory and all its contents.
func (s *StagingDir) Cleanup() error {
	return os.RemoveAll(s.Path)
}

// CleanStale removes any stale /tmp/syllago-sandbox-* directories from previous
// crashed sessions. Called at the start of each new session.
func CleanStale() {
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "syllago-sandbox-") {
			_ = os.RemoveAll(filepath.Join("/tmp", e.Name()))
		}
	}
}
```

**Tests to write first (`staging_test.go`):**
```go
// TestNewStagingDir_CreatesDir: path exists after construction
// TestNewStagingDir_UniqueIDs: two calls produce different IDs
// TestStagingDir_SocketPath: returns path ending in "proxy.sock"
// TestStagingDir_WriteGitconfig: file exists and contains "[user]"
// TestStagingDir_Cleanup: path does not exist after Cleanup()
// TestCleanStale_RemovesOldDirs: pre-existing syllago-sandbox-* dir is removed
```

**Success criteria:**
- [ ] All six tests pass
- [ ] `make test` passes

---

### Task 6.2 — Runner: session orchestrator

**Creates:** `cli/internal/sandbox/runner.go`

**Dependencies:** All previous sandbox tasks (1.1 through 6.1)

The runner is the heart of the feature: it sequences every step in the design doc's lifecycle. It owns the pre-flight check, proxy, staging, bwrap exec, and post-exit diff/approval flow. The `os.Signal` channel with `signal.NotifyContext` ensures cleanup on Ctrl-C.

```go
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// RunConfig is the full configuration for a sandbox session.
type RunConfig struct {
	ProviderSlug   string
	ProjectDir     string
	HomeDir        string
	ForceDir       bool
	AdditionalDomains []string  // --allow-domain flags
	AdditionalPorts   []int     // --allow-port flags
	AdditionalEnv     []string  // --allow-env flags
	AdditionalMountsRO []string // --mount-ro flags (extra read-only bind mounts)
	NoNetwork         bool      // --no-network flag
	SandboxConfig  config.SandboxConfig // from .syllago/config.json
}

// bwrapRunner is the function that actually invokes bwrap.
// Overridable in tests (Task 9.1 depends on this seam).
var bwrapRunner = func(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunSession executes a full sandboxed session for the given provider.
// It writes progress messages to w (typically os.Stdout) and prompts to w.
func RunSession(cfg RunConfig, w *os.File) error {
	// Step 1: Clean stale sessions from previous crashes.
	CleanStale()

	// Step 2: Validate directory safety.
	if cfg.ForceDir {
		fmt.Fprintf(w, "WARNING: Directory safety checks skipped. The entire directory\n%s will be writable inside the sandbox.\n\n", cfg.ProjectDir)
	}
	if err := ValidateDir(cfg.ProjectDir, cfg.ForceDir); err != nil {
		return err
	}

	// Step 3: Pre-flight check for bwrap, socat, and provider binary.
	checkResult := Check(cfg.ProviderSlug, cfg.HomeDir, cfg.ProjectDir)
	if len(checkResult.Errors) > 0 {
		return fmt.Errorf("pre-flight check failed:\n%s", FormatCheckResult(checkResult, cfg.ProviderSlug))
	}

	// Step 4: Load provider mount profile.
	profile, err := ProfileFor(cfg.ProviderSlug, cfg.HomeDir, cfg.ProjectDir)
	if err != nil {
		return err
	}

	// Step 5: Create staging directory.
	staging, err := NewStagingDir()
	if err != nil {
		return err
	}
	defer staging.Cleanup()

	// Signal handler: clean up on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Step 6: Stage provider config files.
	snapshots, err := StageConfigs(staging.Path, profile.GlobalConfigPaths)
	if err != nil {
		return fmt.Errorf("staging configs: %w", err)
	}

	// Step 7: Write sandbox gitconfig.
	if err := staging.WriteGitconfig("Sandbox User", "sandbox@syllago.local"); err != nil {
		return fmt.Errorf("writing gitconfig: %w", err)
	}

	// Step 8: Detect ecosystem and build domain allowlist.
	ecosystemDomains := EcosystemDomains(cfg.ProjectDir)
	ecosystemCaches := EcosystemCacheMounts(cfg.ProjectDir, cfg.HomeDir)

	var allDomains []string
	if cfg.NoNetwork {
		// --no-network: block ALL egress — pass empty allowlist to proxy.
		// Provider API domains, ecosystem domains, and user config are all suppressed.
	} else {
		allDomains = append(profile.AllowedDomains, cfg.SandboxConfig.AllowedDomains...)
		allDomains = append(allDomains, cfg.AdditionalDomains...)
		allDomains = append(allDomains, ecosystemDomains...)
	}

	allPorts := append(cfg.SandboxConfig.AllowedPorts, cfg.AdditionalPorts...)

	// Step 9: Build env allowlist.
	allExtraEnv := append(profile.ProviderEnvVars, cfg.SandboxConfig.AllowedEnv...)
	allExtraEnv = append(allExtraEnv, cfg.AdditionalEnv...)
	envPairs, envReport := FilterEnv(os.Environ(), allExtraEnv)

	// Step 10: Print sandbox summary.
	fmt.Fprintf(w, "\nSandbox environment:\n")
	fmt.Fprintf(w, "  Forwarded: %s\n", strings.Join(envReport.Forwarded, ", "))
	fmt.Fprintf(w, "  Stripped: %d env vars (%s)\n",
		len(envReport.Stripped), strings.Join(envReport.Stripped, ", "))
	fmt.Fprintf(w, "  Network: %s\n", strings.Join(allDomains, ", "))
	fmt.Fprintf(w, "\n")

	// Step 11: Start egress proxy.
	proxy := NewProxy(staging.SocketPath(), allDomains, allPorts)
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("starting proxy: %w", err)
	}
	defer proxy.Shutdown()

	// Step 12: Write git wrapper script.
	realGit, err := exec.LookPath("git")
	if err != nil {
		realGit = "/usr/bin/git"
	}
	gitWrapperPath, err := WriteGitWrapper(staging.Path, realGit)
	if err != nil {
		return fmt.Errorf("writing git wrapper: %w", err)
	}

	// Step 13: Write in-sandbox wrapper script.
	wrapperPath, err := WriteWrapperScript(staging.Path, staging.SocketPath(), profile.BinaryExec, nil)
	if err != nil {
		return fmt.Errorf("writing wrapper script: %w", err)
	}

	// Step 14: Build bwrap arguments.
	sandboxEnv := map[string]string{
		"PATH":              "/usr/local/bin:/usr/bin:/bin",
		"HTTP_PROXY":        "http://127.0.0.1:3128",
		"HTTPS_PROXY":       "http://127.0.0.1:3128",
		"NO_PROXY":          "",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL": staging.GitconfigPath(),
		"GIT_TERMINAL_PROMPT": "0",
		"HOME":              cfg.HomeDir,
	}

	bwrapCfg := BwrapConfig{
		ProjectDir:           cfg.ProjectDir,
		HomeDir:              cfg.HomeDir,
		StagingDir:           staging.Path,
		SocketPath:           staging.SocketPath(),
		GitWrapperPath:       gitWrapperPath,
		WrapperScript:        wrapperPath,
		Profile:              profile,
		Snapshots:            snapshots,
		EcosystemCacheRO:     ecosystemCaches,
		AdditionalMountsRO:   cfg.AdditionalMountsRO,
		EnvPairs:             envPairs,
		SandboxEnvOverrides:  sandboxEnv,
	}
	bwrapArgs := BuildArgs(bwrapCfg)

	// Step 15: Launch bubblewrap (via injectable bwrapRunner for testability).
	start := time.Now()
	if err := bwrapRunner(ctx, bwrapArgs); err != nil && ctx.Err() == nil {
		// Non-zero exit from the provider is expected (e.g. user typed "exit").
		// Only surface errors that aren't from the provider itself exiting.
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == -1 {
			return fmt.Errorf("sandbox exited with error: %w", err)
		}
	}

	duration := time.Since(start).Round(time.Second)

	// Step 16: Diff staged configs.
	diffs, err := ComputeDiffs(snapshots)
	if err != nil {
		fmt.Fprintf(w, "Warning: config diff failed: %s\n", err)
	}

	// Step 17: Show diff and prompt for approval.
	approved, rejected := 0, 0
	for _, d := range diffs {
		if !d.Changed {
			continue
		}
		risk := "low risk"
		if d.IsHighRisk {
			risk = "HIGH RISK (MCP servers or hooks)"
		}
		fmt.Fprintf(w, "\nConfig changed: %s [%s]\n", d.Snapshot.OriginalPath, risk)
		fmt.Fprintf(w, "%s\n", d.DiffText)

		if promptYN(w, "Apply this change?") {
			if err := ApplyDiff(d); err != nil {
				fmt.Fprintf(w, "Error applying diff: %s\n", err)
			} else {
				approved++
			}
		} else {
			rejected++
		}
	}

	// Step 18: Print session summary.
	blocked := proxy.BlockedDomains()
	fmt.Fprintf(w, "\nSandbox session ended.\n")
	fmt.Fprintf(w, "  Duration: %s\n", duration)
	if len(blocked) > 0 {
		fmt.Fprintf(w, "  Blocked domains: %s\n", strings.Join(blocked, ", "))
	}
	if len(diffs) > 0 {
		fmt.Fprintf(w, "  Config changes: %d approved, %d rejected\n", approved, rejected)
	}

	return nil
}

// promptYN prints a [y/N] prompt and reads a single-line answer.
func promptYN(w *os.File, question string) bool {
	fmt.Fprintf(w, "%s [y/N] ", question)
	var answer string
	fmt.Fscan(os.Stdin, &answer)
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}
```

**Success criteria:**
- [ ] Smoke test: `syllago sandbox run claude` in a valid project dir reaches `bwrap` invocation (even if bwrap is not installed, the error should be "bwrap not found", not a Go panic)
- [ ] Signal test: Ctrl-C during sandbox causes staging dir to be cleaned up
- [ ] `make build` passes

---

## Phase 7 — CLI Commands

---

### Task 7.1 — `syllago sandbox` command group

**Creates:** `cli/cmd/syllago/sandbox_cmd.go`

**Dependencies:** All Phase 1–6 tasks (sandbox package complete), Task 1.3 (config schema)

This follows the exact same pattern as `registry_cmd.go`: a parent Cobra command with subcommands registered in `init()`. The `runCmd` is the main user-facing command; the rest are configuration management utilities.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/sandbox"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Run and manage AI CLI tools in bubblewrap sandboxes",
	Long: `Sandbox wraps AI CLI tools in bubblewrap to restrict filesystem access,
network egress, and environment variables.

Linux only: requires bubblewrap >= 0.4.0 and socat >= 1.7.0.`,
}

var sandboxRunCmd = &cobra.Command{
	Use:   "run <provider>",
	Short: "Run a provider in a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}

		forceDir, _ := cmd.Flags().GetBool("force-dir")
		noNetwork, _ := cmd.Flags().GetBool("no-network")
		allowDomains, _ := cmd.Flags().GetStringArray("allow-domain")
		allowEnvs, _ := cmd.Flags().GetStringArray("allow-env")
		allowPortsStr, _ := cmd.Flags().GetStringArray("allow-port")
		mountRO, _ := cmd.Flags().GetStringArray("mount-ro")

		var allowPorts []int
		for _, ps := range allowPortsStr {
			p, err := strconv.Atoi(ps)
			if err != nil {
				return fmt.Errorf("invalid port %q: must be an integer", ps)
			}
			allowPorts = append(allowPorts, p)
		}

		return sandbox.RunSession(sandbox.RunConfig{
			ProviderSlug:       args[0],
			ProjectDir:         cwd,
			HomeDir:            home,
			ForceDir:           forceDir,
			AdditionalDomains:  allowDomains,
			AdditionalPorts:    allowPorts,
			AdditionalEnv:      allowEnvs,
			AdditionalMountsRO: mountRO,
			NoNetwork:          noNetwork,
			SandboxConfig:      cfg.Sandbox,
		}, os.Stdout)
	},
}

var sandboxCheckCmd = &cobra.Command{
	Use:   "check [provider]",
	Short: "Verify bubblewrap, socat, and optionally a provider",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		slug := ""
		if len(args) == 1 {
			slug = args[0]
		}
		result := sandbox.Check(slug, home, cwd)
		fmt.Print(sandbox.FormatCheckResult(result, slug))
		if len(result.Errors) > 0 {
			return output.SilentError("pre-flight check failed")
		}
		return nil
	},
}

var sandboxInfoCmd = &cobra.Command{
	Use:   "info [provider]",
	Short: "Show effective sandbox configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		sb := cfg.Sandbox
		fmt.Printf("Sandbox configuration (.syllago/config.json):\n")
		fmt.Printf("  Allowed domains: %s\n", formatList(sb.AllowedDomains))
		fmt.Printf("  Allowed env vars: %s\n", formatList(sb.AllowedEnv))
		fmt.Printf("  Allowed ports: %v\n", sb.AllowedPorts)

		if len(args) == 1 {
			home, _ := os.UserHomeDir()
			cwd, _ := os.Getwd()
			profile, err := sandbox.ProfileFor(args[0], home, cwd)
			if err != nil {
				return err
			}
			fmt.Printf("\nProvider %q mount profile:\n", args[0])
			fmt.Printf("  Binary: %s\n", profile.BinaryExec)
			fmt.Printf("  Config files: %s\n", strings.Join(profile.GlobalConfigPaths, ", "))
			fmt.Printf("  Provider domains: %s\n", strings.Join(profile.AllowedDomains, ", "))
		}
		return nil
	},
}

var sandboxAllowDomainCmd = &cobra.Command{
	Use:   "allow-domain <domain>",
	Short: "Add a domain to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE:  makeSandboxConfigCmd(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedDomains = appendUnique(sb.AllowedDomains, args[0])
		fmt.Fprintf(output.Writer, "Added domain: %s\n", args[0])
	}),
}

var sandboxDenyDomainCmd = &cobra.Command{
	Use:   "deny-domain <domain>",
	Short: "Remove a domain from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE:  makeSandboxConfigCmd(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedDomains = removeItem(sb.AllowedDomains, args[0])
		fmt.Fprintf(output.Writer, "Removed domain: %s\n", args[0])
	}),
}

var sandboxAllowEnvCmd = &cobra.Command{
	Use:   "allow-env <VAR>",
	Short: "Add an env var to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE:  makeSandboxConfigCmd(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedEnv = appendUnique(sb.AllowedEnv, args[0])
		fmt.Fprintf(output.Writer, "Added env var: %s\n", args[0])
	}),
}

var sandboxDenyEnvCmd = &cobra.Command{
	Use:   "deny-env <VAR>",
	Short: "Remove an env var from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE:  makeSandboxConfigCmd(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedEnv = removeItem(sb.AllowedEnv, args[0])
		fmt.Fprintf(output.Writer, "Removed env var: %s\n", args[0])
	}),
}

var sandboxAllowPortCmd = &cobra.Command{
	Use:   "allow-port <port>",
	Short: "Add a localhost port to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[0])
		}
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Sandbox.AllowedPorts = appendUniqueInt(cfg.Sandbox.AllowedPorts, port)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Added port: %d\n", port)
		return nil
	},
}

var sandboxDenyPortCmd = &cobra.Command{
	Use:   "deny-port <port>",
	Short: "Remove a localhost port from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[0])
		}
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Sandbox.AllowedPorts = removeIntItem(cfg.Sandbox.AllowedPorts, port)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Removed port: %d\n", port)
		return nil
	},
}

var sandboxDomainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "List allowed domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedDomains) == 0 {
			fmt.Println("No domains configured. Provider defaults apply at runtime.")
			return nil
		}
		for _, d := range cfg.Sandbox.AllowedDomains {
			fmt.Println(d)
		}
		return nil
	},
}

var sandboxEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "List allowed env vars",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedEnv) == 0 {
			fmt.Println("No extra env vars configured. Base allowlist applies.")
			return nil
		}
		for _, v := range cfg.Sandbox.AllowedEnv {
			fmt.Println(v)
		}
		return nil
	},
}

var sandboxPortsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List allowed localhost ports",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedPorts) == 0 {
			fmt.Println("No extra ports configured. Only the proxy port (3128) is accessible.")
			return nil
		}
		for _, p := range cfg.Sandbox.AllowedPorts {
			fmt.Println(p)
		}
		return nil
	},
}

// makeSandboxConfigCmd is a factory that reduces boilerplate for simple config mutations.
func makeSandboxConfigCmd(mutate func(sb *config.SandboxConfig, args []string)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		mutate(&cfg.Sandbox, args)
		return config.Save(root, cfg)
	}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeItem(slice []string, item string) []string {
	var out []string
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}

func appendUniqueInt(slice []int, item int) []int {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeIntItem(slice []int, item int) []int {
	var out []int
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func init() {
	sandboxRunCmd.Flags().Bool("force-dir", false, "Skip directory safety checks")
	sandboxRunCmd.Flags().Bool("no-network", false, "Block all network egress (no proxy)")
	sandboxRunCmd.Flags().StringArray("allow-domain", nil, "Allow an additional domain for this session")
	sandboxRunCmd.Flags().StringArray("allow-env", nil, "Forward an additional env var into the sandbox")
	sandboxRunCmd.Flags().StringArray("allow-port", nil, "Allow a localhost port inside the sandbox")
	sandboxRunCmd.Flags().StringArray("mount-ro", nil, "Mount additional path read-only inside sandbox")

	sandboxCmd.AddCommand(
		sandboxRunCmd,
		sandboxCheckCmd,
		sandboxInfoCmd,
		sandboxAllowDomainCmd,
		sandboxDenyDomainCmd,
		sandboxAllowEnvCmd,
		sandboxDenyEnvCmd,
		sandboxAllowPortCmd,
		sandboxDenyPortCmd,
		sandboxDomainsCmd,
		sandboxEnvCmd,
		sandboxPortsCmd,
	)
	rootCmd.AddCommand(sandboxCmd)
}
```

**Success criteria:**
- [ ] `syllago sandbox --help` shows all subcommands
- [ ] `syllago sandbox check` runs without panic (even if bwrap missing, prints error)
- [ ] `syllago sandbox allow-domain foo.com` writes to `.syllago/config.json`
- [ ] `syllago sandbox domains` reads it back
- [ ] `make build` passes

---

### Task 7.2 — Registry add: sandbox allowlist prompt (SAND-003)

**Modifies:** `cli/cmd/syllago/registry_cmd.go` (the `registry add` command handler)

**Dependencies:** Task 1.3 (config schema)

SAND-003 mitigation: "Registry domains require explicit approval." When a user runs `syllago registry add <url>`, the registry's hostname is extracted and the user is prompted whether to add it to the sandbox allowlist. This prevents malicious registry content from silently gaining network access inside the sandbox.

First, add `"net/url"` to the import block of `registry_cmd.go`:

```go
import (
    "fmt"
    "net/url"    // ADD THIS — required for url.Parse below
    "os"
    "path/filepath"
    "strings"
    // ... existing internal imports unchanged ...
)
```

Then add the following prompt logic at the end of the `registry add` RunE handler, after the registry is successfully added:

```go
// After successful registry add, offer to add its domain to the sandbox allowlist.
registryURL := args[0]
parsed, err := url.Parse(registryURL)
if err == nil && parsed.Hostname() != "" {
	host := parsed.Hostname()
	fmt.Printf("\nSecurity: Syllago does not verify registry content. Registry servers can supply\n")
	fmt.Printf("hooks and MCP servers that run on your machine.\n")
	fmt.Printf("Sandbox: Add %s to the sandbox network allowlist? [y/N] ", host)
	var answer string
	fmt.Fscan(os.Stdin, &answer)
	if strings.ToLower(strings.TrimSpace(answer)) == "y" {
		// Inline uniqueness check (do not call appendUnique from sandbox_cmd.go).
		alreadyPresent := false
		for _, d := range cfg.Sandbox.AllowedDomains {
			if d == host {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			cfg.Sandbox.AllowedDomains = append(cfg.Sandbox.AllowedDomains, host)
		}
		if err := config.Save(root, cfg); err != nil {
			fmt.Printf("Warning: failed to save sandbox allowlist: %s\n", err)
		} else {
			fmt.Printf("Added %s to sandbox allowlist.\n", host)
		}
	}
}
```

**Important:** `"net/url"` is **not** currently imported in `registry_cmd.go`. Add it to the import block alongside the existing stdlib imports. `"strings"` and `"os"` are already present.

`appendUnique` is defined in `sandbox_cmd.go` (same `package main`), so it is accessible at compile time. However, to avoid a hidden inter-file dependency, inline the uniqueness check directly in this handler:

```go
// Inline uniqueness check (appendUnique is in sandbox_cmd.go but inlining is cleaner here):
alreadyPresent := false
for _, d := range cfg.Sandbox.AllowedDomains {
    if d == host {
        alreadyPresent = true
        break
    }
}
if !alreadyPresent {
    cfg.Sandbox.AllowedDomains = append(cfg.Sandbox.AllowedDomains, host)
}
```

**Success criteria:**
- [ ] `syllago registry add https://github.com/team/rules` prompts about sandbox allowlist
- [ ] Answering "y" adds `github.com` to `cfg.Sandbox.AllowedDomains` in `.syllago/config.json`
- [ ] Answering "n" (or Enter) skips without modification
- [ ] `make build` passes

---

## Phase 8 — TUI Integration

---

### Task 8.1 — Sandbox settings TUI screen

**Creates:** `cli/internal/tui/sandbox_settings.go`

**Dependencies:** Task 1.3 (config schema), existing `settings.go` patterns

The sandbox settings screen is a new content area in the TUI's Configuration section (alongside Import, Update, Settings, Registries). It follows the same `settingsModel` pattern: a model struct, `Update()`/`View()` methods, zone-marked rows for mouse support.

```go
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// sandboxSettingsModel manages the sandbox configuration TUI.
type sandboxSettingsModel struct {
	repoRoot string
	sb       config.SandboxConfig

	cursor int
	// editMode 0=none, 1=add-domain, 2=add-env, 3=add-port
	editMode  int
	editInput string

	message    string
	messageErr bool
	width      int
	height     int
}

func newSandboxSettingsModel(repoRoot string) sandboxSettingsModel {
	cfg, err := config.Load(repoRoot)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	return sandboxSettingsModel{
		repoRoot: repoRoot,
		sb:       cfg.Sandbox,
	}
}

const (
	sandboxRowDomains = iota
	sandboxRowEnv
	sandboxRowPorts
	sandboxRowCount
)

func (m sandboxSettingsModel) Update(msg tea.Msg) (sandboxSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""

		if m.editMode != 0 {
			switch {
			case msg.Type == tea.KeyEsc:
				m.editMode = 0
				m.editInput = ""
			case msg.Type == tea.KeyEnter:
				m.commitEdit()
				m.editMode = 0
				m.editInput = ""
				m.save()
			case msg.Type == tea.KeyBackspace:
				if len(m.editInput) > 0 {
					m.editInput = m.editInput[:len(m.editInput)-1]
				}
			default:
				m.editInput += msg.String()
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < sandboxRowCount-1 {
				m.cursor++
			}
		case msg.Type == tea.KeyEnter, key.Matches(msg, keys.Space):
			m.editMode = m.cursor + 1
			m.editInput = ""
		case msg.Type == tea.KeyDelete, msg.String() == "d":
			m.deleteSelected()
			m.save()
		case key.Matches(msg, keys.Save):
			m.save()
		}
	}
	return m, nil
}

func (m *sandboxSettingsModel) commitEdit() {
	val := strings.TrimSpace(m.editInput)
	if val == "" {
		return
	}
	switch m.editMode {
	case 1: // domain
		m.sb.AllowedDomains = appendUniqueTUI(m.sb.AllowedDomains, val)
	case 2: // env
		m.sb.AllowedEnv = appendUniqueTUI(m.sb.AllowedEnv, val)
	case 3: // port
		if p, err := strconv.Atoi(val); err == nil {
			m.sb.AllowedPorts = appendUniqueIntTUI(m.sb.AllowedPorts, p)
		}
	}
}

func (m *sandboxSettingsModel) deleteSelected() {
	// Delete the last item in the focused list as a simple MVP behavior.
	switch m.cursor {
	case sandboxRowDomains:
		if len(m.sb.AllowedDomains) > 0 {
			m.sb.AllowedDomains = m.sb.AllowedDomains[:len(m.sb.AllowedDomains)-1]
		}
	case sandboxRowEnv:
		if len(m.sb.AllowedEnv) > 0 {
			m.sb.AllowedEnv = m.sb.AllowedEnv[:len(m.sb.AllowedEnv)-1]
		}
	case sandboxRowPorts:
		if len(m.sb.AllowedPorts) > 0 {
			m.sb.AllowedPorts = m.sb.AllowedPorts[:len(m.sb.AllowedPorts)-1]
		}
	}
}

func (m *sandboxSettingsModel) save() {
	cfg, err := config.Load(m.repoRoot)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	cfg.Sandbox = m.sb
	if err := config.Save(m.repoRoot, cfg); err != nil {
		m.message = fmt.Sprintf("Save failed: %s", err)
		m.messageErr = true
	} else {
		m.message = "Sandbox settings saved"
		m.messageErr = false
	}
}

func (m sandboxSettingsModel) View() string {
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	s := home + helpStyle.Render(" > ") + titleStyle.Render("Sandbox") + "\n\n"

	labels := []string{"Allowed Domains", "Allowed Env Vars", "Allowed Ports"}
	values := []string{
		listOrNone(m.sb.AllowedDomains),
		listOrNone(m.sb.AllowedEnv),
		portsOrNone(m.sb.AllowedPorts),
	}

	for i := 0; i < sandboxRowCount; i++ {
		prefix := "   "
		style := itemStyle
		if i == m.cursor {
			prefix = " > "
			style = selectedItemStyle
		}
		row := fmt.Sprintf("%s%s  %s", prefix, style.Render(labels[i]), helpStyle.Render(values[i]))
		s += zone.Mark(fmt.Sprintf("sandbox-row-%d", i), row) + "\n"
	}

	if m.editMode != 0 {
		prompt := [...]string{"", "Add domain: ", "Add env var: ", "Add port: "}[m.editMode]
		s += "\n" + labelStyle.Render(prompt) + m.editInput + "_\n"
		s += helpStyle.Render("enter to save • esc cancel") + "\n"
	}

	if m.message != "" {
		s += "\n"
		if m.messageErr {
			s += errorMsgStyle.Render("Error: " + m.message)
		} else {
			s += successMsgStyle.Render("Done: " + m.message)
		}
		s += "\n"
	}

	s += "\n" + helpStyle.Render("up/down navigate • enter add • d delete last • s save • esc back")
	return s
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func portsOrNone(ports []int) string {
	if len(ports) == 0 {
		return "(none)"
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

func appendUniqueTUI(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func appendUniqueIntTUI(slice []int, item int) []int {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
```

**Success criteria:**
- [ ] New model compiles without error
- [ ] `make build` passes

---

### Task 8.2 — Wire sandbox settings into the App and sidebar

**Modifies:** `cli/internal/tui/app.go`, `cli/internal/tui/sidebar.go`

**Dependencies:** Task 8.1

Add `screenSandbox` to the `screen` enum, add `sandboxSettings sandboxSettingsModel` to `App`, add a "Sandbox" entry to the sidebar's Configuration section. Wire up navigation in `App.Update()` following the same pattern as `screenSettings`.

In `app.go`, add to the `screen` constants block:
```go
screenSandbox  // after screenRegistries
```

Add to `App` struct:
```go
sandboxSettings sandboxSettingsModel
```

In `App.Update()`, in the `WindowSizeMsg` handler, add:
```go
a.sandboxSettings.width = contentW
a.sandboxSettings.height = ph
```

In the `screenCategory` / `focusSidebar` / Enter handler, add after the `isRegistriesSelected()` block:
```go
if a.sidebar.isSandboxSelected() {
    a.sandboxSettings = newSandboxSettingsModel(a.catalog.RepoRoot)
    a.sandboxSettings.width = a.width - sidebarWidth - 1
    a.sandboxSettings.height = a.panelHeight()
    a.screen = screenSandbox
    a.focus = focusContent
    return a, nil
}
```

Add a `screenSandbox` case in the keyboard handler (analogous to `screenSettings`):
```go
case screenSandbox:
    if key.Matches(msg, keys.Back) {
        a.screen = screenCategory
        a.focus = focusSidebar
        return a, nil
    }
    var cmd tea.Cmd
    a.sandboxSettings, cmd = a.sandboxSettings.Update(msg)
    return a, cmd
```

Add to `App.View()` switch:
```go
case screenSandbox:
    contentView = a.sandboxSettings.View()
```

In `sidebar.go`, `totalItems()` returns `len(m.types) + 6` (adding 1 for Sandbox), and add a "Sandbox" entry after "Registries" in `utilItems` with index `len(m.types) + 5`. Add selector method:
```go
func (m sidebarModel) isSandboxSelected() bool { return m.cursor == len(m.types)+5 }
```

Update `breadcrumb()` in `app.go`:
```go
case screenSandbox:
    return "Sandbox"
```

**Success criteria:**
- [ ] "Sandbox" appears in sidebar Configuration section
- [ ] Navigating to it shows the `sandboxSettingsModel.View()`
- [ ] Esc returns to sidebar
- [ ] `make build` passes

---

### Task 8.3 — TUI: Mount profile display + launch sandbox from provider view

**Modifies:** `cli/internal/tui/detail.go` (the provider detail view — `provider_detail.go` does not exist; the correct file is `detail.go`, with rendering helpers in `detail_render.go`)

**Dependencies:** Tasks 8.1, 8.2, 6.2 (RunSession), Task 3.1 (ProfileFor)

The design specifies two TUI integration features for the provider detail view: (1) "Provider mount profile display" — showing the provider's staged config paths, binary, and allowed domains; and (2) "Launch sandbox from provider view" — a keybinding that runs the sandbox session.

**Provider mount profile display:**

In the provider detail view's `View()` method, after the existing provider info section, add a sandbox info block. Call `sandbox.ProfileFor` with a best-effort resolution (skip if binary not found) and render:

```go
// In View(): append sandbox mount profile info to provider detail output.
home, _ := os.UserHomeDir()
cwd, _ := os.Getwd()
if profile, err := sandbox.ProfileFor(m.provider.Slug, home, cwd); err == nil {
	s += "\n" + sectionStyle.Render("Sandbox Mount Profile") + "\n"
	s += fmt.Sprintf("  Binary:       %s\n", profile.BinaryExec)
	s += fmt.Sprintf("  Config files: %s\n", strings.Join(profile.GlobalConfigPaths, "\n                "))
	s += fmt.Sprintf("  API domains:  %s\n", strings.Join(profile.AllowedDomains, ", "))
	s += helpStyle.Render("  r  run in sandbox") + "\n"
} else {
	s += helpStyle.Render("\n  r  run in sandbox (binary not found: install provider first)") + "\n"
}
```

**Sandbox launch keybinding:**

Because `RunSession` is a blocking call that takes over stdin/stdout (the interactive provider session), the TUI must suspend itself before launching, similar to how editors like `vim` handle shell commands.

In the provider detail view's `Update()` handler, add a case for the sandbox launch key (`"r"` — not `"s"`, which is already `keys.Save`):

```go
case key.Matches(msg, keys.SandboxRun):
	// Suspend TUI and run the sandbox session for this provider.
	return m, tea.ExecProcess(exec.Command("syllago", "sandbox", "run", m.provider.Slug), func(err error) tea.Msg {
		return sandboxExitMsg{err: err}
	})
```

Add `sandboxExitMsg` type:
```go
type sandboxExitMsg struct{ err error }
```

Handle it in `App.Update()`. Note: `setError` does not exist as a method on `App` — use direct field assignment on `a.statusMessage` instead (the same pattern used throughout `app.go`):
```go
case sandboxExitMsg:
	if msg.err != nil {
		a.statusMessage = fmt.Sprintf("Sandbox session ended with error: %s", msg.err)
	}
	return a, nil
```

Add `SandboxRun` to the `keyMap` in `cli/internal/tui/keys.go`. Use `"r"` — `"s"` is already bound to `keys.Save` and would conflict:

First, add the field to the `keyMap` struct:
```go
SandboxRun key.Binding
```

Then add the binding to the `keys` var:
```go
SandboxRun: key.NewBinding(
	key.WithKeys("r"),
	key.WithHelp("r", "run in sandbox"),
),
```

**Note:** `tea.ExecProcess` (available in Bubbletea v0.23+) cleanly suspends and restores the TUI around an external process. Verify the project's Bubbletea version supports it before implementation.

**Success criteria:**
- [ ] Provider detail view shows "Sandbox Mount Profile" section with binary path, config files, and API domains
- [ ] If provider binary not found, section shows a "binary not found" note instead of erroring
- [ ] Pressing `s` on a provider detail screen launches `syllago sandbox run <slug>`
- [ ] TUI resumes cleanly after the sandbox session ends
- [ ] Error message shown if sandbox exits non-zero
- [ ] `make build` passes

---

## Phase 9 — Integration and Smoke Tests

---

### Task 9.1 — End-to-end smoke test (no bwrap required)

**Creates:** `cli/internal/sandbox/runner_test.go`

**Dependencies:** All sandbox package tasks

These tests exercise the runner's pre-launch logic without actually calling bwrap. They use the `bwrapRunner` injectable var already declared in `runner.go` (Task 6.2). **No modification to `runner.go` is needed** — the seam was already added there. In tests, simply reassign the package-level var before calling `RunSession`:

```go
// In your test:
bwrapRunner = func(ctx context.Context, args []string) error {
    return nil // simulate success without executing bwrap
}
```

```go
// TestRunSession_DirSafetyFails: RunSession with an unsafe dir returns DirSafetyError
// TestRunSession_CleansStaleDirs: stale /tmp/syllago-sandbox-* dirs are removed before start
// TestRunSession_StagingDirCleanedOnExit: staging dir does not exist after RunSession returns
// TestRunSession_ProxyStarted: proxy socket file exists between start and bwrap injection
// TestRunSession_EnvSummaryPrinted: stdout contains "Sandbox environment:"
```

**Success criteria:**
- [ ] All five tests pass
- [ ] `make test` passes

---

### Task 9.2 — CLI command integration tests

**Creates:** `cli/cmd/syllago/sandbox_cmd_test.go`

**Dependencies:** Task 7.1

Follows the pattern of existing `*_test.go` files in `cli/cmd/syllago/`.

```go
// TestSandboxCheckCmd_HelpFlag: "syllago sandbox check --help" exits 0
// TestSandboxAllowDomain_WritesConfig: after "allow-domain foo.com", config.Load returns it
// TestSandboxDenyDomain_RemovesFromConfig: deny-domain removes a previously added domain
// TestSandboxAllowPort_WritesConfig: after "allow-port 5432", config.Load returns 5432
// TestSandboxDomains_ListsConfigured: domains cmd prints configured domains
// TestSandboxEnv_ListsConfigured: env cmd prints configured env vars
// TestSandboxPorts_ListsConfigured: ports cmd prints configured ports
```

**Success criteria:**
- [ ] All seven tests pass
- [ ] `make test` passes

---

### Task 9.3 — Final build, vet, and format verification

**Modifies:** Nothing (verification only)

**Dependencies:** All previous tasks

```bash
cd /home/hhewett/.local/src/syllago && make fmt && make vet && make build && make test
```

**Success criteria:**
- [ ] `make fmt` reports no changes
- [ ] `make vet` exits 0
- [ ] `make build` exits 0
- [ ] `make test` exits 0 with no test failures

---

## Implementation Order Summary

| Phase | Tasks | Key Output |
|-------|-------|-----------|
| 1 | 1.1, 1.2, 1.3 | `dirsafety.go`, `envfilter.go`, config schema |
| 2 | 2.1, 2.2 | `proxy.go`, `bridge.go` |
| 3 | 3.1, 3.2 | `profile.go`, `gitwrapper.go` |
| 4 | 4.1 | `configdiff.go` (staging + diff + apply) |
| 5 | 5.1, 5.2 | `bwrap.go`, `check.go` |
| 6 | 6.1, 6.2 | `staging.go`, `runner.go` |
| 7 | 7.1, 7.2 | `sandbox_cmd.go` (all CLI subcommands) + registry add prompt (SAND-003) |
| 8 | 8.1, 8.2, 8.3 | `sandbox_settings.go` + app/sidebar wiring + launch-from-provider-view |
| 9 | 9.1, 9.2, 9.3 | Integration tests + final verification |

**Estimated line counts by file (from design doc):**

| File | Purpose | ~Lines |
|------|---------|--------|
| `cli/internal/sandbox/bwrap.go` | Arg construction | 300 |
| `cli/internal/sandbox/proxy.go` | HTTP CONNECT proxy | 200 |
| `cli/internal/sandbox/bridge.go` | socat wrapper script | 150 |
| `cli/internal/sandbox/runner.go` | Session orchestrator | 200 |
| `cli/internal/sandbox/profile.go` | Provider profiles + ecosystems | 150 |
| `cli/internal/sandbox/envfilter.go` | Env allowlist | 80 |
| `cli/internal/sandbox/dirsafety.go` | CWD validation | 100 |
| `cli/internal/sandbox/configdiff.go` | Copy-diff-approve | 150 |
| `cli/internal/sandbox/gitwrapper.go` | Git wrapper generator | 80 |
| `cli/internal/sandbox/check.go` | Pre-flight check | 100 |
| `cli/internal/sandbox/staging.go` | Staging dir lifecycle | 80 |
| `cli/cmd/syllago/sandbox_cmd.go` | CLI subcommands | 200 |
| `cli/internal/tui/sandbox_settings.go` | TUI settings screen | 150 |
| Tests | Unit + integration | 800 |
| **Total** | | **~2,740** |

## Key Design Decisions (Rationale Summary)

**Why UNIX socket + socat bridge instead of forwarding a TCP port:** `--unshare-net` gives the sandbox a fresh network namespace with no interfaces except loopback. A TCP socket on the host is not reachable from inside. UNIX sockets can be bind-mounted across namespace boundaries, making this the only practical approach short of a virtual NIC.

**Why copy-diff-approve instead of blocking config writes:** Provider config files need to be writable for the tool to save preferences, authentication tokens, and session state. Blocking writes breaks functionality. Instead, we let the tool write freely to the staged copy, then review changes on exit — giving users visibility and control over what persists.

**Why a git wrapper instead of relying on the proxy to block git:** If `github.com` is on the allowlist (common for npm packages), `git push` over HTTPS would succeed despite network policy. The wrapper is a defense-in-depth layer that blocks at the command level.

**Why allowlist over denylist for env vars:** A denylist requires enumerating every secret variable name — an unbounded problem. New tools introduce new conventions (e.g., `DOPPLER_TOKEN`, `OP_SESSION_*`). The allowlist model is conservative by default: if you haven't explicitly allowed a variable, it doesn't enter the sandbox.
