package lifecycle

import (
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

// recordCoord names the install record for item. A library copy of a
// registry item keeps its registry, so its record and pin match the item
// installed straight from the registry.
func recordCoord(item catalog.ContentItem) installstore.Coord {
	registry := item.Registry
	if registry == "" && item.Meta != nil && item.Meta.SourceType == "registry" && item.Meta.SourceRegistry != "" {
		registry = item.Meta.SourceRegistry
	}
	return installstore.Coord{
		Registry: registry,
		Type:     string(item.Type),
		Name:     item.Name,
	}
}

func recordPlacement(provSlug string, pl installer.Placement) installstore.PlacementInput {
	return installstore.PlacementInput{
		Provider:  provSlug,
		Mechanism: installstore.Mechanism(pl.Mechanism),
		Path:      pl.Path,
		Keys:      pl.Keys,
	}
}

func recordSourceSHA(item catalog.ContentItem) string {
	if item.Meta == nil {
		return ""
	}
	return item.Meta.SourceSHA
}
