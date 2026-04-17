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

// clineRulesLandmarkOptions returns the landmark patterns for Cline's rules
// documentation. Anchors derived from .capmon-cache/cline/rules.0/extracted.json
// (cline-rules.md).
//
// Required anchors are unique to the rules doc — skills.0 uses "Where Skills
// Live" (different word), so substring matching does not collide:
//   - "Where Rules Live"
//   - "Conditional Rules"
//
// Per the seeder spec, cline supports a smaller activation_mode vocabulary
// than cursor/kiro/windsurf — only always_on (no conditional) and
// frontmatter_globs (paths conditional). file_imports,
// cross_provider_recognition, and auto_memory are intentionally absent.
func clineRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Where Rules Live", CaseInsensitive: true},
		{Kind: "substring", Value: "Conditional Rules", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Conditional Rules",
			"rules without conditionals load for every request (documented under 'Conditional Rules' / 'How It Works')", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "The paths Conditional",
			"'paths' Conditional uses glob-based path matching to scope rule activation (documented under 'The paths Conditional' / 'Writing Conditional Rules')", required),
		RulesLandmarkPattern("hierarchical_loading", "Global Rules Directory",
			"two-tier scope: Project rules (.clinerules/ in workspace) + Global rules (~/.cline/rules/ user-wide) — documented under 'Where Rules Live' / 'Global Rules Directory'", required),
	)
}

// recognizeCline recognizes skills + rules capabilities for the Cline provider.
// Source for both content types is markdown; recognition uses landmark
// (heading) matching. Static facts merge in at "confirmed" confidence after a
// successful skills landmark match.
func recognizeCline(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, clineLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "Skills stored in .cline/skills/<name>/SKILL.md (recommended), .clinerules/skills/<name>/SKILL.md, or .claude/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "Skills stored in ~/.cline/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, clineRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
