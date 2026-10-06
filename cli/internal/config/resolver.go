package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// PathResolver resolves content paths with a priority chain:
// CLI --base-dir flag > per-type config path > config baseDir > default.
type PathResolver struct {
	ProviderPaths map[string]ProviderPathConfig
	CLIBaseDir    string // per-invocation --base-dir override
}

// NewResolver creates a PathResolver from merged config and an optional CLI flag.
func NewResolver(cfg *Config, cliBaseDir string) *PathResolver {
	r := &PathResolver{CLIBaseDir: cliBaseDir}
	if cfg != nil {
		r.ProviderPaths = cfg.ProviderPaths
	}
	return r
}

// InstallDir resolves the install directory for a content type.
// Priority: per-type path > CLI --base-dir > config baseDir > default (homeDir).
func (r *PathResolver) InstallDir(prov provider.Provider, ct catalog.ContentType, homeDir string) string {
	if r != nil {
		// Per-type path: bypass provider logic entirely
		if path, ok := r.perTypePath(prov.Slug, ct); ok {
			cleaned := filepath.Clean(path)
			// Validate that user-configured path is not a sensitive system path.
			// If sensitive, skip the override and fall through to defaults.
			if !isSensitivePath(cleaned) {
				return cleaned
			}
		}
		// CLI --base-dir
		if r.CLIBaseDir != "" {
			return prov.InstallDir(filepath.Clean(r.CLIBaseDir), ct)
		}
		// Config baseDir
		if baseDir := r.configBaseDir(prov.Slug); baseDir != "" {
			return prov.InstallDir(filepath.Clean(baseDir), ct)
		}
	}
	legacy := prov.InstallDir(homeDir, ct)
	if legacy == provider.ProjectScopeSentinel {
		return legacy
	}
	if dir, ok := matrixInstallDir(prov.Slug, ct, homeDir, runtime.GOOS, os.Getenv("APPDATA")); ok {
		return dir
	}
	return legacy
}

// DiscoveryPaths resolves discovery paths for a content type.
// Priority: per-type path > CLI --base-dir > config baseDir > default (projectRoot).
func (r *PathResolver) DiscoveryPaths(prov provider.Provider, ct catalog.ContentType, projectRoot string) []string {
	if r != nil {
		// Per-type path: return directly, bypasses DiscoveryPaths entirely
		if path, ok := r.perTypePath(prov.Slug, ct); ok {
			return []string{filepath.Clean(path)}
		}
		// CLI --base-dir
		if r.CLIBaseDir != "" {
			return prov.DiscoveryPaths(filepath.Clean(r.CLIBaseDir), ct)
		}
		// Config baseDir
		if baseDir := r.configBaseDir(prov.Slug); baseDir != "" {
			return prov.DiscoveryPaths(filepath.Clean(baseDir), ct)
		}
	}
	return prov.DiscoveryPaths(projectRoot, ct)
}

// HasPerTypePath returns true if a per-type override is configured for this provider/type.
func (r *PathResolver) HasPerTypePath(slug string, ct catalog.ContentType) bool {
	if r == nil {
		return false
	}
	_, ok := r.perTypePath(slug, ct)
	return ok
}

// BaseDir returns the effective base directory for a provider.
// Priority: CLI --base-dir > config baseDir > "".
// Empty string means use the default (home dir).
func (r *PathResolver) BaseDir(slug string) string {
	if r == nil {
		return ""
	}
	if r.CLIBaseDir != "" {
		return r.CLIBaseDir
	}
	return r.configBaseDir(slug)
}

// perTypePath returns the per-type override path for a provider, if configured.
func (r *PathResolver) perTypePath(slug string, ct catalog.ContentType) (string, bool) {
	if r.ProviderPaths == nil {
		return "", false
	}
	ppc, ok := r.ProviderPaths[slug]
	if !ok || ppc.Paths == nil {
		return "", false
	}
	path, ok := ppc.Paths[string(ct)]
	if !ok || path == "" {
		return "", false
	}
	return path, true
}

// configBaseDir returns the config-level baseDir for a provider, if configured.
func (r *PathResolver) configBaseDir(slug string) string {
	if r.ProviderPaths == nil {
		return ""
	}
	return r.ProviderPaths[slug].BaseDir
}

// sensitiveRoots are system directories that should never be install targets.
var sensitiveRoots = []string{"/etc", "/usr", "/var", "/bin", "/sbin", "/lib", "/boot", "/sys", "/proc", "/dev"}

// isSensitivePath checks if a path points to a known sensitive system directory.
func isSensitivePath(path string) bool {
	if path == "/" {
		return true
	}
	for _, root := range sensitiveRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// ValidateProviderPath checks whether a path is a safe install target.
// Exported for use by the doctor command and config validation.
func ValidateProviderPath(path string) error {
	cleaned := filepath.Clean(path)
	if isSensitivePath(cleaned) {
		return fmt.Errorf("provider path %s points to a sensitive system directory", cleaned)
	}
	return nil
}

// ExpandPaths resolves tilde prefixes in all stored paths and makes them
// absolute against the working directory, which is where a relative path
// would resolve anyway. Call this after constructing the resolver.
func (r *PathResolver) ExpandPaths() error {
	if r == nil {
		return nil
	}
	// A loadout records where it places content and refuses a relative
	// path, which would delete from wherever remove later runs.
	// An empty per-type path means unset, not the working directory.
	expand := func(p string) (string, error) {
		if p == "" {
			return "", nil
		}
		expanded, err := ExpandHome(p)
		if err != nil {
			return "", err
		}
		return filepath.Abs(expanded)
	}

	if r.CLIBaseDir != "" {
		expanded, err := expand(r.CLIBaseDir)
		if err != nil {
			return err
		}
		r.CLIBaseDir = expanded
	}

	for slug, ppc := range r.ProviderPaths {
		if ppc.BaseDir != "" {
			expanded, err := expand(ppc.BaseDir)
			if err != nil {
				return err
			}
			ppc.BaseDir = expanded
		}
		for ct, path := range ppc.Paths {
			expanded, err := expand(path)
			if err != nil {
				return err
			}
			ppc.Paths[ct] = expanded
		}
		r.ProviderPaths[slug] = ppc
	}
	return nil
}
