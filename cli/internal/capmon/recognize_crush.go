package capmon

func init() {
	RegisterRecognizer("crush", recognizeCrushSkills)
}

// recognizeCrushSkills recognizes skills capabilities for the Crush provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCrushSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
