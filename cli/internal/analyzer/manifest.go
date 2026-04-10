package analyzer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"gopkg.in/yaml.v3"
)

// ToManifestItem converts a DetectedItem to a registry.ManifestItem.
func ToManifestItem(item *DetectedItem) registry.ManifestItem {
	return registry.ManifestItem{
		Name:         item.Name,
		Type:         string(item.Type),
		Provider:     item.Provider,
		Path:         item.Path,
		HookEvent:    item.HookEvent,
		HookIndex:    item.HookIndex,
		Scripts:      item.Scripts,
		DisplayName:  item.DisplayName,
		Description:  item.Description,
		ContentHash:  item.ContentHash,
		References:   item.References,
		ConfigSource: item.ConfigSource,
		Providers:    item.Providers,
	}
}

// WriteGeneratedManifest writes a registry.yaml to the syllago cache directory
// for the named registry. This file is separate from the repo's own registry.yaml.
func WriteGeneratedManifest(name string, items []*DetectedItem) error {
	cacheDir, err := registry.CacheDir()
	if err != nil {
		return fmt.Errorf("getting cache dir: %w", err)
	}

	destDir := filepath.Join(cacheDir, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating manifest dir: %w", err)
	}

	manifestItems := make([]registry.ManifestItem, 0, len(items))
	for _, item := range items {
		manifestItems = append(manifestItems, ToManifestItem(item))
	}

	m := registry.Manifest{
		Version: "1",
		Items:   manifestItems,
	}

	data, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	// Phase B correction: Use registry.yaml so the scanner finds it.
	dest := filepath.Join(destDir, "registry.yaml")
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}
