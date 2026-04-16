package capmon

func init() {
	RegisterRecognizer("cline", recognizeCline)
}

// recognizeCline recognizes skills capabilities for the Cline provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCline(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
