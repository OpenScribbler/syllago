package acif

// Diagnostic is the ACIF structured diagnostic (adapter protocol §3.1).
type Diagnostic struct {
	ID     string         `json:"id"`
	Params map[string]any `json:"params,omitempty"`
}

// RejectError carries a spec-minted acif.* error identifier.
type RejectError struct {
	ID          string
	Detail      string
	Diagnostics []Diagnostic
}

func (e *RejectError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return e.ID
	}
	return e.ID + ": " + e.Detail
}

const (
	ErrBodySymlink       = "acif.body.symlink"
	ErrBodyPathCollision = "acif.body.path_collision"
	ErrBodyEmpty         = "acif.body.empty"

	ErrHookEventUnrecognized          = "acif.hook.event_unrecognized"
	ErrHookHandlersMissing            = "acif.hook.handlers_missing"
	ErrHookHandlerTypeUnrecognized    = "acif.hook.handler_type_unrecognized"
	ErrHookScriptOSInvalid            = "acif.hook.script_os_invalid"
	ErrHookScriptOSEmpty              = "acif.hook.script_os_empty"
	ErrHookScriptArchEmpty            = "acif.hook.script_arch_empty"
	ErrHookScriptDefaultAmbiguous     = "acif.hook.script_default_ambiguous"
	ErrHookScriptPlatformAmbiguous    = "acif.hook.script_platform_ambiguous"
	ErrHookScriptFileMissing          = "acif.hook.script_file_missing"
	ErrHookScriptPathInvalid          = "acif.hook.script_path_invalid"
	ErrHookPlatformUnmappable         = "acif.hook.platform_unmappable"
	ErrHookNoDefaultForDegradedRender = "acif.hook.no_default_for_degraded_render"

	DiagHookPlatformOverrideDropped     = "acif.hook.platform_override_dropped"
	DiagHookPlatformShellOSProxy        = "acif.hook.platform_shell_os_proxy"
	DiagHookPlatformFilenameInferred    = "acif.hook.platform_filename_inferred"
	DiagHookPlatformFilenameUninferable = "acif.hook.platform_filename_uninferable"
	DiagHookScriptNoPlatformMatch       = "acif.hook.script_no_platform_match"

	ReasonRequiresOrphanKey            = "acif.requires.orphan_key"
	ReasonCommandHandlerScriptsMissing = "command-handler-scripts-missing"
)
