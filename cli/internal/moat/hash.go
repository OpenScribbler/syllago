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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const (
	chunkSize   = 65536 // 64 KiB text streaming buffer — boundary behavior is normative (TV-21).
	nulScanSize = 8192  // 8 KiB NUL-probe window — matches git's binary heuristic (TV-20).
)

// utf8BOM is the UTF-8 byte-order mark (EF BB BF). Stripped from the first
// chunk of any text file before hashing (TV-18).
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// textExtensions is the normative allowlist of extensions that receive text
// normalization. Lowercased, dot-prefixed. Any other extension — or no
// extension — is treated as binary.
var textExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true,
	".yaml": true, ".yml": true, ".json": true, ".toml": true,
	".ini": true, ".cfg": true, ".conf": true,
	".html": true, ".htm": true, ".xml": true, ".svg": true,
	".css": true, ".scss": true, ".less": true,
	".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".mjs": true, ".cjs": true,
	".py": true, ".rb": true, ".lua": true, ".rs": true, ".go": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".csv": true, ".tsv": true, ".sql": true,
	".lock": true, ".sum": true, ".mod": true,
}

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

// finalExtension returns the lowercased final extension of name.
//
//	"foo.tar.gz"  → ".gz"
//	".gitignore"  → ""    (dotfile: starts with "." and has exactly one dot)
//	"Makefile"    → ""    (no dot at all)
//	"SKILL.md"    → ".md"
func finalExtension(name string) string {
	if strings.HasPrefix(name, ".") && strings.Count(name, ".") == 1 {
		return ""
	}
	return strings.ToLower(filepath.Ext(name))
}

// isText reports whether path should be hashed as text. True iff (a) the
// final extension is in textExtensions AND (b) the first 8 KiB contain no
// NUL byte. The NUL probe guards against binary files with text-looking
// extensions (TV-20).
func isText(path string) (bool, error) {
	if !textExtensions[finalExtension(filepath.Base(path))] {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, nulScanSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	return !bytes.Contains(buf[:n], []byte{0x00}), nil
}

// hashBinary returns the SHA-256 of the raw file bytes (no normalization).
// Streaming; O(chunkSize) peak memory.
func hashBinary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashText returns the SHA-256 of path's canonical text form: UTF-8 BOM
// stripped, line endings normalized to LF. The normalization is a single
// streaming left-to-right pass with greedy CRLF matching and pending-CR
// handling across chunk boundaries. Peak memory is O(chunkSize).
//
// Normalization rules:
//
//	CR LF → LF  (including when CR ends one chunk and LF begins the next)
//	CR    → LF  (lone CR, anywhere including EOF)
//	LF    → LF  (unchanged)
//
// io.ReadFull is used so each non-final chunk is exactly chunkSize bytes,
// making the boundary behavior in TV-21 deterministic regardless of the
// filesystem's short-read behavior.
func hashText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	first := true
	pendingCR := false
	buf := make([]byte, chunkSize)

	for {
		n, readErr := io.ReadFull(f, buf)
		if n > 0 {
			chunk := buf[:n]
			if first {
				chunk = bytes.TrimPrefix(chunk, utf8BOM)
				first = false
			}
			if len(chunk) > 0 {
				normalizeChunk(h, chunk, &pendingCR)
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("reading %s: %w", path, readErr)
		}
	}

	if pendingCR {
		// Lone CR at EOF: flush as LF (TV-22).
		_, _ = h.Write([]byte{0x0A})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// normalizeChunk writes the CRLF-normalized form of chunk into h, updating
// *pendingCR for the chunk-boundary case. The output buffer is at most
// len(chunk)+1 bytes.
func normalizeChunk(h io.Writer, chunk []byte, pendingCR *bool) {
	out := make([]byte, 0, len(chunk)+1)
	i := 0

	// Resolve CR carried over from the end of the previous chunk.
	if *pendingCR {
		*pendingCR = false
		if chunk[0] == 0x0A {
			out = append(out, 0x0A) // previous CR + this LF = CRLF → LF
			i = 1
		} else {
			out = append(out, 0x0A) // previous CR was a lone CR
		}
	}

	for i < len(chunk) {
		b := chunk[i]
		if b != 0x0D {
			out = append(out, b)
			i++
			continue
		}
		// CR: emit LF now if we can see the next byte; otherwise defer.
		if i+1 < len(chunk) {
			out = append(out, 0x0A)
			if chunk[i+1] == 0x0A {
				i += 2 // CRLF consumed
			} else {
				i++ // lone CR
			}
		} else {
			*pendingCR = true
			i++
		}
	}

	_, _ = h.Write(out)
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

		isText, err := isText(path)
		if err != nil {
			return fmt.Errorf("classifying %s: %w", posixRel, err)
		}

		var fileHash string
		if isText {
			fileHash, err = hashText(path)
		} else {
			fileHash, err = hashBinary(path)
		}
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
