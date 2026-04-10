package installer

import (
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Conflict describes an install-path collision between providers.
// It occurs when one provider's InstallDir resolves to the same path
// that one or more other providers read via GlobalSharedReadPaths.
// Installing to all of these providers simultaneously would cause
// duplicate content and conflict warnings in the reading providers.
type Conflict struct {
	// SharedPath is the filesystem path that multiple providers use.
	SharedPath string
	// InstallingTo is the provider whose InstallDir resolves to SharedPath.
	InstallingTo provider.Provider
	// AlsoReadBy are the other selected providers that also read from SharedPath.
	AlsoReadBy []provider.Provider
}

// ConflictResolution represents how the user wants to handle an install-path conflict.
type ConflictResolution int

const (
	// ResolutionSharedOnly installs only to the shared path (e.g. ~/.agents/skills/).
	// The installer provider writes there; reader providers are skipped since they
	// will pick up the content automatically via their shared read path.
	ResolutionSharedOnly ConflictResolution = iota
	// ResolutionOwnDirsOnly installs to each provider's own canonical directory.
	// The provider that owns the shared path is skipped; readers each get their
	// own install with no write to the shared path.
	ResolutionOwnDirsOnly
	// ResolutionAll installs to all providers (current behavior).
	// May produce duplicate content warnings in providers that read the shared path.
	ResolutionAll
)

// ApplyConflictResolution filters the provider list based on the user's chosen
// resolution strategy. Providers not involved in any conflict are always kept.
func ApplyConflictResolution(providers []provider.Provider, conflicts []Conflict, resolution ConflictResolution) []provider.Provider {
	if len(conflicts) == 0 || resolution == ResolutionAll {
		return providers
	}

	// Build the set of slugs to remove.
	remove := make(map[string]bool)
	for _, c := range conflicts {
		switch resolution {
		case ResolutionSharedOnly:
			// Remove readers — they get content via the shared path.
			for _, r := range c.AlsoReadBy {
				remove[r.Slug] = true
			}
		case ResolutionOwnDirsOnly:
			// Remove the installer — it writes to the shared path.
			remove[c.InstallingTo.Slug] = true
		}
	}

	result := make([]provider.Provider, 0, len(providers))
	for _, p := range providers {
		if !remove[p.Slug] {
			result = append(result, p)
		}
	}
	return result
}

// DetectConflicts checks whether any pair of providers in the given slice
// would result in content being written to a path that another provider also
// reads, for the given content type. Conflicts are grouped by shared path.
// Returns nil if there are no conflicts.
func DetectConflicts(providers []provider.Provider, ct catalog.ContentType, homeDir string) []Conflict {
	// Build a map of install path → provider for all selected providers.
	installPaths := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		if p.InstallDir == nil {
			continue
		}
		dir := p.InstallDir(homeDir, ct)
		if dir == "" || dir == provider.JSONMergeSentinel || dir == provider.ProjectScopeSentinel {
			continue
		}
		installPaths[dir] = p
	}

	// For each provider that declares GlobalSharedReadPaths, check whether any
	// of those paths match another provider's InstallDir. Group results by path
	// so multiple readers on the same path produce a single Conflict.
	byPath := make(map[string]*Conflict)
	for _, p := range providers {
		if p.GlobalSharedReadPaths == nil {
			continue
		}
		for _, sp := range p.GlobalSharedReadPaths(homeDir, ct) {
			installer, ok := installPaths[sp]
			if !ok || installer.Slug == p.Slug {
				continue
			}
			if _, exists := byPath[sp]; !exists {
				byPath[sp] = &Conflict{
					SharedPath:   sp,
					InstallingTo: installer,
				}
			}
			byPath[sp].AlsoReadBy = append(byPath[sp].AlsoReadBy, p)
		}
	}

	if len(byPath) == 0 {
		return nil
	}

	result := make([]Conflict, 0, len(byPath))
	for _, c := range byPath {
		result = append(result, *c)
	}
	return result
}
