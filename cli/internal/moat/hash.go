package moat

// MOAT v0.6.0 content hash implementation (spec §7.3 "Directory tree content
// hash"). The normative authority is the test vector suite in TV-01..TV-22;
// this file must produce byte-identical output to those vectors.
//
// Overview:
//   1. Walk the directory, skipping VCS metadata dirs and the root-only
//      moat-attestation.json. Reject any symlink.
//   2. For each regular file: classify as text (extension allowlist + NUL
//      probe in first 8 KB) or binary. Text files are SHA-256 hashed over
//      the canonical form: UTF-8 BOM stripped, CRLF/CR normalized to LF.
//   3. NFC-normalize each relative path; reject content on collision.
//   4. Sort entries by raw UTF-8 byte order.
//   5. Build a sha256sum-format manifest ("<hash>  <path>\n" per line) and
//      return "sha256:<hex>" of its SHA-256.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/moathash"
	"golang.org/x/text/unicode/norm"
)

// vcsDirs are directory names whose contents are excluded at any depth.
// A local working copy may carry these; a registry using `git archive`
// should not. Excluding them makes local vs. archived hashes agree.
var vcsDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true, ".bzr": true,
	"_darcs": true, ".fossil": true,
}

// excludedFiles is the set of names excluded from hashing — but only at the
// root of the content directory. A file named moat-attestation.json in a
// subdirectory has no protocol meaning and MUST be included, otherwise
// attackers could hide content outside the attested hash.
var excludedFiles = map[string]bool{
	"moat-attestation.json": true,
}

// ContentHash returns the MOAT directory-tree content hash for dir, formatted
// as "sha256:<64 lowercase hex chars>".
//
// Errors:
//   - If any symlink is present anywhere in the tree (reject-all policy, no
//     resolution or exclusion — eliminates path-traversal attack surface).
//   - If two paths NFC-normalize to the same string (unpublishable collision).
//   - If the tree contains zero hashable files.
//   - Any I/O error encountered during the walk.
func ContentHash(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", dir, err)
	}

	type entry struct {
		rel  string
		hash string
	}
	var entries []entry
	seen := make(map[string]string) // NFC rel → original rel (for collision diagnostics)

	walkErr := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}

		rel, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			return relErr
		}
		posixRel := filepath.ToSlash(rel)

		// Skip the root itself.
		if rel == "." {
			return nil
		}

		// VCS metadata dir — skip the entire subtree.
		if d.IsDir() && vcsDirs[d.Name()] {
			return fs.SkipDir
		}

		// Reject all symlinks. d.Type() reports symlink bit from Lstat,
		// regardless of what the link points to.
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected: %s", posixRel)
		}

		// Only regular files contribute to the manifest.
		if !d.Type().IsRegular() {
			return nil
		}

		// Root-only exclusion (moat-attestation.json).
		if filepath.Dir(rel) == "." && excludedFiles[d.Name()] {
			return nil
		}

		fileHash, err := moathash.FileHash(path)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", posixRel, err)
		}

		nfcRel := norm.NFC.String(posixRel)
		if prior, ok := seen[nfcRel]; ok {
			return fmt.Errorf("NFC collision: %q and %q both normalize to %q — content is unpublishable",
				prior, posixRel, nfcRel)
		}
		seen[nfcRel] = posixRel

		entries = append(entries, entry{rel: nfcRel, hash: fileHash})
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no files found in %s: content is unpublishable", dir)
	}

	// Sort by raw UTF-8 byte order. Go string comparison on UTF-8 strings
	// is exactly byte order — no locale awareness.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].rel < entries[j].rel
	})

	var manifest strings.Builder
	manifest.Grow(len(entries) * 80) // heuristic: ~64 hash + 2 sep + avg path + newline
	for _, e := range entries {
		manifest.WriteString(e.hash)
		manifest.WriteString("  ")
		manifest.WriteString(e.rel)
		manifest.WriteByte('\n')
	}

	sum := sha256.Sum256([]byte(manifest.String()))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
