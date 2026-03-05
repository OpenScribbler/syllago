package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestShareCmdRegisters(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "share <name>" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected share command registered on rootCmd")
	}
}

func TestShareCmdValidatesArgs(t *testing.T) {
	shareCmd.SilenceUsage = true
	shareCmd.SilenceErrors = true

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"two args", []string{"first", "second"}, true},
		// One arg is correct — arg validation passes; RunE will fail later.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := shareCmd.Args(shareCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestFindLibraryItem_NotFound(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "other-skill", Type: catalog.Skills, Source: "global"},
		},
	}

	_, err := findLibraryItem(cat, "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for item not found")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestFindLibraryItem_Found(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Source: "global"},
		},
	}

	item, err := findLibraryItem(cat, "my-skill", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if item.Name != "my-skill" {
		t.Errorf("expected item named 'my-skill', got %q", item.Name)
	}
}

func TestFindLibraryItem_IgnoresNonGlobal(t *testing.T) {
	_, _ = output.SetForTest(t)

	// Item exists but is not in the global library (e.g. shared or registry).
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Source: "shared"},
			{Name: "my-skill", Type: catalog.Skills, Source: "registry-foo"},
		},
	}

	_, err := findLibraryItem(cat, "my-skill", "")
	if err == nil {
		t.Fatal("expected error: non-global items should not be found")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestFindLibraryItem_AmbiguousWithoutTypeFilter(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-item", Type: catalog.Skills, Source: "global"},
			{Name: "my-item", Type: catalog.Rules, Source: "global"},
		},
	}

	_, err := findLibraryItem(cat, "my-item", "")
	if err == nil {
		t.Fatal("expected error for ambiguous name")
	}
	if !strings.Contains(err.Error(), "Use --type") {
		t.Errorf("expected '--type' hint in error, got: %v", err)
	}
}

func TestFindLibraryItem_TypeFilterDisambiguates(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-item", Type: catalog.Skills, Source: "global"},
			{Name: "my-item", Type: catalog.Rules, Source: "global"},
		},
	}

	item, err := findLibraryItem(cat, "my-item", string(catalog.Skills))
	if err != nil {
		t.Fatalf("expected no error with type filter, got: %v", err)
	}
	if item.Type != catalog.Skills {
		t.Errorf("expected skills type, got %q", item.Type)
	}
}

func TestShareItemNotFound(t *testing.T) {
	lib := setupConvertLibrary(t) // reuse: creates a skill named "my-skill"
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	shareCmd.Flags().Set("type", "")
	defer shareCmd.Flags().Set("type", "")

	err := shareCmd.RunE(shareCmd, []string{"nonexistent-item"})
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}
