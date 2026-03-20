package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

// validItemNameRe matches names safe for use as directory names and sjson/gjson
// key paths. Allows letters, numbers, hyphens, and underscores. Must not start
// with a dash (avoids flag-like names and potential CLI confusion).
var validItemNameRe = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]*$`)

// maxItemNameLen is the maximum allowed length for user-entered names.
const maxItemNameLen = 100

// IsValidItemName checks if a name is safe for use as a directory name and
// sjson/gjson key. Returns false for empty names, names starting with a dash,
// names longer than 100 characters, or names containing dots, path separators,
// spaces, or other special characters.
func IsValidItemName(name string) bool {
	return len(name) <= maxItemNameLen && validItemNameRe.MatchString(name)
}

// ValidateUserName checks a user-entered name and returns a human-readable
// error message. Returns empty string if the name is valid. Use this in UI
// code where the user needs to know *why* their name was rejected.
func ValidateUserName(name string) string {
	if name == "" {
		return "name is required"
	}
	if len(name) > maxItemNameLen {
		return fmt.Sprintf("name must be %d characters or fewer", maxItemNameLen)
	}
	if name[0] == '-' {
		return "name must not start with a dash"
	}
	if !validItemNameRe.MatchString(name) {
		return "name may only contain letters, numbers, hyphens, and underscores"
	}
	return ""
}

// validRegistryNameRe matches registry names in owner/repo format.
// Allows letters, numbers, - and _ in each segment, with an optional / separator.
var validRegistryNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)?$`)

// IsValidRegistryName checks if a name is valid for use as a registry name.
// Allows the owner/repo format in addition to plain names.
func IsValidRegistryName(name string) bool {
	return validRegistryNameRe.MatchString(name)
}

// validateRegistryPath resolves symlinks in path and checks that the resolved
// path stays within registryRoot. This prevents symlink-based path traversal
// when scanning untrusted registry directories. registryRoot must already be
// resolved (no symlinks).
func validateRegistryPath(path, registryRoot string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}
	if !strings.HasPrefix(resolved, registryRoot+string(filepath.Separator)) && resolved != registryRoot {
		return fmt.Errorf("path escapes registry boundary: %s resolves to %s", path, resolved)
	}
	return nil
}

// Scan walks contentRoot to discover all content items.
// contentRoot is the directory containing shared content directories (skills/, agents/, etc.).
// projectRoot is unused but kept for API compatibility.
func Scan(contentRoot string, projectRoot string) (*Catalog, error) {
	cat := &Catalog{RepoRoot: contentRoot}

	// Scan shared content (git-tracked)
	if err := scanRoot(cat, contentRoot, false); err != nil {
		return nil, err
	}

	applyPrecedence(cat)
	return cat, nil
}

// ScanWithRegistries scans contentRoot plus the global library plus any provided
// registry sources. Registry items are tagged with their registry name.
// Per-registry scan errors are logged to stderr but do not fail the overall scan.
func ScanWithRegistries(contentRoot string, projectRoot string, registries []RegistrySource) (*Catalog, error) {
	// Start with the standard scan (local + shared repo items)
	cat, err := Scan(contentRoot, projectRoot)
	if err != nil {
		return nil, err
	}

	// Append items from each registry
	for _, reg := range registries {
		before := len(cat.Items)
		if err := scanRoot(cat, reg.Path, false); err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("registry %q scan error: %s", reg.Name, err))
			continue
		}
		// Tag all newly-appended items with the registry name
		for i := before; i < len(cat.Items); i++ {
			cat.Items[i].Registry = reg.Name
		}
	}

	applyPrecedence(cat)
	return cat, nil
}

// ScanRegistriesOnly scans only the provided registry sources without a local repo scan.
func ScanRegistriesOnly(registries []RegistrySource) (*Catalog, error) {
	cat := &Catalog{}
	for _, reg := range registries {
		before := len(cat.Items)
		if err := scanRoot(cat, reg.Path, false); err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("registry %q scan error: %s", reg.Name, err))
			continue
		}
		for i := before; i < len(cat.Items); i++ {
			cat.Items[i].Registry = reg.Name
		}
	}
	return cat, nil
}

// manifestItem mirrors registry.ManifestItem for use within the catalog
// package. It is intentionally unexported to avoid an import cycle: the
// registry package already imports catalog (for IsValidRegistryName), so
// catalog cannot import registry.
type manifestItem struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	Provider  string   `yaml:"provider"`
	Path      string   `yaml:"path"`
	HookEvent string   `yaml:"hookEvent,omitempty"`
	HookIndex int      `yaml:"hookIndex,omitempty"`
	Scripts   []string `yaml:"scripts,omitempty"`
}

