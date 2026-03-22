package promote

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// CheckPrivacyGate implements Gate G1: blocks publishing private content to a public registry.
// It checks both the content's taint (belt) and the target registry's current visibility (suspenders).
func CheckPrivacyGate(item catalog.ContentItem, targetRegistry, repoRoot string) error {
	// Belt: check content taint
	if item.Meta == nil || item.Meta.SourceRegistry == "" || !registry.IsPrivate(item.Meta.SourceVisibility) {
		return nil // public or untainted content can go anywhere
	}

	// Suspenders: check target registry visibility (live probe if stale)
	targetVisibility := registry.VisibilityUnknown
	cfg, err := config.Load(repoRoot)
	if err == nil {
		for _, r := range cfg.Registries {
			if r.Name == targetRegistry {
				targetVisibility = r.Visibility
				// Re-probe if stale
				if registry.NeedsReprobe(r.VisibilityCheckedAt) {
					if vis, probeErr := registry.ProbeVisibility(r.URL); probeErr == nil {
						targetVisibility = vis
					}
				}
				break
			}
		}
	}

	if !registry.IsPrivate(targetVisibility) {
		// Target is public, content is private → BLOCK
		return fmt.Errorf("cannot publish %q to registry %q\n\n"+
			"  Content origin:  %s (private)\n"+
			"  Target registry: %s (public)\n\n"+
			"  Private content cannot be published to public registries.\n"+
			"  Remove the private taint by recreating the content in your\n"+
			"  library without the private registry association.",
			item.Name, targetRegistry, item.Meta.SourceRegistry, targetRegistry)
	}

	return nil // private→private is fine
}

// CheckSharePrivacyGate implements Gate G2: blocks sharing private content to a public repo.
// Probes the current repo's remote URL to determine visibility.
func CheckSharePrivacyGate(item catalog.ContentItem, repoRoot string) error {
	// Check content taint
	if item.Meta == nil || item.Meta.SourceRegistry == "" || !registry.IsPrivate(item.Meta.SourceVisibility) {
		return nil // not tainted
	}

	// Probe the current repo's visibility via its remote URL
	remoteURL, err := commandOutput(repoRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return nil // can't determine remote → allow (fail-open for share)
	}
	remoteURL = strings.TrimSpace(remoteURL)

	repoVis, _ := registry.ProbeVisibility(remoteURL)
	if !registry.IsPrivate(repoVis) {
		// Repo is public, content is private → BLOCK
		return fmt.Errorf("cannot share %q to this repository\n\n"+
			"  Content origin: %s (private)\n"+
			"  Target repo:    %s (public)\n\n"+
			"  Private content cannot be shared to public repositories.\n"+
			"  Remove the private taint by recreating the content in your\n"+
			"  library without the private registry association.",
			item.Name, item.Meta.SourceRegistry, remoteURL)
	}

	return nil
}
