package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

// --- runSign / runVerify (currently 0%, both stub errors) ---

func TestRunSign_NotImplemented(t *testing.T) {
	t.Parallel()
	err := runSign(&cobra.Command{}, []string{"my-hook"})
	if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("got %v, want 'not yet implemented' error", err)
	}
}

func TestRunVerify_NotImplemented(t *testing.T) {
	t.Parallel()
	err := runVerify(&cobra.Command{}, []string{"my-hook"})
	if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("got %v, want 'not yet implemented' error", err)
	}
}

// --- reportFixtureAges (0% — pure print) ---

func TestReportFixtureAges_AlwaysReturnsNil(t *testing.T) {
	// Mutates os.Stdout — not parallel.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err := reportFixtureAges("/some/fixtures/dir")

	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Errorf("got err = %v, want nil", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/some/fixtures/dir") {
		t.Errorf("output %q missing fixtures dir", out)
	}
}

// --- runUpdate (0% — both branches reachable) ---

func TestRunUpdate_DevBuildBranch(t *testing.T) {
	// Mutates package-level buildCommit; cannot run parallel with other
	// tests that also touch it. We are the only such test.
	origCommit := buildCommit
	buildCommit = "deadbeef" // pretend we're a dev build
	t.Cleanup(func() { buildCommit = origCommit })

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runUpdate(cmd, nil)
	if err != nil {
		t.Errorf("got err = %v, want nil (dev build returns nil)", err)
	}
	out := buf.String()
	if !strings.Contains(out, "dev build") {
		t.Errorf("output %q missing 'dev build' message", out)
	}
}

// --- resolveConflictInteractively (0% — reads os.Stdin) ---

// withStdin replaces os.Stdin with a pipe carrying input, restores on cleanup.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		r.Close()
	})
}

func makeConflict() []installer.Conflict {
	return []installer.Conflict{{
		InstallingTo: provider.Provider{Name: "Cursor"},
		AlsoReadBy:   []provider.Provider{{Name: "Windsurf"}},
		SharedPath:   "~/.shared/rules",
	}}
}

