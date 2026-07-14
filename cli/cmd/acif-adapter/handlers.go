package main

import (
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
			"scopes":           []string{"core"},
		})
	case "ingest":
		return handleIngest(req.Input)
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
			return errorResponse(reject.ID)
		}
		return errorResponse("adapter: " + err.Error())
	}
	return okResponse(map[string]any{
		"body_hash":      result.HashHex,
		"classification": result.Classification,
		"conformant":     true,
	})
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
