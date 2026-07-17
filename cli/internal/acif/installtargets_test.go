package acif

import (
	"errors"
	"reflect"
	"testing"
)

func resolveOK(t *testing.T, in InstallResolveInput) ([]InstallTarget, []Diagnostic) {
	t.Helper()
	targets, diags, err := ResolveInstallTargets(in)
	if err != nil {
		t.Fatalf("ResolveInstallTargets: %v", err)
	}
	return targets, diags
}

func resolveReject(t *testing.T, in InstallResolveInput) *RejectError {
	t.Helper()
	_, _, err := ResolveInstallTargets(in)
	var reject *RejectError
	if !errors.As(err, &reject) {
		t.Fatalf("expected RejectError, got %v", err)
	}
	return reject
}

// TV-INSTALL-a: multiplicity and write-target selection — order preserved,
// first current row per scope is the write target.
func TestResolveInstallTargets_MultiplicityWriteTarget(t *testing.T) {
	t.Parallel()
	targets, diags := resolveOK(t, InstallResolveInput{
		Provider: "copilot-cli", ContentType: "agent", ContentName: "reviewer",
		HomeDir: "/home/u", ProjectRoot: "/proj",
	})
	want := []InstallTarget{
		{Scope: "user", Path: "/home/u/.github/agents/reviewer.md", Layout: "single_file", Status: "current", WriteTarget: true},
		{Scope: "project", Path: "/proj/.copilot/agents/reviewer.md", Layout: "single_file", Status: "current", WriteTarget: true},
		{Scope: "project", Path: "/proj/.github/agents/reviewer.md", Layout: "single_file", Status: "current", WriteTarget: false},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("targets = %+v, want %+v", targets, want)
	}
	if len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %+v", diags)
	}
}

// TV-INSTALL-b (subset): layout coverage — one case per layout member,
// including trailing-slash preservation on directory_of_files.
func TestResolveInstallTargets_Layouts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   InstallResolveInput
		want []InstallTarget
	}{
		{
			name: "single_file",
			in: InstallResolveInput{Provider: "claude-code", ContentType: "agent",
				ContentName: "helper", HomeDir: "/h", ProjectRoot: "/p", Scope: "user"},
			want: []InstallTarget{{Scope: "user", Path: "/h/.claude/agents/helper.md",
				Layout: "single_file", Status: "current", WriteTarget: true}},
		},
		{
			name: "merged_into_shared_file",
			in: InstallResolveInput{Provider: "claude-code", ContentType: "hook",
				ContentName: "guard", HomeDir: "/h", ProjectRoot: "/p", Scope: "user"},
			want: []InstallTarget{{Scope: "user", Path: "/h/.claude/settings.json",
				Layout: "merged_into_shared_file", Status: "current", WriteTarget: true}},
		},
		{
			name: "directory_of_files",
			in: InstallResolveInput{Provider: "gemini-cli", ContentType: "skill",
				ContentName: "review", HomeDir: "/h", ProjectRoot: "/p", Scope: "user"},
			want: []InstallTarget{
				{Scope: "user", Path: "/h/.gemini/skills/review/",
					Layout: "directory_of_files", Status: "current", WriteTarget: true},
				{Scope: "user", Path: "/h/.agents/skills/review/",
					Layout: "directory_of_files", Status: "current", WriteTarget: false},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets, _ := resolveOK(t, tc.in)
			if !reflect.DeepEqual(targets, tc.want) {
				t.Errorf("targets = %+v, want %+v", targets, tc.want)
			}
		})
	}
}

// TV-INSTALL-c: §8.2 validity predicate rejects before path formation.
func TestResolveInstallTargets_ContentNameInvalid(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a/b", `a\b`, "..", ".", "", "nul\x00byte"} {
		reject := resolveReject(t, InstallResolveInput{
			Provider: "claude-code", ContentType: "command", ContentName: name,
			HomeDir: "/h", ProjectRoot: "/p",
		})
		if reject.ID != "acif.install.content_name_invalid" {
			t.Errorf("name %q: got %s, want acif.install.content_name_invalid", name, reject.ID)
		}
	}
}

// TV-INSTALL-d: §8.1 placeholder totality — an unrecognized token in the
// write row refuses.
func TestResolveInstallTargets_PlaceholderUnrecognized(t *testing.T) {
	t.Parallel()
	reject := resolveReject(t, InstallResolveInput{
		Provider: "claude-code", ContentType: "command", ContentName: "deploy",
		HomeDir: "/h", ProjectRoot: "/p",
		Entry: &InstallEntry{Scope: "user",
			PathTemplate: "~/.claude/commands/<unknown-variant>/<content-name>.md",
			Layout:       "single_file", Status: "current"},
	})
	if reject.ID != "acif.install.placeholder_unrecognized" {
		t.Errorf("got %s, want acif.install.placeholder_unrecognized", reject.ID)
	}
}

