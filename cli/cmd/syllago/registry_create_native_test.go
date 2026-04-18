package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestAllItemsFromScan_Empty(t *testing.T) {
	t.Parallel()
	result := catalog.NativeScanResult{}
	items := allItemsFromScan(result)
	if len(items) != 0 {
		t.Errorf("expected empty, got %d items", len(items))
	}
}

func TestAllItemsFromScan_MultipleProviders(t *testing.T) {
	t.Parallel()
	result := catalog.NativeScanResult{
		Providers: []catalog.NativeProviderContent{
			{
				ProviderSlug: "claude-code",
				ProviderName: "Claude Code",
				Items: map[string][]catalog.NativeItem{
					"rules": {
						{Name: "my-rule", Path: "rules/my-rule.md"},
					},
					"skills": {
						{Name: "my-skill", Path: "skills/my-skill/"},
					},
				},
			},
			{
				ProviderSlug: "gemini-cli",
				ProviderName: "Gemini CLI",
				Items: map[string][]catalog.NativeItem{
					"rules": {
						{Name: "gem-rule", Path: "rules/gem-rule.md"},
					},
				},
			},
		},
	}
	items := allItemsFromScan(result)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	provSlugs := map[string]bool{}
	for _, item := range items {
		provSlugs[item.Provider] = true
	}
	if !provSlugs["claude-code"] || !provSlugs["gemini-cli"] {
		t.Errorf("expected both providers, got %v", provSlugs)
	}
}

func TestAllItemsFromScan_HookFields(t *testing.T) {
	t.Parallel()
	result := catalog.NativeScanResult{
		Providers: []catalog.NativeProviderContent{
			{
				ProviderSlug: "claude-code",
				ProviderName: "Claude Code",
				Items: map[string][]catalog.NativeItem{
					"hooks": {
						{Name: "my-hook", Path: "hooks/my-hook.json", HookEvent: "PreToolUse", HookIndex: 2},
					},
				},
			},
		},
	}
	items := allItemsFromScan(result)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].HookEvent != "PreToolUse" {
		t.Errorf("HookEvent = %q, want PreToolUse", items[0].HookEvent)
	}
	if items[0].HookIndex != 2 {
		t.Errorf("HookIndex = %d, want 2", items[0].HookIndex)
	}
}

func testScanResult() catalog.NativeScanResult {
	return catalog.NativeScanResult{
		Providers: []catalog.NativeProviderContent{
			{
				ProviderSlug: "claude-code",
				ProviderName: "Claude Code",
				Items: map[string][]catalog.NativeItem{
					"rules": {
						{Name: "rule-a", Path: "rules/a.md"},
						{Name: "rule-b", Path: "rules/b.md"},
					},
				},
			},
			{
				ProviderSlug: "gemini-cli",
				ProviderName: "Gemini CLI",
				Items: map[string][]catalog.NativeItem{
					"rules": {
						{Name: "gem-rule", Path: "rules/gem.md"},
					},
				},
			},
		},
	}
}

func TestSelectByProvider_ValidSelection(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("1\n"))
	items, err := selectByProvider(testScanResult(), scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items from provider 1, got %d", len(items))
	}
	for _, item := range items {
		if item.Provider != "claude-code" {
			t.Errorf("expected provider claude-code, got %q", item.Provider)
		}
	}
}

func TestSelectByProvider_MultipleProviders(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("1,2\n"))
	items, err := selectByProvider(testScanResult(), scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items from both providers, got %d", len(items))
	}
}

func TestSelectByProvider_InvalidIndex(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("99\n"))
	items, err := selectByProvider(testScanResult(), scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for invalid index, got %d", len(items))
	}
}

func TestSelectByProvider_NoInput(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader(""))
	_, err := selectByProvider(testScanResult(), scanner)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestSelectIndividualItems_SpecificItems(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("1,3\n"))
	items, err := selectIndividualItems(testScanResult(), scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 selected items, got %d", len(items))
	}
}

func TestSelectIndividualItems_All(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("all\n"))
	items, err := selectIndividualItems(testScanResult(), scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items for 'all', got %d", len(items))
	}
}

