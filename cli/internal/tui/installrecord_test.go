package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestTUIInstallRecordCoordRegistryFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		item         catalog.ContentItem
		wantRegistry string
	}{
		{
			name: "metadata registry used when item registry empty",
			item: catalog.ContentItem{
				Name: "writer",
				Type: catalog.Skills,
				Meta: &metadata.Meta{SourceType: "registry", SourceRegistry: "acme/tools"},
			},
			wantRegistry: "acme/tools",
		},
		{
			name: "non registry source type leaves registry empty",
			item: catalog.ContentItem{
				Name: "writer",
				Type: catalog.Skills,
				Meta: &metadata.Meta{SourceType: "provider", SourceRegistry: "acme/tools"},
			},
		},
		{
			name: "explicit item registry wins",
			item: catalog.ContentItem{
				Name:     "writer",
				Type:     catalog.Skills,
				Registry: "explicit",
				Meta:     &metadata.Meta{SourceType: "registry", SourceRegistry: "acme/tools"},
			},
			wantRegistry: "explicit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tuiInstallRecordCoord(tt.item)
			if got.Registry != tt.wantRegistry {
				t.Fatalf("Registry = %q, want %q", got.Registry, tt.wantRegistry)
			}
			if got.Type != string(tt.item.Type) || got.Name != tt.item.Name {
				t.Fatalf("Coord = %#v, want type/name from item", got)
			}
		})
	}
}
