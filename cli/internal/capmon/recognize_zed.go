package capmon

func init() {
	RegisterRecognizer("zed", recognizeZed)
}

// recognizeZed recognizes skills capabilities for the Zed provider.
// Zed does not support Agent Skills (FormatDoc status: unsupported).
// Returning an empty map is the confirmed-negative signal.
func recognizeZed(fields map[string]FieldValue) map[string]string {
	return make(map[string]string)
}
