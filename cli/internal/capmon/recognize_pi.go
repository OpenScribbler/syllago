package capmon

func init() {
	RegisterRecognizer("pi", recognizePi)
}

// recognizePi recognizes skills capabilities for the Pi provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizePi(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
