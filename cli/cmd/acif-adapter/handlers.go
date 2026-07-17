package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/OpenScribbler/syllago/cli/internal/acif"
	"github.com/google/uuid"
)

const adapterVersion = "0.0.0-dev"

type request struct {
	Op             string          `json:"op"`
	Input          json.RawMessage `json:"input"`
	RunnerProtocol int             `json:"runner_protocol"`
}

type ingestInput struct {
	Kind           string          `json:"kind"`
	BodyRoot       string          `json:"body_root"`
	EntryFile      string          `json:"entry_file"`
	Sidecar        json.RawMessage `json:"sidecar"`
	ProviderConfig json.RawMessage `json:"provider_config"`
	Context        json.RawMessage `json:"context"`
}

type okEnvelope struct {
	OK     bool `json:"ok"`
	Result any  `json:"result"`
}

type errorEnvelope struct {
	OK          bool              `json:"ok"`
	Error       string            `json:"error"`
	Diagnostics []acif.Diagnostic `json:"diagnostics,omitempty"`
}

type unsupportedEnvelope struct {
	Unsupported bool `json:"unsupported"`
}

func dispatch(req request) any {
	switch req.Op {
	case "hello":
		return okResponse(map[string]any{
			"implementation":   "syllago",
			"version":          adapterVersion,
			"adapter_protocol": 2,
			"scopes":           []string{"core", "hook", "skill", "rule", "command", "agent", "mcp", "publisher", "registry", "render", "install"},
		})
	case "ingest":
		return handleIngest(req.Input)
	case "normalize_uri":
		return handleNormalizeURI(req.Input)
	case "fetch_uri":
		return handleFetchURI(req.Input)
	case "derive_url_name":
		return handleDeriveURLName(req.Input)
	case "evaluate_freshness":
		return handleEvaluateFreshness(req.Input)
	case "reconcile_frontmatter":
		return handleReconcileFrontmatter(req.Input)
	case "project":
		return handleProject(req.Input)
	case "render":
		return handleRender(req.Input)
	case "evaluate_install":
		return handleEvaluateInstall(req.Input)
	case "evaluate_requires":
		return handleEvaluateRequires(req.Input)
	case "derive_pack_id":
		return handleDerivePackID(req.Input)
	case "resolve_pack":
		return handleResolvePack(req.Input)
	case "resolve_reference":
		return handleResolveReference(req.Input)
	case "resolve_install_targets":
		return handleResolveInstallTargets(req.Input)
	default:
		return unsupportedResponse()
	}
}

func okResponse(result any) okEnvelope {
	return okEnvelope{OK: true, Result: result}
}

func errorResponse(message string) errorEnvelope {
	return errorEnvelope{OK: false, Error: message}
}

func rejectResponse(reject *acif.RejectError) errorEnvelope {
	return errorEnvelope{OK: false, Error: reject.ID, Diagnostics: reject.Diagnostics}
}

func unsupportedResponse() unsupportedEnvelope {
	return unsupportedEnvelope{Unsupported: true}
}

