package acif

import (
	"encoding/json"
	"testing"
)

func rawObject(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	return obj
}

func TestValidateEnvelopeTV11InvalidCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		raw    string
		reason string
	}{
		{
			name:   "kind case mismatch",
			raw:    `{"kind":"Skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo"}`,
			reason: "kind-not-in-closed-enum",
		},
		{
			name:   "id not uuid",
			raw:    `{"kind":"skill","id":"not-a-uuid","display_name":"Demo"}`,
			reason: "id-not-uuid-v4",
		},
		{
			name:   "version not semver",
			raw:    `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo","version":"1.0"}`,
			reason: "version-not-semver",
		},
		{
			name:   "license spdx not identifier",
			raw:    `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo","license":{"spdx":"MIT License"}}`,
			reason: "license-spdx-not-identifier",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			section := rawObject(t, tc.raw)
			got := ValidateEnvelope(section, []map[string]json.RawMessage{section})
			if got.Conformant {
				t.Fatalf("ValidateEnvelope() conformant, want rejection")
			}
			if got.Reason != tc.reason {
				t.Fatalf("reason = %q, want %q", got.Reason, tc.reason)
			}
		})
	}
}

func TestValidateEnvelopeTV6ForbiddenEffectiveVersion(t *testing.T) {
	t.Parallel()
	section := rawObject(t, `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo","effective_version":"1.0.0"}`)
	got := ValidateEnvelope(section, []map[string]json.RawMessage{section})
	if got.Conformant {
		t.Fatalf("ValidateEnvelope() conformant, want forbidden-field rejection")
	}
	if got.Reason != "forbidden-field effective_version" {
		t.Fatalf("reason = %q", got.Reason)
	}
}

func TestValidateEnvelopeValid(t *testing.T) {
	t.Parallel()
	section := rawObject(t, `{"kind":"agent","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo Agent","version":"1.2.3-alpha+build","license":{"spdx":"MIT"}}`)
	got := ValidateEnvelope(section, []map[string]json.RawMessage{section})
	if !got.Conformant {
		t.Fatalf("ValidateEnvelope() rejected valid envelope: %q", got.Reason)
	}
	if got.Reason != "" {
		t.Fatalf("reason = %q, want empty", got.Reason)
	}
}

func TestValidateEnvelopeForbiddenFieldsInEachSection(t *testing.T) {
	t.Parallel()

	forbidden := []string{"effective_version", "derived_version", "pack_inherited_version", "resolved_version"}
	locations := []string{"top-level", "publisher-section", "registry-section"}

	for _, field := range forbidden {
		field := field
		for _, location := range locations {
			location := location
			t.Run(location+"/"+field, func(t *testing.T) {
				t.Parallel()
				valid := `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo"}`
				top := rawObject(t, `{}`)
				publisher := rawObject(t, valid)
				registry := rawObject(t, `{}`)

				switch location {
				case "top-level":
					top[field] = json.RawMessage(`"x"`)
				case "publisher-section":
					publisher[field] = json.RawMessage(`"x"`)
				case "registry-section":
					registry[field] = json.RawMessage(`"x"`)
				}

				got := ValidateEnvelope(publisher, []map[string]json.RawMessage{top, publisher, registry})
				if got.Conformant {
					t.Fatalf("ValidateEnvelope() conformant with forbidden %s in %s", field, location)
				}
				if want := "forbidden-field " + field; got.Reason != want {
					t.Fatalf("reason = %q, want %q", got.Reason, want)
				}
			})
		}
	}
}

func TestValidateEnvelopeMissingRequiredFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		raw    string
		reason string
	}{
		{name: "kind", raw: `{"id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo"}`, reason: "missing-required-field kind"},
		{name: "id", raw: `{"kind":"skill","display_name":"Demo"}`, reason: "missing-required-field id"},
		{name: "display_name", raw: `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479"}`, reason: "missing-required-field display_name"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			section := rawObject(t, tc.raw)
			got := ValidateEnvelope(section, []map[string]json.RawMessage{section})
			if got.Conformant {
				t.Fatalf("ValidateEnvelope() conformant, want rejection")
			}
			if got.Reason != tc.reason {
				t.Fatalf("reason = %q, want %q", got.Reason, tc.reason)
			}
		})
	}
}
