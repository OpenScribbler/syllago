package capmon

func init() {
	RegisterRecognizer("pi", recognizePiSkills)
}

// recognizePiSkills recognizes skills capabilities for the Pi provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizePiSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
