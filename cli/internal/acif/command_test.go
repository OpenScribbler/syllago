package acif

import (
	"reflect"
	"testing"
)

func TestCommandRewriteTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		want      string
		diagCount int
	}{
		{"claude args", "Review {{args}} carefully.\n", "Review $ARGUMENTS carefully.\n", 0},
		{"named input", "Open ${input:filename} now.\n", "Open $ARGUMENTS now.\n", 1},
		{"named input placeholder", "Open ${input:filename:path} now.\n", "Open $ARGUMENTS now.\n", 1},
		{"invalid named grammar", "Open ${input:} now.\n", "Open ${input:} now.\n", 0},
		{"positional verbatim", "Do $1 and ${@:3} and !{x} and @{y}.\n", "Do $1 and ${@:3} and !{x} and @{y}.\n", 0},
		{"fenced code rewritten", "```sh\necho {{args}}\n```\n", "```sh\necho $ARGUMENTS\n```\n", 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, diags := RewriteCommandPlaceholders(tc.in)
			if got != tc.want {
				t.Fatalf("rewrite = %q, want %q", got, tc.want)
			}
			if len(diags) != tc.diagCount {
				t.Fatalf("diagnostics = %#v, want %d", diags, tc.diagCount)
			}
			for _, diag := range diags {
				if diag.ID != DiagCommandPlaceholderNamedArgCollapsed {
					t.Fatalf("diagnostic id = %q", diag.ID)
				}
			}
		})
	}
}

func TestCommandFileIngestPinnedHashes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		hash string
	}{
		{"claude args", "---\ndescription: review\n---\nReview PR {{args}} carefully.\n", tvCommandA},
		{"named input", "---\ndescription: open\n---\nOpen ${input:filename} now.\n", tvCommandB},
		{"frontmatter stripped", "---\ndescription: task\n---\nDo the task with $ARGUMENTS.\n", tvCommandH},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeBodyFixture(t, dir, map[string]string{"COMMAND.md": tc.body})
			got, err := IngestFrontmatterFile("command", dir, "COMMAND.md")
			if err != nil {
				t.Fatalf("IngestFrontmatterFile(command): %v", err)
			}
			if got.BodyHash != tc.hash {
				t.Fatalf("body_hash = %s, want %s", got.BodyHash, tc.hash)
			}
		})
	}
}

func TestCommandAdvisoryAndRenderBoundaries(t *testing.T) {
	t.Parallel()

	item := map[string]any{"command": map[string]any{"body": "$ARGUMENTS $ARGUMENTS[0] $ARGUMENTSFOO $ARGUMENTS"}}
	projection, err := CommandAdvisoryProjection(item)
	if err != nil {
		t.Fatalf("CommandAdvisoryProjection(): %v", err)
	}
	wantProjection := map[string]any{
		"argument_substitution_token": map[string]any{
			"present": true,
			"method":  "substring-canonical-v1",
		},
	}
	if !reflect.DeepEqual(projection, wantProjection) {
		t.Fatalf("projection = %#v, want %#v", projection, wantProjection)
	}
	absentProjection, err := CommandAdvisoryProjection(map[string]any{"command": map[string]any{"body": "git rebase --onto $1 $2 && echo ${@:3}\n"}})
	if err != nil {
		t.Fatalf("CommandAdvisoryProjection(absent): %v", err)
	}
	wantAbsent := map[string]any{
		"argument_substitution_token": map[string]any{"present": false},
	}
	if !reflect.DeepEqual(absentProjection, wantAbsent) {
		t.Fatalf("absent projection = %#v, want %#v", absentProjection, wantAbsent)
	}

	renders := []struct {
		target string
		want   string
		diag   string
	}{
		{"gemini-form", "{{args}} $ARGUMENTS[0] $ARGUMENTSFOO {{args}}", ""},
		{"input-form", "${input:args} $ARGUMENTS[0] $ARGUMENTSFOO ${input:args}", ""},
		{"no-row-target", "$ARGUMENTS $ARGUMENTS[0] $ARGUMENTSFOO $ARGUMENTS", DiagCommandPlaceholderUntranslated},
	}
	for _, tc := range renders {
		tc := tc
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			got, err := RenderCommand(item, tc.target)
			if err != nil {
				t.Fatalf("RenderCommand(%s): %v", tc.target, err)
			}
			if got.Output != tc.want {
				t.Fatalf("output = %q, want %q", got.Output, tc.want)
			}
			if tc.diag == "" && len(got.Diagnostics) != 0 {
				t.Fatalf("diagnostics = %#v, want none", got.Diagnostics)
			}
			if tc.diag != "" {
				if len(got.Diagnostics) != 1 || got.Diagnostics[0].ID != tc.diag {
					t.Fatalf("diagnostics = %#v, want %s", got.Diagnostics, tc.diag)
				}
			}
		})
	}
}

func TestCommandMetadataMovesOnFrontmatterOnly(t *testing.T) {
	t.Parallel()

	dirA := t.TempDir()
	dirB := t.TempDir()
	body := "Do the task with $ARGUMENTS.\n"
	writeBodyFixture(t, dirA, map[string]string{"COMMAND.md": "---\ndescription: one\n---\n" + body})
	writeBodyFixture(t, dirB, map[string]string{"COMMAND.md": "---\ndescription: two\n---\n" + body})
	a, err := IngestFrontmatterFile("command", dirA, "COMMAND.md")
	if err != nil {
		t.Fatalf("IngestFrontmatterFile(a): %v", err)
	}
	b, err := IngestFrontmatterFile("command", dirB, "COMMAND.md")
	if err != nil {
		t.Fatalf("IngestFrontmatterFile(b): %v", err)
	}
	if a.BodyHash != b.BodyHash {
		t.Fatalf("frontmatter-only change moved body hash: %s != %s", a.BodyHash, b.BodyHash)
	}
	if a.MetadataHash == b.MetadataHash {
		t.Fatalf("frontmatter change did not move metadata hash: %s", a.MetadataHash)
	}
}
