package analyzer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// knownContentExts are extensions treated as resolvable content references.
var knownContentExts = map[string]bool{
	".md": true, ".json": true, ".yaml": true, ".yml": true,
	".ts": true, ".py": true, ".sh": true, ".js": true,
}

// mdLinkRe matches markdown relative links: [text](../path)
var mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)#]+)\)`)

// backtickPathRe matches backtick-quoted strings that look like file paths.
var backtickPathRe = regexp.MustCompile("`([^`]+\\.[a-zA-Z]{1,5})`")

// knownSubdirs are subdirectory names that indicate supporting content.
var knownSubdirs = []string{"references", "scripts", "helpers", "assets"}

const maxRefDepth = 3

// ResolveReferences finds files referenced by a content item.
// path is the item's primary file path (relative to repoRoot).
// repoRoot must be filepath.EvalSymlinks-resolved.
// Returns relative paths of referenced files that exist within repoRoot.
func ResolveReferences(path string, repoRoot string) []string {
	return resolveRefs(path, repoRoot, 0, make(map[string]bool))
}

func resolveRefs(path, repoRoot string, depth int, visited map[string]bool) []string {
	if depth >= maxRefDepth || visited[path] {
		return nil
	}
	visited[path] = true

	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil {
		return nil
	}

	dir := filepath.Dir(path)
	var refs []string

	// Check known subdirs (for skills/agents with supporting content).
	itemDir := filepath.Dir(absPath)
	for _, sub := range knownSubdirs {
		subDir := filepath.Join(itemDir, sub)
		if info, err := os.Stat(subDir); err == nil && info.IsDir() {
			filepath.WalkDir(subDir, func(p string, d os.DirEntry, e error) error {
				if e != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(repoRoot, p)
				refs = append(refs, rel)
				return nil
			})
		}
	}

	// Parse markdown links.
	for _, match := range mdLinkRe.FindAllStringSubmatch(string(data), -1) {
		linkPath := match[2]
		if strings.HasPrefix(linkPath, "http://") || strings.HasPrefix(linkPath, "https://") {
			continue
		}
		resolved := filepath.Clean(filepath.Join(dir, linkPath))
		absResolved := filepath.Join(repoRoot, resolved)
		if !strings.HasPrefix(absResolved, repoRoot) {
			continue // boundary check
		}
		if _, err := os.Stat(absResolved); err == nil {
			refs = append(refs, resolved)
		}
	}

	// Parse backtick paths.
	for _, match := range backtickPathRe.FindAllStringSubmatch(string(data), -1) {
		candidate := match[1]
		ext := filepath.Ext(candidate)
		if !knownContentExts[strings.ToLower(ext)] {
			continue
		}
		if strings.ContainsAny(candidate, " \t\"'<>|") {
			continue
		}
		resolved := filepath.Clean(filepath.Join(dir, candidate))
		absResolved := filepath.Join(repoRoot, resolved)
		if !strings.HasPrefix(absResolved, repoRoot) {
			continue
		}
		if _, err := os.Stat(absResolved); err == nil {
			refs = append(refs, resolved)
		}
	}

	return uniqueStrings(refs)
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
