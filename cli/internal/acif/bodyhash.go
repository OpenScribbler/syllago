package acif

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/moathash"
	"golang.org/x/text/unicode/norm"
)

type BodyResult struct {
	Classification string
	HashHex        string
}

type bodyHashOptions struct {
	entryOverride []byte
}

type bodyFile struct {
	abs                 string
	rel                 string
	rootLicenseOrReadme bool
}

var acifVCSDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true, ".bzr": true,
	"_darcs": true, ".fossil": true,
}

// BodyHash computes body_hash for a frontmatter-bearing content type.
func BodyHash(bodyRoot, entryFile string) (*BodyResult, error) {
	return bodyHash(bodyRoot, entryFile, bodyHashOptions{})
}

func BodyHashWithEntryBytes(bodyRoot, entryFile string, entryContent []byte) (*BodyResult, error) {
	return bodyHash(bodyRoot, entryFile, bodyHashOptions{entryOverride: entryContent})
}

func bodyHash(bodyRoot, entryFile string, opts bodyHashOptions) (*BodyResult, error) {
	absRoot, err := filepath.Abs(bodyRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving body root: %w", err)
	}
	entryRel := filepath.ToSlash(filepath.Clean(entryFile))

	var files []bodyFile
	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		posixRel := filepath.ToSlash(rel)

		if d.Type()&fs.ModeSymlink != 0 {
			return &RejectError{ID: ErrBodySymlink, Detail: posixRel}
		}
		if d.IsDir() {
			if acifVCSDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		rootFile := filepath.Dir(rel) == "."
		if rootFile && d.Name() == "acif-sidecar.yaml" {
			return nil
		}

		files = append(files, bodyFile{
			abs:                 path,
			rel:                 posixRel,
			rootLicenseOrReadme: rootFile && isRootLicenseOrReadme(d.Name()),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	var contentCandidates []bodyFile
	for _, f := range files {
		if !f.rootLicenseOrReadme {
			contentCandidates = append(contentCandidates, f)
		}
	}
	if len(contentCandidates) == 0 {
		return nil, &RejectError{ID: ErrBodyEmpty, Detail: bodyRoot}
	}

	if len(contentCandidates) == 1 && contentCandidates[0].rel == entryRel {
		hash, err := hashEntryFile(contentCandidates[0].abs, opts.entryOverride)
		if err != nil {
			return nil, err
		}
		return &BodyResult{Classification: "single-file", HashHex: hash}, nil
	}

	hash, err := multiFileBodyHash(files, entryRel, opts.entryOverride)
	if err != nil {
		return nil, err
	}
	return &BodyResult{Classification: "multi-file", HashHex: hash}, nil
}

func isRootLicenseOrReadme(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "README")
}

func hashEntryFile(path string, override []byte) (string, error) {
	if override != nil {
		return sha256Hex(override), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading entry file: %w", err)
	}
	return sha256Hex(stripFrontmatter(moathash.CanonicalText(data))), nil
}

func multiFileBodyHash(files []bodyFile, entryRel string, entryOverride []byte) (string, error) {
	type manifestEntry struct {
		rel  string
		hash string
	}
	entries := make([]manifestEntry, 0, len(files))
	seen := make(map[string]string, len(files))

	for _, f := range files {
		nfcRel := norm.NFC.String(f.rel)
		if prior, ok := seen[nfcRel]; ok && prior != f.rel {
			return "", &RejectError{
				ID:     ErrBodyPathCollision,
				Detail: fmt.Sprintf("%q and %q normalize to %q", prior, f.rel, nfcRel),
			}
		}
		seen[nfcRel] = f.rel

		var fileHash string
		var err error
		if f.rel == entryRel {
			fileHash, err = hashEntryFile(f.abs, entryOverride)
		} else {
			fileHash, err = moathash.FileHash(f.abs)
		}
		if err != nil {
			return "", fmt.Errorf("hashing %s: %w", f.rel, err)
		}
		entries = append(entries, manifestEntry{rel: nfcRel, hash: fileHash})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].rel < entries[j].rel
	})

	var manifest strings.Builder
	for _, e := range entries {
		manifest.WriteString(e.hash)
		manifest.WriteString("  ")
		manifest.WriteString(e.rel)
		manifest.WriteByte('\n')
	}
	return sha256Hex([]byte(manifest.String())), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func stripFrontmatter(text []byte) []byte {
	if !bytes.HasPrefix(text, []byte("---\n")) {
		return text
	}
	const closing = "\n---\n"
	if idx := bytes.Index(text[3:], []byte(closing)); idx >= 0 {
		start := idx + 3
		return text[start+len(closing):]
	}
	if bytes.HasSuffix(text, []byte("\n---")) {
		return nil
	}
	return text
}
