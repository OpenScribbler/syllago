package capmon

func init() {
	RegisterRecognizer("roo-code", recognizeRooCodeSkills)
}

// recognizeRooCodeSkills recognizes skills capabilities for the Roo Code provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeRooCodeSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
