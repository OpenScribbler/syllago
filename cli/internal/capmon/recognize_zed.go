package capmon

func init() {
	RegisterRecognizer("zed", RecognizerKindUnknown, recognizeZed)
}

// recognizeZed recognizes skills capabilities for the Zed provider.
// Zed does not support Agent Skills (FormatDoc status: unsupported).
// Returning an empty result is the confirmed-negative signal.
func recognizeZed(ctx RecognitionContext) RecognitionResult {
	return wrapCapabilities(make(map[string]string))
}
