package capmon

func init() {
	RegisterRecognizer("codex", recognizeCodexSkills)
}

// recognizeCodexSkills recognizes skills capabilities for the Codex provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCodexSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
