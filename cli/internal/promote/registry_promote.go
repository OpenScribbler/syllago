package promote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// RegistryResult holds the outcome of a promote-to-registry operation.
type RegistryResult struct {
	Branch     string
	PRUrl      string // empty if gh not available or PR creation failed
	CompareURL string // fallback browser URL for manual PR creation
}

// PromoteToRegistry copies a library content item into a registry clone, creates a
// contribution branch, commits, pushes, and optionally opens a PR via the gh CLI.
//
// This is the external-contribution workflow: the user forks a registry, clones it
// locally, and this function creates a branch with their content ready to PR upstream.
// noInput suppresses interactive prompts even on a TTY (e.g. when --no-input is passed).
func PromoteToRegistry(repoRoot string, registryName string, item catalog.ContentItem, noInput bool) (*RegistryResult, error) {
	// Privacy gate G1: block private content from being published to public registries.
	if gateErr := CheckPrivacyGate(item, registryName, repoRoot); gateErr != nil {
		return nil, gateErr
	}

	// Privacy gate G4: for loadouts, check if contained items have private taint.
	if item.Type == catalog.Loadouts {
		if gateErr := checkLoadoutItemTaint(item, registryName, repoRoot); gateErr != nil {
			return nil, gateErr
		}
	}

	// 1. Get registry clone directory
	cloneDir, err := registry.CloneDir(registryName)
	if err != nil {
		return nil, fmt.Errorf("resolving registry path: %w", err)
	}
	if _, err := os.Stat(cloneDir); err != nil {
		return nil, fmt.Errorf("registry %q is not cloned locally (run `syllago registry add` first)", registryName)
	}

	// 2. Determine destination path within the registry.
	// Universal types (skills, agents, mcp): type/name
	// Provider-specific types (rules, hooks, commands): type/provider/name
	var destPath string
	if item.Type.IsUniversal() {
		destPath = filepath.Join(cloneDir, string(item.Type), item.Name)
	} else {
		if item.Provider == "" {
			return nil, fmt.Errorf("provider-specific content (%s) requires a provider field", item.Type)
		}
		destPath = filepath.Join(cloneDir, string(item.Type), item.Provider, item.Name)
	}

	// 3. Check item doesn't already exist in the registry
	if _, err := os.Stat(destPath); err == nil {
		return nil, fmt.Errorf("item already exists in registry at %s", destPath)
	}

	// 4. Detect default branch of the registry repo
	defaultBranch := detectDefaultBranch(cloneDir)

	// 5. Create contribution branch
	branchName := fmt.Sprintf("syllago/contribute/%s/%s", item.Type, item.Name)
	if err := gitRun(cloneDir, "checkout", "-b", branchName); err != nil {
		// Branch might already exist — append timestamp
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
		if err := gitRun(cloneDir, "checkout", "-b", branchName); err != nil {
			return nil, fmt.Errorf("creating branch: %w", err)
		}
	}

	// On any error after branch creation, return to default branch
	cleanup := func() {
		_ = gitRun(cloneDir, "checkout", defaultBranch)
	}

	// 6. Copy content using the shared helper (excludes scaffold artifacts like LLM-PROMPT.md)
	if err := copyForPromote(item.Path, destPath); err != nil {
		cleanup()
		return nil, fmt.Errorf("copying content: %w", err)
	}

	// 7. Git add + commit
	if err := gitRun(cloneDir, "add", destPath); err != nil {
		cleanup()
		return nil, fmt.Errorf("staging files: %w", err)
	}
	commitMsg := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
	if err := gitRun(cloneDir, "commit", "-m", commitMsg); err != nil {
		cleanup()
		return nil, fmt.Errorf("committing: %w", err)
	}

	// 8. Push to origin
	if err := gitRun(cloneDir, "push", "-u", "origin", branchName); err != nil {
		cleanup()
		return nil, fmt.Errorf("pushing: %w", err)
	}

	result := &RegistryResult{Branch: branchName}

	// 9. If gh CLI is available, try to create a PR
	if ghPath, err := exec.LookPath("gh"); err == nil && ghPath != "" {
		prTitle := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
		prBody := fmt.Sprintf("Contributes `%s` to the registry.\n\nType: %s",
			item.Name, item.Type)
		out, err := commandOutput(cloneDir, "gh", "pr", "create",
			"--title", prTitle,
			"--body", prBody,
			"--base", defaultBranch)
		if err == nil {
			result.PRUrl = strings.TrimSpace(out)
		}
	}

	// 10. Construct compare URL as fallback
	result.CompareURL = buildCompareURL(cloneDir, branchName)

	// Return to default branch
	_ = gitRun(cloneDir, "checkout", defaultBranch)

	return result, nil
}

// checkLoadoutItemTaint resolves the items referenced by a loadout manifest
// and checks if any have private taint. Blocks if the target registry is public.
func checkLoadoutItemTaint(item catalog.ContentItem, registryName, repoRoot string) error {
	// Parse the loadout manifest from the item's directory.
	manifestPath := filepath.Join(item.Path, "loadout.yaml")
	m, err := loadout.Parse(manifestPath)
	if err != nil {
		return nil // can't parse manifest — skip G4 check (G1 already passed)
	}

	// Determine target registry visibility.
	targetVis := registry.VisibilityUnknown
	cfg, err := config.Load(repoRoot)
	if err == nil {
		for _, r := range cfg.Registries {
			if r.Name == registryName {
				targetVis = r.Visibility
				break
			}
		}
	}
	if registry.IsPrivate(targetVis) {
		return nil // publishing to private registry is always fine
	}

	// Resolve referenced items to catalog entries and check their taint.
	globalDir := catalog.GlobalContentDir()
	var items []catalog.ContentItem
	for ct, refs := range m.RefsByType() {
		for _, ref := range refs {
			itemDir := resolveItemDir(globalDir, ct, m.Provider, ref.Name)
			meta, _ := metadata.Load(itemDir)
			items = append(items, catalog.ContentItem{
				Name: ref.Name,
				Type: ct,
				Meta: meta,
			})
		}
	}

	return loadout.CheckLoadoutPublishGate(items, targetVis)
}

// resolveItemDir returns the library directory for an item.
func resolveItemDir(globalDir string, ct catalog.ContentType, provider, name string) string {
	if ct.IsUniversal() {
		return filepath.Join(globalDir, string(ct), name)
	}
	return filepath.Join(globalDir, string(ct), provider, name)
}
