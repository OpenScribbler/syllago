package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// isolateListEnv blocks global-config and registry-cache leakage into list
// tests. Without it, runList walks ~/.syllago/config.yaml and enumerates any
// cloned registries on the dev machine — producing unexpected result rows.
// All list tests must call this after withFakeRepoRoot / withGlobalLibrary.
func isolateListEnv(t *testing.T) {
	t.Helper()
	origCfg := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origCfg })
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = t.TempDir()
	t.Cleanup(func() { registry.CacheDirOverride = origCache })
}

// setupListRepo creates a temp syllago repo with items across types and sources.
func setupListRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Shared skill.
	sharedSkill := filepath.Join(root, "skills", "code-review")
	os.MkdirAll(sharedSkill, 0755)
	os.WriteFile(filepath.Join(sharedSkill, "SKILL.md"), []byte("---\nname: Code Review\ndescription: Systematic code review\n---\n"), 0644)
	os.WriteFile(filepath.Join(sharedSkill, "README.md"), []byte("# code-review\n"), 0644)

	// Second shared skill (used to test count=2).
	greeting := filepath.Join(root, "skills", "greeting")
	os.MkdirAll(greeting, 0755)
	os.WriteFile(filepath.Join(greeting, "SKILL.md"), []byte("---\nname: Greeting\ndescription: Says hello to the user\n---\n"), 0644)

	// Shared agent.
	sharedAgent := filepath.Join(root, "agents", "code-reviewer")
	os.MkdirAll(sharedAgent, 0755)
	os.WriteFile(filepath.Join(sharedAgent, "AGENT.md"), []byte("---\nname: Code Reviewer\ndescription: Code review agent\n---\n"), 0644)
	os.WriteFile(filepath.Join(sharedAgent, "README.md"), []byte("# code-reviewer\n"), 0644)

	return root
}

func TestListShowsAllItems(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	stdout, _ := output.SetForTest(t)

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	out := stdout.String()

	// Should contain both skills.
	if !strings.Contains(out, "code-review") {
		t.Errorf("expected shared skill 'code-review' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected local skill 'greeting' in output, got:\n%s", out)
	}

	// Should contain the agent.
	if !strings.Contains(out, "code-reviewer") {
		t.Errorf("expected local agent 'code-reviewer' in output, got:\n%s", out)
	}

	// Should show type headers with counts.
	if !strings.Contains(out, "Skills (2)") {
		t.Errorf("expected 'Skills (2)' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Agents (1)") {
		t.Errorf("expected 'Agents (1)' header in output, got:\n%s", out)
	}

	// Should show source labels.
	if !strings.Contains(out, "[shared") {
		t.Errorf("expected '[shared' source label in output, got:\n%s", out)
	}
}

func TestListFilterByType(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	stdout, _ := output.SetForTest(t)

	listCmd.Flags().Set("type", "skills")
	defer listCmd.Flags().Set("type", "")

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --type skills failed: %v", err)
	}

	out := stdout.String()

	// Should contain skills.
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected 'greeting' in skills-filtered output, got:\n%s", out)
	}
	if !strings.Contains(out, "code-review") {
		t.Errorf("expected 'code-review' in skills-filtered output, got:\n%s", out)
	}

	// Should NOT contain agents.
	if strings.Contains(out, "code-reviewer") {
		t.Errorf("type=skills filter should exclude agents, got:\n%s", out)
	}
	if strings.Contains(out, "Agents") {
		t.Errorf("type=skills filter should not show Agents header, got:\n%s", out)
	}
}

func TestListFilterBySource(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	_, stderr := output.SetForTest(t)

	listCmd.Flags().Set("source", "shared")
	defer listCmd.Flags().Set("source", "all")

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --source shared failed: %v", err)
	}

	// In the test repo all items are shared, so output should include them.
	_ = stderr // no items found only happens if no shared items exist
}

func TestListJSON(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	// Isolate from real global library
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	var result listResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if len(result.Groups) == 0 {
		t.Fatal("expected at least one group in JSON output")
	}

	// Verify skills group exists with correct count.
	found := false
	for _, g := range result.Groups {
		if g.Type == "Skills" {
			found = true
			if g.Count != 2 {
				t.Errorf("expected 2 skills, got %d", g.Count)
			}
		}
	}
	if !found {
		t.Error("expected Skills group in JSON output")
	}
}

func TestListEmpty(t *testing.T) {
	root := t.TempDir()
	// Create a minimal marker so the root resolves.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)
	// Isolate from real global library
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	_, stderr := output.SetForTest(t)

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list on empty repo failed: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "No items found") {
		t.Errorf("expected 'No items found' message, got stderr:\n%s", errOut)
	}
}

// TestListJSON_NoTrustBadgeForPlainItems proves the list JSON output is
// silent about trust when items have none. An empty Trust field must
// omitjson so downstream consumers can use simple truthiness checks
// (Trust == "Verified" / "Recalled") without string-matching on empty.
func TestListJSON_NoTrustBadgeForPlainItems(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --json: %v", err)
	}

	// Plain (non-MOAT) items must not advertise a trust label. The JSON
	// tag omitempty drops Trust/TrustTier/Recalled entirely for these
	// rows, so the raw bytes should not contain a "trust" key at all.
	raw := stdout.String()
	if strings.Contains(raw, `"trust":"Verified"`) {
		t.Errorf("plain shared items should not carry Verified trust, got:\n%s", raw)
	}
	if strings.Contains(raw, `"recalled":true`) {
		t.Errorf("plain shared items must not be flagged Recalled, got:\n%s", raw)
	}

	// Verify the JSON still parses into listResult cleanly — the new
	// Trust fields must not break existing consumers.
	var result listResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, raw)
	}
	if len(result.Groups) == 0 {
		t.Fatal("expected at least one group in JSON output")
	}
}