func TestResolveConflictInteractively_Choice1(t *testing.T) {
	// Sequential: mutates os.Stdin.
	withStdin(t, "1\n")
	var stdout, stderr bytes.Buffer
	got, err := resolveConflictInteractively(makeConflict(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != installer.ResolutionSharedOnly {
		t.Errorf("got %v, want ResolutionSharedOnly", got)
	}
	// Sanity: prompt was emitted.
	if !strings.Contains(stdout.String(), "Shared path only") {
		t.Errorf("stdout missing prompt: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Cursor") {
		t.Errorf("stderr missing conflict info: %q", stderr.String())
	}
}

func TestResolveConflictInteractively_Choice2(t *testing.T) {
	withStdin(t, "2\n")
	var stdout, stderr bytes.Buffer
	got, err := resolveConflictInteractively(makeConflict(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != installer.ResolutionOwnDirsOnly {
		t.Errorf("got %v, want ResolutionOwnDirsOnly", got)
	}
}

func TestResolveConflictInteractively_DefaultOnEmpty(t *testing.T) {
	withStdin(t, "\n")
	var stdout, stderr bytes.Buffer
	got, err := resolveConflictInteractively(makeConflict(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != installer.ResolutionAll {
		t.Errorf("got %v, want ResolutionAll (default)", got)
	}
}

func TestResolveConflictInteractively_DefaultOnUnknown(t *testing.T) {
	withStdin(t, "garbage\n")
	var stdout, stderr bytes.Buffer
	got, err := resolveConflictInteractively(makeConflict(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != installer.ResolutionAll {
		t.Errorf("got %v, want ResolutionAll (unknown choice)", got)
	}
}

// --- runMoatSign error paths (0%) ---

func TestRunMoatSign_MissingManifest(t *testing.T) {
	t.Parallel()
	cmd := newMoatSignTestCmd(t, "")
	cmd.Flags().Set("manifest", "/nonexistent/manifest.json")
	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "reading manifest") {
		t.Errorf("got %v, want 'reading manifest' error", err)
	}
}

func TestRunMoatSignOnline_MissingRekorRaw(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--rekor-raw is required") {
		t.Errorf("got %v, want '--rekor-raw required' error", err)
	}
}

func TestRunMoatSignDev_HappyPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	devDir := filepath.Join(tmp, "dev-trusted-root")

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("dev-trusted-root", devDir)
	cmd.Flags().Set("out", filepath.Join(tmp, "manifest.sigstore"))

	if err := runMoatSign(cmd, nil); err != nil {
		t.Fatalf("runMoatSign: %v", err)
	}

	// Verify both bundle and trusted root were written.
	if _, err := os.Stat(filepath.Join(tmp, "manifest.sigstore")); err != nil {
		t.Errorf("bundle missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(devDir, "trusted_root.json")); err != nil {
		t.Errorf("trusted root missing: %v", err)
	}
}

// --- resolveSigningProfileForSign (0%) ---

func TestResolveSigningProfileForSign_FromIdentityFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	idPath := filepath.Join(tmp, "id.json")
	if err := os.WriteFile(idPath, []byte(`{"issuer":"https://example.com","subject":"alice@example.com"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveSigningProfileForSign(idPath, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Issuer != "https://example.com" || got.Subject != "alice@example.com" {
		t.Errorf("got %+v, want issuer/subject from file", got)
	}
}

func TestResolveSigningProfileForSign_MissingIdentityFile(t *testing.T) {
	t.Parallel()
	_, err := resolveSigningProfileForSign("/does/not/exist.json", nil)
	if err == nil || !strings.Contains(err.Error(), "reading identity file") {
		t.Errorf("got %v, want 'reading identity file' error", err)
	}
}

func TestResolveSigningProfileForSign_BadJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	idPath := filepath.Join(tmp, "id.json")
	if err := os.WriteFile(idPath, []byte(`{not valid json`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveSigningProfileForSign(idPath, nil)
	if err == nil || !strings.Contains(err.Error(), "parsing identity JSON") {
		t.Errorf("got %v, want 'parsing identity JSON' error", err)
	}
}

func TestResolveSigningProfileForSign_EmptyFields(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	idPath := filepath.Join(tmp, "id.json")
	if err := os.WriteFile(idPath, []byte(`{"issuer":"","subject":""}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveSigningProfileForSign(idPath, nil)
	if err == nil || !strings.Contains(err.Error(), "non-empty issuer and subject") {
		t.Errorf("got %v, want 'non-empty issuer and subject' error", err)
	}
}

func TestResolveSigningProfileForSign_AutoExtractFails(t *testing.T) {
	t.Parallel()
	// No identityPath, garbage rekorRaw → ExtractIdentityFromRekorRaw should fail.
	_, err := resolveSigningProfileForSign("", []byte("not a rekor entry"))
	if err == nil || !strings.Contains(err.Error(), "auto-extracting identity") {
		t.Errorf("got %v, want 'auto-extracting identity' error", err)
	}
}

// --- resolveTrustedRootForSign (0%) ---

func TestResolveTrustedRootForSign_BundledDefault(t *testing.T) {
	t.Parallel()
	got, err := resolveTrustedRootForSign("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got.Bytes) == 0 {
		t.Errorf("got empty bundled trusted root bytes")
	}
}

func TestResolveTrustedRootForSign_FromPathMissing(t *testing.T) {
	t.Parallel()
	_, err := resolveTrustedRootForSign("/does/not/exist.json")
	if err == nil {
		t.Fatal("expected error from missing trusted root file")
	}
}

// --- moatNow (0% — trivial clock seam) ---

func TestMoatNow_DefaultUsesWallClock(t *testing.T) {
	// Sequential because we mutate the package-level moatClockOverride.
	orig := moatClockOverride
	moatClockOverride = nil
	t.Cleanup(func() { moatClockOverride = orig })

	before := time.Now()
	got := moatNow()
	after := time.Now()
	if got.Before(before) || got.After(after.Add(time.Second)) {
		t.Errorf("moatNow returned %v, expected within [%v, %v]", got, before, after)
	}
}

func TestMoatNow_OverrideTakesPrecedence(t *testing.T) {
	orig := moatClockOverride
	pinned := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	moatClockOverride = func() time.Time { return pinned }
	t.Cleanup(func() { moatClockOverride = orig })

	if got := moatNow(); !got.Equal(pinned) {
		t.Errorf("moatNow with override = %v, want %v", got, pinned)
	}
}

// --- converterAdapter.Canonicalize (66.7% — covers all 4 branches) ---

// stubConverter is a hand-rolled converter.Converter used only to drive
// Canonicalize through its branches. Render and ContentType are unused in
// these tests and panic if hit so a future refactor doesn't accidentally
// start invoking them.
type stubConverter struct {
	result *converter.Result
	err    error
}

func (s stubConverter) Canonicalize(_ []byte, _ string) (*converter.Result, error) {
	return s.result, s.err
}

func (s stubConverter) Render(_ []byte, _ provider.Provider) (*converter.Result, error) {
	panic("Render must not be called in Canonicalize tests")
}

func (s stubConverter) ContentType() catalog.ContentType {
	panic("ContentType must not be called in Canonicalize tests")
}

func TestConverterAdapter_Canonicalize_Success(t *testing.T) {
	t.Parallel()
	a := &converterAdapter{conv: stubConverter{
		result: &converter.Result{Content: []byte("hello"), Filename: "out.md"},
	}}
	content, name, err := a.Canonicalize([]byte("raw"), "claude-code")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if string(content) != "hello" || name != "out.md" {
		t.Errorf("got (%q, %q), want (hello, out.md)", content, name)
	}
}

func TestConverterAdapter_Canonicalize_Error(t *testing.T) {
	t.Parallel()
	wantErr := os.ErrInvalid
	a := &converterAdapter{conv: stubConverter{err: wantErr}}
	_, _, err := a.Canonicalize([]byte("raw"), "claude-code")
	if err != wantErr {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

func TestConverterAdapter_Canonicalize_NilResult(t *testing.T) {
	t.Parallel()
	a := &converterAdapter{conv: stubConverter{result: nil}}
	content, name, err := a.Canonicalize([]byte("raw"), "claude-code")
	if err != nil || content != nil || name != "" {
		t.Errorf("got (%v, %q, %v), want (nil, empty, nil)", content, name, err)
	}
}

func TestConverterAdapter_Canonicalize_NilContent(t *testing.T) {
	t.Parallel()
	a := &converterAdapter{conv: stubConverter{
		result: &converter.Result{Content: nil, Filename: "x"},
	}}
	content, name, err := a.Canonicalize([]byte("raw"), "claude-code")
	if err != nil || content != nil || name != "" {
		t.Errorf("got (%v, %q, %v), want (nil, empty, nil)", content, name, err)
	}
}

// --- runMoatSign online happy path using captured Phase 0 fixtures ---
// The Rekor JSON + matching attestation + bundled trusted root are captured
// from the syllago-meta-registry Publisher Action — same fixtures that drive
// the moat package's own integration tests.

func TestRunMoatSignOnline_HappyPath_WithFixture(t *testing.T) {
	t.Parallel()
	moatPkg := "../../internal/moat/testdata"
	rekorPath := filepath.Join(moatPkg, "rekor-syllago-guide.json")
	trustedRootPath := filepath.Join(moatPkg, "trusted-root-public-good.json")

	// Build the canonical payload matching the captured Rekor entry.
	// content_hash for syllago-guide is pinned in moat-attestation.json.
	const guideHash = "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"
	manifestBytes := moat.CanonicalPayloadFor(guideHash)

	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(tmp, "id.json")
	// Identity must match the workflow that produced the fixture entry.
	if err := os.WriteFile(identityPath, []byte(`{
		"issuer":"https://token.actions.githubusercontent.com",
		"subject":"https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/moat.yml@refs/heads/master"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(tmp, "manifest.sigstore")

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("rekor-raw", rekorPath)
	cmd.Flags().Set("trusted-root", trustedRootPath)
	cmd.Flags().Set("identity", identityPath)
	cmd.Flags().Set("out", outPath)

	if err := runMoatSign(cmd, nil); err != nil {
		t.Fatalf("runMoatSign online happy path: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("bundle missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("bundle file is empty")
	}
}

func TestRunMoatSignOnline_BadRekorRaw(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("rekor-raw", "/does/not/exist.json")

	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "reading rekor raw") {
		t.Errorf("got %v, want 'reading rekor raw' error", err)
	}
}

func TestRunMoatSignOnline_GarbageRekorRaw(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	rekorPath := filepath.Join(tmp, "rekor.json")
	if err := os.WriteFile(rekorPath, []byte("not a rekor entry at all"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("rekor-raw", rekorPath)

	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "building bundle") {
		t.Errorf("got %v, want 'building bundle' error", err)
	}
}

// --- exportWithConverter (48.6%) ---
//
// Covers the three top-level branches: same-provider .source/ passthrough,
// cross-provider canonicalize+render, and the default fall-through. Error
// paths inside each branch (missing source file, read/canonicalize/render
// failure, nil rendered Content, extra files) are covered by dedicated cases.

// fullConverter is a flexible converter.Converter that lets each test pin
// distinct behavior for Canonicalize and Render. Unlike stubConverter, both
// methods are usable. ContentType still panics — exportWithConverter never
// calls it.
type fullConverter struct {
	canonResult *converter.Result
	canonErr    error
	renderFn    func([]byte, provider.Provider) (*converter.Result, error)
}

func (f fullConverter) Canonicalize(_ []byte, _ string) (*converter.Result, error) {
	return f.canonResult, f.canonErr
}

func (f fullConverter) Render(content []byte, p provider.Provider) (*converter.Result, error) {
	if f.renderFn != nil {
		return f.renderFn(content, p)
	}
	return nil, nil
}

func (f fullConverter) ContentType() catalog.ContentType {
	panic("ContentType must not be called in exportWithConverter tests")
}

// makeSourceItem creates a catalog item on disk with .source/<fname> inside.
// Returns the item ready to pass to exportWithConverter.
func makeSourceItem(t *testing.T, dir, name, srcFname, content, provSlug string) catalog.ContentItem {
	t.Helper()
	itemDir := filepath.Join(dir, name)
	sourceDir := filepath.Join(itemDir, converter.SourceDir)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if srcFname != "" {
		if err := os.WriteFile(filepath.Join(sourceDir, srcFname), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return catalog.ContentItem{
		Name:     name,
		Type:     catalog.Rules,
		Path:     itemDir,
		Provider: provSlug,
	}
}

// makeContentItem creates an item with a rule.md file (no .source/).
func makeContentItem(t *testing.T, dir, name, provSlug string) catalog.ContentItem {
	t.Helper()
	itemDir := filepath.Join(dir, name)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# rule body"), 0644); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{
		Name:     name,
		Type:     catalog.Rules,
		Path:     itemDir,
		Provider: provSlug,
	}
}

func TestExportWithConverter_SameProviderSourceCopy(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	item := makeSourceItem(t, tmp, "my-rule", "orig.md", "original bytes", "claude-code")
	prov := provider.Provider{Slug: "claude-code", Name: "Claude Code"}
	installDir := filepath.Join(tmp, "install")

	got, handled := exportWithConverter(item, prov, "claude-code", fullConverter{}, installDir)
	if !handled {
		t.Fatal("handled=false, want true (same-provider .source/ branch)")
	}
	if got == nil || got.Name != "my-rule" {
		t.Fatalf("got %+v, want syncInstalledItem for my-rule", got)
	}
	destBytes, err := os.ReadFile(got.Destination)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(destBytes) != "original bytes" {
		t.Errorf("dest content = %q, want 'original bytes'", destBytes)
	}
}

func TestExportWithConverter_SameProviderNoSourceFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// .source/ exists but has no regular files → SourceFilePath returns ""
	item := makeSourceItem(t, tmp, "empty-src", "", "", "claude-code")
	prov := provider.Provider{Slug: "claude-code"}
	installDir := filepath.Join(tmp, "install")

	got, handled := exportWithConverter(item, prov, "claude-code", fullConverter{}, installDir)
	if handled || got != nil {
		t.Errorf("got (%v, %v), want (nil, false) when .source/ has no files", got, handled)
	}
}

func TestExportWithConverter_CrossProviderRender(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	item := makeContentItem(t, tmp, "my-rule", "cursor")
	prov := provider.Provider{Slug: "claude-code", Name: "Claude Code"}
	installDir := filepath.Join(tmp, "install")

	conv := fullConverter{
		canonResult: &converter.Result{Content: []byte("canonical")},
		renderFn: func(_ []byte, _ provider.Provider) (*converter.Result, error) {
			return &converter.Result{
				Content:  []byte("claude rendered"),
				Filename: "rule.mdc",
				Warnings: []string{"lost frontmatter"},
				ExtraFiles: map[string][]byte{
					"wrapper.sh": []byte("#!/bin/sh\necho hi\n"),
				},
			}, nil
		},
	}

	got, handled := exportWithConverter(item, prov, "claude-code", conv, installDir)
	if !handled || got == nil {
		t.Fatalf("got (%v, %v), want handled item", got, handled)
	}
	if !got.Converted {
		t.Error("Converted = false, want true")
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "lost frontmatter" {
		t.Errorf("warnings = %v, want ['lost frontmatter']", got.Warnings)
	}
	mainBytes, err := os.ReadFile(got.Destination)
	if err != nil || string(mainBytes) != "claude rendered" {
		t.Errorf("main content = %q, err = %v", mainBytes, err)
	}
	extraBytes, err := os.ReadFile(filepath.Join(filepath.Dir(got.Destination), "wrapper.sh"))
	if err != nil || !strings.Contains(string(extraBytes), "echo hi") {
		t.Errorf("extra file content = %q, err = %v", extraBytes, err)
	}
}

func TestExportWithConverter_CrossProviderCanonicalizeError(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	item := makeContentItem(t, tmp, "broken", "cursor")
	prov := provider.Provider{Slug: "claude-code"}
	installDir := filepath.Join(tmp, "install")

	conv := fullConverter{canonErr: os.ErrInvalid}
	got, handled := exportWithConverter(item, prov, "claude-code", conv, installDir)
	if got != nil || handled {
		t.Errorf("got (%v, %v), want (nil, false) on canonicalize err", got, handled)
	}
}

func TestExportWithConverter_CrossProviderRenderError(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	item := makeContentItem(t, tmp, "broken-render", "cursor")
	prov := provider.Provider{Slug: "claude-code"}
	installDir := filepath.Join(tmp, "install")

	conv := fullConverter{
		canonResult: &converter.Result{Content: []byte("ok")},
		renderFn: func(_ []byte, _ provider.Provider) (*converter.Result, error) {
			return nil, os.ErrPermission
		},
	}
	got, handled := exportWithConverter(item, prov, "claude-code", conv, installDir)
	if got != nil || handled {
		t.Errorf("got (%v, %v), want (nil, false) on render err", got, handled)
	}
}

func TestExportWithConverter_CrossProviderRenderSkip(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	item := makeContentItem(t, tmp, "skip-me", "cursor")
	prov := provider.Provider{Slug: "claude-code", Name: "Claude Code"}
	installDir := filepath.Join(tmp, "install")

	conv := fullConverter{
		canonResult: &converter.Result{Content: []byte("ok")},
		renderFn: func(_ []byte, _ provider.Provider) (*converter.Result, error) {
			return &converter.Result{Content: nil}, nil // nil Content = skip
		},
	}
	got, handled := exportWithConverter(item, prov, "claude-code", conv, installDir)
	if got != nil || !handled {
		t.Errorf("got (%v, %v), want (nil, true) on skip", got, handled)
	}
}

func TestExportWithConverter_CrossProviderMissingContentFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Empty item dir → ResolveContentFile returns ""
	itemDir := filepath.Join(tmp, "no-content")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{
		Name:     "no-content",
		Type:     catalog.Rules,
		Path:     itemDir,
		Provider: "cursor",
	}
	prov := provider.Provider{Slug: "claude-code"}
	installDir := filepath.Join(tmp, "install")

	got, handled := exportWithConverter(item, prov, "claude-code", fullConverter{}, installDir)
	if got != nil || handled {
		t.Errorf("got (%v, %v), want (nil, false) with no content file", got, handled)
	}
}

func TestExportWithConverter_NoProviderFallThrough(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// srcProvider == "" → skips both branches, falls through
	itemDir := filepath.Join(tmp, "orphan")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{
		Name: "orphan",
		Type: catalog.Skills,
		Path: itemDir,
		// Provider empty, Meta nil
	}
	prov := provider.Provider{Slug: "claude-code"}
	installDir := filepath.Join(tmp, "install")

	got, handled := exportWithConverter(item, prov, "claude-code", fullConverter{}, installDir)
	if got != nil || handled {
		t.Errorf("got (%v, %v), want (nil, false) on fall-through", got, handled)
	}
}

// Ensure the installer package import survives gofmt: referenced here so this
// test file imports it only if we actually use installer-returning helpers.
var _ = installer.CopyContent

// --- runMoatSignDev filesystem error paths ---
//
// runMoatSignDev wraps moat.SignManifestDev (always succeeds for empty manifest
// bytes) and writes two files: trusted_root.json under trustedRootDir, and the
// bundle under outPath. Each os.MkdirAll/os.WriteFile call is a separate
// branch — feed one a path whose parent is a regular file to trip MkdirAll.

func TestRunMoatSignDev_TrustedRootMkdirFails(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Place a regular file where the trusted-root dir's parent should be.
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	// trusted-root-dir under that file → MkdirAll fails.
	trustedRootDir := filepath.Join(blocker, "trusted")

	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(tmp, "out.sigstore")

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("dev-trusted-root", trustedRootDir)
	cmd.Flags().Set("out", outPath)

	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "creating dev trusted root dir") {
		t.Errorf("got %v, want 'creating dev trusted root dir' error", err)
	}
}

func TestRunMoatSignDev_OutMkdirFails(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	trustedRootDir := filepath.Join(tmp, "trusted")
	// Put a regular file where outPath's parent dir should be.
	blocker := filepath.Join(tmp, "out-blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(blocker, "subdir", "out.sigstore")

	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newMoatSignTestCmd(t, tmp)
	cmd.Flags().Set("manifest", manifestPath)
	cmd.Flags().Set("dev-trusted-root", trustedRootDir)
	cmd.Flags().Set("out", outPath)

	err := runMoatSign(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "creating output directory") {
		t.Errorf("got %v, want 'creating output directory' error", err)
	}
}

// --- registry subcommands (registry_cmd.go) ---
//
// registry_cmd.go has 222 uncovered lines, almost all in inline RunE closures
// for remove, list, sync, and items. These cover the cheap branches: empty
// config, registry-not-found, plus a happy path for remove and items.

// withRegistryProjectAndCache sets up a temp project root with a config + a
// registry cache override. Returns the project root.
//
// Registries live in the global config (~/.syllago/config.json), so the helper
// also redirects config.GlobalDirOverride at `root`. The returned `root` is
// thus both the project root (for findProjectRoot) and the global config
// directory — kept colocated so existing tests that pass `root` to LoadGlobal
// see the same fixture.
func withRegistryProjectAndCache(t *testing.T, regs []provider.Provider, cfg interface{}) string {
	t.Helper()
	_ = regs // unused, signature parity with similar helpers
	root := t.TempDir()
	cacheDir := t.TempDir()

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = root
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	if cfg != nil {
		c, ok := cfg.(*config.Config)
		if !ok {
			t.Fatalf("withRegistryProjectAndCache: cfg must be *config.Config, got %T", cfg)
		}
		if err := config.SaveGlobal(c); err != nil {
			t.Fatalf("config.SaveGlobal: %v", err)
		}
	}
	return root
}

func TestRegistryRemove_NotFound(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "exists", URL: "https://example.com/exists.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registryRemoveCmd.SilenceUsage = true
	registryRemoveCmd.SilenceErrors = true
	err := registryRemoveCmd.RunE(registryRemoveCmd, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("got %v, want 'missing' not-found error", err)
	}
}

func TestRegistryRemove_HappyPath(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "to-delete", URL: "https://example.com/to-delete.git"},
			{Name: "keep", URL: "https://example.com/keep.git"},
		},
	}
	root := withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)

	// Place a fake clone dir so registry.Remove has something to delete.
	cacheDir := registry.CacheDirOverride
	cloneDir := filepath.Join(cacheDir, "to-delete")
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatal(err)
	}

	registryRemoveCmd.SilenceUsage = true
	registryRemoveCmd.SilenceErrors = true
	if err := registryRemoveCmd.RunE(registryRemoveCmd, []string{"to-delete"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(stdout.String(), "to-delete") {
		t.Errorf("stdout missing 'to-delete', got: %s", stdout.String())
	}

	// Verify config was saved with only "keep" remaining.
	_ = root
	saved, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("config.LoadGlobal: %v", err)
	}
	if len(saved.Registries) != 1 || saved.Registries[0].Name != "keep" {
		t.Errorf("expected 1 remaining registry 'keep', got %v", saved.Registries)
	}
	// Clone dir should be gone.
	if _, err := os.Stat(cloneDir); !os.IsNotExist(err) {
		t.Errorf("clone dir still present after remove: err=%v", err)
	}
}

func TestRegistryList_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Providers: []string{"claude-code"}}
	withRegistryProjectAndCache(t, nil, cfg)

	// list cmd uses fmt.Println to stdout — capture via os.Pipe to be sure.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	output.SetForTest(t)

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true
	err := registryListCmd.RunE(registryListCmd, nil)
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(buf.String(), "No registries configured") {
		t.Errorf("expected 'No registries configured' in output, got: %s", buf.String())
	}
}

func TestRegistrySync_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Providers: []string{"claude-code"}}
	withRegistryProjectAndCache(t, nil, cfg)

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	output.SetForTest(t)

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	err := registrySyncCmd.RunE(registrySyncCmd, nil)
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !strings.Contains(buf.String(), "No registries configured") {
		t.Errorf("expected 'No registries configured' in output, got: %s", buf.String())
	}
}

func TestRegistrySync_NameNotFound(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "real", URL: "https://example.com/real.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	err := registrySyncCmd.RunE(registrySyncCmd, []string{"missing-reg"})
	if err == nil || !strings.Contains(err.Error(), "missing-reg") {
		t.Errorf("got %v, want 'missing-reg' not-found error", err)
	}
}

func TestRegistrySync_NameNotCloned(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "uncloned", URL: "https://example.com/uncloned.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	err := registrySyncCmd.RunE(registrySyncCmd, []string{"uncloned"})
	if err == nil || !strings.Contains(err.Error(), "not cloned") {
		t.Errorf("got %v, want 'not cloned' error", err)
	}
}

func TestRegistryItems_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Providers: []string{"claude-code"}}
	withRegistryProjectAndCache(t, nil, cfg)

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	output.SetForTest(t)

	registryItemsCmd.SilenceUsage = true
	registryItemsCmd.SilenceErrors = true
	err := registryItemsCmd.RunE(registryItemsCmd, nil)
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if !strings.Contains(buf.String(), "No registries configured") {
		t.Errorf("expected 'No registries configured', got: %s", buf.String())
	}
}

func TestRegistryItems_NameNotFound(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "real", URL: "https://example.com/real.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registryItemsCmd.SilenceUsage = true
	registryItemsCmd.SilenceErrors = true
	err := registryItemsCmd.RunE(registryItemsCmd, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("got %v, want 'missing' not-found error", err)
	}
}

func TestRegistryItems_NameNotCloned(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "uncloned", URL: "https://example.com/uncloned.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registryItemsCmd.SilenceUsage = true
	registryItemsCmd.SilenceErrors = true
	err := registryItemsCmd.RunE(registryItemsCmd, []string{"uncloned"})
	if err == nil || !strings.Contains(err.Error(), "not cloned") {
		t.Errorf("got %v, want 'not cloned' error", err)
	}
}

func TestRegistryItems_HappyPath(t *testing.T) {
	const regName = "test-reg-cov"
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: regName, URL: "https://example.com/" + regName + ".git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)

	// Set up a fake registry clone with one skill.
	cloneDir := filepath.Join(registry.CacheDirOverride, regName)
	skillDir := filepath.Join(cloneDir, "skills", "items-canary")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: items-canary\ndescription: probe-skill\n---\n# items-canary\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte("name: "+regName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Items command prints with fmt.Printf (stdout) — capture the real os.Stdout.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	output.SetForTest(t)

	registryItemsCmd.SilenceUsage = true
	registryItemsCmd.SilenceErrors = true
	err := registryItemsCmd.RunE(registryItemsCmd, []string{regName})
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if !strings.Contains(buf.String(), "items-canary") {
		t.Errorf("expected 'items-canary' in items output, got: %s", buf.String())
	}
}

// --- helpers ---

// --- explainCmd (~93% uncovered — covers --list, lookup happy/error/JSON, missing-arg) ---

func TestExplainCmd_MissingArg(t *testing.T) {
	output.SetForTest(t)

	err := explainCmd.RunE(explainCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "no error code provided") {
		t.Errorf("got %v, want 'no error code provided' error", err)
	}
}

func TestExplainCmd_UnknownCode(t *testing.T) {
	output.SetForTest(t)

	err := explainCmd.RunE(explainCmd, []string{"NOPE_999"})
	if err == nil || !strings.Contains(err.Error(), "unknown error code") {
		t.Errorf("got %v, want 'unknown error code' error", err)
	}
}

func TestExplainCmd_KnownCode(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	err := explainCmd.RunE(explainCmd, []string{"catalog_001"})
	if err != nil {
		t.Fatalf("explain catalog_001: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Error CATALOG_001") {
		t.Errorf("output missing 'Error CATALOG_001':\n%s", out)
	}
}

func TestExplainCmd_JSONOutput(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	t.Cleanup(func() { output.JSON = false })

	err := explainCmd.RunE(explainCmd, []string{"catalog_001"})
	if err != nil {
		t.Fatalf("explain catalog_001 --json: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"code"`) || !strings.Contains(out, "CATALOG_001") {
		t.Errorf("expected JSON envelope with code field, got:\n%s", out)
	}
}

func TestExplainCmd_ListAll(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	explainListAll = true
	t.Cleanup(func() { explainListAll = false })

	err := explainCmd.RunE(explainCmd, nil)
	if err != nil {
		t.Fatalf("explain --list: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Available error codes") || !strings.Contains(out, "CATALOG_001") {
		t.Errorf("--list output missing expected lines:\n%s", out)
	}
}

func TestExplainCmd_ListAllJSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	explainListAll = true
	output.JSON = true
	t.Cleanup(func() {
		explainListAll = false
		output.JSON = false
	})

	err := explainCmd.RunE(explainCmd, nil)
	if err != nil {
		t.Fatalf("explain --list --json: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "CATALOG_001") || !strings.Contains(out, "[") {
		t.Errorf("--list --json output missing JSON array of codes:\n%s", out)
	}
}

// newMoatSignTestCmd builds a cobra command with the same flag definitions as
// moatSignCmd so tests can drive runMoatSign without colliding with the real
// command's flag state across parallel tests. Output is captured into the
// provided dir under stdout.txt / stderr.txt for debugging if needed.
func newMoatSignTestCmd(t *testing.T, _ string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "sign"}
	cmd.Flags().String("manifest", "", "")
	cmd.Flags().String("rekor-raw", "", "")
	cmd.Flags().String("out", "", "")
	cmd.Flags().String("identity", "", "")
	cmd.Flags().String("trusted-root", "", "")
	cmd.Flags().String("dev-trusted-root", "", "")
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd
}
