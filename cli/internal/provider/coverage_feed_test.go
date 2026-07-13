package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// feedFixtureRoot builds a minimal repo root whose
// docs/provider-capabilities/capabilities/<slug>.json is the given document.
func feedFixtureRoot(t *testing.T, slug, capDoc string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"docs/provider-sources", "docs/provider-formats"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if capDoc != "" {
		dir := filepath.Join(root, "docs", "provider-capabilities", "capabilities")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, slug+".json"), []byte(capDoc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// fakeProvider claims support for exactly the given content types.
func fakeProvider(slug string, supported ...catalog.ContentType) Provider {
	set := map[catalog.ContentType]bool{}
	for _, ct := range supported {
		set[ct] = true
	}
	return Provider{
		Name: slug, Slug: slug,
		SupportsType: func(ct catalog.ContentType) bool { return set[ct] },
	}
}

func TestCheckFeedCoverage_Contradictions(t *testing.T) {
	tests := []struct {
		name       string
		capDoc     string
		provider   Provider
		wantDrifts int
		wantCT     catalog.ContentType
		wantErr    bool
	}{
		{
			name:       "feed says supported, Go says no: contradiction",
			capDoc:     `{"slug":"fake","content_types":{"skills":{"supported":true}}}`,
			provider:   fakeProvider("fake" /* no skills */),
			wantDrifts: 1,
			wantCT:     catalog.Skills,
		},
		{
			name:       "feed says unsupported, Go claims it: missed capability",
			capDoc:     `{"slug":"fake","content_types":{"hooks":{"supported":false}}}`,
			provider:   fakeProvider("fake", catalog.Hooks),
			wantDrifts: 1,
			wantCT:     catalog.Hooks,
		},
		{
			name:       "supported absent means unknown: no finding",
			capDoc:     `{"slug":"fake","content_types":{"skills":{"capabilities":{}}}}`,
			provider:   fakeProvider("fake", catalog.Skills),
			wantDrifts: 0,
		},
		{
			name:       "agreement in both directions: no findings",
			capDoc:     `{"slug":"fake","content_types":{"skills":{"supported":true},"hooks":{"supported":false}}}`,
			provider:   fakeProvider("fake", catalog.Skills),
			wantDrifts: 0,
		},
		{
			name:       "no Capability Document for the provider: no findings",
			capDoc:     "",
			provider:   fakeProvider("fake", catalog.Skills),
			wantDrifts: 0,
		},
		{
			name:       "unknown JSON fields are ignored (tolerant reader)",
			capDoc:     `{"slug":"fake","future_key":[1,2],"content_types":{"skills":{"supported":true,"future":{"x":1}}}}`,
			provider:   fakeProvider("fake", catalog.Skills),
			wantDrifts: 0,
		},
		{
			name:     "malformed JSON is an error, not silence",
			capDoc:   `{"slug":"fake","content_types":`,
			provider: fakeProvider("fake", catalog.Skills),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := feedFixtureRoot(t, "fake", tt.capDoc)
			drifts, err := checkFeedCoverage(root, tt.provider)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("checkFeedCoverage returned %d drifts; want error", len(drifts))
				}
				return
			}
			if err != nil {
				t.Fatalf("checkFeedCoverage: %v", err)
			}
			if len(drifts) != tt.wantDrifts {
				t.Fatalf("got %d drifts (%v); want %d", len(drifts), drifts, tt.wantDrifts)
			}
			if tt.wantDrifts == 1 {
				d := drifts[0]
				if d.Assertion != AssertionGoVsCapabilityFeed {
					t.Errorf("drift assertion = %q; want %q", d.Assertion, AssertionGoVsCapabilityFeed)
				}
				if d.ContentType != tt.wantCT {
					t.Errorf("drift content type = %q; want %q", d.ContentType, tt.wantCT)
				}
				if d.Provider != "fake" {
					t.Errorf("drift provider = %q; want fake", d.Provider)
				}
			}
		})
	}
}

func TestCheckCoverage_FeedAssertionIntegrated(t *testing.T) {
	// A Capability Document contradicting a real provider's Go claim:
	// claude-code's SupportsType(skills) is true, the doc asserts false.
	root := feedFixtureRoot(t, "claude-code",
		`{"slug":"claude-code","content_types":{"skills":{"supported":false}}}`)

	drifts, err := CheckCoverage(root)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}
	var feedDrifts []CoverageDrift
	for _, d := range drifts {
		if d.Assertion == AssertionGoVsCapabilityFeed {
			feedDrifts = append(feedDrifts, d)
		}
	}
	if len(feedDrifts) != 1 {
		t.Fatalf("feed-assertion drifts = %v; want exactly 1 (claude-code/skills)", feedDrifts)
	}
	if feedDrifts[0].Provider != "claude-code" || feedDrifts[0].ContentType != catalog.Skills {
		t.Errorf("feed drift = %+v; want claude-code/skills", feedDrifts[0])
	}
}

// TestCoverageFeedDrift is the CI Coverage Drift gate: red (non-required)
// when Go's SupportsType contradicts the committed Capability Documents.
// Reproduce locally with: SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift
func TestCoverageFeedDrift(t *testing.T) {
	if os.Getenv("SYLLAGO_COVERAGE_FEED") != "1" {
		t.Skip("set SYLLAGO_COVERAGE_FEED=1 to run the Coverage Drift gate")
	}
	root := mustFindRepoRoot(t)
	drifts, err := CheckCoverage(root)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}
	var failed bool
	for _, d := range drifts {
		if d.Assertion == AssertionGoVsCapabilityFeed {
			failed = true
			t.Errorf("Coverage Drift: %s", d)
		}
	}
	if failed {
		t.Log("Go SupportsType claims contradict the mirrored Capability Feed data; reconcile the provider's SupportsType or await a feed correction")
	}
}