type catalogManifest struct {
	Items []manifestItem `yaml:"items"`
}

// loadManifestItems reads registry.yaml from dir and returns its items list.
// Returns nil, nil if the file does not exist (manifest is optional).
func loadManifestItems(dir string) ([]manifestItem, error) {
	data, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry.yaml in %q: %w", dir, err)
	}
	var m catalogManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing registry.yaml in %q: %w", dir, err)
	}
	return m.Items, nil
}

// Per-registry scanning limits to prevent resource exhaustion from malicious registries.
const (
	maxScanItems = 10000  // Maximum items per registry scan
	maxScanDepth = 50     // Maximum directory depth
	maxScanDirs  = 100000 // Maximum directory entries read per registry scan
)

// scanRoot scans a base directory for content items of all types.
// If local is true, discovered items are marked as Library items.
func scanRoot(cat *Catalog, baseDir string, local bool) error {
	// Resolve the base directory once so symlink checks inside scanners
	// compare against a canonical, symlink-free root path.
	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return fmt.Errorf("resolving base directory: %w", err)
	}

	beforeCount := len(cat.Items)

	// Check for registry.yaml with indexed items; if present, use index-based scan.
	items, _ := loadManifestItems(baseDir)
	if len(items) > 0 {
		if len(items) > maxScanItems {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("registry at %s has %d manifest items, exceeding limit of %d; truncating", baseDir, len(items), maxScanItems))
			items = items[:maxScanItems]
		}
		return scanFromIndex(cat, baseDir, resolvedBase, items, local)
	}

	for _, ct := range AllContentTypes() {
		typeDir := filepath.Join(baseDir, string(ct))

		entries, err := os.ReadDir(typeDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue // type directory doesn't exist yet, skip
		}
		if err != nil {
			return err
		}

		// Enforce per-registry item limit
		if len(cat.Items)-beforeCount >= maxScanItems {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("registry at %s exceeded %d item limit; skipping remaining content types", baseDir, maxScanItems))
			break
		}

		if ct.IsUniversal() {
			if err := scanUniversal(cat, typeDir, ct, entries, local, resolvedBase); err != nil {
				return err
			}
		} else {
			if err := scanProviderSpecific(cat, typeDir, ct, entries, local, resolvedBase); err != nil {
				return err
			}
		}
	}
	return nil
}

// scanFromIndex builds ContentItems from a registry manifest's Items list.
// Each manifestItem maps a name/type/provider to a relative path in the registry.
// Missing paths produce a warning and are skipped (not an error), so a partial
// index doesn't abort the whole scan. resolvedBase is the symlink-resolved
// base directory used for path traversal checks.
func scanFromIndex(cat *Catalog, baseDir string, resolvedBase string, items []manifestItem, local bool) error {
	for _, mi := range items {
		itemPath := filepath.Join(baseDir, mi.Path)
		ct := ContentType(mi.Type)

		// Validate the path stays within the registry boundary.
		if err := validateRegistryPath(itemPath, resolvedBase); err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("index item %q: %s, skipping", mi.Name, err))
			continue
		}

		info, err := os.Stat(itemPath)
		if err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("index item %q: path %q not found, skipping", mi.Name, mi.Path))
			continue
		}

		item := ContentItem{
			Name:     mi.Name,
			Type:     ct,
			Path:     itemPath,
			Provider: mi.Provider,
			Library:  local,
		}

		if info.IsDir() {
			// Directory items: extract metadata based on type.
			switch ct {
			case Skills:
				data, readErr := os.ReadFile(filepath.Join(itemPath, "SKILL.md"))
				if readErr == nil {
					fm, fmErr := ParseFrontmatter(data)
					if fmErr == nil {
						if fm.Name != "" {
							item.DisplayName = fm.Name
						}
						item.Description = fm.Description
					}
				}
			case Agents:
				data, readErr := os.ReadFile(filepath.Join(itemPath, "AGENT.md"))
				if readErr == nil {
					fm, fmErr := ParseFrontmatter(data)
					if fmErr == nil {
						if fm.Name != "" {
							item.DisplayName = fm.Name
						}
						item.Description = fm.Description
					}
				}
			case Hooks:
				hookPath := filepath.Join(itemPath, "hook.json")
				data, readErr := os.ReadFile(hookPath)
				if readErr == nil {
					item.Description = describeHookJSON(data)
				}
			case Commands:
				item.Description = readDescription(filepath.Join(itemPath, "command.md"))
				if item.Description == "" {
					entries, _ := os.ReadDir(itemPath)
					for _, e := range entries {
						if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "LLM-PROMPT.md" {
							item.Description = readDescription(filepath.Join(itemPath, e.Name()))
							break
						}
					}
				}
			case Rules:
				item.Description = readDescription(filepath.Join(itemPath, "rule.md"))
				if item.Description == "" {
					entries, _ := os.ReadDir(itemPath)
					for _, e := range entries {
						if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "LLM-PROMPT.md" {
							item.Description = readDescription(filepath.Join(itemPath, e.Name()))
							break
						}
					}
				}
			}
			item.Files = collectFiles(itemPath, itemPath)

			// Load .syllago.yaml metadata if present (directory items only).
			meta, metaErr := metadata.Load(itemPath)
			if metaErr == nil {
				item.Meta = meta
			}
		} else {
			// Single file items.
			switch ct {
			case Agents:
				data, readErr := os.ReadFile(itemPath)
				if readErr == nil {
					fm, fmErr := ParseFrontmatter(data)
					if fmErr == nil {
						if fm.Name != "" {
							item.DisplayName = fm.Name
						}
						item.Description = fm.Description
					}
				}
			case Rules, Commands:
				item.Description = readDescription(itemPath)
			case Hooks:
				data, readErr := os.ReadFile(itemPath)
				if readErr == nil {
					item.Description = describeHookJSON(data)
				}
			}
			item.Files = []string{filepath.Base(itemPath)}
		}

		cat.Items = append(cat.Items, item)
	}
	return nil
}

