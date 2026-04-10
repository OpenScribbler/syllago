package capmon

func init() {
	RegisterRecognizer("kiro", recognizeKiroSkills)
}

// recognizeKiroSkills recognizes skills capabilities for the Kiro provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeKiroSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
