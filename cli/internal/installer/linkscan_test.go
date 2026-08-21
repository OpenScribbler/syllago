package installer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func linkScanProvider(slug string, dirs map[catalog.ContentType]string) provider.Provider {
	return provider.Provider{
		Name: slug,
		Slug: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			return dirs[ct]
		},
	}
}

func TestScanProviderLinks_HealthyLinkIntoRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	source := filepath.Join(root, "skills", "alpha")
	installDir := filepath.Join(tmp, "install")
	linkPath := filepath.Join(installDir, "alpha")
	mustWriteFile(t, source, "skill")
	mustSymlink(t, source, linkPath)

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1: %#v", len(links), links)
	}
	got := links[0]
	if got.Provider != "test" || got.ContentType != catalog.Skills || got.Path != linkPath || got.Target != source || got.Class != LinkHealthy {
		t.Errorf("ScannedLink = %#v", got)
	}
}

func TestScanProviderLinks_BrokenLinkIntoRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	target := filepath.Join(root, "missing")
	installDir := filepath.Join(tmp, "install")
	linkPath := filepath.Join(installDir, "missing")
	mustSymlink(t, target, linkPath)

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1: %#v", len(links), links)
	}
	if links[0].Class != LinkBroken {
		t.Errorf("Class = %q, want %q", links[0].Class, LinkBroken)
	}
}

func TestScanProviderLinks_OutsideRootExcluded(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	outside := filepath.Join(tmp, "outside", "skill")
	installDir := filepath.Join(tmp, "install")
	mustWriteFile(t, outside, "outside")
	mustSymlink(t, outside, filepath.Join(installDir, "outside"))

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 0 {
		t.Fatalf("len(links) = %d, want 0: %#v", len(links), links)
	}
}

func TestScanProviderLinks_RegularFileAndDirectoryExcluded(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	installDir := filepath.Join(tmp, "install")
	mustWriteFile(t, filepath.Join(installDir, "file"), "plain")
	if err := os.MkdirAll(filepath.Join(installDir, "dir"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 0 {
		t.Fatalf("len(links) = %d, want 0: %#v", len(links), links)
	}
}

func TestScanProviderLinks_RelativeTargetResolvingIntoRootIncluded(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	source := filepath.Join(root, "skills", "relative")
	installDir := filepath.Join(tmp, "provider", "skills")
	linkPath := filepath.Join(installDir, "relative")
	mustWriteFile(t, source, "skill")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	rel, err := filepath.Rel(installDir, source)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if err := os.Symlink(rel, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1: %#v", len(links), links)
	}
	if links[0].Target != source {
		t.Errorf("Target = %q, want %q", links[0].Target, source)
	}
}

func TestScanProviderLinks_NonexistentInstallDirSkipped(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("test", map[catalog.ContentType]string{catalog.Skills: filepath.Join(tmp, "missing")}),
	}, tmp, []string{filepath.Join(tmp, "root")})

	if len(links) != 0 {
		t.Fatalf("len(links) = %d, want 0: %#v", len(links), links)
	}
}

func TestScanProviderLinks_DedupesSharedDirectory(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	source := filepath.Join(root, "skills", "shared")
	installDir := filepath.Join(tmp, "install")
	linkPath := filepath.Join(installDir, "shared")
	mustWriteFile(t, source, "skill")
	mustSymlink(t, source, linkPath)

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("first", map[catalog.ContentType]string{catalog.Skills: installDir}),
		linkScanProvider("second", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, tmp, []string{root})

	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1: %#v", len(links), links)
	}
	if links[0].Provider != "first" {
		t.Errorf("Provider = %q, want first", links[0].Provider)
	}
}

