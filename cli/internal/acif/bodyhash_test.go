package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeBodyFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, data := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

func assertReject(t *testing.T, err error, id string) {
	t.Helper()
	var reject *RejectError
	if !errors.As(err, &reject) {
		t.Fatalf("error = %T %v, want *RejectError %s", err, err, id)
	}
	if reject.ID != id {
		t.Fatalf("RejectError.ID = %q, want %q", reject.ID, id)
	}
}

func TestBodyHashVectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		files          map[string]string
		entry          string
		classification string
		hash           string
	}{
		{
			name: "TV-1 single file strips frontmatter",
			files: map[string]string{
				"SKILL.md": "---\ndescription: demo\n---\nUse this skill to demo hashing.\n",
			},
			entry:          "SKILL.md",
			classification: "single-file",
			hash:           "916e570331167c16f8112573d1b6020c134cc3d4019e8011693676d019b88ffe",
		},
		{
			name: "TV-12 nested sidecar included",
			files: map[string]string{
				"SKILL.md":              "Body prose.\n",
				"sub/acif-sidecar.yaml": "kind: skill\n",
			},
			entry:          "SKILL.md",
			classification: "multi-file",
			hash:           "581e9b6b2dbd5a5947758f8ecdf67896c2a1225936ab5bd744a3973460b85169",
		},
		{
			name: "TV-12 root sidecar excluded",
			files: map[string]string{
				"SKILL.md":              "Body prose.\n",
				"acif-sidecar.yaml":     "kind: skill\n",
				"sub/acif-sidecar.yaml": "kind: skill\n",
			},
			entry:          "SKILL.md",
			classification: "multi-file",
			hash:           "581e9b6b2dbd5a5947758f8ecdf67896c2a1225936ab5bd744a3973460b85169",
		},
		{
			name: "TV-12 nested sidecar edit changes hash",
			files: map[string]string{
				"SKILL.md":              "Body prose.\n",
				"sub/acif-sidecar.yaml": "kind: skill\nid: f47ac10b\n",
			},
			entry:          "SKILL.md",
			classification: "multi-file",
			hash:           "b4e9079283b7852f98fc2a9f9719699a1c5e3b48101695f8f7567314d3a2eae4",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeBodyFixture(t, dir, tc.files)
			got, err := BodyHash(dir, tc.entry)
			if err != nil {
				t.Fatalf("BodyHash() error: %v", err)
			}
			if got.Classification != tc.classification {
				t.Fatalf("classification = %q, want %q", got.Classification, tc.classification)
			}
			if got.HashHex != tc.hash {
				t.Fatalf("hash = %s, want %s", got.HashHex, tc.hash)
			}
		})
	}
}

func TestBodyHashRejectsSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	dir := t.TempDir()
	writeBodyFixture(t, dir, map[string]string{"SKILL.md": "Body prose.\n"})
	if err := os.Symlink("SKILL.md", filepath.Join(dir, "link.md")); err != nil {
		t.Skipf("creating symlink: %v", err)
	}

	_, err := BodyHash(dir, "SKILL.md")
	assertReject(t, err, ErrBodySymlink)
}

func TestBodyHashRejectsNFCPathCollision(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBodyFixture(t, dir, map[string]string{
		"SKILL.md": "Body prose.\n",
		"café.md":  "nfc\n",
		"café.md": "nfd\n",
	})

	_, err := BodyHash(dir, "SKILL.md")
	assertReject(t, err, ErrBodyPathCollision)
}

func TestBodyHashRejectsEmptyAfterRootLicenseAndReadmeExclusions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBodyFixture(t, dir, map[string]string{
		"LICENSE":   "MIT\n",
		"README.md": "# Demo\n",
	})

	_, err := BodyHash(dir, "SKILL.md")
	assertReject(t, err, ErrBodyEmpty)
}

func TestBodyHashRootReadmeExcludedFromClassification(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBodyFixture(t, dir, map[string]string{
		"SKILL.md":  "Body prose.\n",
		"Readme.md": "# Demo\n",
	})

	got, err := BodyHash(dir, "SKILL.md")
	if err != nil {
		t.Fatalf("BodyHash() error: %v", err)
	}
	if got.Classification != "single-file" {
		t.Fatalf("classification = %q, want single-file", got.Classification)
	}
}

func TestBodyHashRootLicenseIncludedInMultiFileManifest(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	writeBodyFixture(t, base, map[string]string{
		"SKILL.md": "Body prose.\n",
		"extra.md": "Extra prose.\n",
	})
	withLicense := t.TempDir()
	writeBodyFixture(t, withLicense, map[string]string{
		"SKILL.md": "Body prose.\n",
		"extra.md": "Extra prose.\n",
		"LICENSE":  "MIT\n",
	})

	baseHash, err := BodyHash(base, "SKILL.md")
	if err != nil {
		t.Fatalf("BodyHash(base): %v", err)
	}
	licenseHash, err := BodyHash(withLicense, "SKILL.md")
	if err != nil {
		t.Fatalf("BodyHash(withLicense): %v", err)
	}
	if baseHash.Classification != "multi-file" || licenseHash.Classification != "multi-file" {
		t.Fatalf("classifications = %q/%q, want multi-file/multi-file", baseHash.Classification, licenseHash.Classification)
	}
	if baseHash.HashHex == licenseHash.HashHex {
		t.Fatalf("adding root LICENSE did not change multi-file body hash: %s", baseHash.HashHex)
	}
}

func TestStripFrontmatterEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "no frontmatter", in: "Body prose.\n", want: "Body prose.\n"},
		{name: "unterminated block", in: "---\ncrud\n", want: "---\ncrud\n"},
		{name: "closed by delimiter line", in: "---\nkind: x\n---\nBody\n", want: "Body\n"},
		{name: "closed by eof", in: "---\nkind: x\n---", want: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := string(stripFrontmatter([]byte(tc.in))); got != tc.want {
				t.Fatalf("stripFrontmatter() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripFrontmatterClosedByEOFHashesEmptyInput(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256(stripFrontmatter([]byte("---\nkind: x\n---")))
	if got, want := hex.EncodeToString(sum[:]), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"; got != want {
		t.Fatalf("hash of stripped EOF frontmatter = %s, want %s", got, want)
	}
}
