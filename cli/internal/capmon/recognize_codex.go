package capmon

func init() {
	RegisterRecognizer("codex", recognizeCodex)
}

// recognizeCodex recognizes skills capabilities for the Codex provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCodex(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
