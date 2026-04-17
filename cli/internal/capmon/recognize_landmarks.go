package capmon

import (
	"sort"
	"strings"
)

// StringMatcher describes how to match a string against a landmark or item value.
// Two kinds in v1: "exact" and "substring". Regex is deliberately deferred —
// every regex we'd want today expresses cleanly as exact-or-substring, and
// adding regex later is a strictly additive change.
type StringMatcher struct {
	// Kind is "exact" or "substring". Empty kind defaults to "substring"
	// (the lower-precision, higher-recall option) so partial configurations
	// don't silently fail to match.
	Kind string
	// Value is the literal string compared to landmarks. Stored verbatim;
	// case-folding is controlled by CaseInsensitive at compare time.
	Value string
	// CaseInsensitive folds both the matcher value and the landmark to
	// lowercase ASCII before comparison. Defaults to true (most landmarks
	// are headings whose case may drift across doc updates).
	CaseInsensitive bool
}

// matches reports whether the matcher matches the given landmark string.
func (m StringMatcher) matches(landmark string) bool {
	value := m.Value
	candidate := landmark
	if m.CaseInsensitive {
		value = strings.ToLower(value)
		candidate = strings.ToLower(candidate)
	}
	switch m.Kind {
	case "exact":
		return candidate == value
	case "", "substring":
		return strings.Contains(candidate, value)
	default:
		return false
	}
}

// LandmarkPattern declares one capability detection rule for landmark-based
// recognizers (RecognizerKindDoc). One pattern produces zero or one capability.
//
// Semantics:
//   - Required is the AND-anchor set. Every entry MUST find at least one landmark
//     in the input or the pattern is suppressed and its anchors are reported
//     under MissingAnchors. Required is the false-positive guardrail — it prevents
//     a passing mention of "skill" from triggering the capability.
//   - Matchers is the OR-trigger set. If ANY entry matches a landmark in the
//     input (and Required passed), the capability is emitted with confidence
//     "inferred" and the matched landmark recorded under MatchedAnchors.
//   - Scope is an optional dot-path extension under <content_type>.capabilities.
//     When empty, the capability is emitted at <content_type>.supported only.
//   - Mechanism is the human-readable explanation written to
//     <content_type>.capabilities.<scope>.mechanism.
type LandmarkPattern struct {
	// Capability is the canonical capability key segment (e.g., "frontmatter",
	// "project_scope"). When empty, the pattern only contributes to the
	// top-level <content_type>.supported = "true" emission.
	Capability string
	// Matchers must contain at least one entry. If any entry matches a landmark,
	// the capability is detected (provided Required also passes).
	Matchers []StringMatcher
	// Required is the optional anchor set — every entry must match some landmark
	// or the capability is suppressed. Empty Required means no anchor guard
	// (the pattern fires on Matchers alone).
	Required []StringMatcher
	// Mechanism populates the .mechanism dot-path. Required when Capability is set.
	Mechanism string
}

// LandmarkOptions configures recognizeLandmarks for a specific content type.
type LandmarkOptions struct {
	// ContentType is the top-level dot-path prefix: "skills", "rules", etc.
	ContentType string
	// Patterns is the ordered list of capability detection rules.
	Patterns []LandmarkPattern
}

// recognizeLandmarks evaluates a set of landmark patterns against the
// RecognitionContext's landmarks and returns a RecognitionResult.
//
// Confidence is fixed at "inferred" for every emitted capability — landmark
// recognition is structurally weaker than typed-source recognition. The
// fixed-confidence rule is enforced here (not by recognizer authors) so a
// landmark recognizer cannot accidentally claim "confirmed".
//
// Status decision:
//   - "recognized"      — at least one capability emitted AND no required anchors missing
//   - "anchors_missing" — at least one pattern's Required failed; surfaces anchor names
//   - "not_evaluated"   — no patterns fired and no anchors failed (no signal at all)
func recognizeLandmarks(ctx RecognitionContext, opts LandmarkOptions) RecognitionResult {
	caps := make(map[string]string)
	missingSet := make(map[string]struct{})
	matchedSet := make(map[string]struct{})

	for _, pat := range opts.Patterns {
		anchorOK, missing := evaluateRequired(ctx.Landmarks, pat.Required)
		if !anchorOK {
			for _, m := range missing {
				missingSet[m] = struct{}{}
			}
			continue
		}

		matched, ok := evaluateMatchers(ctx.Landmarks, pat.Matchers)
		if !ok {
			continue
		}
		matchedSet[matched] = struct{}{}

		caps[opts.ContentType+".supported"] = "true"
		if pat.Capability == "" {
			continue
		}
		prefix := opts.ContentType + ".capabilities." + pat.Capability
		caps[prefix+".supported"] = "true"
		caps[prefix+".mechanism"] = pat.Mechanism
		caps[prefix+".confidence"] = confidenceInferred
	}

	missing := sortedSetKeys(missingSet)
	matched := sortedSetKeys(matchedSet)

	status := StatusNotEvaluated
	switch {
	case len(missing) > 0:
		status = StatusAnchorsMissing
	case len(caps) > 0:
		status = StatusRecognized
	}

	return RecognitionResult{
		Capabilities:   caps,
		Status:         status,
		MissingAnchors: missing,
		MatchedAnchors: matched,
	}
}

// evaluateRequired checks every Required matcher against the landmark list.
// Returns (true, nil) when all required matchers find a landmark, or
// (false, names) listing the matcher Values that did not find a match.
func evaluateRequired(landmarks []string, required []StringMatcher) (bool, []string) {
	if len(required) == 0 {
		return true, nil
	}
	var missing []string
	for _, req := range required {
		found := false
		for _, lm := range landmarks {
			if req.matches(lm) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, req.Value)
		}
	}
	return len(missing) == 0, missing
}

// evaluateMatchers returns the first landmark matched by any of the matchers,
// or empty + false when no matcher matches.
func evaluateMatchers(landmarks []string, matchers []StringMatcher) (string, bool) {
	for _, m := range matchers {
		for _, lm := range landmarks {
			if m.matches(lm) {
				return lm, true
			}
		}
	}
	return "", false
}

func sortedSetKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// confidenceInferred is set automatically by recognizeLandmarks for
// landmark-based recognition where the capability is derived from
// documentation structure rather than a typed schema. Recognizers MUST NOT
// override this — the helper hardcodes it for every emitted capability.
const confidenceInferred = "inferred"
