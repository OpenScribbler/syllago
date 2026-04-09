package capmon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
	"gopkg.in/yaml.v3"
)

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
