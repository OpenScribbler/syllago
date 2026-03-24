package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestMetadataSummaryView(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "alpha", Type: catalog.Skills, Source: "team-rules", Registry: "team-rules"},
		{Name: "beta", Type: catalog.Skills, Source: "library", Library: true},
	}
	m := metadataModel{
		ct:    catalog.Skills,
		items: items,
		width: 80,
	}
	view := m.View()

	if !strings.Contains(view, "Skills (2 items)") {
		t.Errorf("summary should show item count, got: %s", view)
	}
}

func TestMetadataItemView(t *testing.T) {
	item := catalog.ContentItem{
		Name:        "alpha-skill",
		Type:        catalog.Skills,
		Source:      "team-rules",
		Description: "A helpful skill",
		Files:       []string{"SKILL.md", "helpers.md"},
	}
	m := metadataModel{
		item:  &item,
		ct:    catalog.Skills,
		width: 120,
	}
	view := m.View()

	if !strings.Contains(view, "alpha-skill") {
		t.Error("should contain item name")
	}
	if !strings.Contains(view, "Files: 2") {
		t.Error("should contain file count for Skills")
	}
	if !strings.Contains(view, "A helpful skill") {
		t.Error("should contain description")
	}
}

func TestMetadataAllTypes(t *testing.T) {
	types := []catalog.ContentType{
		catalog.Skills, catalog.Agents, catalog.MCP,
		catalog.Rules, catalog.Hooks, catalog.Commands,
	}
	for _, ct := range types {
		t.Run(string(ct), func(t *testing.T) {
			item := catalog.ContentItem{Name: "test", Type: ct, Description: "desc"}
			m := metadataModel{item: &item, ct: ct, width: 80}
			view := m.View()
			if view == "" {
				t.Error("view should not be empty")
			}
		})
	}
}

func TestDisplayTypeName(t *testing.T) {
	tests := []struct {
		ct   catalog.ContentType
		want string
	}{
		{catalog.Skills, "Skills"},
		{catalog.MCP, "MCP Configs"},
		{catalog.Loadouts, "Loadouts"},
	}
	for _, tt := range tests {
		got := displayTypeName(tt.ct)
		if got != tt.want {
			t.Errorf("displayTypeName(%s) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}
