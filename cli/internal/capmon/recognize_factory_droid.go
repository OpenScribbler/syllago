package capmon

func init() {
	RegisterRecognizer("factory-droid", recognizeFactoryDroid)
}

// recognizeFactoryDroid recognizes skills capabilities for the Factory Droid provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeFactoryDroid(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
