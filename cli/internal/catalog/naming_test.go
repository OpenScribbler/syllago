package catalog

import (
	"strings"
	"testing"
)

func TestCheckNamingWarnings(t *testing.T) {
	tests := []struct {
		name        string
		items       []ContentItem
		wantWarns   int
		wantContain string
	}{
		{
			name:      "no hooks or MCP, no warnings",
			items:     []ContentItem{{Name: "my-skill", Type: Skills}},
			wantWarns: 0,
		},
		{
			name:        "hook with no display name",
			items:       []ContentItem{{Name: "pre-commit", Type: Hooks}},
			wantWarns:   1,
			wantContain: "pre-commit",
		},
		{
			name:        "hook with DisplayName == Name",
			items:       []ContentItem{{Name: "pre-commit", DisplayName: "pre-commit", Type: Hooks}},
			wantWarns:   1,
			wantContain: "no display name",
		},
		{
			name:      "hook with meaningful display name",
			items:     []ContentItem{{Name: "pre-commit", DisplayName: "Pre-Commit Linter", Type: Hooks}},
			wantWarns: 0,
		},
		{
			name:        "MCP with no display name",
			items:       []ContentItem{{Name: "config", Type: MCP}},
			wantWarns:   1,
			wantContain: "config",
		},
		{
			name:      "MCP with meaningful display name",
			items:     []ContentItem{{Name: "config", DisplayName: "GitHub MCP Server", Type: MCP}},
			wantWarns: 0,
		},
		{
			name: "mixed items, only unnamed hooks flagged",
			items: []ContentItem{
				{Name: "skill-a", Type: Skills},
				{Name: "unnamed-hook", Type: Hooks},
				{Name: "named-hook", DisplayName: "My Hook", Type: Hooks},
				{Name: "unnamed-mcp", Type: MCP},
				{Name: "named-mcp", DisplayName: "My MCP", Type: MCP},
			},
			wantWarns: 2, // unnamed-hook + unnamed-mcp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := &Catalog{Items: tt.items}
			cat.checkNamingWarnings()

			if len(cat.Warnings) != tt.wantWarns {
				t.Errorf("expected %d warnings, got %d: %v", tt.wantWarns, len(cat.Warnings), cat.Warnings)
			}

			if tt.wantContain != "" && len(cat.Warnings) > 0 {
				found := false
				for _, w := range cat.Warnings {
					if strings.Contains(w, tt.wantContain) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected warning containing %q, got %v", tt.wantContain, cat.Warnings)
				}
			}
		})
	}
}
