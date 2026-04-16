package capmon

func init() {
	RegisterRecognizer("windsurf", recognizeWindsurf)
}

// recognizeWindsurf recognizes skills capabilities for the Windsurf provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeWindsurf(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
