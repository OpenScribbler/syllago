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
