package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// CacheDirOverride overrides the default cache directory for testing.
// When non-empty, CacheDir returns this value instead of ~/.syllago/registries.
var CacheDirOverride string

// CacheDir returns the global registry cache directory (~/.syllago/registries).
func CacheDir() (string, error) {
	if CacheDirOverride != "" {
		return CacheDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".syllago", "registries"), nil
}

// CloneDir returns the path where a named registry is cloned.
func CloneDir(name string) (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, name), nil
}

// IsCloned returns true if the registry clone directory exists.
func IsCloned(name string) bool {
	dir, err := CloneDir(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(dir)
	return err == nil
}

// NameFromURL derives a registry name from a git URL.
// Returns "owner/repo" format when the URL has an org/owner prefix,
// or just "repo" for bare single-segment names.
// Examples:
//
//	"git@github.com:acme/my-tools.git"          → "acme/my-tools"
//	"https://github.com/acme/my-tools"           → "acme/my-tools"
//	"https://github.com/acme/my-tools.git"       → "acme/my-tools"
//	"https://example.com/my-tools.git"           → "my-tools"
func NameFromURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// HTTPS format: https://host/owner/repo or https://host/repo
	// Check this BEFORE SSH to avoid matching the colon in "https:"
	if i := strings.Index(url, "://"); i >= 0 {
		path := url[i+3:]
		// strip host
		if j := strings.Index(path, "/"); j >= 0 {
			path = path[j+1:]
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
		if len(parts) == 1 && parts[0] != "" {
			return parts[0]
		}
	}

	// git@ SSH format: git@host:owner/repo
	if i := strings.Index(url, ":"); i >= 0 {
		path := url[i+1:]
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
		return parts[len(parts)-1]
	}

	// Fallback: last segment after any / or :
	last := url
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		last = url[i+1:]
	}
	return last
}

// checkGit returns an error if git is not on PATH.
func checkGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is required for registry operations but was not found on PATH")
	}
	return nil
}

// Clone clones the given URL into the registry cache as name.
// If ref is non-empty, checks out that branch/tag after cloning.
func Clone(url, name, ref string) error {
	if !catalog.IsValidRegistryName(name) {
		return fmt.Errorf("registry name %q is invalid (use letters, numbers, - and _ with optional owner/repo format)", name)
	}
	if err := checkGit(); err != nil {
		return err
	}

	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("creating registry cache: %w", err)
	}

	args := cloneArgs(url, dir, ref)
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up partial clone
		_ = os.RemoveAll(dir)
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// cloneArgs builds the git argument list for a secure clone.
// It disables hooks via core.hooksPath and prevents submodule recursion.
// GIT_CONFIG_NOSYSTEM=1 must also be set on the command's Env by the caller.
func cloneArgs(url, dir, ref string) []string {
	args := []string{
		"-c", "core.hooksPath=/dev/null",
		"clone",
		"--no-recurse-submodules",
		url, dir,
	}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	return args
}

// Sync runs git pull --ff-only in the registry clone directory.
// Returns an error if the clone does not exist or git pull fails.
func Sync(name string) error {
	if err := checkGit(); err != nil {
		return err
	}
	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	cmd := exec.Command("git",
		"-C", dir,
		"-c", "core.hooksPath=/dev/null",
		"pull", "--ff-only", "--no-recurse-submodules",
	)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed for %q: %s\n(Hint: delete the clone at ~/.syllago/registries/%s and re-run `syllago registry add`)", name, strings.TrimSpace(string(out)), name)
	}
	return nil
}

// SyncResult holds the outcome of a single registry sync.
type SyncResult struct {
	Name string
	Err  error
}

// SyncAll syncs all registries concurrently (up to 4 at a time) and returns results.
func SyncAll(names []string) []SyncResult {
	results := make([]SyncResult, len(names))
	sem := make(chan struct{}, 4) // max 4 concurrent syncs

	done := make(chan struct{}, len(names))
	for i, name := range names {
		go func(i int, name string) {
			sem <- struct{}{}
			results[i] = SyncResult{Name: name, Err: Sync(name)}
			<-sem
			done <- struct{}{}
		}(i, name)
	}
	for range names {
		<-done
	}
	return results
}

// Remove deletes the registry clone directory.
func Remove(name string) error {
	dir, err := CloneDir(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// ManifestItem maps a provider-native content item to its location in the repo.
type ManifestItem struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`                // skills, agents, rules, hooks, commands, mcp
	Provider  string   `yaml:"provider"`            // provider slug
	Path      string   `yaml:"path"`                // relative to repo root
	HookEvent string   `yaml:"hookEvent,omitempty"` // for hooks: event name
	HookIndex int      `yaml:"hookIndex,omitempty"` // for hooks: index in event array
	Scripts   []string `yaml:"scripts,omitempty"`   // for hooks: associated script files

	// Extended fields (populated by analyzer; optional in authored manifests)
	DisplayName  string   `yaml:"displayName,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	ContentHash  string   `yaml:"contentHash,omitempty"`
	References   []string `yaml:"references,omitempty"`
	ConfigSource string   `yaml:"configSource,omitempty"`
	Providers    []string `yaml:"providers,omitempty"`
}

// Manifest holds optional metadata from registry.yaml at the registry root.
// Its purpose is display-only: teams can describe their registry for the TUI
// and CLI output. Registries without a manifest still work normally.
type Manifest struct {
	Name              string         `yaml:"name"`
	Description       string         `yaml:"description,omitempty"`
	Maintainers       []string       `yaml:"maintainers,omitempty"`
	Version           string         `yaml:"version,omitempty"`
	MinSyllagoVersion string         `yaml:"min_syllago_version,omitempty"`
	Items             []ManifestItem `yaml:"items,omitempty"`
	Visibility        string         `yaml:"visibility,omitempty"` // "public", "private" — declared by registry owner
}

// LoadManifest reads registry.yaml from the clone directory for the named registry.
// Returns nil, nil if the file does not exist (manifest is optional).
func LoadManifest(name string) (*Manifest, error) {
	dir, err := CloneDir(name)
	if err != nil {
		return nil, err
	}
	return LoadManifestFromDir(dir)
}

// KnownAliases maps short names to full git URLs.
// Empty by default — syllago is a platform, not a content source.
// Users bring their own registries.
var KnownAliases = map[string]string{}

// ExpandAlias returns the full URL for a known alias, or the input unchanged if not an alias.
// An alias is identified by not containing "/" or ":" — these characters appear in all valid git URLs.
func ExpandAlias(input string) (url string, expanded bool) {
	if !strings.Contains(input, "/") && !strings.Contains(input, ":") {
		if full, ok := KnownAliases[input]; ok {
			return full, true
		}
	}
	return input, false
}

// LoadManifestFromDir reads registry.yaml from an explicit directory path.
// Exported so callers (e.g., the native scanner) can load a manifest without
// needing a named registry clone — useful for local directories and tests.
func LoadManifestFromDir(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry.yaml in %q: %w", dir, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing registry.yaml in %q: %w", dir, err)
	}
	return &m, nil
}
