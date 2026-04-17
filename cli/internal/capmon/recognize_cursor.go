package capmon

func init() {
	RegisterRecognizer("cursor", RecognizerKindUnknown, recognizeCursor)
}

// recognizeCursor recognizes skills capabilities for the Cursor provider.
// Cursor does not support Agent Skills (FormatDoc status: unsupported).
// Returning an empty result is the confirmed-negative signal.
func recognizeCursor(ctx RecognitionContext) RecognitionResult {
	return wrapCapabilities(make(map[string]string))
}
