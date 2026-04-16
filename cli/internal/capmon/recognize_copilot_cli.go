package capmon

func init() {
	RegisterRecognizer("copilot-cli", recognizeCopilotCli)
}

// recognizeCopilotCli recognizes skills capabilities for the Copilot CLI provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCopilotCli(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
