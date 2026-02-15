package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/metadata"
	"github.com/tidwall/gjson"
)

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
// Structure is <type>/<provider>/<file>. Each file inside a provider subdir is one item.
func scanProviderSpecific(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool) error {
	for _, providerEntry := range entries {
		if !providerEntry.IsDir() || shouldSkip(providerEntry.Name()) {
			continue
		}

		providerDir := filepath.Join(typeDir, providerEntry.Name())
		providerName := providerEntry.Name()

		files, err := os.ReadDir(providerDir)
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() || shouldSkip(file.Name()) {
				continue
			}

			filePath := filepath.Join(providerDir, file.Name())
			item := ContentItem{
				Name:     file.Name(),
				Type:     ct,
				Path:     filePath,
				Provider: providerName,
				Local:    local,
			}
			if strings.HasSuffix(file.Name(), ".md") {
				item.Description = readDescription(filePath)
			}
			// For .json hook files, extract description from event + matcher
			if filepath.Ext(file.Name()) == ".json" {
				data, err := os.ReadFile(filePath)
				if err == nil {
					event := gjson.GetBytes(data, "event").String()
					matcher := gjson.GetBytes(data, "matcher").String()
					if event != "" {
						if matcher != "" {
							item.Description = fmt.Sprintf("%s hook for %s", event, matcher)
						} else {
							item.Description = fmt.Sprintf("%s hook", event)
						}
					}
				}
			}
			// Load provider-specific metadata
			meta, err := metadata.LoadProvider(providerDir, file.Name())
			if err != nil {
				return err
			}
			item.Meta = meta

			cat.Items = append(cat.Items, item)
		}
	}
	return nil
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
