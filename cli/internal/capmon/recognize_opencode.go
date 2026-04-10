package capmon

func init() {
	RegisterRecognizer("opencode", recognizeOpencodeSkills)
}

// recognizeOpencodeSkills recognizes skills capabilities for the OpenCode provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeOpencodeSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
