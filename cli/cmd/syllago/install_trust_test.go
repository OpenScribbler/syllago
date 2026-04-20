package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// findTestProvider fetches the test provider by slug from AllProviders.
// addTestProvider appends to the slice; this pulls the pointer back out.
func findTestProvider(t *testing.T, slug string) *provider.Provider {
	t.Helper()
	for i := range provider.AllProviders {
		if provider.AllProviders[i].Slug == slug {
			return &provider.AllProviders[i]
		}
	}
	t.Fatalf("test provider %q not found in AllProviders", slug)
	return nil
}

// buildTrustItems builds a single ContentItem backed by a real library path
// so installer.InstallWithResolver can symlink the source on disk.
func buildTrustItems(t *testing.T, globalDir string, tier catalog.TrustTier, recalled bool, reason string) []catalog.ContentItem {
	t.Helper()
	return []catalog.ContentItem{{
		Name:         "my-skill",
		Type:         catalog.Skills,
		Path:         filepath.Join(globalDir, "skills", "my-skill"),
		Library:      true,
		Source:       "library",
		TrustTier:    tier,
		Recalled:     recalled,
		RecallReason: reason,
	}}
}

func TestInstallTrustLine_DualAttested(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "trust-prov-dual", "Trust Provider Dual", installBase)
	prov := findTestProvider(t, "trust-prov-dual")

	stdout, _ := output.SetForTest(t)

	items := buildTrustItems(t, globalDir, catalog.TrustTierDualAttested, false, "")
	result, err := installToProvider(items, *prov, globalDir, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, t.TempDir())
	if err != nil {
		t.Fatalf("installToProvider: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Verified (dual-attested") {
		t.Errorf("expected dual-attested drill-down text in stdout, got: %s", out)
	}
	if !strings.Contains(out, "\u2713") {
		t.Errorf("expected Verified glyph (U+2713) in stdout, got: %s", out)
	}

	if len(result.Installed) != 1 {
		t.Fatalf("expected 1 installed item, got %d", len(result.Installed))
	}
	if got := result.Installed[0].Trust; got != "Verified (dual-attested by publisher and registry)" {
		t.Errorf("unexpected Trust text: %q", got)
	}
}

func TestInstallTrustLine_Signed(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "trust-prov-signed", "Trust Provider Signed", installBase)
	prov := findTestProvider(t, "trust-prov-signed")

	stdout, _ := output.SetForTest(t)

	items := buildTrustItems(t, globalDir, catalog.TrustTierSigned, false, "")
	result, _ := installToProvider(items, *prov, globalDir, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, t.TempDir())

	if !strings.Contains(stdout.String(), "Verified (registry-attested)") {
		t.Errorf("expected signed-tier drill-down text, got: %s", stdout.String())
	}
	if result.Installed[0].Trust != "Verified (registry-attested)" {
		t.Errorf("unexpected Trust text: %q", result.Installed[0].Trust)
	}
}

func TestInstallTrustLine_Recalled(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "trust-prov-recalled", "Trust Provider Recalled", installBase)
	prov := findTestProvider(t, "trust-prov-recalled")

	stdout, _ := output.SetForTest(t)

	// Recalled takes precedence over TrustTier per AD-7 collapse rule —
	// DualAttested + Recalled must still render as Recalled, not Verified.
	items := buildTrustItems(t, globalDir, catalog.TrustTierDualAttested, true, "publisher revoked 2026-04-18")
	result, _ := installToProvider(items, *prov, globalDir, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, t.TempDir())

	out := stdout.String()
	if !strings.Contains(out, "Recalled \u2014 publisher revoked 2026-04-18") {
		t.Errorf("expected recall reason in drill-down, got: %s", out)
	}
	if !strings.Contains(out, "\u2717") {
		t.Errorf("expected Recalled glyph (U+2717) in stdout, got: %s", out)
	}
	if strings.Contains(out, "Verified") {
		t.Errorf("Recalled must suppress Verified badge, but output contained Verified: %s", out)
	}
	if !strings.HasPrefix(result.Installed[0].Trust, "Recalled") {
		t.Errorf("expected Trust field to start with Recalled, got: %q", result.Installed[0].Trust)
	}
}

func TestInstallTrustLine_UnknownSuppressed(t *testing.T) {
	// Zero-value TrustTier (items not sourced from MOAT) must emit no trust
	// line and no "trust" JSON field. This is AD-7's "absence is not a
	// negative signal" contract — a git registry item must look identical
	// to pre-G-6 output.
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "trust-prov-unknown", "Trust Provider Unknown", installBase)
	prov := findTestProvider(t, "trust-prov-unknown")

	stdout, _ := output.SetForTest(t)

	items := buildTrustItems(t, globalDir, catalog.TrustTierUnknown, false, "")
	result, _ := installToProvider(items, *prov, globalDir, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, t.TempDir())

	out := stdout.String()
	if strings.Contains(out, "Verified") || strings.Contains(out, "Recalled") ||
		strings.Contains(out, "\u2713") || strings.Contains(out, "\u2717") {
		t.Errorf("Unknown tier must not render trust badge, but output had one: %s", out)
	}
	if result.Installed[0].Trust != "" {
		t.Errorf("expected empty Trust field for Unknown tier, got %q", result.Installed[0].Trust)
	}
}

func TestInstallTrustLine_JSONModeSuppressesText(t *testing.T) {
	// JSON mode must emit the `trust` key but no loose trust line in stdout;
	// the whole stdout is a single JSON document.
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "trust-prov-json", "Trust Provider JSON", installBase)
	prov := findTestProvider(t, "trust-prov-json")

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	t.Cleanup(func() { output.JSON = false })

	items := buildTrustItems(t, globalDir, catalog.TrustTierSigned, false, "")
	result, _ := installToProvider(items, *prov, globalDir, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, t.TempDir())

	// installToProvider does not print the result JSON itself — that's the
	// caller's job. In JSON mode it must not print the human trust line.
	if strings.Contains(stdout.String(), "Verified") {
		t.Errorf("JSON mode must not emit human trust line, got: %s", stdout.String())
	}

	// The Trust field must still be populated on the struct so the caller
	// can marshal it.
	payload, err := json.Marshal(result.Installed[0])
	if err != nil {
		t.Fatalf("marshal installed item: %v", err)
	}
	if !strings.Contains(string(payload), `"trust":"Verified (registry-attested)"`) {
		t.Errorf("expected trust key in JSON, got: %s", payload)
	}

	// And an Unknown-tier item must omit the trust key entirely (omitempty).
	itemsUnknown := buildTrustItems(t, globalDir, catalog.TrustTierUnknown, false, "")
	resultU, _ := installToProvider(itemsUnknown, *prov, globalDir, installer.MethodSymlink,
		true /* dryRun so we don't re-install to the same target */, config.NewResolver(nil, ""), prov.Slug, t.TempDir())
	if len(resultU.Installed) > 0 {
		t.Fatalf("dry-run should not produce Installed entries, got %d", len(resultU.Installed))
	}
	// The zero-value case is already covered by TestInstallTrustLine_UnknownSuppressed;
	// this just confirms dry-run doesn't accidentally emit the line either.
	if strings.Contains(stdout.String(), "Verified") || strings.Contains(stdout.String(), "Recalled") {
		t.Errorf("dry-run + Unknown must not emit trust line, got: %s", stdout.String())
	}
}
