package loadout

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// PrivateItemWarning describes a private item found in a loadout.
type PrivateItemWarning struct {
	Name     string
	Type     catalog.ContentType
	Registry string // source registry name
}

// CheckPrivateItems scans resolved catalog items for private taint.
// Returns warnings for each private item found (Gate G3).
// This is a warning, not a block — loadouts can contain private items for local use.
func CheckPrivateItems(items []catalog.ContentItem) []PrivateItemWarning {
	var warnings []PrivateItemWarning
	for _, item := range items {
		if item.Meta != nil && item.Meta.SourceRegistry != "" && registry.IsPrivate(item.Meta.SourceVisibility) {
			warnings = append(warnings, PrivateItemWarning{
				Name:     item.Name,
				Type:     item.Type,
				Registry: item.Meta.SourceRegistry,
			})
		}
	}
	return warnings
}

// FormatPrivateWarnings formats private item warnings into a human-readable string.
func FormatPrivateWarnings(warnings []PrivateItemWarning) string {
	if len(warnings) == 0 {
		return ""
	}
	msg := fmt.Sprintf("Warning: %d item(s) from private registries:\n", len(warnings))
	for _, w := range warnings {
		msg += fmt.Sprintf("  - %s (%s, from %s)\n", w.Name, w.Type, w.Registry)
	}
	msg += "\nThis loadout can be used locally but cannot be published to a public registry."
	return msg
}

// CheckLoadoutPublishGate implements Gate G4: blocks publishing a loadout to a
// public registry when the loadout contains private items.
func CheckLoadoutPublishGate(items []catalog.ContentItem, targetRegistryVisibility string) error {
	if registry.IsPrivate(targetRegistryVisibility) {
		return nil // publishing to a private registry is always allowed
	}

	warnings := CheckPrivateItems(items)
	if len(warnings) == 0 {
		return nil
	}

	msg := fmt.Sprintf("cannot publish loadout to public registry\n\n  Contains private content:\n")
	for _, w := range warnings {
		msg += fmt.Sprintf("    - %s (from %s, private)\n", w.Name, w.Registry)
	}
	msg += "\n  Private content cannot be published to public registries.\n"
	msg += "  Remove private items from the loadout, or recreate them\n"
	msg += "  without the private registry association."
	return fmt.Errorf("%s", msg)
}