func handleIngest(raw json.RawMessage) any {
	var rawInput map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawInput); err != nil {
		return errorResponse("adapter: " + err.Error())
	}

	var input ingestInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}

	if input.Kind == "hook" {
		return handleHookIngest(rawInput, input)
	}

	if _, ok := rawInput["provider_config"]; ok {
		return handleProviderConfigIngest(input.Kind, input.ProviderConfig)
	}
	if input.Kind == "pack" {
		if rawManifests, ok := rawInput["manifests"]; ok {
			return handlePackManifests(rawManifests)
		}
		if rawSidecar, ok := rawInput["sidecar"]; ok {
			return handlePackSidecar(rawSidecar)
		}
	}

	if _, hasBodyRoot := rawInput["body_root"]; hasBodyRoot {
		if isFrontmatterKind(input.Kind) {
			return handleBodyIngest(input.Kind, input.BodyRoot, input.EntryFile)
		}
		return unsupportedResponse()
	}

	if _, hasSidecar := rawInput["sidecar"]; hasSidecar {
		if input.Kind == "mcp_config" {
			var block map[string]any
			if err := decodeJSONUseNumber(input.Sidecar, &block); err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			result, err := acif.CanonicalizeMCP(block)
			if err != nil {
				return hookErrorResponse(err)
			}
			if err := attachEnvelopePublisher(result, block); err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			return okResponse(recordResponse(result))
		}
		if isFrontmatterKind(input.Kind) {
			sidecar, ok := parseJSONObject(input.Sidecar)
			if !ok {
				return okResponse(map[string]any{
					"conformant": false,
					"reason":     "sidecar-not-object",
				})
			}
			var sidecarValue map[string]any
			if err := decodeJSONUseNumber(input.Sidecar, &sidecarValue); err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			if result, handled, err := acif.ValidateRegistryEmitSidecar(sidecarValue); handled {
				if err != nil {
					return hookErrorResponse(err)
				}
				return okResponse(result)
			}
			if _, ok := sidecar[input.Kind]; !ok {
				return handleSidecarIngest(input.Sidecar)
			}
			var block map[string]any
			if err := decodeJSONUseNumber(input.Sidecar, &block); err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			result, err := acif.IngestExtensionBlock(input.Kind, block)
			if err != nil {
				return hookErrorResponse(err)
			}
			return okResponse(recordResponse(result))
		}
		return handleSidecarIngest(input.Sidecar)
	}

	return unsupportedResponse()
}

type normalizeURIInput struct {
	URI string `json:"uri"`
}

func handleNormalizeURI(raw json.RawMessage) any {
	var input normalizeURIInput
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	normalized, err := acif.NormalizeSourceURI(input.URI)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(map[string]any{"source_uri": normalized})
}

type fetchURIInput struct {
	URL     string            `json:"url"`
	TrustCA string            `json:"trust_ca"`
	Resolve map[string]string `json:"resolve"`
}

func handleFetchURI(raw json.RawMessage) any {
	var input fetchURIInput
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	if input.Resolve == nil {
		input.Resolve = map[string]string{}
	}
	sourceURI, err := acif.FetchSourceURI(input.URL, input.TrustCA, input.Resolve)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(map[string]any{"source_uri": sourceURI})
}

type deriveURLNameInput struct {
	URI                string  `json:"uri"`
	BodyClassification string  `json:"body_classification"`
	FrontmatterName    *string `json:"frontmatter_name"`
}

