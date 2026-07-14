package acif

import (
	"reflect"
	"strings"
	"testing"
)

func TestRuleFileIngestPinnedHashes(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeBodyFixture(t, dir, map[string]string{
			"RULE.md": "---\nactivation:\n  mode: always\n---\nPrefer functional patterns.\n",
		})
		got, err := IngestFrontmatterFile("rule", dir, "RULE.md")
		if err != nil {
			t.Fatalf("IngestFrontmatterFile(rule): %v", err)
		}
		if got.BodyHash != tvRuleA {
			t.Fatalf("body_hash = %s, want %s", got.BodyHash, tvRuleA)
		}
	})

	t.Run("prose imports opaque", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		body := "Follow @security.md and @~/company-standards.md.\n"
		writeBodyFixture(t, dir, map[string]string{"RULE.md": "---\nactivation:\n  mode: always\n---\n" + body})
		got, err := IngestFrontmatterFile("rule", dir, "RULE.md")
		if err != nil {
			t.Fatalf("IngestFrontmatterFile(rule): %v", err)
		}
		if got.BodyHash != tvRuleK {
			t.Fatalf("body_hash = %s, want %s", got.BodyHash, tvRuleK)
		}
		rendered, err := RenderRule(got.Canonical, "native-rule-format")
		if err != nil {
			t.Fatalf("RenderRule(native): %v", err)
		}
		if rendered.Output != body {
			t.Fatalf("native render = %q, want %q", rendered.Output, body)
		}
	})
}

func TestRuleCanonicalization(t *testing.T) {
	t.Parallel()

	defaulted, err := CanonicalizeRule(map[string]any{})
	if err != nil {
		t.Fatalf("CanonicalizeRule(default) error: %v", err)
	}
	if !reflect.DeepEqual(defaulted.Canonical["activation"], map[string]any{"mode": "always"}) {
		t.Fatalf("default activation = %#v", defaulted.Canonical["activation"])
	}

	rejects := []struct {
		name string
		in   map[string]any
		id   string
	}{
		{"mode missing", map[string]any{"activation": map[string]any{}}, ErrRuleActivationModeMissing},
		{"mode invalid", map[string]any{"activation": map[string]any{"mode": " always"}}, ErrRuleActivationModeInvalid},
		{"glob mode without globs", map[string]any{"activation": map[string]any{"mode": "glob"}}, ErrRuleGlobModeWithoutGlobs},
		{"glob mode empty globs", map[string]any{"activation": map[string]any{"mode": "glob", "globs": []any{}}}, ErrRuleGlobModeWithoutGlobs},
		{"globs without glob mode", map[string]any{"activation": map[string]any{"mode": "manual", "globs": []any{"*.go"}}}, ErrRuleGlobsWithoutGlobMode},
	}
	for _, tc := range rejects {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalizeRule(tc.in)
			assertReject(t, err, tc.id)
		})
	}

	requires := map[string]any{
		"activation": map[string]any{"mode": "glob"},
		"requires":   map[string]any{"future": true},
	}
	_, err = CanonicalizeRule(requires)
	assertReject(t, err, ErrRuleGlobModeWithoutGlobs)
}

func TestRuleMechanismDecode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  map[string]any
		want map[string]any
	}{
		{
			name: "always on",
			cfg:  map[string]any{"provider": "rule-activation-source", "content": map[string]any{"source_sub_mode": "always_on"}},
			want: map[string]any{"mode": "always"},
		},
		{
			name: "frontmatter globs",
			cfg:  map[string]any{"provider": "rule-activation-source", "content": map[string]any{"source_sub_mode": "frontmatter_globs", "globs": []any{"*.go"}}},
			want: map[string]any{"mode": "glob", "globs": []any{"*.go"}},
		},
		{
			name: "legacy residual",
			cfg:  map[string]any{"provider": "rule-activation-source", "content": map[string]any{"source_sub_mode": "legacy", "always_apply": false}},
			want: map[string]any{"mode": "model_decision"},
		},
		{
			name: "slash command",
			cfg:  map[string]any{"provider": "rule-activation-source", "content": map[string]any{"source_sub_mode": "slash_command"}},
			want: map[string]any{"mode": "manual"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CanonicalizeRuleProviderConfig(tc.cfg)
			if err != nil {
				t.Fatalf("CanonicalizeRuleProviderConfig() error: %v", err)
			}
			if !reflect.DeepEqual(got.Canonical["activation"], tc.want) {
				t.Fatalf("activation = %#v, want %#v", got.Canonical["activation"], tc.want)
			}
		})
	}

	_, err := CanonicalizeRuleProviderConfig(map[string]any{
		"provider": "rule-activation-source",
		"content":  map[string]any{"source_sub_mode": "sometimes"},
	})
	assertReject(t, err, ErrRuleActivationModeUnmappable)
}

func TestRuleDerivedProjectionAndDegradedRender(t *testing.T) {
	t.Parallel()

	item := map[string]any{"rule": map[string]any{"activation": map[string]any{"mode": "glob", "globs": []any{"*.go", "cmd/**", "internal/**"}}}}
	caps, err := DerivedCapabilitiesForItem(item)
	if err != nil {
		t.Fatalf("DerivedCapabilitiesForItem(rule): %v", err)
	}
	if !reflect.DeepEqual(caps, map[string]bool{"activation_mode": true}) {
		t.Fatalf("rule caps = %#v", caps)
	}

	projection, err := RuleActivationProjection(item)
	if err != nil {
		t.Fatalf("RuleActivationProjection(): %v", err)
	}
	wantProjection := map[string]any{
		"derivable": true,
		"mode":      "glob",
		"globs": map[string]any{
			"present":   true,
			"count":     3,
			"sample":    []any{"*.go", "cmd/**", "internal/**"},
			"truncated": false,
		},
	}
	if !reflect.DeepEqual(projection, wantProjection) {
		t.Fatalf("projection = %#v, want %#v", projection, wantProjection)
	}

	rendered, err := RenderRule(map[string]any{"rule": map[string]any{"activation": map[string]any{"mode": "manual"}}}, "no-declaration-surface-provider")
	if err != nil {
		t.Fatalf("RenderRule(degraded): %v", err)
	}
	if !strings.Contains(rendered.Output, "rule-body") {
		t.Fatalf("degraded output = %q, want placeholder", rendered.Output)
	}
	wantDiag := Diagnostic{
		ID: ErrRuleActivationDegraded,
		Params: map[string]any{
			"mode_lost":          "manual",
			"effective_behavior": "always-on",
		},
	}
	if !reflect.DeepEqual(rendered.Diagnostics, []Diagnostic{wantDiag}) {
		t.Fatalf("diagnostics = %#v, want %#v", rendered.Diagnostics, []Diagnostic{wantDiag})
	}
}
