package capmon

func init() {
	RegisterRecognizer("cursor", recognizeCursor)
}

// recognizeCursor recognizes skills capabilities for the Cursor provider.
// TODO(Phase 6): implement real recognition after seeder spec is approved.
func recognizeCursor(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
