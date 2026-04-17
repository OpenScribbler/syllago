package capmon

func init() {
	RegisterRecognizer("factory-droid", RecognizerKindDoc, recognizeFactoryDroid)
}

// factoryDroidLandmarkOptions returns the landmark patterns for Factory Droid's
// skills doc.
//
// SPIKE NOTE (2026-04-16): Factory Droid's skills page
// (https://docs.factory.ai/cli/configuration/skills) is a Mintlify-rendered
// Next.js app. The current `markdown` extractor sees only the rendered HTML
// shell and produces no landmarks. As a result, this recognizer will emit
// status="anchors_missing" against the live cache until the upstream extractor
// is upgraded to handle JS-rendered docs (or the source manifest is switched
// to format=html with a stronger HTML extractor).
//
// The patterns below describe what we EXPECT the doc to surface once
// extraction works — they are exercised against synthetic landmarks in tests.
// When extraction is fixed, the recognizer will start emitting capabilities
// without further code changes.
func factoryDroidLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Skills", CaseInsensitive: true},
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
			pattern("frontmatter", "Frontmatter", "frontmatter format documented under 'Frontmatter' heading"),
			pattern("creation_workflow", "Creating a Skill", "documented under 'Creating a Skill' heading"),
			pattern("directory_structure", "Skill Locations", "documented under 'Skill Locations' heading"),
			pattern("invocation", "Invoking Skills", "documented under 'Invoking Skills' heading"),
			// Bare anchor-only pattern guarantees skills.supported when only the
			// page title is present (no specific-capability anchor).
			{
				Required: required,
				Matchers: required,
			},
		},
	}
}

// recognizeFactoryDroid recognizes skills capabilities for the Factory Droid
// provider. Source is markdown documentation; recognition uses landmark
// matching. See factoryDroidLandmarkOptions for the upstream extraction caveat.
func recognizeFactoryDroid(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, factoryDroidLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", "<repo>/.factory/skills/<skill-name>/SKILL.md or .agent/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "global_scope", "~/.factory/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (skill.mdx also accepted)", "confirmed"))
	return landmarkResult
}
