package capmon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
	"gopkg.in/yaml.v3"
)

// GeneratedBannerStart is the sentinel that marks the beginning of a generated section
// in spec markdown files. Content between this marker and GeneratedBannerEnd is managed
// by GenerateHooksSpecTables and must not be edited by hand.
const GeneratedBannerStart = "<!-- GENERATED FROM provider-capabilities/*.yaml -->"

// GeneratedBannerEnd marks the end of a generated section.
const GeneratedBannerEnd = "<!-- END GENERATED -->"

// ErrNoGeneratedSection is returned by ReplaceGeneratedSection when the file does
// not contain a GeneratedBannerStart marker.
var ErrNoGeneratedSection = errors.New("no generated section markers found")

// ReplaceGeneratedSection replaces the content between GeneratedBannerStart and
// GeneratedBannerEnd in the file at path with newContent.
// Returns ErrNoGeneratedSection if GeneratedBannerStart is absent.
// Returns an error if GeneratedBannerEnd is absent after the start marker (malformed file).
func ReplaceGeneratedSection(path, newContent string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	text := string(data)
	startIdx := strings.Index(text, GeneratedBannerStart)
	if startIdx == -1 {
		return fmt.Errorf("%s: %w", path, ErrNoGeneratedSection)
	}
	endIdx := strings.Index(text, GeneratedBannerEnd)
	if endIdx == -1 {
		return fmt.Errorf("%s: missing end marker %s", path, GeneratedBannerEnd)
	}
	endIdx += len(GeneratedBannerEnd)
	replacement := GeneratedBannerStart + "\n" + newContent + "\n" + GeneratedBannerEnd
	result := text[:startIdx] + replacement + text[endIdx:]
	return os.WriteFile(path, []byte(result), 0644)
}

// providerInfo holds the event and tool mappings extracted from a capability YAML.
type providerInfo struct {
	slug   string
	events map[string]string // canonical → native name
}

// GenerateHooksSpecTables reads all provider-capabilities/*.yaml files and
// regenerates the generated sections in *.md files within specDir.
// Files without sentinel markers are silently skipped.
// If no provider YAML files are found, the function is a no-op.
func GenerateHooksSpecTables(capsDir, specDir string) error {
	entries, err := os.ReadDir(capsDir)
	if err != nil {
		return fmt.Errorf("read capabilities dir: %w", err)
	}

	var providers []providerInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		caps, err := capyaml.LoadCapabilityYAML(filepath.Join(capsDir, e.Name()))
		if err != nil {
			return fmt.Errorf("load %s: %w", e.Name(), err)
		}
		// Skip per-content-type seed YAMLs (e.g. amp-skills.yaml). Those use
		// `provider:` at the top level instead of `slug:`, so they parse with
		// an empty Slug and would otherwise produce blank columns in the table.
		if caps.Slug == "" {
			continue
		}
		pi := providerInfo{slug: caps.Slug, events: make(map[string]string)}
		if hooksEntry, ok := caps.ContentTypes["hooks"]; ok {
			for canonical, ev := range hooksEntry.Events {
				pi.events[canonical] = ev.NativeName
			}
		}
		providers = append(providers, pi)
	}

	if len(providers) == 0 {
		return nil
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].slug < providers[j].slug
	})

	eventsTable := buildEventsMatrix(providers)

	specFiles, err := filepath.Glob(filepath.Join(specDir, "*.md"))
	if err != nil {
		return fmt.Errorf("glob spec files: %w", err)
	}
	for _, f := range specFiles {
		if err := ReplaceGeneratedSection(f, eventsTable); err != nil {
			if errors.Is(err, ErrNoGeneratedSection) {
				continue
			}
			return err
		}
	}
	return nil
}

// buildEventsMatrix constructs a Markdown table mapping canonical event names to
// their provider-native names, one column per provider.
func buildEventsMatrix(providers []providerInfo) string {
	// Collect all canonical event names across all providers.
	eventSet := make(map[string]struct{})
	for _, p := range providers {
		for ev := range p.events {
			eventSet[ev] = struct{}{}
		}
	}
	events := make([]string, 0, len(eventSet))
	for ev := range eventSet {
		events = append(events, ev)
	}
	sort.Strings(events)

	var sb strings.Builder
	// Header row
	sb.WriteString("| Canonical Event |")
	for _, p := range providers {
		sb.WriteString(" " + p.slug + " |")
	}
	sb.WriteString("\n|---|")
	for range providers {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")
	// Data rows
	for _, ev := range events {
		sb.WriteString("| `" + ev + "` |")
		for _, p := range providers {
			native := p.events[ev]
			if native == "" {
				native = "--"
			}
			sb.WriteString(" " + native + " |")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// GenerateContentTypeViews reads all provider-capabilities/*.yaml and writes
// docs/provider-capabilities/by-content-type/<type>.yaml files.
// Each generated file begins with a THIS FILE IS GENERATED banner.
func GenerateContentTypeViews(capsDir, outDir string) error {
	entries, err := os.ReadDir(capsDir)
	if err != nil {
		return fmt.Errorf("read capabilities dir: %w", err)
	}

	// Collect by content type
	byType := make(map[string]map[string]interface{}) // contentType → provider → entry

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		caps, err := capyaml.LoadCapabilityYAML(filepath.Join(capsDir, e.Name()))
		if err != nil {
			return fmt.Errorf("load %s: %w", e.Name(), err)
		}
		// Skip per-content-type seed YAMLs that have no top-level `slug:`.
		if caps.Slug == "" {
			continue
		}
		for ct, entry := range caps.ContentTypes {
			if _, ok := byType[ct]; !ok {
				byType[ct] = make(map[string]interface{})
			}
			byType[ct][caps.Slug] = entry
		}
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir output dir: %w", err)
	}

	for ct, providers := range byType {
		outPath := filepath.Join(outDir, ct+".yaml")
		banner := fmt.Sprintf("# THIS FILE IS GENERATED. Do not edit directly.\n# Source: %s/*.yaml\n# Generated at: %s\n\n",
			capsDir, time.Now().UTC().Format(time.RFC3339))

		data, err := yaml.Marshal(map[string]interface{}{
			"schema_version": "1",
			"content_type":   ct,
			"providers":      providers,
		})
		if err != nil {
			return fmt.Errorf("marshal %s: %w", ct, err)
		}

		full := banner + strings.TrimSpace(string(data)) + "\n"
		if err := os.WriteFile(outPath, []byte(full), 0644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
	}
	return nil
}