func handleDeriveURLName(raw json.RawMessage) any {
	var input deriveURLNameInput
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.DeriveURLName(input.URI, input.BodyClassification, derefString(input.FrontmatterName))
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(result)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func handleEvaluateFreshness(raw json.RawMessage) any {
	var input acif.FreshnessInput
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.EvaluateFreshness(input)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(result)
}

func handleProviderConfigIngest(kind string, rawProviderConfig json.RawMessage) any {
	var config map[string]any
	if err := decodeJSONUseNumber(rawProviderConfig, &config); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	if provider, _ := config["provider"].(string); provider == "provider-native-frontmatter" && isFrontmatterKind(kind) {
		content, ok := config["content"].(map[string]any)
		if !ok {
			return errorResponse("adapter: provider-native-frontmatter content must be object")
		}
		frontmatter, ok := content["frontmatter"].(map[string]any)
		if !ok {
			return errorResponse("adapter: provider-native-frontmatter frontmatter must be object")
		}
		result, err := acif.IngestProviderNativeFrontmatter(kind, frontmatter)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(recordResponse(result))
	}
	switch kind {
	case "rule":
		result, err := acif.CanonicalizeRuleProviderConfig(config)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(map[string]any{
			"conformant": true,
			"canonical": map[string]any{
				"kind": "rule",
				"rule": result.Canonical,
			},
		})
	case "agent":
		result, err := acif.CanonicalizeAgentProviderConfig(config)
		if err != nil {
			return hookErrorResponse(err)
		}
		response := map[string]any{
			"conformant":  true,
			"installable": true,
			"canonical": map[string]any{
				"kind":  "agent",
				"agent": result.Canonical,
			},
		}
		if result.Verdict != nil {
			response["conformant"] = false
			response["installable"] = false
			response["reason"] = result.Verdict.Reason
			if len(result.Verdict.Params) > 0 {
				response["params"] = result.Verdict.Params
			}
		}
		return okResponse(response)
	case "mcp_config":
		result, err := acif.CanonicalizeMCPProviderConfig(config)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(recordResponse(result))
	default:
		return unsupportedResponse()
	}
}

func handleHookIngest(rawInput map[string]json.RawMessage, input ingestInput) any {
	opts := acif.HookOpts{BodyRoot: input.BodyRoot}
	var result *acif.HookResult
	var sidecarBlock map[string]any
	var err error
	if rawProviderConfig, ok := rawInput["provider_config"]; ok {
		var config map[string]any
		if err := decodeJSONUseNumber(rawProviderConfig, &config); err != nil {
			return errorResponse("adapter: " + err.Error())
		}
		result, err = acif.CanonicalizeProviderConfig(config, opts)
	} else if rawSidecar, ok := rawInput["sidecar"]; ok {
		var block map[string]any
		if err := decodeJSONUseNumber(rawSidecar, &block); err != nil {
			return errorResponse("adapter: " + err.Error())
		}
		sidecarBlock = block
		result, err = acif.CanonicalizeHook(block, opts)
	} else {
		return unsupportedResponse()
	}
	if err != nil {
		return hookErrorResponse(err)
	}
	if result.Verdict != nil {
		verdict := map[string]any{
			"conformant": false,
			"reason":     result.Verdict.Reason,
		}
		if len(result.Verdict.Params) > 0 {
			verdict["params"] = result.Verdict.Params
		}
		return okResponse(verdict)
	}

	rawCanonical, err := json.Marshal(result.Canonical)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	canonicalBytes, err := acif.CanonicalJSON(rawCanonical)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	bodyHash, computed, err := acif.HookBodyHash(result.Canonical, input.BodyRoot)
	if err != nil {
		return hookErrorResponse(err)
	}

	response := map[string]any{
		"conformant":      true,
		"installable":     true,
		"canonical":       result.Canonical,
		"canonical_bytes": string(canonicalBytes),
		"provenance":      result.Provenance,
	}
	if computed {
		response["body_hash"] = bodyHash
	}
	if sidecarBlock != nil {
		section := acif.EnvelopePublisherSection(sidecarBlock)
		if section != nil {
			rawSection, err := json.Marshal(section)
			if err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			hashHex, _, err := acif.MetadataHash(rawSection)
			if err != nil {
				return errorResponse("adapter: " + err.Error())
			}
			response["publisher_section"] = section
			response["metadata_hash"] = hashHex
		}
	}
	if len(result.Diagnostics) > 0 {
		response["diagnostics"] = result.Diagnostics
	}
	return okResponse(response)
}

func handlePackManifests(rawManifests json.RawMessage) any {
	var manifests []acif.PackManifest
	if err := json.Unmarshal(rawManifests, &manifests); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(acif.ReconcilePackManifests(manifests))
}

func handlePackSidecar(rawSidecar json.RawMessage) any {
	if _, ok := parseJSONObject(rawSidecar); !ok {
		return okResponse(map[string]any{
			"conformant": false,
			"reason":     "sidecar-not-object",
		})
	}
	var block map[string]any
	if err := decodeJSONUseNumber(rawSidecar, &block); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.IngestPackSidecar(block)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(recordResponse(result))
}

func isFrontmatterKind(kind string) bool {
	switch kind {
	case "skill", "rule", "agent", "command":
		return true
	default:
		return false
	}
}

func handleBodyIngest(kind, bodyRoot, entryFile string) any {
	result, err := acif.IngestFrontmatterFile(kind, bodyRoot, entryFile)
	if err != nil {
		var reject *acif.RejectError
		if errors.As(err, &reject) {
			return rejectResponse(reject)
		}
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(recordResponse(result))
}

func recordResponse(result *acif.RecordResult) map[string]any {
	response := map[string]any{
		"conformant":  result.Conformant,
		"installable": result.Installable,
	}
	if result.Reason != "" {
		response["reason"] = result.Reason
	}
	if len(result.Params) > 0 {
		response["params"] = result.Params
	}
	if result.Classification != "" {
		response["classification"] = result.Classification
	}
	if result.BodyHash != "" {
		response["body_hash"] = result.BodyHash
	}
	if result.Canonical != nil {
		response["canonical"] = result.Canonical
	}
	if result.CanonicalBytes != "" {
		response["canonical_bytes"] = result.CanonicalBytes
	}
	if result.PublisherSection != nil {
		response["publisher_section"] = result.PublisherSection
	}
	if result.MetadataHash != "" {
		response["metadata_hash"] = result.MetadataHash
	}
	if len(result.Diagnostics) > 0 {
		response["diagnostics"] = result.Diagnostics
	}
	return response
}

type reconcileFrontmatterInput struct {
	SidecarValue      map[string]any `json:"sidecar_value"`
	SourceFrontmatter map[string]any `json:"source_frontmatter"`
	Mode              string         `json:"mode"`
}

func handleReconcileFrontmatter(raw json.RawMessage) any {
	var input reconcileFrontmatterInput
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(acif.ReconcileFrontmatter(input.SidecarValue, input.SourceFrontmatter, input.Mode))
}

type projectInput struct {
	Projection string          `json:"projection"`
	Targets    []string        `json:"targets"`
	Item       json.RawMessage `json:"item"`
	Canonical  json.RawMessage `json:"canonical"`
	Hook       json.RawMessage `json:"hook"`
	Sidecar    json.RawMessage `json:"sidecar"`
}

func handleProject(raw json.RawMessage) any {
	var input projectInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	switch input.Projection {
	case "script_selection", "derived_capabilities", "os_coverage", "rule_activation", "advisory", "builtin_shadowing_advisory", "tuple_endpoint", "install_scope_capabilities":
	default:
		return unsupportedResponse()
	}
	if input.Projection == "tuple_endpoint" || input.Projection == "install_scope_capabilities" {
		item, err := decodeProjectItem(input.Item)
		if err != nil {
			return errorResponse("adapter: " + err.Error())
		}
		switch input.Projection {
		case "tuple_endpoint":
			return okResponse(map[string]any{"projection": acif.TupleEndpointProjection(item)})
		case "install_scope_capabilities":
			return okResponse(acif.ValidateInstallScopeCapabilities(item))
		}
	}
	if input.Projection == "advisory" {
		item, err := decodeProjectItem(input.Item)
		if err != nil {
			return errorResponse("adapter: " + err.Error())
		}
		if _, hasCommand := item["command"]; !hasCommand {
			if _, hasBody := item["body"]; !hasBody {
				return okResponse(acif.ValidateRegistryAdvisory(item))
			}
		}
	}
	block, err := decodeHookBlockInput(input.Item, input.Canonical, input.Hook, input.Sidecar)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	switch input.Projection {
	case "script_selection":
		selection, diagnostics, err := acif.ScriptSelection(block, input.Targets)
		if err != nil {
			return hookErrorResponse(err)
		}
		result := map[string]any{"selection": selection}
		if len(diagnostics) > 0 {
			result["diagnostics"] = diagnostics
		}
		return okResponse(result)
	case "derived_capabilities":
		caps, err := acif.DerivedCapabilitiesForItem(block)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(map[string]any{"derived_capabilities": caps})
	case "os_coverage":
		projection, err := acif.OSCoverage(block)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(map[string]any{"projection": projection})
	case "rule_activation":
		projection, err := acif.RuleActivationProjection(block)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(map[string]any{"projection": projection})
	case "advisory":
		projection, err := acif.CommandAdvisoryProjection(block)
		if err != nil {
			return hookErrorResponse(err)
		}
		return okResponse(map[string]any{"projection": projection})
	case "builtin_shadowing_advisory":
		return okResponse(map[string]any{})
	default:
		return unsupportedResponse()
	}
}

func decodeProjectItem(raw json.RawMessage) (map[string]any, error) {
	var item map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	if err := decodeJSONUseNumber(raw, &item); err != nil {
		return nil, err
	}
	return item, nil
}

type renderInput struct {
	Canonical  json.RawMessage `json:"canonical"`
	Item       json.RawMessage `json:"item"`
	Sidecar    json.RawMessage `json:"sidecar"`
	Target     string          `json:"target"`
	Invocation map[string]any  `json:"invocation"`
}

func handleRender(raw json.RawMessage) any {
	var input renderInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	block, err := decodeHookBlockInput(input.Canonical, input.Item, nil, input.Sidecar)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := renderItem(block, input.Target, input.Invocation)
	if err != nil {
		return hookErrorResponse(err)
	}
	if result.Unsupported {
		return unsupportedResponse()
	}
	response := map[string]any{"output": result.Output}
	if len(result.Diagnostics) > 0 {
		response["diagnostics"] = result.Diagnostics
	}
	if len(result.Lossy) > 0 {
		response["lossy"] = result.Lossy
	}
	return okResponse(response)
}

func renderItem(block map[string]any, target string, invocation map[string]any) (*acif.RenderResult, error) {
	renderers := []func(map[string]any, string) (*acif.RenderResult, error){
		acif.RenderStructured,
		acif.RenderCommand,
		acif.RenderRule,
		acif.RenderAgent,
		acif.RenderMCP,
	}
	for _, render := range renderers {
		result, err := render(block, target)
		if err != nil {
			return nil, err
		}
		if !result.Unsupported {
			return result, nil
		}
	}
	return acif.RenderHook(block, target, invocation)
}

type evaluateInstallInput struct {
	Item            json.RawMessage `json:"item"`
	InstallTargetOS string          `json:"install_target_os"`
}

func handleEvaluateInstall(raw json.RawMessage) any {
	var input evaluateInstallInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	item, err := decodeProjectItem(input.Item)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	if result, handled := acif.EvaluateRegistryInstallCrossReferences(item); handled {
		return okResponse(result)
	}
	if !isHookInstallItem(item) {
		return okResponse(map[string]any{"install": "proceed"})
	}
	result, err := acif.EvaluateInstall(item, input.InstallTargetOS)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(result)
}

func isHookInstallItem(item map[string]any) bool {
	if item == nil {
		return false
	}
	for _, key := range []string{"hook", "event", "handlers"} {
		if _, ok := item[key]; ok {
			return true
		}
	}
	return false
}

func handleEvaluateRequires(raw json.RawMessage) any {
	var rawInput map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawInput); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	itemRequires := make(map[string]any)
	if rawRequires, ok := rawInput["item_requires"]; ok {
		if err := decodeJSONUseNumber(rawRequires, &itemRequires); err != nil {
			return errorResponse("adapter: " + err.Error())
		}
	}
	var recognizes []string
	if rawRecognizes, ok := rawInput["consumer_recognizes"]; ok {
		if err := json.Unmarshal(rawRecognizes, &recognizes); err != nil {
			return errorResponse("adapter: " + err.Error())
		}
	}
	return okResponse(acif.EvaluateRequires(itemRequires, recognizes))
}

func handleResolveReference(raw json.RawMessage) any {
	var input map[string]any
	if err := decodeJSONUseNumber(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.ResolveReference(input)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(result)
}

type sidecarResult struct {
	PublisherSection json.RawMessage `json:"publisher_section"`
	CanonicalBytes   string          `json:"canonical_bytes"`
	MetadataHash     string          `json:"metadata_hash"`
	Conformant       bool            `json:"conformant"`
	Reason           string          `json:"reason,omitempty"`
	Params           map[string]any  `json:"params,omitempty"`
	Installable      bool            `json:"installable"`
}

func handleSidecarIngest(rawSidecar json.RawMessage) any {
	sidecar, ok := parseJSONObject(rawSidecar)
	if !ok {
		return okResponse(map[string]any{
			"conformant": false,
			"reason":     "sidecar-not-object",
		})
	}

	sectionRaw := rawSidecar
	section := sidecar
	allSections := []map[string]json.RawMessage{sidecar}
	if publisherRaw, ok := sidecar["publisher_section"]; ok {
		parsedPublisher, ok := parseJSONObject(publisherRaw)
		if !ok {
			return okResponse(map[string]any{
				"conformant": false,
				"reason":     "sidecar-not-object",
			})
		}
		sectionRaw = publisherRaw
		section = parsedPublisher
		allSections = []map[string]json.RawMessage{sidecar, parsedPublisher}
		if registryRaw, ok := sidecar["registry_section"]; ok {
			if parsedRegistry, ok := parseJSONObject(registryRaw); ok {
				allSections = append(allSections, parsedRegistry)
			}
		}
	}

	verdict := acif.ValidateEnvelope(section, allSections)
	hashHex, canonicalBytes, err := acif.MetadataHash(sectionRaw)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}

	return okResponse(sidecarResult{
		PublisherSection: sectionRaw,
		CanonicalBytes:   string(canonicalBytes),
		MetadataHash:     hashHex,
		Conformant:       verdict.Conformant,
		Reason:           verdict.Reason,
		Params:           verdict.Params,
		Installable:      verdict.Conformant,
	})
}

func attachEnvelopePublisher(result *acif.RecordResult, block map[string]any) error {
	section := acif.EnvelopePublisherSection(block)
	if section == nil {
		return nil
	}
	rawSection, err := json.Marshal(section)
	if err != nil {
		return err
	}
	hashHex, _, err := acif.MetadataHash(rawSection)
	if err != nil {
		return err
	}
	result.PublisherSection = section
	result.MetadataHash = hashHex
	return nil
}

func parseJSONObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func decodeHookBlockInput(candidates ...json.RawMessage) (map[string]any, error) {
	for _, raw := range candidates {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var block map[string]any
		if err := decodeJSONUseNumber(raw, &block); err != nil {
			return nil, err
		}
		return block, nil
	}
	return nil, errors.New("missing hook block")
}

func decodeJSONUseNumber(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v)
}

