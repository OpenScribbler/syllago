package acif

import (
	"encoding/json"
	"regexp"

	"github.com/google/uuid"
)

type EnvelopeVerdict struct {
	Conformant bool
	Reason     string
}

var (
	semverRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	spdxIDRe = regexp.MustCompile(`^[A-Za-z0-9.+-]+$`)
)

// ValidateEnvelope validates a publisher_section / flat sidecar declaration
// (a parsed JSON object) plus the enclosing item record's sections.
func ValidateEnvelope(section map[string]json.RawMessage, allSections []map[string]json.RawMessage) EnvelopeVerdict {
	forbidden := []string{"effective_version", "derived_version", "pack_inherited_version", "resolved_version"}
	if len(allSections) == 0 {
		allSections = []map[string]json.RawMessage{section}
	}
	for _, name := range forbidden {
		for _, s := range allSections {
			if _, ok := s[name]; ok {
				return EnvelopeVerdict{Reason: "forbidden-field " + name}
			}
		}
	}

	kind, ok := stringField(section, "kind")
	if !ok {
		return EnvelopeVerdict{Reason: "missing-required-field kind"}
	}
	if !validKind(kind) {
		return EnvelopeVerdict{Reason: "kind-not-in-closed-enum"}
	}

	id, ok := stringField(section, "id")
	if !ok {
		return EnvelopeVerdict{Reason: "missing-required-field id"}
	}
	parsedID, err := uuid.Parse(id)
	if err != nil || parsedID.Version() != 4 || parsedID.Variant() != uuid.RFC4122 {
		return EnvelopeVerdict{Reason: "id-not-uuid-v4"}
	}

	displayName, ok := stringField(section, "display_name")
	if !ok || displayName == "" {
		return EnvelopeVerdict{Reason: "missing-required-field display_name"}
	}

	if rawVersion, ok := section["version"]; ok {
		var version string
		if err := json.Unmarshal(rawVersion, &version); err != nil || !semverRe.MatchString(version) {
			return EnvelopeVerdict{Reason: "version-not-semver"}
		}
	}

	if rawLicense, ok := section["license"]; ok {
		var license map[string]json.RawMessage
		if err := json.Unmarshal(rawLicense, &license); err != nil {
			return EnvelopeVerdict{Reason: "license-spdx-not-identifier"}
		}
		spdx, ok := stringField(license, "spdx")
		if !ok || !spdxIDRe.MatchString(spdx) {
			return EnvelopeVerdict{Reason: "license-spdx-not-identifier"}
		}
	}

	return EnvelopeVerdict{Conformant: true}
}

func stringField(section map[string]json.RawMessage, name string) (string, bool) {
	raw, ok := section[name]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func validKind(kind string) bool {
	switch kind {
	case "hook", "skill", "rule", "command", "agent", "mcp_config", "pack":
		return true
	default:
		return false
	}
}
