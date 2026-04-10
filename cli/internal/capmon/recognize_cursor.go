package capmon

func init() {
	RegisterRecognizer("cursor", recognizeCursorSkills)
}

// recognizeCursorSkills recognizes skills capabilities for the Cursor provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCursorSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
