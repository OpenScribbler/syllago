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
			"adapter_protocol": 1,
			"scopes":           []string{"core", "hook"},
		})
	case "ingest":
		return handleIngest(req.Input)
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
		return unsupportedResponse()
	}
	if input.Kind == "pack" {
		if _, ok := rawInput["manifests"]; ok {
			return unsupportedResponse()
		}
	}

	if _, hasBodyRoot := rawInput["body_root"]; hasBodyRoot {
		if isFrontmatterKind(input.Kind) {
			return handleBodyIngest(input.BodyRoot, input.EntryFile)
		}
		return unsupportedResponse()
	}

	if _, hasSidecar := rawInput["sidecar"]; hasSidecar {
		return handleSidecarIngest(input.Sidecar)
	}

	return unsupportedResponse()
}

func handleHookIngest(rawInput map[string]json.RawMessage, input ingestInput) any {
	opts := acif.HookOpts{BodyRoot: input.BodyRoot}
	var result *acif.HookResult
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
		result, err = acif.CanonicalizeHook(block, opts)
	} else {
		return unsupportedResponse()
	}
	if err != nil {
		return hookErrorResponse(err)
	}
	if result.Verdict != nil {
		return okResponse(map[string]any{
			"conformant": false,
			"reason":     result.Verdict.Reason,
		})
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
	if len(result.Diagnostics) > 0 {
		response["diagnostics"] = result.Diagnostics
	}
	return okResponse(response)
}

func isFrontmatterKind(kind string) bool {
	switch kind {
	case "skill", "rule", "agent", "command":
		return true
	default:
		return false
	}
}

func handleBodyIngest(bodyRoot, entryFile string) any {
	result, err := acif.BodyHash(bodyRoot, entryFile)
	if err != nil {
		var reject *acif.RejectError
		if errors.As(err, &reject) {
			return rejectResponse(reject)
		}
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(map[string]any{
		"body_hash":      result.HashHex,
		"classification": result.Classification,
		"conformant":     true,
	})
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
	case "script_selection", "derived_capabilities", "os_coverage":
	default:
		return unsupportedResponse()
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
		caps, err := acif.DerivedCapabilities(block)
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
	default:
		return unsupportedResponse()
	}
}

type renderInput struct {
	Canonical  json.RawMessage `json:"canonical"`
	Target     string          `json:"target"`
	Invocation map[string]any  `json:"invocation"`
}

func handleRender(raw json.RawMessage) any {
	var input renderInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	block, err := decodeHookBlockInput(nil, input.Canonical, nil, nil)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.RenderHook(block, input.Target, input.Invocation)
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
	return okResponse(response)
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
	block, err := decodeHookBlockInput(input.Item, nil, nil, nil)
	if err != nil {
		return errorResponse("adapter: " + err.Error())
	}
	result, err := acif.EvaluateInstall(block, input.InstallTargetOS)
	if err != nil {
		return hookErrorResponse(err)
	}
	return okResponse(result)
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

type sidecarResult struct {
	PublisherSection json.RawMessage `json:"publisher_section"`
	CanonicalBytes   string          `json:"canonical_bytes"`
	MetadataHash     string          `json:"metadata_hash"`
	Conformant       bool            `json:"conformant"`
	Reason           string          `json:"reason,omitempty"`
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
		Installable:      verdict.Conformant,
	})
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
