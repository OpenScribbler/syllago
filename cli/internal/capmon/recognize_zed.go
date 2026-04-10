package capmon

func init() {
	RegisterRecognizer("zed", recognizeZedSkills)
}

// recognizeZedSkills recognizes skills capabilities for the Zed provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeZedSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