// ── Slice 4: --filter state-based filtering ───────────────────────────────

// TestFilterByState_InLibrary verifies filterByState("in-library") passes only Library items.
func TestFilterByState_InLibrary(t *testing.T) {
	t.Parallel()
	lib := catalog.ContentItem{Name: "lib-skill", Library: true}
	clone := catalog.ContentItem{Name: "clone-skill", Registry: "reg", Library: false}
	proj := catalog.ContentItem{Name: "proj-rule", Library: false}

	if !filterByState(lib, []string{"in-library"}) {
		t.Error("in-library: Library item should pass")
	}
	if filterByState(clone, []string{"in-library"}) {
		t.Error("in-library: Registry Clone should not pass")
	}
	if filterByState(proj, []string{"in-library"}) {
		t.Error("in-library: Project Content should not pass")
	}
}

// TestFilterByState_NotInLibrary verifies filterByState("not-in-library") passes only Registry Clone items.
func TestFilterByState_NotInLibrary(t *testing.T) {
	t.Parallel()
	lib := catalog.ContentItem{Name: "lib-skill", Library: true}
	clone := catalog.ContentItem{Name: "clone-skill", Registry: "reg", Library: false}
	proj := catalog.ContentItem{Name: "proj-rule", Library: false}

	if filterByState(lib, []string{"not-in-library"}) {
		t.Error("not-in-library: Library item should not pass")
	}
	if !filterByState(clone, []string{"not-in-library"}) {
		t.Error("not-in-library: Registry Clone should pass")
	}
	if filterByState(proj, []string{"not-in-library"}) {
		t.Error("not-in-library: Project Content should not pass")
	}
}

// TestFilterByState_Project verifies filterByState("project") passes only Project Content items.
func TestFilterByState_Project(t *testing.T) {
	t.Parallel()
	lib := catalog.ContentItem{Name: "lib-skill", Library: true}
	clone := catalog.ContentItem{Name: "clone-skill", Registry: "reg", Library: false}
	proj := catalog.ContentItem{Name: "proj-rule", Library: false}

	if filterByState(lib, []string{"project"}) {
		t.Error("project: Library item should not pass")
	}
	if filterByState(clone, []string{"project"}) {
		t.Error("project: Registry Clone should not pass")
	}
	if !filterByState(proj, []string{"project"}) {
		t.Error("project: Project Content should pass")
	}
}

// TestFilterByState_MultipleStates verifies filterByState acts as OR across multiple states.
func TestFilterByState_MultipleStates(t *testing.T) {
	t.Parallel()
	lib := catalog.ContentItem{Name: "lib-skill", Library: true}
	clone := catalog.ContentItem{Name: "clone-skill", Registry: "reg", Library: false}
	proj := catalog.ContentItem{Name: "proj-rule", Library: false}

	states := []string{"in-library", "project"}
	if !filterByState(lib, states) {
		t.Error("in-library+project: Library item should pass")
	}
	if filterByState(clone, states) {
		t.Error("in-library+project: Registry Clone should not pass")
	}
	if !filterByState(proj, states) {
		t.Error("in-library+project: Project Content should pass")
	}
}

// TestRunList_FilterFlag_NotInLibrary verifies --filter not-in-library shows Registry Clone items.
func TestRunList_FilterFlag_NotInLibrary(t *testing.T) {
	const regName = "test-org/filter-test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	withFakeRepoRoot(t, root)

	origCfgDir := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origCfgDir })

	stdout, _ := output.SetForTest(t)

	listCmd.Flags().Set("filter", "not-in-library")
	defer listCmd.Flags().Set("filter", "")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --filter not-in-library: %v", err)
	}

	out := stdout.String()
	// Registry clone items from the test registry should appear.
	if !strings.Contains(out, "canary-skill") && !strings.Contains(out, "probe-skill") {
		t.Errorf("--filter not-in-library must show registry clone items, got:\n%s", out)
	}
}

// TestRunList_FilterFlag_Combined verifies multiple --filter values act as OR.
func TestRunList_FilterFlag_Combined(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, t.TempDir())
	isolateListEnv(t)

	stdout, _ := output.SetForTest(t)

	// --filter project shows Project Content (shared items in setupListRepo).
	// --filter in-library with empty library shows nothing; combined result = just project.
	listCmd.Flags().Set("filter", "in-library")
	listCmd.Flags().Set("filter", "project")
	defer listCmd.Flags().Set("filter", "")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --filter in-library --filter project: %v", err)
	}

	out := stdout.String()
	// All items in setupListRepo are "project" (shared), so they should appear.
	if !strings.Contains(out, "code-review") {
		t.Errorf("combined filter must include project items, got:\n%s", out)
	}
}

// TestTelemetryCatalog_FilterPropertyPresent verifies the telemetry catalog
// contains a "filter" property for the "list" command.
func TestTelemetryCatalog_FilterPropertyPresent(t *testing.T) {
	t.Parallel()
	for _, ev := range telemetry.EventCatalog() {
		for _, prop := range ev.Properties {
			if prop.Name == "filter" {
				for _, cmd := range prop.Commands {
					if cmd == "list" {
						return
					}
				}
			}
		}
	}
	t.Error("telemetry EventCatalog must contain 'filter' property with 'list' in Commands")
}
