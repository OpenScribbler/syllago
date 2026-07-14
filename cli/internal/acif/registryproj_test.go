package acif

import (
	"reflect"
	"testing"
)

func TestTupleEndpointProjection(t *testing.T) {
	t.Parallel()

	got := TupleEndpointProjection(map[string]any{
		"pack_members": []any{
			map[string]any{"item_id": "one", "publisher_section": "present"},
			map[string]any{"item_id": "two", "publisher_section": "absent"},
		},
	})
	want := map[string]any{
		"tuple_fields": []any{"item_id", "body_hash", "metadata_hash_if_present", "version_if_declared"},
		"member_1":     map[string]any{"metadata_hash": "present"},
		"member_2":     map[string]any{"metadata_hash": nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TupleEndpointProjection() = %#v, want %#v", got, want)
	}
}

func TestInstallScopeCapabilitiesProjection(t *testing.T) {
	t.Parallel()

	missing := ValidateInstallScopeCapabilities(map[string]any{
		"filesystem": map[string]any{"read": true},
	})
	if missing["conformant"] != false || missing["reason"] != ReasonRegistryProvenanceTagMissing {
		t.Fatalf("missing source verdict = %#v", missing)
	}

	tagged := ValidateInstallScopeCapabilities(map[string]any{
		"filesystem": map[string]any{"source": "publisher", "read": true},
		"network":    map[string]any{"source": "registry", "egress": true},
	})
	if !reflect.DeepEqual(tagged, map[string]any{"conformant": true}) {
		t.Fatalf("tagged verdict = %#v", tagged)
	}
}

func TestRegistryAdvisoryValidation(t *testing.T) {
	t.Parallel()

	missing := ValidateRegistryAdvisory(map[string]any{
		"warning": map[string]any{"text": "needs review"},
	})
	if missing["conformant"] != false || missing["reason"] != ReasonRegistryMethodStampMissing {
		t.Fatalf("missing method verdict = %#v", missing)
	}

	stamped := ValidateRegistryAdvisory(map[string]any{
		"warning": map[string]any{"method": "registry-review-v1", "text": "needs review"},
	})
	if !reflect.DeepEqual(stamped, map[string]any{"conformant": true}) {
		t.Fatalf("stamped verdict = %#v", stamped)
	}
}

func TestEvaluateRegistryInstallCrossReferences(t *testing.T) {
	t.Parallel()

	got, handled := EvaluateRegistryInstallCrossReferences(map[string]any{
		"cross_references": []any{
			map[string]any{"resolution": "resolved"},
			map[string]any{"resolution": "revoked"},
		},
	})
	if !handled || got["install"] != "refuse-unless-operator-opt-in" {
		t.Fatalf("revoked cross reference = %#v handled=%v", got, handled)
	}

	got, handled = EvaluateRegistryInstallCrossReferences(map[string]any{
		"cross_references": []any{map[string]any{"resolution": "resolved"}},
	})
	if handled || got != nil {
		t.Fatalf("resolved cross references = %#v handled=%v", got, handled)
	}
}

func TestValidateRegistryEmitSidecar(t *testing.T) {
	t.Parallel()

	got, handled, err := ValidateRegistryEmitSidecar(map[string]any{
		"registry_section": map[string]any{"source_uri": "https://example.com/skill.md"},
	})
	if err != nil {
		t.Fatalf("ValidateRegistryEmitSidecar(valid) error: %v", err)
	}
	if !handled || !reflect.DeepEqual(got, map[string]any{"conformant": true}) {
		t.Fatalf("valid registry emit = %#v handled=%v", got, handled)
	}

	_, handled, err = ValidateRegistryEmitSidecar(map[string]any{
		"registry_section": map[string]any{},
	})
	if !handled {
		t.Fatal("missing source_uri was not handled as registry emit")
	}
	assertRejectID(t, err, ErrSourceURIMissing)

	got, handled, err = ValidateRegistryEmitSidecar(map[string]any{
		"kind":             "skill",
		"registry_section": map[string]any{"source_uri": "https://example.com/skill.md"},
	})
	if err != nil || handled || got != nil {
		t.Fatalf("envelope-like sidecar should not be handled: got=%#v handled=%v err=%v", got, handled, err)
	}
}
