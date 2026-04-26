package output

import "strings"

// DocsBaseURL is the base URL for syllago error documentation.
const DocsBaseURL = "https://syllago.dev"

// Error code constants. Each code is namespaced by category and numbered.
// Format: CATEGORY_NNN
//
// Every constant MUST have a corresponding docs/errors/<slug>.md file.
// The test TestErrorCodeDocsCoverage enforces this.
const (
	// Catalog: library/repo discovery and scanning.
	ErrCatalogNotFound   = "CATALOG_001" // no syllago repo or library found
	ErrCatalogScanFailed = "CATALOG_002" // catalog scan returned an error

	// Registry: remote content registry operations.
	ErrRegistryClone      = "REGISTRY_001" // git clone of registry failed
	ErrRegistryNotFound   = "REGISTRY_002" // named registry not in config
	ErrRegistryNotAllowed = "REGISTRY_003" // URL blocked by allowedRegistries policy
	ErrRegistryInvalid    = "REGISTRY_004" // invalid registry name or structure
	ErrRegistryDuplicate  = "REGISTRY_005" // registry with this name already exists
	ErrRegistryNotCloned  = "REGISTRY_006" // registry exists in config but not cloned locally
	ErrRegistrySyncFailed = "REGISTRY_007" // git pull failed during sync
	ErrRegistrySaveFailed = "REGISTRY_008" // could not save registry config changes

	// Provider: AI coding tool provider detection and lookup.
	ErrProviderNotFound    = "PROVIDER_001" // unknown provider slug
	ErrProviderNotDetected = "PROVIDER_002" // provider not detected on disk

	// Install: installing/uninstalling content to providers.
	ErrInstallNotWritable   = "INSTALL_001" // install path not writable
	ErrInstallItemNotFound  = "INSTALL_002" // item not found in library for install
	ErrInstallMethodInvalid = "INSTALL_003" // invalid install method for content type
	ErrInstallConflict      = "INSTALL_004" // active loadout or other conflict prevents install
	ErrInstallNotInstalled  = "INSTALL_005" // item is not currently installed

	// Convert: content format conversion between providers.
	ErrConvertNotSupported = "CONVERT_001" // content type does not support conversion
	ErrConvertParseFailed  = "CONVERT_002" // parsing content for conversion failed
	ErrConvertRenderFailed = "CONVERT_003" // rendering to target format failed

	// Import: importing content from external sources.
	ErrImportCloneFailed = "IMPORT_001" // cloning import source failed
	ErrImportConflict    = "IMPORT_002" // import conflicts with existing content

	// Export: exporting content to provider directories.
	ErrExportNotSupported = "EXPORT_001" // export not supported for content type
	ErrExportFailed       = "EXPORT_002" // export operation failed

	// Config: configuration loading, saving, and validation.
	ErrConfigInvalid  = "CONFIG_001" // configuration file is malformed
	ErrConfigNotFound = "CONFIG_002" // configuration file does not exist
	ErrConfigPath     = "CONFIG_003" // invalid path override (not absolute, unknown type)
	ErrConfigSave     = "CONFIG_004" // failed to save configuration

	// Loadout: loadout creation, application, and removal.
	ErrLoadoutNotFound = "LOADOUT_001" // loadout not found in library or registry
	ErrLoadoutParse    = "LOADOUT_002" // loadout.yaml is malformed
	ErrLoadoutConflict = "LOADOUT_003" // another loadout is already active
	ErrLoadoutProvider = "LOADOUT_004" // loadout references unknown provider
	ErrLoadoutNoItems  = "LOADOUT_005" // no items selected for loadout

	// Privacy: registry privacy gates blocking content flow.
	ErrPrivacyPublishBlocked = "PRIVACY_001" // private content cannot be published to public registry
	ErrPrivacyShareBlocked   = "PRIVACY_002" // private content cannot be shared to public repo
	ErrPrivacyLoadoutWarn    = "PRIVACY_003" // loadout contains private items (warning)

	// Promote: sharing/publishing content via git operations.
	ErrPromoteDirtyTree  = "PROMOTE_001" // uncommitted changes in working tree
	ErrPromoteValidation = "PROMOTE_002" // content validation failed before promote
	ErrPromoteGitFailed  = "PROMOTE_003" // git operation (branch, commit, push) failed

	// Item: content item lookup and resolution.
	ErrItemNotFound    = "ITEM_001" // named item not found in library
	ErrItemAmbiguous   = "ITEM_002" // item name exists in multiple types
	ErrItemTypeUnknown = "ITEM_003" // unknown content type specified

	// Input: CLI flag and argument validation.
	ErrInputMissing  = "INPUT_001" // required flag or argument not provided
	ErrInputConflict = "INPUT_002" // mutually exclusive flags used together
	ErrInputInvalid  = "INPUT_003" // flag value is invalid (wrong format, out of range)
	ErrInputTerminal = "INPUT_004" // command requires interactive terminal

	// Init: project initialization.
	ErrInitExists = "INIT_001" // project already initialized

	// System: environment and filesystem issues.
	ErrSystemHomedir = "SYSTEM_001" // cannot determine home directory
	ErrSystemIO      = "SYSTEM_002" // filesystem read/write/mkdir failure

	// MOAT: manifest-based signing / verification.
	ErrMoatIdentityUnpinned    = "MOAT_001" // registry add has neither allowlist match nor --signing-identity
	ErrMoatIdentityInvalid     = "MOAT_002" // --signing-* flags are incomplete or malformed
	ErrMoatIdentityMismatch    = "MOAT_003" // manifest cert does not match pinned profile (numeric ID, issuer, or subject)
	ErrMoatInvalid             = "MOAT_004" // manifest or bundle malformed, missing, or unreadable
	ErrMoatTrustedRootStale    = "MOAT_005" // bundled trusted root exceeded its 365-day cliff
	ErrMoatUnsignedWithPin     = "MOAT_006" // registry has a pinned profile but no manifest/bundle found in checkout
	ErrMoatTrustedRootOverride = "MOAT_007" // operator-supplied trusted_root.json path (--trusted-root or reg.trusted_root) unusable
	ErrMoatRevocationBlock     = "MOAT_008" // archival or live registry-source revocation refuses install (ADR 0007 G-8/G-15)
	ErrMoatTierBelowPolicy     = "MOAT_009" // resolved content trust tier is below the caller-configured policy floor
)

