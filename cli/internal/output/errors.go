package output

import "strings"

// DocsBaseURL is the base URL for syllago error documentation.
const DocsBaseURL = "https://openscribbler.github.io/syllago-docs"

// Error code constants. Each code is namespaced by category and numbered.
// Format: CATEGORY_NNN
const (
	ErrCatalogNotFound      = "CATALOG_001"
	ErrCatalogScanFailed    = "CATALOG_002"
	ErrRegistryClone        = "REGISTRY_001"
	ErrRegistryNotFound     = "REGISTRY_002"
	ErrRegistryNotAllowed   = "REGISTRY_003"
	ErrProviderNotFound     = "PROVIDER_001"
	ErrProviderNotDetected  = "PROVIDER_002"
	ErrInstallNotWritable   = "INSTALL_001"
	ErrInstallItemNotFound  = "INSTALL_002"
	ErrInstallMethodInvalid = "INSTALL_003"
	ErrConvertNotSupported  = "CONVERT_001"
	ErrConvertParseFailed   = "CONVERT_002"
	ErrImportCloneFailed    = "IMPORT_001"
	ErrImportConflict       = "IMPORT_002"
	ErrExportNotSupported   = "EXPORT_001"
	ErrConfigInvalid        = "CONFIG_001"
	ErrConfigNotFound       = "CONFIG_002"
	ErrPrivacyGateBlocked   = "PRIVACY_001" // private content -> public target (hard block)
	ErrPrivacyGateWarn      = "PRIVACY_002" // private content in loadout (warning)
)

// docsURL converts an error code like "CATALOG_001" to its documentation URL.
// "CATALOG_001" -> "https://openscribbler.github.io/syllago-docs/errors/catalog-001/"
func docsURL(code string) string {
	slug := strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	return DocsBaseURL + "/errors/" + slug + "/"
}

// NewStructuredError creates a StructuredError with Code, Message, Suggestion,
// and an auto-generated DocsURL derived from the code.
func NewStructuredError(code, message, suggestion string) StructuredError {
	return StructuredError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
		DocsURL:    docsURL(code),
	}
}

// NewStructuredErrorDetail creates a StructuredError with all fields including Details.
func NewStructuredErrorDetail(code, message, suggestion, details string) StructuredError {
	return StructuredError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
		DocsURL:    docsURL(code),
		Details:    details,
	}
}

// allErrorCodes returns all registered error code values for validation.
// Used by tests to assert uniqueness and format correctness.
func allErrorCodes() []string {
	return []string{
		ErrCatalogNotFound,
		ErrCatalogScanFailed,
		ErrRegistryClone,
		ErrRegistryNotFound,
		ErrRegistryNotAllowed,
		ErrProviderNotFound,
		ErrProviderNotDetected,
		ErrInstallNotWritable,
		ErrInstallItemNotFound,
		ErrInstallMethodInvalid,
		ErrConvertNotSupported,
		ErrConvertParseFailed,
		ErrImportCloneFailed,
		ErrImportConflict,
		ErrExportNotSupported,
		ErrConfigInvalid,
		ErrConfigNotFound,
		ErrPrivacyGateBlocked,
		ErrPrivacyGateWarn,
	}
}

// errorCodeValue returns the string value of an error code constant.
// Since constants are already string values, this is an identity function —
// it exists to give the test helper a uniform API over the allErrorCodes slice.
func errorCodeValue(code string) string {
	return code
}