// scanUniversal discovers items for universal types (skills, agents, mcp).
// Each subdirectory inside the type directory is one item. resolvedBase is the
// symlink-resolved registry root used for path traversal checks.
func scanUniversal(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool, resolvedBase string) error {
	for _, entry := range entries {
		if !entry.IsDir() || shouldSkip(entry.Name()) {
			continue
		}
		if !IsValidItemName(entry.Name()) {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — name contains characters unsafe for JSON key paths", entry.Name()))
			continue
		}

		itemDir := filepath.Join(typeDir, entry.Name())

		// Validate the item directory stays within the registry boundary.
		if err := validateRegistryPath(itemDir, resolvedBase); err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q: %s", entry.Name(), err))
			continue
		}
		item := ContentItem{
			Name:    entry.Name(),
			Type:    ct,
			Path:    itemDir,
			Library: local,
		}

		switch ct {
		case Skills:
			// Try to parse SKILL.md for name and description.
			skillPath := filepath.Join(itemDir, "SKILL.md")
			data, err := os.ReadFile(skillPath)
			if err == nil {
				fm, fmErr := ParseFrontmatter(data)
				if fmErr == nil {
					if fm.Name != "" {
						item.DisplayName = fm.Name
					}
					item.Description = fm.Description
				}
			}
		case Agents:
			// Try to parse AGENT.md for name and description.
			agentPath := filepath.Join(itemDir, "AGENT.md")
			data, err := os.ReadFile(agentPath)
			if err == nil {
				fm, fmErr := ParseFrontmatter(data)
				if fmErr == nil {
					if fm.Name != "" {
						item.DisplayName = fm.Name
					}
					item.Description = fm.Description
				}
			}
		default:
			// For other universal types, no additional description extraction.
		}

		// Collect file listing
		item.Files = collectFiles(itemDir, itemDir)

		// Load metadata if present
		meta, err := metadata.Load(itemDir)
		if err != nil {
			return err
		}
		item.Meta = meta

		cat.Items = append(cat.Items, item)
	}
	return nil
}

