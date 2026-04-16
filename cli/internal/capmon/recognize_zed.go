package capmon

func init() {
	RegisterRecognizer("zed", recognizeZed)
}

// recognizeZed recognizes skills capabilities for the Zed provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeZed(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
