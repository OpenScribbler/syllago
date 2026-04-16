package capmon

func init() {
	RegisterRecognizer("gemini-cli", recognizeGeminiCli)
}

// recognizeGeminiCli recognizes skills capabilities for the Gemini CLI provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeGeminiCli(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