// scanProviderSpecific discovers items for provider-specific types (rules, hooks, commands).
// Supports two layouts:
//   - Directory-per-item (new): <type>/<provider>/<item-name>/ with content file + .syllago.yaml
//   - Single file (legacy):    <type>/<provider>/<file> with .syllago.<file>.yaml alongside
//
// resolvedBase is the symlink-resolved registry root used for path traversal checks.
func scanProviderSpecific(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool, resolvedBase string) error {
	for _, providerEntry := range entries {
		if !providerEntry.IsDir() || shouldSkip(providerEntry.Name()) {
			continue
		}

		providerDir := filepath.Join(typeDir, providerEntry.Name())
		providerName := providerEntry.Name()

		// Validate the provider directory stays within the registry boundary.
		if err := validateRegistryPath(providerDir, resolvedBase); err != nil {
			cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping provider %q: %s", providerName, err))
			continue
		}

		children, err := os.ReadDir(providerDir)
		if err != nil {
			return err
		}

		for _, child := range children {
			if shouldSkip(child.Name()) {
				continue
			}
			if child.IsDir() && !IsValidItemName(child.Name()) {
				cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q — name contains characters unsafe for JSON key paths", child.Name()))
				continue
			}

			// Validate child path stays within the registry boundary.
			childPath := filepath.Join(providerDir, child.Name())
			if err := validateRegistryPath(childPath, resolvedBase); err != nil {
				cat.Warnings = append(cat.Warnings, fmt.Sprintf("skipping item %q: %s", child.Name(), err))
				continue
			}

			if child.IsDir() {
				// New directory-per-item format
				item, err := scanProviderDir(filepath.Join(providerDir, child.Name()), ct, providerName, local)
				if err != nil {
					return err
				}
				if item != nil {
					cat.Items = append(cat.Items, *item)
				}
			} else {
				// Legacy single-file format
				filePath := filepath.Join(providerDir, child.Name())
				item := ContentItem{
					Name:     child.Name(),
					Type:     ct,
					Path:     filePath,
					Provider: providerName,
					Library:  local,
				}
				if strings.HasSuffix(child.Name(), ".md") {
					item.Description = readDescription(filePath)
				}
				if filepath.Ext(child.Name()) == ".json" {
					data, readErr := os.ReadFile(filePath)
					if readErr == nil {
						item.Description = describeHookJSON(data)
					}
				}
				meta, metaErr := metadata.LoadProvider(providerDir, child.Name())
				if metaErr != nil {
					return metaErr
				}
				item.Meta = meta
				if meta != nil && meta.Description != "" {
					item.Description = meta.Description
				}
				cat.Items = append(cat.Items, item)
			}
		}
	}
	return nil
}

// scanProviderDir scans a directory-format provider-specific item.
// Looks for the content file by type convention and loads metadata.
func scanProviderDir(itemDir string, ct ContentType, providerName string, local bool) (*ContentItem, error) {
	dirName := filepath.Base(itemDir)
	item := ContentItem{
		Name:     dirName,
		Type:     ct,
		Path:     itemDir,
		Provider: providerName,
		Library:  local,
	}

	// Find the content file by type
	switch ct {
	case Rules:
		// Look for rule.md or any .md file
		item.Description = readDescription(filepath.Join(itemDir, "rule.md"))
		if item.Description == "" {
			// Try any .md file
			entries, _ := os.ReadDir(itemDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "LLM-PROMPT.md" {
					item.Description = readDescription(filepath.Join(itemDir, e.Name()))
					break
				}
			}
		}
	case Hooks:
		// Look for hook.json or any .json file
		hookPath := filepath.Join(itemDir, "hook.json")
		data, err := os.ReadFile(hookPath)
		if err != nil {
			// Try any .json file
			entries, _ := os.ReadDir(itemDir)
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
					data, err = os.ReadFile(filepath.Join(itemDir, e.Name()))
					break
				}
			}
		}
		if err == nil && data != nil {
			item.Description = describeHookJSON(data)
		}
	case Commands:
		// Look for command.md or any .md file
		item.Description = readDescription(filepath.Join(itemDir, "command.md"))
		if item.Description == "" {
			entries, _ := os.ReadDir(itemDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "LLM-PROMPT.md" {
					item.Description = readDescription(filepath.Join(itemDir, e.Name()))
					break
				}
			}
		}
	case Loadouts:
		// Load loadout.yaml for description
		loadoutPath := filepath.Join(itemDir, "loadout.yaml")
		data, readErr := os.ReadFile(loadoutPath)
		if readErr == nil {
			// Quick YAML parse for description field
			var parsed struct {
				Description string `yaml:"description"`
			}
			if yamlErr := yaml.Unmarshal(data, &parsed); yamlErr == nil && parsed.Description != "" {
				item.Description = parsed.Description
			}
		}
	}

	// Collect file listing
	item.Files = collectFiles(itemDir, itemDir)

	// Load metadata from inside the item directory
	meta, err := metadata.Load(itemDir)
	if err != nil {
		return nil, err
	}
	item.Meta = meta

	// Use metadata description as primary source (overrides content-file parsing)
	if meta != nil && meta.Description != "" {
		item.Description = meta.Description
	}

	return &item, nil
}

// shouldSkip returns true for files/dirs that should always be ignored.
func shouldSkip(name string) bool {
	if name == ".gitkeep" || name == "LLM-PROMPT.md" {
		return true
	}
	if name == metadata.FileName || strings.HasPrefix(name, ".syllago.") {
		return true
	}
	return false
}

