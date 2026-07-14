package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"golang.org/x/text/unicode/norm"
)

const hookSHA256Prefix = "sha256:"

func HookBodyHash(canonical map[string]any, bodyRoot string) (string, bool, error) {
	paths := referencedHookFiles(canonical)
	if bodyRoot == "" && len(paths) > 0 {
		return "", false, nil
	}

	manifest, err := hookReferencedFileManifest(paths, bodyRoot)
	if err != nil {
		return "", false, err
	}

	raw, err := json.Marshal(unwrapHookBlock(canonical))
	if err != nil {
		return "", false, err
	}
	wire, err := CanonicalJSON(raw)
	if err != nil {
		return "", false, err
	}

	digestHeader := hookSHA256Prefix + sha256HexLocal(manifest)
	preimage := make([]byte, 0, len(digestHeader)+1+len(wire)+1)
	preimage = append(preimage, digestHeader...)
	preimage = append(preimage, '\n')
	preimage = append(preimage, wire...)
	preimage = append(preimage, '\n')
	return sha256HexLocal(preimage), true, nil
}

func referencedHookFiles(canonical map[string]any) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, handler := range hookHandlers(canonical) {
		if handler["type"] != "command" {
			continue
		}
		for _, script := range hookScripts(handler) {
			if script["type"] == "file" {
				if path, ok := script["path"].(string); ok && !seen[path] {
					seen[path] = true
					paths = append(paths, path)
				}
			}
		}
	}
	for _, item := range anySlice(unwrapHookBlock(canonical)["auxiliary_files"]) {
		aux, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if path, ok := aux["path"].(string); ok && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func hookReferencedFileManifest(paths []string, bodyRoot string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	type entry struct {
		key  string
		hash string
	}
	entries := make([]entry, 0, len(paths))
	seenKeys := make(map[string]string, len(paths))
	for _, path := range paths {
		if bodyRoot == "" {
			return nil, hookReject(ErrHookScriptFileMissing, path)
		}
		abs := filepath.Join(bodyRoot, filepath.FromSlash(path))
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			return nil, hookReject(ErrHookScriptFileMissing, path)
		}
		key := norm.NFC.String(path)
		if prior, ok := seenKeys[key]; ok && prior != path {
			return nil, fmt.Errorf("hook path collision: %q and %q normalize to %q", prior, path, key)
		}
		seenKeys[key] = path
		fileHash, err := moat.FileHash(abs)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{key: key, hash: fileHash})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	var manifest strings.Builder
	for _, entry := range entries {
		manifest.WriteString(entry.hash)
		manifest.WriteString("  ")
		manifest.WriteString(entry.key)
		manifest.WriteByte('\n')
	}
	return []byte(manifest.String()), nil
}

func sha256HexLocal(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hookHandlers(block map[string]any) []map[string]any {
	var out []map[string]any
	for _, raw := range anySlice(unwrapHookBlock(block)["handlers"]) {
		if handler, ok := raw.(map[string]any); ok {
			out = append(out, handler)
		}
	}
	return out
}

func hookScripts(handler map[string]any) []map[string]any {
	var out []map[string]any
	for _, raw := range anySlice(handler["scripts"]) {
		if script, ok := raw.(map[string]any); ok {
			out = append(out, script)
		}
	}
	return out
}

func anySlice(raw any) []any {
	if items, ok := raw.([]any); ok {
		return items
	}
	return nil
}
