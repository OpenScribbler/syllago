package capmon

func init() {
	RegisterRecognizer("kiro", RecognizerKindDoc, recognizeKiro)
}

// kiroLandmarkOptions returns the landmark patterns for Kiro's "Powers" doc.
// Kiro brands skills as "Powers" (file: POWER.md); the canonical mapping
// still populates skills.* dot-paths for cross-provider portability.
//
// Anchors derived from .capmon-cache/kiro/skills.0/extracted.json. Required
// anchors "Create powers" and "Creating POWER.md" guard against false
// positives from other kiro content-type docs (agents, hooks, mcp, rules) —
// none of those mention powers/POWER.md in their landmarks.
func kiroLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Create powers", CaseInsensitive: true},
		{Kind: "substring", Value: "Creating POWER.md", CaseInsensitive: true},
	}
	pattern := func(cap, anchor, mechanism string) LandmarkPattern {
		return LandmarkPattern{
			Capability: cap,
			Required:   required,
			Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
			Mechanism:  mechanism,
		}
	}
	return LandmarkOptions{
		ContentType: "skills",
		Patterns: []LandmarkPattern{
			pattern("frontmatter", "Frontmatter: When to activate", "documented under 'Frontmatter: When to activate' heading"),
			pattern("onboarding_instructions", "Onboarding instructions", "documented under 'Onboarding instructions' heading"),
			pattern("steering_instructions", "Steering instructions", "documented under 'Steering instructions' heading"),
			pattern("mcp_integration", "Adding MCP servers", "documented under 'Adding MCP servers' heading"),
			pattern("directory_structure", "Directory structure", "documented under 'Directory structure' heading"),
			pattern("testing", "Testing locally", "documented under 'Testing locally' heading"),
			pattern("sharing", "Sharing your power", "documented under 'Sharing your power' heading"),
		},
	}
}

// recognizeKiro recognizes skills capabilities for the Kiro provider.
// Source is markdown documentation; recognition uses landmark (heading)
// matching. Static facts (project_scope, canonical_filename) merge in at
// "confirmed" confidence after a successful landmark match. Note: Kiro has no
// global_scope — Powers are installed via the Kiro Powers panel UI without a
// fixed user-wide filesystem path.
func recognizeKiro(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, kiroLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", "Self-contained directory installed via Kiro Powers panel (UI installation, no fixed filesystem path)", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "POWER.md (fixed, all caps)", "confirmed"))
	return landmarkResult
}