// docsURL converts an error code like "CATALOG_001" to its documentation URL.
// "CATALOG_001" -> "https://syllago.dev/errors/catalog-001/"
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

// AllErrorCodes returns all registered error code values for validation.
// Used by tests to assert uniqueness, format correctness, and doc parity.
func AllErrorCodes() []string {
	return []string{
		// Catalog
		ErrCatalogNotFound,
		ErrCatalogScanFailed,
		// Registry
		ErrRegistryClone,
		ErrRegistryNotFound,
		ErrRegistryNotAllowed,
		ErrRegistryInvalid,
		ErrRegistryDuplicate,
		ErrRegistryNotCloned,
		ErrRegistrySyncFailed,
		ErrRegistrySaveFailed,
		// Provider
		ErrProviderNotFound,
		ErrProviderNotDetected,
		// Install
		ErrInstallNotWritable,
		ErrInstallItemNotFound,
		ErrInstallMethodInvalid,
		ErrInstallConflict,
		ErrInstallNotInstalled,
		// Convert
		ErrConvertNotSupported,
		ErrConvertParseFailed,
		ErrConvertRenderFailed,
		// Import
		ErrImportCloneFailed,
		ErrImportConflict,
		// Export
		ErrExportNotSupported,
		ErrExportFailed,
		// Config
		ErrConfigInvalid,
		ErrConfigNotFound,
		ErrConfigPath,
		ErrConfigSave,
		// Loadout
		ErrLoadoutNotFound,
		ErrLoadoutParse,
		ErrLoadoutConflict,
		ErrLoadoutProvider,
		ErrLoadoutNoItems,
		// Privacy
		ErrPrivacyPublishBlocked,
		ErrPrivacyShareBlocked,
		ErrPrivacyLoadoutWarn,
		// Promote
		ErrPromoteDirtyTree,
		ErrPromoteValidation,
		ErrPromoteGitFailed,
		// Item
		ErrItemNotFound,
		ErrItemAmbiguous,
		ErrItemTypeUnknown,
		// Input
		ErrInputMissing,
		ErrInputConflict,
		ErrInputInvalid,
		ErrInputTerminal,
		// Init
		ErrInitExists,
		// System
		ErrSystemHomedir,
		ErrSystemIO,
		// MOAT
		ErrMoatIdentityUnpinned,
		ErrMoatIdentityInvalid,
		ErrMoatIdentityMismatch,
		ErrMoatInvalid,
		ErrMoatTrustedRootStale,
		ErrMoatUnsignedWithPin,
		ErrMoatTrustedRootOverride,
		ErrMoatRevocationBlock,
		ErrMoatTierBelowPolicy,
	}
}

// errorCodeValue returns the string value of an error code constant.
// Since constants are already string values, this is an identity function —
// it exists to give the test helper a uniform API over the allErrorCodes slice.
func errorCodeValue(code string) string {
	return code
}
