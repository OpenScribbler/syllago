package capmon

func init() {
	RegisterRecognizer("cline", RecognizerKindDoc, recognizeCline)
}

// clineLandmarkOptions returns the landmark patterns for Cline's skills doc.
// Anchors derived from .capmon-cache/cline/skills.0/extracted.json. The two
// required anchors guard against false positives from other cline content
// docs (rules, hooks, mcp, commands) — those docs do not contain these
// skills-specific phrases, so the recognizer suppresses cleanly when only
// non-skills sources are present.
//
// Capability names are intentionally distinct from amp/claude-code where the
// underlying feature differs. Cline's skills doc emphasizes the bundled-files
// concept (docs/, templates/, scripts/) and a per-skill enable/disable toggle
// — both are surfaced as named capabilities here.
func clineLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "How Skills Work", CaseInsensitive: true},
		{Kind: "substring", Value: "Where Skills Live", CaseInsensitive: true},
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
			pattern("directory_structure", "Skill Structure", "documented under 'Skill Structure' heading"),
			pattern("creation_workflow", "Creating a Skill", "documented under 'Creating a Skill' heading"),
			pattern("toggling", "Toggling Skills", "per-skill enable/disable documented under 'Toggling Skills' heading"),
			pattern("frontmatter", "Writing Your SKILL.md", "frontmatter format documented under 'Writing Your SKILL.md' heading"),
			pattern("naming_conventions", "Naming Conventions", "documented under 'Naming Conventions' heading"),
			pattern("description_guidance", "Writing Effective Descriptions", "documented under 'Writing Effective Descriptions' heading"),
			pattern("bundled_files", "Bundling Supporting Files", "documented under 'Bundling Supporting Files' heading"),
			pattern("file_references", "Referencing Bundled Files", "documented under 'Referencing Bundled Files' heading"),
		},
	}
}

// recognizeCline recognizes skills capabilities for the Cline provider.
// Cline's skills doc is markdown; recognition uses landmark (heading) matching.
// Static facts (project_scope, global_scope, canonical_filename) merge in at
// "confirmed" confidence after a successful landmark match.
func recognizeCline(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, clineLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", "Skills stored in .cline/skills/<name>/SKILL.md (recommended), .clinerules/skills/<name>/SKILL.md, or .claude/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "global_scope", "Skills stored in ~/.cline/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return landmarkResult
}
