package analyzer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Size limits for file reads during analysis.
const (
	limitMarkdown = 1 * 1024 * 1024 // 1 MB
	limitJSON     = 256 * 1024      // 256 KB
)

// readFileLimited reads a file, returning an error if it exceeds maxBytes.
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file %s exceeds size limit (%d bytes)", path, maxBytes)
	}
	data := make([]byte, info.Size())
	_, err = f.Read(data)
	return data, err
}

// hashBytes returns the SHA-256 hex digest of data.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// basicFrontmatter holds the name and description fields extracted from YAML frontmatter.
type basicFrontmatter struct {
	name        string
	description string
}

// parseFrontmatterBasic extracts name and description from YAML frontmatter.
// Handles the "---\n...\n---" format. Returns nil if no frontmatter found.
func parseFrontmatterBasic(data []byte) *basicFrontmatter {
	s := string(data)
	if !strings.HasPrefix(strings.TrimSpace(s), "---") {
		return nil
	}
	rest := s[strings.Index(s, "---")+3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	block := rest[:end]
	fm := &basicFrontmatter{}
	scanner := bufio.NewScanner(bytes.NewBufferString(block))
	for scanner.Scan() {
		line := scanner.Text()
		if k, v, ok := strings.Cut(line, ":"); ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch k {
			case "name":
				fm.name = v
			case "description":
				fm.description = v
			}
		}
	}
	return fm
}