func TestSelectIndividualItems_NoInput(t *testing.T) {
	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader(""))
	_, err := selectIndividualItems(testScanResult(), scanner)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestSelectIndividualItems_DisplayName(t *testing.T) {
	output.SetForTest(t)
	result := catalog.NativeScanResult{
		Providers: []catalog.NativeProviderContent{
			{
				ProviderSlug: "claude-code",
				ProviderName: "Claude Code",
				Items: map[string][]catalog.NativeItem{
					"rules": {
						{Name: "my-rule", DisplayName: "My Pretty Rule", Path: "rules/my-rule.md"},
					},
				},
			},
		},
	}
	scanner := bufio.NewScanner(strings.NewReader("1\n"))
	items, err := selectIndividualItems(result, scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "my-rule" {
		t.Errorf("Name = %q, want my-rule", items[0].Name)
	}
}

// Compile-time check that ManifestItem type is used correctly.
var _ = []registry.ManifestItem{}

// --- registryCreateFromNative (0% coverage) ---

// chdirToNative chdirs into a tmp dir seeded with a `.cursorrules` file and an
// empty $HOME so promptUserScopedHooks finds nothing. Returns the directory.
func chdirToNative(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, ".cursorrules"), []byte("# rules\n"), 0644)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Empty home to keep promptUserScopedHooks quiet.
	t.Setenv("HOME", t.TempDir())
	return tmp
}

// stubRegistryStdin wires registryCreateNativeStdin to a canned multi-line input.
func stubRegistryStdin(t *testing.T, input string) {
	t.Helper()
	orig := registryCreateNativeStdin
	registryCreateNativeStdin = strings.NewReader(input)
	t.Cleanup(func() { registryCreateNativeStdin = orig })
}

func TestRegistryCreateFromNative_ExistingManifestErrors(t *testing.T) {
	tmp := chdirToNative(t)
	os.WriteFile(filepath.Join(tmp, "registry.yaml"), []byte("name: x\n"), 0644)
	output.SetForTest(t)

	err := registryCreateFromNative("")
	if err == nil {
		t.Fatal("expected error when registry.yaml already exists")
	}
	if !strings.Contains(err.Error(), "registry.yaml") {
		t.Errorf("error should mention registry.yaml, got %v", err)
	}
}

func TestRegistryCreateFromNative_SyllagoStructureErrors(t *testing.T) {
	// HasSyllagoStructure is triggered by a registry.yaml OR by syllago content dirs.
	// The registry.yaml path is guarded earlier (above), so use a content dir.
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "rules"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	t.Cleanup(func() { os.Chdir(origDir) })
	t.Setenv("HOME", t.TempDir())

	output.SetForTest(t)

	err := registryCreateFromNative("")
	if err == nil {
		t.Fatal("expected error for syllago structure")
	}
	if !strings.Contains(err.Error(), "syllago structure") {
		t.Errorf("error should mention syllago structure, got %v", err)
	}
}

func TestRegistryCreateFromNative_NoContentErrors(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	t.Cleanup(func() { os.Chdir(origDir) })
	t.Setenv("HOME", t.TempDir())

	output.SetForTest(t)

	err := registryCreateFromNative("")
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no AI coding tool content") {
		t.Errorf("error should mention no content, got %v", err)
	}
}

