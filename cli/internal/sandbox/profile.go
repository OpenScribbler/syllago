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
	// ExtraPATH are directories to prepend to the sandbox PATH (e.g. node bin dir).
	ExtraPATH []string
	// ProviderEnvVars are provider-specific env var names to forward from the host.
	ProviderEnvVars []string
	// SandboxEnvVars are env vars to inject into the sandbox (not from the host).
	SandboxEnvVars map[string]string
	// ProtectFiles are glob patterns (relative to staged config dirs) for files that
	// should be made read-only in staging to prevent deletion by the provider.
	// Useful for credential files that providers delete after migrating to keychains.
	ProtectFiles []string
	// SkipDiffFiles are glob patterns for staged files whose changes should be
	// silently discarded after the session. Used for state files (counters,
	// metrics, OAuth tokens) that change every session but aren't meaningful config.
	SkipDiffFiles []string
	// AuthHint is an optional message printed before launch, advising users
	// about authentication requirements for the sandbox.
	AuthHint string
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
		SkipDiffFiles:   []string{".claude.json", ".claude"},
		AllowedDomains:  []string{"api.anthropic.com", "console.anthropic.com", "mcp-proxy.anthropic.com", "sentry.io"},
	}, nil
}

func geminiProfile(homeDir, projectDir string) (*MountProfile, error) {
	bin, binPaths, extraPATH, err := resolveNodeBinary("gemini")
	if err != nil {
		return nil, fmt.Errorf("gemini binary not found: %w", err)
	}
	return &MountProfile{
		GlobalConfigPaths: []string{
			filepath.Join(homeDir, ".gemini"),
		},
		ProjectConfigDirs: []string{
			filepath.Join(projectDir, ".gemini"),
		},
		BinaryPaths:     binPaths,
		BinaryExec:      bin,
		ExtraPATH:       extraPATH,
		ProviderEnvVars: []string{"GOOGLE_API_KEY", "GEMINI_API_KEY"},
		SandboxEnvVars:  map[string]string{"GEMINI_FORCE_FILE_STORAGE": "true"},
		SkipDiffFiles:   []string{".gemini"},
		ProtectFiles:    []string{"oauth_creds.json"},
		AuthHint:        "Gemini CLI stores OAuth tokens in your system keychain, which the sandbox\ncannot access. Make sure you've authenticated with 'gemini' outside the sandbox\nfirst -- the sandbox will use your cached credentials from ~/.gemini/oauth_creds.json.",
		AllowedDomains:  []string{"generativelanguage.googleapis.com", "oauth2.googleapis.com", "accounts.google.com", "cloudcode-pa.googleapis.com"},
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

// resolveNodeBinary resolves a Node.js CLI tool (installed via npm) and returns
// the script path, all paths to mount, and extra PATH directories.
// Mounts the entire node installation directory (the prefix containing bin/node)
// so that all globally installed packages and their dependencies are available.
func resolveNodeBinary(name string) (string, []string, []string, error) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%q not found on PATH", name)
	}
	resolved, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolving symlinks for %q: %w", bin, err)
	}

	// Find the node installation prefix: walk up from the resolved script to find
	// the directory containing bin/node. This covers the entire node installation
	// including all globally installed packages and their dependencies.
	// e.g. .../node/23.11.1/lib/node_modules/@google/gemini-cli/dist/index.js
	//   -> .../node/23.11.1/ (contains bin/node, lib/node_modules/*)
	nodePrefix, err := findNodePrefix(resolved)
	if err != nil {
		return "", nil, nil, fmt.Errorf("node installation not found for %q: %w", name, err)
	}
	nodeBinDir := filepath.Join(nodePrefix, "bin")

	return resolved, []string{nodePrefix}, []string{nodeBinDir}, nil
}

// findNodePrefix walks up from a resolved script path to find the node installation
// prefix — the directory containing bin/node.
func findNodePrefix(scriptPath string) (string, error) {
	dir := filepath.Dir(scriptPath)
	for dir != "/" {
		candidate := filepath.Join(dir, "bin", "node")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	// Fallback: try to find node on PATH and use its prefix.
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return "", fmt.Errorf("node not found on PATH")
	}
	resolved, err := filepath.EvalSymlinks(nodeBin)
	if err != nil {
		return "", err
	}
	// node binary is at <prefix>/bin/node
	return filepath.Dir(filepath.Dir(resolved)), nil
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
