package capmon

func init() {
	RegisterRecognizer("claude-code", RecognizerKindDoc, recognizeClaudeCode)
}

// recognizeClaudeCode recognizes skills capabilities for the Claude Code provider.
// Claude Code source is documentation-only (markdown); current implementation uses
// the GoStruct preset and produces no output until PR4 introduces landmark-based
// recognition with claude-code as the canary. Tagged RecognizerKindDoc to reflect
// the intended strategy.
func recognizeClaudeCode(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".claude/skills/<skill-name>/SKILL.md committed to version control", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.claude/skills/<skill-name>/SKILL.md in user home directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
