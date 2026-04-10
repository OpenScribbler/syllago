package capmon

func init() {
	RegisterRecognizer("copilot-cli", recognizeCopilotCliSkills)
}

// recognizeCopilotCliSkills recognizes skills capabilities for the Copilot CLI provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCopilotCliSkills(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
