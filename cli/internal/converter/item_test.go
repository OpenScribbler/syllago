package converter

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func compatRows(t *testing.T, item catalog.ContentItem) map[string]ProviderCompat {
	t.Helper()
	report, err := CompatReport(item)
	if err != nil {
		t.Fatalf("CompatReport: %v", err)
	}
	rows := map[string]ProviderCompat{}
	for _, r := range report {
		rows[r.Provider.Slug] = r
	}
	return rows
}

func TestCompatReport_LevelIsTheWorstHook(t *testing.T) {
	// Copilot CLI ignores matchers, so only the second hook loses anything.
	item := writeItem(t, catalog.Hooks, "hook.json", `{"spec":"hooks/0.1","hooks":[`+
		`{"event":"session_start","handler":{"type":"command","command":"./a.sh"}},`+
		`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"./b.sh"}}]}`)
	rows := compatRows(t, item)

	copilot := rows["copilot-cli"]
	if copilot.Level != CompatBroken || !slices.Contains(copilot.Warnings, "hook fires on ALL tool calls") {
		t.Errorf("copilot-cli = %+v, want broken because the second hook's matcher is ignored", copilot)
	}
	// Crush drops the session_start hook and writes the other one.
	crush := rows["crush"]
	if !crush.Supported || crush.Level != CompatBroken {
		t.Errorf("crush = %+v, want supported and broken because one hook is dropped", crush)
	}
	if cc := rows["claude-code"]; cc.Level != CompatFull {
		t.Errorf("claude-code = %+v, want full", cc)
	}
}

func TestCompatReport_HookReadAsItsSourceProvider(t *testing.T) {
	// A legacy hook.json copied from Claude Code settings names Claude
	// Code's event and tool.
	item := writeItem(t, catalog.Hooks, "hook.json",
		`{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`)
	item.Provider = "claude-code"
	rows := compatRows(t, item)
	if g := rows["gemini-cli"]; !g.Supported || g.Level >= CompatBroken {
		t.Errorf("gemini-cli = %+v, want the hook read as before_tool_execute on shell", g)
	}
	if k := rows["kiro"]; !k.Supported || k.Level >= CompatBroken {
		t.Errorf("kiro = %+v, want the hook read as before_tool_execute on shell", k)
	}
}

func TestCompatReport_LevelFollowsSupport(t *testing.T) {
	for _, item := range []catalog.ContentItem{
		writeItem(t, catalog.Hooks, "hook.json", libraryHook),
		writeItem(t, catalog.Rules, "rule.md", "Always write tests.\n"),
		writeItem(t, catalog.Loadouts, "loadout.yaml", "name: x\n"),
	} {
		for slug, r := range compatRows(t, item) {
			if (r.Level == CompatNone) == r.Supported {
				t.Errorf("%s %s: supported = %v with level %s", item.Type, slug, r.Supported, r.Level.Label())
			}
			if r.Supported && r.Level == CompatFull && len(r.Warnings) > 0 {
				t.Errorf("%s %s: full with warnings %v", item.Type, slug, r.Warnings)
			}
		}
	}
}

func TestCompatReport_SingleFileHook(t *testing.T) {
	// A hook stored as one file in its provider's directory is its own
	// content file.
	path := filepath.Join(t.TempDir(), "guard.json")
	hook := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./check.sh"}]}]}}`
	if err := os.WriteFile(path, []byte(hook), 0o644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "guard.json", Type: catalog.Hooks, Provider: "claude-code", Path: path}
	rows := compatRows(t, item)
	if cc := rows["claude-code"]; !cc.Supported || cc.Level != CompatFull {
		t.Errorf("claude-code = %+v, want supported and full", cc)
	}
}

func TestCompatReport_DroppedHTTPHookBreaksTheItem(t *testing.T) {
	// Gemini CLI has no HTTP hooks, so it writes the command hook and drops
	// the other.
	item := writeItem(t, catalog.Hooks, "hook.json", `{"spec":"hooks/0.1","hooks":[`+
		`{"event":"before_tool_execute","handler":{"type":"command","command":"./a.sh"}},`+
		`{"event":"before_tool_execute","handler":{"type":"http","url":"https://example.com/hook"}}]}`)
	rows := compatRows(t, item)
	if gemini := rows["gemini-cli"]; !gemini.Supported || gemini.Level != CompatBroken || !slices.Contains(gemini.Warnings, "no HTTP hooks") {
		t.Errorf("gemini-cli = %+v, want supported and broken because the HTTP hook is dropped", gemini)
	}
	if cc := rows["claude-code"]; cc.Level != CompatFull {
		t.Errorf("claude-code = %+v, want full", cc)
	}
}

func TestConvertItem_TypedAddIsReadAsCanonical(t *testing.T) {
	// An add that names the content type stores the canonical form and
	// records the provider the command came from.
	item := writeItem(t, catalog.Commands, "command.md", "---\ndescription: Review code\n---\n\nReview the diff.\n")
	item.Meta = &metadata.Meta{SourceProvider: "gemini-cli"}

	c, err := ConvertItem(item, providerBySlug(t, "claude-code"), "")
	if err != nil {
		t.Fatalf("ConvertItem: %v", err)
	}
	if c.From != "" || len(c.Content) == 0 {
		t.Errorf("From = %q, Content = %q; want the canonical content converted", c.From, c.Content)
	}
	// Naming the source format says how to read the content, so content
	// that is not in it stays an error.
	if _, err := ConvertItem(item, providerBySlug(t, "claude-code"), "gemini-cli"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("explicit from: err = %v, want ErrUnreadable", err)
	}
}

func TestCompatReport_DroppedEventBreaksTheItemForClaudeCode(t *testing.T) {
	// Claude Code has no before_model event, so it writes one hook of two.
	item := writeItem(t, catalog.Hooks, "hook.json", `{"spec":"hooks/0.1","hooks":[`+
		`{"event":"session_start","handler":{"type":"command","command":"./a.sh"}},`+
		`{"event":"before_model","handler":{"type":"command","command":"./b.sh"}}]}`)
	rows := compatRows(t, item)
	if cc := rows["claude-code"]; !cc.Supported || cc.Level != CompatBroken {
		t.Errorf("claude-code = %+v, want supported and broken because before_model is dropped", cc)
	}
}
