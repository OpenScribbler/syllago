package capmon

func init() {
	RegisterRecognizer("claude-code", recognizeClaudeCode)
}

// recognizeClaudeCode recognizes skills capabilities for the Claude Code provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeClaudeCode(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
