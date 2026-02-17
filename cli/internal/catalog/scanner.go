package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/metadata"
	"github.com/tidwall/gjson"
)

// validItemNameRe matches names safe for use in sjson/gjson key paths.
// Rejects . (path separator), * (wildcard), # (array modifier), | (pipe),
// spaces, and slashes.
var validItemNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// IsValidItemName checks if a name is safe for use as an sjson/gjson key.
func IsValidItemName(name string) bool {
	return validItemNameRe.MatchString(name)
}

// Scan walks the repo root and my-tools/ to discover all content items.
// It reads one directory level at a time using os.ReadDir for controlled traversal.
func Scan(repoRoot string) (*Catalog, error) {
	cat := &Catalog{RepoRoot: repoRoot}

	// Scan shared content (git-tracked)
	if err := scanRoot(cat, repoRoot, false); err != nil {
		return nil, err
	}

	// Scan local content (my-tools/, gitignored)
	myToolsDir := filepath.Join(repoRoot, "my-tools")
	if _, err := os.Stat(myToolsDir); err == nil {
		if err := scanRoot(cat, myToolsDir, true); err != nil {
			return nil, err
		}
	}

	return cat, nil
}

// scanRoot scans a base directory for content items of all types.
// If local is true, discovered items are marked as local (my-tools/).
func scanRoot(cat *Catalog, baseDir string, local bool) error {
	for _, ct := range AllContentTypes() {
		typeDir := filepath.Join(baseDir, string(ct))

		entries, err := os.ReadDir(typeDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue // type directory doesn't exist yet, skip
		}
		if err != nil {
			return err
		}

		if ct.IsUniversal() {
			if err := scanUniversal(cat, typeDir, ct, entries, local); err != nil {
				return err
			}
		} else {
			if err := scanProviderSpecific(cat, typeDir, ct, entries, local); err != nil {
				return err
			}
		}
	}
	return nil
}

// scanUniversal discovers items for universal types (skills, agents, prompts, mcp, apps).
// Each subdirectory inside the type directory is one item.
func scanUniversal(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool) error {
	for _, entry := range entries {
		if !entry.IsDir() || shouldSkip(entry.Name()) {
			continue
		}
		if !IsValidItemName(entry.Name()) {
			fmt.Fprintf(os.Stderr, "Warning: skipping item %q — name contains characters unsafe for JSON key paths\n", entry.Name())
			continue
		}

		itemDir := filepath.Join(typeDir, entry.Name())
		item := ContentItem{
			Name:  entry.Name(),
			Type:  ct,
			Path:  itemDir,
			Local: local,
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
			if item.Description == "" {
				item.Description = readDescription(filepath.Join(itemDir, "README.md"))
			}
		case Prompts:
			// Try to parse PROMPT.md for name, description, and body.
			promptPath := filepath.Join(itemDir, "PROMPT.md")
			data, err := os.ReadFile(promptPath)
			if err == nil {
				fm, body, fmErr := ParseFrontmatterWithBody(data)
				if fmErr == nil {
					if fm.Name != "" {
						item.DisplayName = fm.Name
					}
					item.Description = fm.Description
					item.Body = body
				}
			}
			if item.Description == "" {
				item.Description = readDescription(filepath.Join(itemDir, "README.md"))
			}
		case Apps:
			readmePath := filepath.Join(itemDir, "README.md")
			data, err := os.ReadFile(readmePath)
			if err == nil {
				fm, body, fmErr := ParseFrontmatterWithBody(data)
				if fmErr == nil {
					if fm.Name != "" {
						item.DisplayName = fm.Name
					}
					if fm.Description != "" {
						item.Description = fm.Description
					}
					item.SupportedProviders = fm.Providers
					item.Body = body
				} else {
					item.Description = readDescription(readmePath)
					item.Body = string(data)
				}
			}
		default:
			// For other universal types, try README.md for description.
			item.Description = readDescription(filepath.Join(itemDir, "README.md"))
		}

		// Load README.md into ReadmeBody (for all universal types)
		item.ReadmeBody = loadReadme(itemDir)

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
//   - Directory-per-item (new): <type>/<provider>/<item-name>/ with content file + README.md + .romanesco.yaml
//   - Single file (legacy):    <type>/<provider>/<file> with .romanesco.<file>.yaml alongside
func scanProviderSpecific(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool) error {
	for _, providerEntry := range entries {
		if !providerEntry.IsDir() || shouldSkip(providerEntry.Name()) {
			continue
		}

		providerDir := filepath.Join(typeDir, providerEntry.Name())
		providerName := providerEntry.Name()

		children, err := os.ReadDir(providerDir)
		if err != nil {
			return err
		}

		for _, child := range children {
			if shouldSkip(child.Name()) {
				continue
			}
			if child.IsDir() && !IsValidItemName(child.Name()) {
				fmt.Fprintf(os.Stderr, "Warning: skipping item %q — name contains characters unsafe for JSON key paths\n", child.Name())
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
					Local:    local,
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
// Looks for the content file by type convention and loads README + metadata.
func scanProviderDir(itemDir string, ct ContentType, providerName string, local bool) (*ContentItem, error) {
	dirName := filepath.Base(itemDir)
	item := ContentItem{
		Name:     dirName,
		Type:     ct,
		Path:     itemDir,
		Provider: providerName,
		Local:    local,
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
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" && e.Name() != "LLM-PROMPT.md" {
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
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" && e.Name() != "LLM-PROMPT.md" {
					item.Description = readDescription(filepath.Join(itemDir, e.Name()))
					break
				}
			}
		}
	}

	// Load README.md
	item.ReadmeBody = loadReadme(itemDir)

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

	// Warn to stderr if README.md is missing (non-fatal)
	if item.ReadmeBody == "" {
		fmt.Fprintf(os.Stderr, "warning: %s/%s/%s missing README.md\n", ct, providerName, dirName)
	}

	return &item, nil
}

// shouldSkip returns true for files/dirs that should always be ignored.
func shouldSkip(name string) bool {
	if name == ".gitkeep" || name == "README.md" || name == "LLM-PROMPT.md" {
		return true
	}
	if name == metadata.FileName || strings.HasPrefix(name, ".romanesco.") {
		return true
	}
	return false
}

// loadReadme reads README.md from an item directory.
// Returns the raw content or "" if not found.
func loadReadme(itemDir string) string {
	data, err := os.ReadFile(filepath.Join(itemDir, "README.md"))
	if err != nil {
		return ""
	}
	return string(data)
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
		// Skip hidden files and directories (e.g. .romanesco.yaml, .git)
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
// Handles Claude Code format: {"hooks":{"Event":[{"matcher":"..."}]}}
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