func TestScanProviderLinks_SortedDeterministically(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	alphaDir := filepath.Join(tmp, "alpha")
	zetaDir := filepath.Join(tmp, "zeta")
	mustWriteFile(t, filepath.Join(root, "rules", "a"), "a")
	mustWriteFile(t, filepath.Join(root, "rules", "b"), "b")
	mustWriteFile(t, filepath.Join(root, "skills", "c"), "c")
	mustSymlink(t, filepath.Join(root, "rules", "b"), filepath.Join(alphaDir, "b"))
	mustSymlink(t, filepath.Join(root, "rules", "a"), filepath.Join(alphaDir, "a"))
	mustSymlink(t, filepath.Join(root, "skills", "c"), filepath.Join(zetaDir, "c"))

	links := ScanProviderLinks([]provider.Provider{
		linkScanProvider("zeta", map[catalog.ContentType]string{catalog.Skills: zetaDir}),
		linkScanProvider("alpha", map[catalog.ContentType]string{catalog.Rules: alphaDir}),
	}, tmp, []string{root})

	var got []string
	for _, link := range links {
		got = append(got, link.Provider+"/"+string(link.ContentType)+"/"+filepath.Base(link.Path))
	}
	want := []string{"alpha/rules/a", "alpha/rules/b", "zeta/skills/c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestPlanLinkFixes_BrokenSkillWithMatchingLibraryRelinks(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	source := filepath.Join(tmp, "content", "skills", "repair-me")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	broken := []ScannedLink{{
		ContentType: catalog.Skills,
		Path:        filepath.Join(tmp, "install", "repair-me"),
		Target:      filepath.Join(tmp, "root", "gone"),
		Class:       LinkBroken,
	}}

	actions := PlanLinkFixes(broken, []catalog.ContentItem{{
		Name:   "repair-me",
		Type:   catalog.Skills,
		Path:   source,
		Source: "global",
	}})

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1: %#v", len(actions), actions)
	}
	if actions[0].Kind != FixRelink || actions[0].NewSource != source {
		t.Fatalf("action = %#v, want relink to %q", actions[0], source)
	}
}

func TestPlanLinkFixes_BrokenAgentWithMatchingLibraryRelinksToAgentFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	itemDir := filepath.Join(tmp, "content", "agents", "foo")
	source := filepath.Join(itemDir, "AGENT.md")
	mustWriteFile(t, source, "agent")
	broken := []ScannedLink{{
		ContentType: catalog.Agents,
		Path:        filepath.Join(tmp, "install", "foo.md"),
		Target:      filepath.Join(tmp, "root", "gone.md"),
		Class:       LinkBroken,
	}}

	actions := PlanLinkFixes(broken, []catalog.ContentItem{{
		Name:   "foo",
		Type:   catalog.Agents,
		Path:   itemDir,
		Source: "global",
	}})

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1: %#v", len(actions), actions)
	}
	if actions[0].Kind != FixRelink || actions[0].NewSource != source {
		t.Fatalf("action = %#v, want relink to %q", actions[0], source)
	}
}

func TestPlanLinkFixes_NoLibraryMatchPrunes(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	actions := PlanLinkFixes([]ScannedLink{{
		ContentType: catalog.Skills,
		Path:        filepath.Join(tmp, "install", "gone"),
		Target:      filepath.Join(tmp, "root", "gone"),
		Class:       LinkBroken,
	}}, nil)

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}
	if actions[0].Kind != FixPrune {
		t.Fatalf("Kind = %q, want %q", actions[0].Kind, FixPrune)
	}
}

func TestPlanLinkFixes_MissingLibrarySourcePrunes(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	missingSource := filepath.Join(tmp, "content", "skills", "gone")
	actions := PlanLinkFixes([]ScannedLink{{
		ContentType: catalog.Skills,
		Path:        filepath.Join(tmp, "install", "gone"),
		Target:      filepath.Join(tmp, "root", "gone"),
		Class:       LinkBroken,
	}}, []catalog.ContentItem{{
		Name:   "gone",
		Type:   catalog.Skills,
		Path:   missingSource,
		Source: "global",
	}})

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}
	if actions[0].Kind != FixPrune {
		t.Fatalf("Kind = %q, want %q", actions[0].Kind, FixPrune)
	}
}

func TestApplyLinkFixes_RelinkRewritesSymlinkTarget(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	linkPath := filepath.Join(tmp, "install", "repair-me")
	oldTarget := filepath.Join(tmp, "root", "gone")
	newSource := filepath.Join(tmp, "content", "skills", "repair-me")
	mustWriteFile(t, newSource, "skill")
	mustSymlink(t, oldTarget, linkPath)

	errs := ApplyLinkFixes([]FixAction{{
		Kind:      FixRelink,
		Link:      ScannedLink{Path: linkPath, Target: oldTarget},
		NewSource: newSource,
	}})
	if len(errs) != 0 {
		t.Fatalf("ApplyLinkFixes errors = %v", errs)
	}

	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if got != newSource {
		t.Fatalf("target = %q, want %q", got, newSource)
	}
}

func TestApplyLinkFixes_PruneRemovesLink(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	linkPath := filepath.Join(tmp, "install", "gone")
	oldTarget := filepath.Join(tmp, "root", "gone")
	mustSymlink(t, oldTarget, linkPath)

	errs := ApplyLinkFixes([]FixAction{{
		Kind: FixPrune,
		Link: ScannedLink{Path: linkPath, Target: oldTarget},
	}})
	if len(errs) != 0 {
		t.Fatalf("ApplyLinkFixes errors = %v", errs)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("Lstat err = %v, want not exist", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func mustSymlink(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
}