// TV-INSTALL-e: disposition lanes — no_entry_point, scope_unavailable with
// pinned params, and the supersession warn with targets returned.
func TestResolveInstallTargets_DispositionLanes(t *testing.T) {
	t.Parallel()

	t.Run("no_entry_point", func(t *testing.T) {
		t.Parallel()
		reject := resolveReject(t, InstallResolveInput{
			Provider: "windsurf", ContentType: "hook", ContentName: "guard",
			HomeDir: "/h", ProjectRoot: "/p",
		})
		if reject.ID != "acif.install.no_entry_point" {
			t.Errorf("got %s, want acif.install.no_entry_point", reject.ID)
		}
	})

	t.Run("scope_unavailable", func(t *testing.T) {
		t.Parallel()
		reject := resolveReject(t, InstallResolveInput{
			Provider: "kiro", ContentType: "rule", ContentName: "style",
			HomeDir: "/h", ProjectRoot: "/p", Scope: "user",
		})
		if reject.ID != "acif.install.scope_unavailable" {
			t.Fatalf("got %s, want acif.install.scope_unavailable", reject.ID)
		}
		if len(reject.Diagnostics) != 1 {
			t.Fatalf("diagnostics = %+v, want one entry", reject.Diagnostics)
		}
		want := map[string]any{"available_scopes": []string{"project"}}
		if !reflect.DeepEqual(reject.Diagnostics[0].Params, want) {
			t.Errorf("params = %+v, want %+v", reject.Diagnostics[0].Params, want)
		}
	})

	t.Run("scope_unavailable_sorts_available_scopes", func(t *testing.T) {
		t.Parallel()
		// copilot-cli agent rows are ordered user, project, project; the
		// PROTOCOL Appendix A param contract pins available_scopes sorted.
		reject := resolveReject(t, InstallResolveInput{
			Provider: "copilot-cli", ContentType: "agent", ContentName: "reviewer",
			HomeDir: "/h", ProjectRoot: "/p", Scope: "managed",
		})
		if reject.ID != "acif.install.scope_unavailable" {
			t.Fatalf("got %s, want acif.install.scope_unavailable", reject.ID)
		}
		want := map[string]any{"available_scopes": []string{"project", "user"}}
		if !reflect.DeepEqual(reject.Diagnostics[0].Params, want) {
			t.Errorf("params = %+v, want %+v", reject.Diagnostics[0].Params, want)
		}
	})

	t.Run("superseded_warns_and_proceeds", func(t *testing.T) {
		t.Parallel()
		targets, diags := resolveOK(t, InstallResolveInput{
			Provider: "claude-code", ContentType: "command", ContentName: "deploy",
			HomeDir: "/h", ProjectRoot: "/p",
			Entry: &InstallEntry{Scope: "user",
				PathTemplate: "~/.claude/commands/<content-name>.md",
				Layout:       "single_file", Status: "superseded"},
		})
		want := []InstallTarget{{Scope: "user", Path: "/h/.claude/commands/deploy.md",
			Layout: "single_file", Status: "superseded", WriteTarget: false}}
		if !reflect.DeepEqual(targets, want) {
			t.Errorf("targets = %+v, want %+v", targets, want)
		}
		if len(diags) != 1 || diags[0].ID != "acif.install.entry_point_superseded" {
			t.Errorf("diagnostics = %+v, want acif.install.entry_point_superseded", diags)
		}
	})
}

// TV-INSTALL-f: anchor resolution — home-anchored, project-anchored, and
// absolute managed templates resolve against invocation-supplied roots.
func TestResolveInstallTargets_Anchors(t *testing.T) {
	t.Parallel()

	userTargets, _ := resolveOK(t, InstallResolveInput{
		Provider: "pi", ContentType: "hook", ContentName: "tracer",
		HomeDir: "/home/dev", ProjectRoot: "/work/repo", Scope: "user",
	})
	wantUser := []InstallTarget{{Scope: "user", Path: "/home/dev/.pi/agent/extensions/tracer.ts",
		Layout: "single_file", Status: "current", WriteTarget: true}}
	if !reflect.DeepEqual(userTargets, wantUser) {
		t.Errorf("user targets = %+v, want %+v", userTargets, wantUser)
	}

	projTargets, _ := resolveOK(t, InstallResolveInput{
		Provider: "pi", ContentType: "hook", ContentName: "tracer",
		HomeDir: "/home/dev", ProjectRoot: "/work/repo", Scope: "project",
	})
	wantProj := []InstallTarget{{Scope: "project", Path: "/work/repo/.pi/extensions/tracer.ts",
		Layout: "single_file", Status: "current", WriteTarget: true}}
	if !reflect.DeepEqual(projTargets, wantProj) {
		t.Errorf("project targets = %+v, want %+v", projTargets, wantProj)
	}

	managedTargets, _ := resolveOK(t, InstallResolveInput{
		Provider: "claude-code", ContentType: "agent", ContentName: "auditor",
		HomeDir: "/home/dev", ProjectRoot: "/work/repo",
		Entry: &InstallEntry{Scope: "managed",
			PathTemplate: "/etc/acme/agents/<content-name>.md",
			Layout:       "single_file", Status: "current"},
	})
	wantManaged := []InstallTarget{{Scope: "managed", Path: "/etc/acme/agents/auditor.md",
		Layout: "single_file", Status: "current", WriteTarget: true}}
	if !reflect.DeepEqual(managedTargets, wantManaged) {
		t.Errorf("managed targets = %+v, want %+v", managedTargets, wantManaged)
	}
}