func hookErrorResponse(err error) any {
	var reject *acif.RejectError
	if errors.As(err, &reject) {
		return rejectResponse(reject)
	}
	return errorResponse("adapter: " + err.Error())
}

type derivePackIDInput struct {
	Namespace     string `json:"namespace"`
	RepositoryURL string `json:"repository_url"`
	DisplayName   string `json:"display_name"`
}

func handleDerivePackID(raw json.RawMessage) any {
	var input derivePackIDInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	namespace, err := uuid.Parse(input.Namespace)
	if err != nil {
		return errorResponse("adapter: bad namespace")
	}
	return okResponse(map[string]any{
		"inferred_pack_id": acif.DerivePackID(namespace, input.RepositoryURL, input.DisplayName).String(),
	})
}

type resolvePackInput struct {
	Item struct {
		PublisherSection struct {
			PackID string `json:"pack_id"`
		} `json:"publisher_section"`
		RegistrySection struct {
			InferredPackID string `json:"inferred_pack_id"`
		} `json:"registry_section"`
	} `json:"item"`
	KnownPacks []struct {
		ID string `json:"id"`
	} `json:"known_packs"`
}

type resolvePackResult struct {
	PackResolution string `json:"pack_resolution"`
	MemberOf       string `json:"member_of,omitempty"`
	Install        string `json:"install"`
}

func handleResolvePack(raw json.RawMessage) any {
	var input resolvePackInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	known := make([]string, 0, len(input.KnownPacks))
	for _, pack := range input.KnownPacks {
		known = append(known, pack.ID)
	}
	resolution := acif.ResolvePack(
		input.Item.PublisherSection.PackID,
		input.Item.RegistrySection.InferredPackID,
		known,
	)
	return okResponse(resolvePackResult{
		PackResolution: resolution.Resolution,
		MemberOf:       resolution.MemberOf,
		Install:        resolution.Install,
	})
}
