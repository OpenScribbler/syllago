package capmon

func init() {
	RegisterRecognizer("amp", RecognizerKindDoc, recognizeAmp)
}

// ampLandmarkOptions returns the landmark patterns for Amp's Agent Skills doc.
// Anchors derived from .capmon-cache/amp/skills.0/extracted.json. The two
// required anchors guard against passing mentions: "Agent Skills" alone could
// appear in unrelated docs, so we additionally require "Skill Format" to
// confirm the page actually documents the format.
func ampLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Agent Skills", CaseInsensitive: true},
		{Kind: "substring", Value: "Skill Format", CaseInsensitive: true},
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
			pattern("creation_workflow", "Creating Skills", "documented under 'Creating Skills' heading"),
			pattern("installation_workflow", "Installing Skills", "documented under 'Installing Skills' heading"),
			pattern("mcp_integration", "MCP Servers in Skills", "documented under 'MCP Servers in Skills' heading"),
			pattern("executable_tools", "Executable Tools in Skills", "documented under 'Executable Tools in Skills' heading"),
		},
	}
}

// recognizeAmp recognizes skills capabilities for the Amp provider.
// Amp's source is markdown documentation; recognition uses landmark (heading)
// matching. Static facts (project_scope, global_scope, canonical_filename)
// merge in at "confirmed" confidence after a successful landmark match.
func recognizeAmp(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, ampLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", "skill directory placed under .agents/skills/<name>/ or .claude/skills/<name>/ within the project root", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "global_scope", "skill directory placed under ~/.config/agents/skills/<name>/, ~/.config/amp/skills/<name>/, or ~/.claude/skills/<name>/", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return landmarkResult
}
