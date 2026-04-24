package rulestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRawHashFormatting enforces the D11 hash-format invariant: the
// canonical "<algo>:<hex>" string is the only thing code passes around.
// Anywhere code touches the filesystem layer it must go through
// hashToFilename / filenameToHash. This test grep-fails the build on the
// two common bug patterns:
//
//	(a) "sha256-" outside rulestore/hash.go (ad-hoc filename construction)
//	(b) `"sha256:" + `  outside rulestore/loader.go (ad-hoc canonical string)
func TestNoRawHashFormatting(t *testing.T) {
	root, err := filepath.Abs("../..") // cli/
	if err != nil {
		t.Fatal(err)
	}
	allowedFilenameConstruction := map[string]bool{
		filepath.Join(root, "internal/rulestore/hash.go"): true,
	}
	allowedConcat := map[string]bool{
		// hash.go owns HashBody — the single sanctioned library-rule entry
		// point that constructs the canonical "sha256:<hex>" form.
		filepath.Join(root, "internal/rulestore/hash.go"): true,
		// Pre-existing unrelated hash constructions outside the library-rule
		// storage domain. The D11 lint intent is to funnel library-rule hash
		// construction through rulestore; these callers hash MOAT artifacts
		// and capmon cache keys, neither of which touch .history/ filenames.
		filepath.Join(root, "cmd/syllago/install_moat_fetch.go"): true,
		filepath.Join(root, "internal/capmon/cache.go"):          true,
		filepath.Join(root, "internal/moat/hash.go"):             true,
	}
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		s := string(data)
		if strings.Contains(s, `"sha256-"`) && !allowedFilenameConstruction[p] {
			t.Errorf("%s: forbidden raw filename construction `\"sha256-\"` — use hashToFilename", p)
		}
		if strings.Contains(s, `"sha256:" +`) && !allowedConcat[p] {
			t.Errorf("%s: forbidden raw concat `\"sha256:\" +` — canonical hash construction belongs only in rulestore/loader.go", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
