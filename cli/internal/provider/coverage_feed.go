package provider

// Coverage Drift against the Capability Feed mirror (capmon-pull slice 5).
// checkFeedCoverage compares Go's SupportsType claims with the Capability
// Documents that Capmon Pull mirrors into docs/provider-capabilities/
// (verbatim, attestation-verified copies of the feed's
// capabilities/<slug>.json). Tolerant reader: supported is three-state —
// absent means unknown and yields no finding, matching the format-YAML
// assertion's "empty status = not asserted" stance.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// acceptedFeedDrift records provider/content-type pairs where the feed's
// concept-level `supported` and Go's install-level SupportsType disagree and
// BOTH are correct: the provider has the capability concept, but offers no
// installable form. The feed answers "does the provider have this concept?";
// SupportsType answers "can syllago install this content type there?" — for
// these pairs the answers legitimately diverge, so the CI gate reports them
// as accepted instead of failing. Every entry MUST carry a reason. Delete
// the entry (and implement install support) if the provider ships an
// installable form.
var acceptedFeedDrift = map[string]string{
	// Zed Agent Profiles carry tool restrictions and per-profile MCP scoping
	// (so the feed's auto-flipped agents.supported is true), but profiles are
	// settings.json entries with no prompt and no definition file — there is
	// nothing an agent content item could be installed into (syllago-s1zou).
	"zed/agents": "Zed Agent Profiles have no prompt or definition file; agent items cannot be installed",
}

// capabilityDocument mirrors the subset of capabilities/<slug>.json the
// feed assertion needs. Unknown fields at every level are ignored (default
// encoding/json semantics — never DisallowUnknownFields).
type capabilityDocument struct {
	ContentTypes map[string]capabilityContentEntry `json:"content_types"`
}

type capabilityContentEntry struct {
	Supported *bool `json:"supported,omitempty"`
}

// checkFeedCoverage returns a CoverageDrift for each content type where the
// provider's Capability Document asserts `supported` and Go's SupportsType
// disagrees — in either direction. A missing document is no finding (the
// feed does not track every provider); a malformed one is an error so CI
// surfaces corruption instead of silently passing.
func checkFeedCoverage(repoRoot string, p Provider) ([]CoverageDrift, error) {
	path := filepath.Join(repoRoot, "docs", "provider-capabilities", "capabilities", p.Slug+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("capability document %s: %w", path, err)
	}

	var doc capabilityDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("capability document %s: decode: %w", path, err)
	}

	var drifts []CoverageDrift
	for _, ct := range CoverageContentTypes {
		entry, has := doc.ContentTypes[string(ct)]
		if !has || entry.Supported == nil {
			continue // unknown, never a finding
		}
		goSupported := p.SupportsType(ct)
		feedSupported := *entry.Supported
		if goSupported == feedSupported {
			continue
		}
		var msg, accepted string
		if goSupported {
			msg = "Go SupportsType claims support but the Capability Feed asserts supported: false"
			// Never accepted: Go claiming support the feed denies is always
			// a real finding, whatever acceptedFeedDrift says.
		} else {
			msg = "Capability Feed asserts supported: true but Go SupportsType returns false (missed capability)"
			accepted = acceptedFeedDrift[p.Slug+"/"+string(ct)]
		}
		drifts = append(drifts, CoverageDrift{
			Provider:       p.Slug,
			ContentType:    ct,
			Assertion:      AssertionGoVsCapabilityFeed,
			Message:        msg,
			AcceptedReason: accepted,
		})
	}
	return drifts, nil
}
