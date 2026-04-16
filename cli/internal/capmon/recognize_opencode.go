package capmon

func init() {
	RegisterRecognizer("opencode", recognizeOpencode)
}

// recognizeOpencode recognizes skills capabilities for the OpenCode provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeOpencode(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
