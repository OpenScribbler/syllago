package config

import (
	"path/filepath"

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
			return filepath.Clean(path)
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
	return prov.InstallDir(homeDir, ct)
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

// ExpandPaths resolves tilde prefixes in all stored paths.
// Call this after constructing the resolver to ensure paths are absolute.
func (r *PathResolver) ExpandPaths() error {
	if r == nil {
		return nil
	}

	if r.CLIBaseDir != "" {
		expanded, err := ExpandHome(r.CLIBaseDir)
		if err != nil {
			return err
		}
		r.CLIBaseDir = expanded
	}

	for slug, ppc := range r.ProviderPaths {
		if ppc.BaseDir != "" {
			expanded, err := ExpandHome(ppc.BaseDir)
			if err != nil {
				return err
			}
			ppc.BaseDir = expanded
		}
		for ct, path := range ppc.Paths {
			expanded, err := ExpandHome(path)
			if err != nil {
				return err
			}
			ppc.Paths[ct] = expanded
		}
		r.ProviderPaths[slug] = ppc
	}
	return nil
}
