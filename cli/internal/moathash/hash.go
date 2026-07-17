package moathash

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	chunkSize   = 65536
	nulScanSize = 8192
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

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

func finalExtension(name string) string {
	if strings.HasPrefix(name, ".") && strings.Count(name, ".") == 1 {
		return ""
	}
	return strings.ToLower(filepath.Ext(name))
}

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
		_, _ = h.Write([]byte{0x0A})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizeChunk(h io.Writer, chunk []byte, pendingCR *bool) {
	out := make([]byte, 0, len(chunk)+1)
	i := 0

	if *pendingCR {
		*pendingCR = false
		if chunk[0] == 0x0A {
			out = append(out, 0x0A)
			i = 1
		} else {
			out = append(out, 0x0A)
		}
	}

	for i < len(chunk) {
		b := chunk[i]
		if b != 0x0D {
			out = append(out, b)
			i++
			continue
		}
		if i+1 < len(chunk) {
			out = append(out, 0x0A)
			if chunk[i+1] == 0x0A {
				i += 2
			} else {
				i++
			}
		} else {
			*pendingCR = true
			i++
		}
	}

	_, _ = h.Write(out)
}

// FileHash returns the MOAT file-level hash as lowercase hex without an
// algorithm prefix.
func FileHash(path string) (string, error) {
	text, err := isText(path)
	if err != nil {
		return "", err
	}
	if text {
		return hashText(path)
	}
	return hashBinary(path)
}

// CanonicalText returns the bytes-level canonical text form used by MOAT.
func CanonicalText(data []byte) []byte {
	chunk := bytes.TrimPrefix(data, utf8BOM)
	var out bytes.Buffer
	pendingCR := false
	if len(chunk) > 0 {
		normalizeChunk(&out, chunk, &pendingCR)
	}
	if pendingCR {
		_ = out.WriteByte(0x0A)
	}
	return out.Bytes()
}
