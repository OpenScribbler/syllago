package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const DirName = ".syllago"
const FileName = "config.json"

// Registry represents a git-based content source registered in this project.
type Registry struct {
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	Ref                 string     `json:"ref,omitempty"`                   // branch/tag/commit, defaults to default branch
	Trust               string     `json:"trust,omitempty"`                 // "trusted", "verified", "community" (default: "community")
	Visibility          string     `json:"visibility,omitempty"`            // "public", "private", "unknown"
	VisibilityCheckedAt *time.Time `json:"visibility_checked_at,omitempty"` // for TTL cache (re-probe after 1 hour)
}

// ProviderPathConfig holds custom path overrides for a single provider.
// BaseDir replaces the default home directory as the root for provider paths.
// Paths maps content type names (e.g., "skills") to absolute directory paths,
// bypassing the provider's directory structure entirely.
type ProviderPathConfig struct {
	BaseDir string            `json:"base_dir,omitempty"`
	Paths   map[string]string `json:"paths,omitempty"` // keyed by content type (e.g., "skills")
}

// SandboxConfig holds project-level sandbox policy.
// Git-tracked so teams share the same sandbox settings.
type SandboxConfig struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	AllowedEnv     []string `json:"allowed_env,omitempty"`
	AllowedPorts   []int    `json:"allowed_ports,omitempty"`
}

type Config struct {
	Providers         []string                      `json:"providers"`              // enabled provider slugs
	ContentRoot       string                        `json:"content_root,omitempty"` // relative path to content directory (default: project root)
	Registries        []Registry                    `json:"registries,omitempty"`
	AllowedRegistries []string                      `json:"allowed_registries,omitempty"` // URL allowlist; empty means any URL is permitted
	Preferences       map[string]string             `json:"preferences,omitempty"`
	Sandbox           SandboxConfig                 `json:"sandbox,omitempty"`
	ProviderPaths     map[string]ProviderPathConfig `json:"provider_paths,omitempty"` // keyed by provider slug
}

// IsRegistryAllowed returns true if url is permitted given the config.
// When AllowedRegistries is empty, any URL is allowed (solo-user default).
// When non-empty, url must appear in the list (exact string match).
func (c *Config) IsRegistryAllowed(url string) bool {
	if len(c.AllowedRegistries) == 0 {
		return true
	}
	for _, allowed := range c.AllowedRegistries {
		if allowed == url {
			return true
		}
	}
	return false
}

func DirPath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName)
}

func FilePath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName, FileName)
}

func Load(projectRoot string) (*Config, error) {
	data, err := os.ReadFile(FilePath(projectRoot))
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(projectRoot string, cfg *Config) error {
	dir := DirPath(projectRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file then rename
	target := FilePath(projectRoot)
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tempPath := target + ".tmp." + hex.EncodeToString(suffix)

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func Exists(projectRoot string) bool {
	_, err := os.Stat(FilePath(projectRoot))
	return err == nil
}

// GlobalDirPath returns the global syllago config directory (~/.syllago/).
func GlobalDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// GlobalFilePath returns the path to the global config file (~/.syllago/config.json).
func GlobalFilePath() (string, error) {
	dir, err := GlobalDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// LoadGlobal loads the global config from ~/.syllago/config.json.
// Returns an empty Config if the file does not exist.
func LoadGlobal() (*Config, error) {
	path, err := GlobalFilePath()
	if err != nil {
		return nil, fmt.Errorf("global config path: %w", err)
	}
	return LoadFromPath(path)
}

// LoadFromPath loads a config from an explicit file path.
// Returns an empty Config if the file does not exist.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveGlobal writes cfg to ~/.syllago/config.json, creating the directory if needed.
func SaveGlobal(cfg *Config) error {
	dir, err := GlobalDirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(dir, FileName)
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tempPath := target + ".tmp." + hex.EncodeToString(suffix)

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

// Merge combines global and project configs.
// Rules:
//   - Providers: project wins if non-empty, else global
//   - Registries: global + project (deduplicated by name, project entries after global)
//   - ContentRoot: project wins if non-empty, else global
//   - AllowedRegistries: project wins if non-empty, else global
//   - Preferences: merged per-key, project overrides global
//   - Sandbox: project wins if any sandbox fields set, else global
func Merge(global, project *Config) *Config {
	if global == nil {
		global = &Config{}
	}
	if project == nil {
		project = &Config{}
	}

	merged := &Config{}

	// Providers: project wins if set
	if len(project.Providers) > 0 {
		merged.Providers = project.Providers
	} else {
		merged.Providers = global.Providers
	}

	// Registries: merge both (global first, then project), deduplicate by name
	seen := map[string]bool{}
	for _, r := range global.Registries {
		if !seen[r.Name] {
			merged.Registries = append(merged.Registries, r)
			seen[r.Name] = true
		}
	}
	for _, r := range project.Registries {
		if !seen[r.Name] {
			merged.Registries = append(merged.Registries, r)
			seen[r.Name] = true
		}
	}

	// ContentRoot: project wins
	if project.ContentRoot != "" {
		merged.ContentRoot = project.ContentRoot
	} else {
		merged.ContentRoot = global.ContentRoot
	}

	// AllowedRegistries: project wins
	if len(project.AllowedRegistries) > 0 {
		merged.AllowedRegistries = project.AllowedRegistries
	} else {
		merged.AllowedRegistries = global.AllowedRegistries
	}

	// Preferences: merge per-key, project overrides
	if len(global.Preferences) > 0 || len(project.Preferences) > 0 {
		merged.Preferences = make(map[string]string)
		for k, v := range global.Preferences {
			merged.Preferences[k] = v
		}
		for k, v := range project.Preferences {
			merged.Preferences[k] = v
		}
	}

	// Sandbox: project wins if non-zero
	if len(project.Sandbox.AllowedDomains) > 0 ||
		len(project.Sandbox.AllowedEnv) > 0 ||
		len(project.Sandbox.AllowedPorts) > 0 {
		merged.Sandbox = project.Sandbox
	} else {
		merged.Sandbox = global.Sandbox
	}

	// ProviderPaths: deep merge per-provider, project overrides within each
	if len(global.ProviderPaths) > 0 || len(project.ProviderPaths) > 0 {
		merged.ProviderPaths = make(map[string]ProviderPathConfig)
		for slug, gpc := range global.ProviderPaths {
			merged.ProviderPaths[slug] = gpc
		}
		for slug, ppc := range project.ProviderPaths {
			existing := merged.ProviderPaths[slug]
			if ppc.BaseDir != "" {
				existing.BaseDir = ppc.BaseDir
			}
			if len(ppc.Paths) > 0 {
				if existing.Paths == nil {
					existing.Paths = make(map[string]string)
				}
				for k, v := range ppc.Paths {
					existing.Paths[k] = v
				}
			}
			merged.ProviderPaths[slug] = existing
		}
	}

	return merged
}
