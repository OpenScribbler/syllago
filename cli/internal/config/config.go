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

// Registry types.
//
// Absent Type (empty string) means git-backed — this is the historical default
// and preserves back-compat with configs written before the MOAT work landed.
// New registries added via `syllago registry add` set Type explicitly.
const (
	RegistryTypeGit  = "git"
	RegistryTypeMOAT = "moat"
)

// SigningProfile is the issuer+subject tuple recorded at TOFU approval time
// for a MOAT registry. Compared on every Sync against the manifest's
// registry_signing_profile — a mismatch requires explicit re-approval (spec
// §Registry Signing + G-4). Name/operator changes in the manifest do NOT
// require re-approval; only this pair does.
type SigningProfile struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// IsZero reports whether no signing profile has been recorded yet. Used to
// distinguish "first time seeing this registry" (TOFU prompt) from "profile
// changed since last approval" (re-approval prompt).
func (s SigningProfile) IsZero() bool {
	return s.Issuer == "" && s.Subject == ""
}

// Equal reports exact issuer+subject match. Both fields must match — the
// signing profile is a pair, not a disjunction.
func (s SigningProfile) Equal(other SigningProfile) bool {
	return s.Issuer == other.Issuer && s.Subject == other.Subject
}

// Registry represents a content source registered in this project.
//
// Two backends: git (the original; default when Type is empty) and MOAT
// (manifest-based, signature-verified). MOAT fields are populated only when
// Type == RegistryTypeMOAT and are zero-valued for git registries.
type Registry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Ref  string `json:"ref,omitempty"` // branch/tag/commit, defaults to default branch (git only)

	// Type is the registry backend. Empty = git for back-compat; new
	// entries populate this explicitly.
	Type string `json:"type,omitempty"`

	// MOAT-only fields. Ignored (and MUST be zero) when Type != "moat".
	// SigningProfile is a pointer so unset profiles omit cleanly from JSON
	// (struct-value omitempty doesn't work — a zero struct still serializes
	// its empty fields).
	ManifestURI    string          `json:"manifest_uri,omitempty"`    // HTTPS URL of the MOAT manifest JSON
	SigningProfile *SigningProfile `json:"signing_profile,omitempty"` // TOFU-approved issuer+subject
	LastFetchedAt  *time.Time      `json:"last_fetched_at,omitempty"` // last successful manifest fetch
	Operator       string          `json:"operator,omitempty"`        // display label from manifest
	ManifestETag   string          `json:"manifest_etag,omitempty"`   // If-None-Match on next fetch

	Trust               string     `json:"trust,omitempty"`                 // "trusted", "verified", "community" (default: "community")
	Visibility          string     `json:"visibility,omitempty"`            // "public", "private", "unknown"
	VisibilityCheckedAt *time.Time `json:"visibility_checked_at,omitempty"` // for TTL cache (re-probe after 1 hour)
}

// IsMOAT reports whether this registry is MOAT-backed. Treats the zero/empty
// value of Type as git for back-compat with pre-MOAT configs.
func (r *Registry) IsMOAT() bool {
	return r.Type == RegistryTypeMOAT
}

// IsGit reports whether this registry is git-backed. Matches both the
// explicit "git" value and the empty default (pre-MOAT configs).
func (r *Registry) IsGit() bool {
	return r.Type == "" || r.Type == RegistryTypeGit
}

// NeedsSigningProfileReapproval reports whether syncing against `incoming`
// would require a re-approval prompt. Returns false when no signing profile
// has been recorded yet (that is a TOFU case, not a re-approval case) and
// when the incoming profile matches the recorded one exactly.
//
// Name/Operator changes alone do NOT trigger re-approval — they live
// elsewhere on the struct and are intentionally not consulted here. Only
// the signing-profile pair is trust-load-bearing.
func (r *Registry) NeedsSigningProfileReapproval(incoming SigningProfile) bool {
	if r.SigningProfile == nil || r.SigningProfile.IsZero() {
		return false
	}
	return !r.SigningProfile.Equal(incoming)
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

// GlobalDirOverride redirects global config to a test-controlled directory.
// Set in tests to prevent reading the real ~/.syllago/config.json.
var GlobalDirOverride string

// GlobalDirPath returns the global syllago config directory (~/.syllago/).
func GlobalDirPath() (string, error) {
	if GlobalDirOverride != "" {
		return GlobalDirOverride, nil
	}
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
