package catalog

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestIsExample(t *testing.T) {
	tests := []struct {
		name string
		item ContentItem
		want bool
	}{
		{
			name: "nil meta returns false",
			item: ContentItem{Name: "no-meta"},
			want: false,
		},
		{
			name: "empty tags returns false",
			item: ContentItem{
				Name: "empty-tags",
				Meta: &metadata.Meta{Tags: nil},
			},
			want: false,
		},
		{
			name: "unrelated tags returns false",
			item: ContentItem{
				Name: "other-tags",
				Meta: &metadata.Meta{Tags: []string{"builtin", "featured"}},
			},
			want: false,
		},
		{
			name: "example tag returns true",
			item: ContentItem{
				Name: "example-item",
				Meta: &metadata.Meta{Tags: []string{"example"}},
			},
			want: true,
		},
		{
			name: "example among other tags returns true",
			item: ContentItem{
				Name: "mixed-tags",
				Meta: &metadata.Meta{Tags: []string{"featured", "example", "starter"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.IsExample()
			if got != tt.want {
				t.Errorf("IsExample() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		name string
		item ContentItem
		want bool
	}{
		{
			name: "nil meta returns false",
			item: ContentItem{Name: "no-meta"},
			want: false,
		},
		{
			name: "builtin tag returns true",
			item: ContentItem{
				Name: "builtin-item",
				Meta: &metadata.Meta{Tags: []string{"builtin"}},
			},
			want: true,
		},
		{
			name: "example tag does not match builtin",
			item: ContentItem{
				Name: "example-item",
				Meta: &metadata.Meta{Tags: []string{"example"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.IsBuiltin()
			if got != tt.want {
				t.Errorf("IsBuiltin() = %v, want %v", got, tt.want)
			}
		})
	}
}