// collectFiles returns relative paths of all non-hidden files in an item directory.
// Walks recursively to match the behavior of installer.CopyContent.
func collectFiles(itemDir string, baseDir string) []string {
	var files []string
	filepath.WalkDir(itemDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		// Skip hidden files and directories (e.g. .syllago.yaml, .git)
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Only include files, not directories
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files
}

// describeHookJSON generates a description from hook JSON content.
// Handles canonical format: {"hooks":{"Event":[{"matcher":"..."}]}}
// Falls back to flat format: {"event":"...", "matcher":"..."}
func describeHookJSON(data []byte) string {
	// Try Claude Code hooks format: {"hooks":{"PostToolUse":[...]}}
	hooksObj := gjson.GetBytes(data, "hooks")
	if hooksObj.Exists() && hooksObj.IsObject() {
		var events []string
		hooksObj.ForEach(func(key, _ gjson.Result) bool {
			events = append(events, key.String())
			return true
		})
		if len(events) > 0 {
			return strings.Join(events, ", ") + " hook"
		}
	}
	// Fall back to flat format
	event := gjson.GetBytes(data, "event").String()
	matcher := gjson.GetBytes(data, "matcher").String()
	if event != "" {
		if matcher != "" {
			return fmt.Sprintf("%s hook for %s", event, matcher)
		}
		return fmt.Sprintf("%s hook", event)
	}
	return ""
}

// readDescription reads a markdown file and returns the first non-empty,
// non-heading line as a description. Returns "" on any error or if no
// description line is found.
func readDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}

	return ""
}

// GlobalContentDirOverride overrides the default global content directory for testing.
// When non-empty, GlobalContentDir returns this value instead of ~/.syllago/content.
var GlobalContentDirOverride string

// GlobalContentDir returns the path to the global syllago content directory.
// Returns "" if home directory cannot be determined.
func GlobalContentDir() string {
	if GlobalContentDirOverride != "" {
		return GlobalContentDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".syllago", "content")
}

// ScanWithGlobalAndRegistries scans the global content dir, project content,
// and registry sources. Global items are tagged Source="global", project items
// Source="project". Project items shadow global items of the same name+type.
func ScanWithGlobalAndRegistries(contentRoot string, projectRoot string, registries []RegistrySource) (*Catalog, error) {
	// Scan project content first (takes precedence)
	cat, err := ScanWithRegistries(contentRoot, projectRoot, registries)
	if err != nil {
		return nil, err
	}

	// Tag existing items with their source
	for i := range cat.Items {
		if cat.Items[i].Source == "" {
			if cat.Items[i].Library {
				cat.Items[i].Source = "library"
			} else if cat.Items[i].Registry != "" {
				cat.Items[i].Source = cat.Items[i].Registry
			} else {
				cat.Items[i].Source = "project"
			}
		}
	}

	// Scan global content dir
	globalDir := GlobalContentDir()
	if globalDir == "" {
		return cat, nil
	}
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		return cat, nil
	}

	globalCat := &Catalog{RepoRoot: globalDir}
	if err := scanRoot(globalCat, globalDir, false); err != nil {
		cat.Warnings = append(cat.Warnings, fmt.Sprintf("global content scan error: %s", err))
		return cat, nil
	}
	// Merge warnings from global scan into main catalog
	cat.Warnings = append(cat.Warnings, globalCat.Warnings...)

	// Tag global items and append only those not already in project
	projectNames := make(map[string]bool)
	for _, item := range cat.Items {
		projectNames[string(item.Type)+"/"+item.Name] = true
	}

	// applyPrecedence must run before processing global items so that its
	// replacement of cat.Overridden does not discard global shadow entries.
	applyPrecedence(cat)

	// Re-build projectNames from the post-precedence Items list.
	projectNames = make(map[string]bool)
	for _, item := range cat.Items {
		projectNames[string(item.Type)+"/"+item.Name] = true
	}

	for i := range globalCat.Items {
		globalCat.Items[i].Source = "global"
		globalCat.Items[i].Library = true
		key := string(globalCat.Items[i].Type) + "/" + globalCat.Items[i].Name
		if !projectNames[key] {
			cat.Items = append(cat.Items, globalCat.Items[i])
		} else {
			// Project item takes precedence; keep global item in Overridden
			// so CleanupPromotedItems can find and remove it if needed.
			cat.Overridden = append(cat.Overridden, globalCat.Items[i])
		}
	}
	return cat, nil
}

// PrintWarnings writes any collected scan warnings to stderr.
// CLI commands should call this after scanning; TUI code should not.
func (c *Catalog) PrintWarnings() {
	for _, w := range c.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
}
