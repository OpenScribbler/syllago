package converter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func writeItem(t *testing.T, typ catalog.ContentType, file, content string) catalog.ContentItem {
	t.Helper()
	dir := t.TempDir()
	if file != "" {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return catalog.ContentItem{Name: "item", Type: typ, Path: dir}
}

func providerBySlug(t *testing.T, slug string) provider.Provider {
	t.Helper()
	for _, p := range provider.AllProviders {
		if p.Slug == slug {
			return p
		}
	}
	t.Fatalf("no provider %q", slug)
	return provider.Provider{}
}

func TestConvertItem_SourceProviderPrecedence(t *testing.T) {
	cases := []struct {
		name     string
		from     string
		meta     string
		itemProv string
		want     string
	}{
		{"flag wins", "windsurf", "cursor", "claude-code", "windsurf"},
		{"metadata over directory", "", "cursor", "claude-code", "cursor"},
		{"directory last", "", "", "claude-code", "claude-code"},
		{"canonical", "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := writeItem(t, catalog.Rules, "rule.md", "Always write tests.\n")
			item.Provider = tc.itemProv
			if tc.meta != "" {
				item.Meta = &metadata.Meta{SourceProvider: tc.meta}
			}
			c, err := ConvertItem(item, providerBySlug(t, "cursor"), tc.from)
			if err != nil {
				t.Fatalf("ConvertItem: %v", err)
			}
			if c.From != tc.want {
				t.Errorf("From = %q, want %q", c.From, tc.want)
			}
			if len(c.Content) == 0 || string(c.Source) != "Always write tests.\n" {
				t.Errorf("Content = %q, Source = %q", c.Content, c.Source)
			}
		})
	}
}

func TestConvertItem_Errors(t *testing.T) {
	cases := []struct {
		name    string
		item    catalog.ContentItem
		to      string
		wantErr error
	}{
		{"type without conversion", writeItem(t, catalog.Loadouts, "loadout.yaml", "name: x\n"), "claude-code", ErrNotConvertible},
		{"no content file", writeItem(t, catalog.Rules, "", ""), "claude-code", ErrNoContentFile},
		{"unreadable hooks", writeItem(t, catalog.Hooks, "hooks.json", "{not json"), "claude-code", ErrUnreadable},
		{"unreadable mcp", writeItem(t, catalog.MCP, "mcp.json", "{not json"), "claude-code", ErrUnreadable},
		{"no hook encoder", writeItem(t, catalog.Hooks, "hooks.json", libraryHook), "codex", ErrNoHookEncoder},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ConvertItem(tc.item, providerBySlug(t, tc.to), "")
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

const libraryHook = `{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"./check.sh"}}]}`

func TestCompatReport_Hook(t *testing.T) {
	item := writeItem(t, catalog.Hooks, "hooks.json", libraryHook)
	report, err := CompatReport(item)
	if err != nil {
		t.Fatalf("CompatReport: %v", err)
	}
	rows := map[string]ProviderCompat{}
	for _, r := range report {
		rows[r.Provider.Slug] = r
	}
	if len(rows) != len(provider.AllProviders) {
		t.Errorf("%d rows, want one per provider (%d)", len(rows), len(provider.AllProviders))
	}
	if !rows["claude-code"].Supported {
		t.Errorf("claude-code = %+v, want supported", rows["claude-code"])
	}
	codex := rows["codex"]
	if codex.Supported || len(codex.Warnings) != 1 || codex.Warnings[0] != "syllago cannot write hooks for Codex" {
		t.Errorf("codex = %+v, want unsupported because syllago cannot write its hooks", codex)
	}
}

func TestCompatReport_UnreadableItemIsAnError(t *testing.T) {
	item := writeItem(t, catalog.MCP, "mcp.json", "{not json")
	if _, err := CompatReport(item); !errors.Is(err, ErrUnreadable) {
		t.Errorf("err = %v, want ErrUnreadable", err)
	}
}

func TestCompatReport_TypeWithoutConversion(t *testing.T) {
	item := writeItem(t, catalog.Loadouts, "loadout.yaml", "name: x\n")
	report, err := CompatReport(item)
	if err != nil {
		t.Fatalf("CompatReport: %v", err)
	}
	for _, r := range report {
		holds := r.Provider.SupportsType != nil && r.Provider.SupportsType(catalog.Loadouts)
		if r.Supported != holds {
			t.Errorf("%s supported = %v, want %v", r.Provider.Slug, r.Supported, holds)
		}
	}
}
