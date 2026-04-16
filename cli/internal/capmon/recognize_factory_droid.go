package capmon

func init() {
	RegisterRecognizer("factory-droid", recognizeFactoryDroid)
}

// recognizeFactoryDroid recognizes skills capabilities for the Factory Droid provider.
// Factory Droid implements the Agent Skills open standard (GoStruct pattern).
func recognizeFactoryDroid(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "<repo>/.factory/skills/<skill-name>/SKILL.md or .agent/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.factory/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (skill.mdx also accepted)", "confirmed"))
	return result
}
