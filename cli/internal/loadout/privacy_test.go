package loadout

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestCheckPrivateItems(t *testing.T) {
	t.Parallel()

	t.Run("detects private items", func(t *testing.T) {
		t.Parallel()
		items := []catalog.ContentItem{
			{Name: "public-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "public", SourceRegistry: "open/repo"}},
			{Name: "private-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "private", SourceRegistry: "acme/internal"}},
			{Name: "no-meta", Type: catalog.Rules},
		}

		warnings := CheckPrivateItems(items)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Name != "private-rule" {
			t.Errorf("warning name = %q, want %q", warnings[0].Name, "private-rule")
		}
		if warnings[0].Registry != "acme/internal" {
			t.Errorf("warning registry = %q, want %q", warnings[0].Registry, "acme/internal")
		}
	})

	t.Run("no private items returns empty", func(t *testing.T) {
		t.Parallel()
		items := []catalog.ContentItem{
			{Name: "public-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "public", SourceRegistry: "open/repo"}},
		}
		warnings := CheckPrivateItems(items)
		if len(warnings) != 0 {
			t.Errorf("expected 0 warnings, got %d", len(warnings))
		}
	})
}

func TestFormatPrivateWarnings(t *testing.T) {
	t.Parallel()

	t.Run("formats warnings", func(t *testing.T) {
		t.Parallel()
		warnings := []PrivateItemWarning{
			{Name: "secret-rule", Type: catalog.Rules, Registry: "acme/internal"},
		}
		msg := FormatPrivateWarnings(warnings)
		if !strings.Contains(msg, "1 item(s) from private registries") {
			t.Errorf("format should mention count, got: %s", msg)
		}
		if !strings.Contains(msg, "secret-rule") {
			t.Errorf("format should mention item name, got: %s", msg)
		}
	})

	t.Run("empty returns empty", func(t *testing.T) {
		t.Parallel()
		msg := FormatPrivateWarnings(nil)
		if msg != "" {
			t.Errorf("empty warnings should return empty string, got: %s", msg)
		}
	})
}

func TestCheckLoadoutPublishGate_PrivateItemsToPublic_Blocked(t *testing.T) {
	t.Parallel()
	items := []catalog.ContentItem{
		{Name: "private-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "private", SourceRegistry: "acme/internal"}},
	}

	err := CheckLoadoutPublishGate(items, "public")
	if err == nil {
		t.Fatal("expected G4 gate to block, got nil")
	}
	if !strings.Contains(err.Error(), "cannot publish loadout") {
		t.Errorf("error should mention 'cannot publish loadout', got: %s", err)
	}
}

func TestCheckLoadoutPublishGate_PrivateItemsToPrivate_Allowed(t *testing.T) {
	t.Parallel()
	items := []catalog.ContentItem{
		{Name: "private-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "private", SourceRegistry: "acme/internal"}},
	}

	err := CheckLoadoutPublishGate(items, "private")
	if err != nil {
		t.Fatalf("private->private should be allowed, got: %s", err)
	}
}

func TestCheckLoadoutPublishGate_PublicItems_Allowed(t *testing.T) {
	t.Parallel()
	items := []catalog.ContentItem{
		{Name: "public-rule", Type: catalog.Rules, Meta: &metadata.Meta{SourceVisibility: "public", SourceRegistry: "open/repo"}},
	}

	err := CheckLoadoutPublishGate(items, "public")
	if err != nil {
		t.Fatalf("public items to public registry should be allowed, got: %s", err)
	}
}
