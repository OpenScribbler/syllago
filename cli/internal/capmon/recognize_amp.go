package capmon

func init() {
	RegisterRecognizer("amp", recognizeAmp)
}

// recognizeAmp recognizes skills capabilities for the Amp provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeAmp(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
