package capmon

func init() {
	RegisterRecognizer("kiro", recognizeKiro)
}

// recognizeKiro recognizes skills capabilities for the Kiro provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeKiro(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
