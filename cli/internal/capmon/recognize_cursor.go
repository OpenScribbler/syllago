package capmon

func init() {
	RegisterRecognizer("cursor", recognizeCursor)
}

// recognizeCursor recognizes skills capabilities for the Cursor provider.
// Cursor does not support Agent Skills (FormatDoc status: unsupported).
// Returning an empty map is the confirmed-negative signal.
func recognizeCursor(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