func TestRegistryCreateFromNative_HappyPathAll(t *testing.T) {
	tmp := chdirToNative(t)
	// Inputs: choice (empty → default "1" = all), registry name (empty → default basename), desc (prompt when desc flag empty).
	stubRegistryStdin(t, "\n\n\n")
	output.SetForTest(t)

	if err := registryCreateFromNative("my description"); err != nil {
		t.Fatalf("happy path should succeed, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "registry.yaml"))
	if err != nil {
		t.Fatalf("registry.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "my description") {
		t.Errorf("description missing from manifest: %s", data)
	}
}

func TestRegistryCreateFromNative_HappyPathByProvider(t *testing.T) {
	tmp := chdirToNative(t)
	// Inputs: "2" (by provider), "1" (select first provider), name empty, desc prompt empty.
	stubRegistryStdin(t, "2\n1\n\n\n")
	output.SetForTest(t)

	if err := registryCreateFromNative(""); err != nil {
		t.Fatalf("by-provider path should succeed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "registry.yaml")); err != nil {
		t.Errorf("registry.yaml not written: %v", err)
	}
}

func TestRegistryCreateFromNative_HappyPathIndividual(t *testing.T) {
	tmp := chdirToNative(t)
	// Inputs: "3" (individual), "1" (select item), name empty, desc prompt empty.
	stubRegistryStdin(t, "3\n1\n\n\n")
	output.SetForTest(t)

	if err := registryCreateFromNative(""); err != nil {
		t.Fatalf("individual-selection path should succeed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "registry.yaml")); err != nil {
		t.Errorf("registry.yaml not written: %v", err)
	}
}

func TestRegistryCreateFromNative_InvalidChoiceErrors(t *testing.T) {
	chdirToNative(t)
	stubRegistryStdin(t, "99\n")
	output.SetForTest(t)

	err := registryCreateFromNative("")
	if err == nil {
		t.Fatal("expected error for invalid choice")
	}
	if !strings.Contains(err.Error(), "invalid choice") {
		t.Errorf("error should mention invalid choice, got %v", err)
	}
}

func TestRegistryCreateFromNative_EmptySelectionErrors(t *testing.T) {
	chdirToNative(t)
	// Choice 3 (individual), then empty selection.
	stubRegistryStdin(t, "3\n\n")
	output.SetForTest(t)

	err := registryCreateFromNative("")
	if err == nil {
		t.Fatal("expected error for no items selected")
	}
	if !strings.Contains(err.Error(), "no items selected") {
		t.Errorf("error should mention no items selected, got %v", err)
	}
}

func TestRegistryCreateFromNative_CustomName(t *testing.T) {
	tmp := chdirToNative(t)
	// Choice 1 (all), custom name "my-registry", desc empty.
	stubRegistryStdin(t, "1\nmy-registry\n\n")
	output.SetForTest(t)

	if err := registryCreateFromNative(""); err != nil {
		t.Fatalf("custom-name path should succeed, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "registry.yaml"))
	if err != nil {
		t.Fatalf("registry.yaml missing: %v", err)
	}
	if !strings.Contains(string(data), "my-registry") {
		t.Errorf("custom name missing from manifest: %s", data)
	}
}

// --- promptUserScopedHooks (0% coverage) ---

func TestPromptUserScopedHooks_NoAvailableReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no settings.json anywhere
	scanner := bufio.NewScanner(strings.NewReader(""))
	items, err := promptUserScopedHooks(scanner, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items when no sources available, got %d", len(items))
	}
}

func TestPromptUserScopedHooks_UserSkipsReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Create a settings.json with at least one hook so source is available.
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)

	output.SetForTest(t)
	scanner := bufio.NewScanner(strings.NewReader("0\n"))
	items, err := promptUserScopedHooks(scanner, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil when user chooses 0, got %d items", len(items))
	}
}

func TestPromptUserScopedHooks_UserDeniesSecurityWarningReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)

	output.SetForTest(t)
	// Choose source "1", then deny security warning with "n".
	scanner := bufio.NewScanner(strings.NewReader("1\nn\n"))
	items, err := promptUserScopedHooks(scanner, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil when user denies security warning, got %d items", len(items))
	}
}

func TestPromptUserScopedHooks_SelectAllExtractsHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)

	repoRoot := t.TempDir()
	output.SetForTest(t)
	// Source 1, confirm "y", select "all".
	scanner := bufio.NewScanner(strings.NewReader("1\ny\nall\n"))
	items, err := promptUserScopedHooks(scanner, repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one extracted hook item")
	}
	// Verify hook was written to .syllago/hooks/
	hooksDir := filepath.Join(repoRoot, ".syllago", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		t.Fatalf("hooks dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no hooks extracted to .syllago/hooks/")
	}
}

func TestPromptUserScopedHooks_InvalidChoiceReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)

	output.SetForTest(t)
	// "abc" is not a valid index.
	scanner := bufio.NewScanner(strings.NewReader("abc\n"))
	items, err := promptUserScopedHooks(scanner, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil for non-numeric choice, got %d items", len(items))
	}
}
