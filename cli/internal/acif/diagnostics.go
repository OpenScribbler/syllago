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
	ErrHookPlatformMechanismMalformed = "acif.hook.platform_mechanism_malformed"
	ErrHookNoDefaultForDegradedRender = "acif.hook.no_default_for_degraded_render"

	DiagHookPlatformOverrideDropped     = "acif.hook.platform_override_dropped"
	DiagHookPlatformShellOSProxy        = "acif.hook.platform_shell_os_proxy"
	DiagHookPlatformFilenameInferred    = "acif.hook.platform_filename_inferred"
	DiagHookPlatformFilenameUninferable = "acif.hook.platform_filename_uninferable"
	DiagHookScriptNoPlatformMatch       = "acif.hook.script_no_platform_match"

	ErrSkillActivationTypeMissing = "acif.skill.activation_type_missing"
	ErrSkillActivationTypeInvalid = "acif.skill.activation_type_invalid"
	ErrSkillHookRefForbidden      = "acif.skill.hook_ref_forbidden"
	ErrSkillHookRefIDMissing      = "acif.skill.hook_ref_id_missing"

	ErrRuleActivationModeMissing    = "acif.rule.activation_mode_missing"
	ErrRuleActivationModeInvalid    = "acif.rule.activation_mode_invalid"
	ErrRuleGlobModeWithoutGlobs     = "acif.rule.glob_mode_without_globs"
	ErrRuleGlobsWithoutGlobMode     = "acif.rule.globs_without_glob_mode"
	ErrRuleActivationModeUnmappable = "acif.rule.activation_mode_unmappable"
	ErrRuleActivationDegraded       = "acif.rule.activation_degraded"

	DiagCommandPlaceholderNamedArgCollapsed = "acif.command.placeholder_named_arg_collapsed"
	DiagCommandPlaceholderUntranslated      = "acif.command.placeholder_untranslated"

	ErrMCPServersMissing               = "acif.mcp.servers_missing"
	ErrMCPTransportTypeInvalid         = "acif.mcp.transport_type_invalid"
	ErrMCPTransportDefaultAmbiguous    = "acif.mcp.transport_default_ambiguous"
	ErrMCPTransportDefaultUndetermined = "acif.mcp.transport_default_undetermined"
	DiagMCPServerNameUnconventional    = "acif.mcp.server_name_unconventional"
	DiagRegistryReferenceUnresolved    = "acif.registry.reference_unresolved"

	ErrSourceURIMissing                 = "acif.source_uri.missing"
	ErrSourceURIMalformed               = "acif.source_uri.malformed"
	ErrSourceURISchemeForbidden         = "acif.source_uri.scheme_forbidden"
	ErrSourceURIUserinfoPresent         = "acif.source_uri.userinfo_present"
	ErrSourceURIQueryPresent            = "acif.source_uri.query_present"
	ErrSourceURIRedirectDowngrade       = "acif.source_uri.redirect_downgrade"
	ErrSourceURIRedirectLimit           = "acif.source_uri.redirect_limit"
	ErrSourceURIDirectFileTrailingSlash = "acif.source_uri.direct_file_trailing_slash"
	ErrSourceURIFilenameConflict        = "acif.source_uri.filename_conflict"

	ErrRegistryExpiresBeforeFetchedAt = "acif.registry.expires_before_fetched_at"
	ErrRegistryStale                  = "acif.registry.stale"

	ReasonRequiresOrphanKey            = "acif.requires.orphan_key"
	ReasonCommandHandlerScriptsMissing = "command-handler-scripts-missing"

	ReasonEnvelopeForbiddenField     = "acif.envelope.forbidden_field"
	ReasonEnvelopeKindInvalid        = "acif.envelope.kind_invalid"
	ReasonEnvelopeIDInvalid          = "acif.envelope.id_invalid"
	ReasonEnvelopeVersionInvalid     = "acif.envelope.version_invalid"
	ReasonEnvelopeLicenseSPDXInvalid = "acif.envelope.license_spdx_invalid"

	ReasonRegistryTimestampOffsetMissing = "acif.registry.timestamp_offset_missing"
	ReasonRegistryMethodStampMissing     = "acif.registry.method_stamp_missing"
	ReasonRegistryProvenanceTagMissing   = "acif.registry.provenance_tag_missing"
)
