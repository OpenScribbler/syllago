package main

import (
	"bufio"
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
