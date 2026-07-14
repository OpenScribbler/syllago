package acif

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSkillFileIngestPinnedHashes(t *testing.T) {
	t.Parallel()

	t.Run("single-file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeBodyFixture(t, dir, map[string]string{
			"SKILL.md": "---\ndescription: demo\n---\nUse this to demo classification.\n",
		})
		got, err := IngestFrontmatterFile("skill", dir, "SKILL.md")
		if err != nil {
			t.Fatalf("IngestFrontmatterFile() error: %v", err)
		}
		if got.BodyHash != tvSkillG1 || got.Classification != "single-file" {
			t.Fatalf("hash/classification = %s/%s", got.BodyHash, got.Classification)
		}
		if got.Canonical["classification"] != "single-file" {
			t.Fatalf("canonical classification = %#v", got.Canonical)
		}
		body := got.Canonical["skill"].(map[string]any)["body"]
		if body != "Use this to demo classification.\n" {
			t.Fatalf("canonical skill body = %#v", body)
		}
		if got.MetadataHash == "" || got.PublisherSection == nil {
			t.Fatalf("frontmatter did not produce metadata: %#v", got)
		}
	})

	t.Run("multi-file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeBodyFixture(t, dir, map[string]string{
			"SKILL.md":       "Use this to demo classification.\n",
			"scripts/run.sh": "#!/bin/sh\necho hi\n",
		})
		got, err := IngestFrontmatterFile("skill", dir, "SKILL.md")
		if err != nil {
			t.Fatalf("IngestFrontmatterFile() error: %v", err)
		}
		if got.BodyHash != tvSkillG3 || got.Classification != "multi-file" {
			t.Fatalf("hash/classification = %s/%s", got.BodyHash, got.Classification)
		}
	})

	t.Run("multi-file-entry-frontmatter-stripped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeBodyFixture(t, dir, map[string]string{
			"SKILL.md":       "---\ndescription: one\n---\nProse body.\n",
			"scripts/run.sh": "#!/bin/sh\necho hi\n",
		})
		got, err := IngestFrontmatterFile("skill", dir, "SKILL.md")
		if err != nil {
			t.Fatalf("IngestFrontmatterFile() error: %v", err)
		}
		if got.BodyHash != tvSkillN {
			t.Fatalf("body_hash = %s, want %s", got.BodyHash, tvSkillN)
		}
	})
}

func TestSkillCanonicalizationAndPredicates(t *testing.T) {
	t.Parallel()

	defaulted, err := CanonicalizeSkill(map[string]any{})
	if err != nil {
		t.Fatalf("CanonicalizeSkill(default) error: %v", err)
	}
	wantActivation := map[string]any{"type": "auto", "user_invocable": true}
	if !reflect.DeepEqual(defaulted.Canonical["activation"], wantActivation) {
		t.Fatalf("default activation = %#v, want %#v", defaulted.Canonical["activation"], wantActivation)
	}

	rejects := []struct {
		name string
		in   map[string]any
		id   string
	}{
		{"missing type", map[string]any{"activation": map[string]any{}}, ErrSkillActivationTypeMissing},
		{"invalid type", map[string]any{"activation": map[string]any{"type": "automatic"}}, ErrSkillActivationTypeInvalid},
		{"hook ref forbidden", map[string]any{"activation": map[string]any{"type": "auto", "hook_ref": map[string]any{"id": "h1"}}}, ErrSkillHookRefForbidden},
		{"hook ref id missing", map[string]any{"activation": map[string]any{"type": "hook", "hook_ref": map[string]any{}}}, ErrSkillHookRefIDMissing},
	}
	for _, tc := range rejects {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalizeSkill(tc.in)
			assertReject(t, err, tc.id)
		})
	}

	hookSkill, err := CanonicalizeSkill(map[string]any{"activation": map[string]any{"type": "hook"}})
	if err != nil {
		t.Fatalf("hook skill without hook_ref rejected: %v", err)
	}
	if hookSkill.Canonical["activation"].(map[string]any)["user_invocable"] != true {
		t.Fatalf("hook skill did not materialize user_invocable: %#v", hookSkill.Canonical)
	}

	caps, err := DerivedCapabilitiesForItem(map[string]any{
		"kind":           "skill",
		"classification": "multi-file",
		"skill": map[string]any{
			"activation": map[string]any{"type": "manual", "user_invocable": false},
		},
	})
	if err != nil {
		t.Fatalf("DerivedCapabilitiesForItem(skill) error: %v", err)
	}
	wantCaps := map[string]bool{
		"auto_invocable":           false,
		"disable_model_invocation": true,
		"user_invocable":           true,
		"skill_bundled_resources":  true,
	}
	if !reflect.DeepEqual(caps, wantCaps) {
		t.Fatalf("skill capabilities = %#v, want %#v", caps, wantCaps)
	}
}

func TestSkillDeclaredMetadataMovesIndependentlyFromBody(t *testing.T) {
	t.Parallel()

	dirA := t.TempDir()
	dirB := t.TempDir()
	body := "Use this to demo classification.\n"
	writeBodyFixture(t, dirA, map[string]string{
		"SKILL.md": "---\nactivation:\n  type: auto\n---\n" + body,
	})
	writeBodyFixture(t, dirB, map[string]string{
		"SKILL.md": "---\nactivation:\n  type: auto\n  user_invocable: true\n---\n" + body,
	})
	a, err := IngestFrontmatterFile("skill", dirA, "SKILL.md")
	if err != nil {
		t.Fatalf("IngestFrontmatterFile(a): %v", err)
	}
	b, err := IngestFrontmatterFile("skill", dirB, "SKILL.md")
	if err != nil {
		t.Fatalf("IngestFrontmatterFile(b): %v", err)
	}
	if a.BodyHash != b.BodyHash {
		t.Fatalf("declared metadata changed body hash: %s != %s", a.BodyHash, b.BodyHash)
	}
	if a.MetadataHash == b.MetadataHash {
		t.Fatalf("declared user_invocable did not move metadata hash: %s", a.MetadataHash)
	}

	if err := os.WriteFile(filepath.Join(dirB, "extra.md"), []byte("extra\n"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	b2, err := IngestFrontmatterFile("skill", dirB, "SKILL.md")
	if err != nil {
		t.Fatalf("IngestFrontmatterFile(b2): %v", err)
	}
	if b.MetadataHash != b2.MetadataHash {
		t.Fatalf("bundled-resource edit moved metadata hash: %s != %s", b.MetadataHash, b2.MetadataHash)
	}
	if b.BodyHash == b2.BodyHash {
		t.Fatalf("bundled-resource edit did not move body hash: %s", b.BodyHash)
	}
}
