package regdiff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type gitPathStatus struct {
	path   string
	status byte
}

type itemKey struct {
	Type string
	Name string
}

type itemAccum struct {
	key      itemKey
	dir      string
	paths    []string
	statuses []byte
}

const logExcerptItemLimit = 20

// GitDiff computes the item-level change set between two commits of a git
// registry checkout at repoDir. items describes the CURRENT checkout's
// items; knownTypeDirs lists top-level content-type directory names
// (e.g. "skills", "rules") used to attribute paths of items that no
// longer exist in the current checkout.
func GitDiff(registry, repoDir, oldHead, newHead string, items []ItemRef, knownTypeDirs []string) (Diff, error) {
	diff := Diff{
		Registry: registry,
		OldRef:   oldHead,
		NewRef:   newHead,
		UpToDate: oldHead == newHead && oldHead != "",
	}
	if oldHead == "" || oldHead == newHead {
		return diff, nil
	}

	out, err := runGit(repoDir, "diff", "--name-status", "--no-renames", oldHead, newHead)
	if err != nil {
		return diff, fmt.Errorf("git diff %s..%s: %w", oldHead, newHead, err)
	}

	knownTypes := make(map[string]bool, len(knownTypeDirs))
	for _, dir := range knownTypeDirs {
		knownTypes[dir] = true
	}

	current := make(map[itemKey]*itemAccum)
	fallback := make(map[itemKey]*itemAccum)
	changeDirs := make(map[itemKey]string)
	var otherPaths []string

	for _, ps := range parseNameStatus(out) {
		if item, ok := longestItemMatch(ps.path, items); ok {
			key := itemKey{Type: item.Type, Name: item.Name}
			acc := current[key]
			if acc == nil {
				acc = &itemAccum{key: key, dir: item.Dir}
				current[key] = acc
			}
			acc.paths = append(acc.paths, ps.path)
			acc.statuses = append(acc.statuses, ps.status)
			continue
		}

		if key, dir, ok := fallbackItem(ps.path, knownTypes); ok {
			acc := fallback[key]
			if acc == nil {
				acc = &itemAccum{key: key, dir: dir}
				fallback[key] = acc
			}
			acc.paths = append(acc.paths, ps.path)
			acc.statuses = append(acc.statuses, ps.status)
			continue
		}

		otherPaths = append(otherPaths, ps.path)
	}

	for _, acc := range current {
		kind := KindModified
		if allStatus(acc.statuses, 'A') {
			exists, err := gitDirExists(repoDir, oldHead, acc.dir)
			if err != nil {
				return diff, err
			}
			if !exists {
				kind = KindAdded
			}
		}
		diff.Changes = append(diff.Changes, itemChangeFromAccum(acc, kind))
		changeDirs[acc.key] = acc.dir
	}

	for _, acc := range fallback {
		kind := KindModified
		if allStatus(acc.statuses, 'D') {
			exists, err := gitDirExists(repoDir, newHead, acc.dir)
			if err != nil {
				return diff, err
			}
			if !exists {
				kind = KindRemoved
			}
		}
		diff.Changes = append(diff.Changes, itemChangeFromAccum(acc, kind))
		changeDirs[acc.key] = acc.dir
	}

	sort.Slice(diff.Changes, func(i, j int) bool {
		if diff.Changes[i].Type != diff.Changes[j].Type {
			return diff.Changes[i].Type < diff.Changes[j].Type
		}
		return diff.Changes[i].Name < diff.Changes[j].Name
	})
	sort.Strings(otherPaths)
	diff.OtherPaths = otherPaths
	if len(diff.Changes) <= logExcerptItemLimit {
		populateGitLogLines(repoDir, oldHead, newHead, diff.Changes, changeDirs)
	}

	return diff, nil
}

func populateGitLogLines(repoDir, oldHead, newHead string, changes []ItemChange, changeDirs map[itemKey]string) {
	revRange := oldHead + ".." + newHead
	for i := range changes {
		dir := changeDirs[itemKey{Type: changes[i].Type, Name: changes[i].Name}]
		if dir == "" {
			continue
		}
		out, err := runGit(repoDir, "log", "--no-merges", "--format=%s", "-n", "3", revRange, "--", dir)
		if err != nil {
			continue
		}
		lines := splitNonEmptyLines(out)
		if len(lines) > 0 {
			changes[i].LogLines = lines
		}
	}
}

func splitNonEmptyLines(out []byte) []string {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func parseNameStatus(out []byte) []gitPathStatus {
	var parsed []gitPathStatus
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		statusText, pathText, ok := strings.Cut(line, "\t")
		if !ok || statusText == "" || pathText == "" {
			continue
		}
		status := statusText[0]
		if status != 'A' && status != 'M' && status != 'D' {
			status = 'M'
		}
		parsed = append(parsed, gitPathStatus{path: pathText, status: status})
	}
	return parsed
}

func longestItemMatch(changedPath string, items []ItemRef) (ItemRef, bool) {
	var matched ItemRef
	longest := -1
	for _, item := range items {
		dir := cleanRelDir(item.Dir)
		if dir == "" {
			continue
		}
		if changedPath == dir || strings.HasPrefix(changedPath, dir+"/") {
			if len(dir) > longest {
				matched = item
				matched.Dir = dir
				longest = len(dir)
			}
		}
	}
	return matched, longest >= 0
}

func fallbackItem(changedPath string, knownTypes map[string]bool) (itemKey, string, bool) {
	segments := strings.Split(changedPath, "/")
	if len(segments) < 2 || !knownTypes[segments[0]] {
		return itemKey{}, "", false
	}
	key := itemKey{Type: segments[0], Name: segments[1]}
	return key, segments[0] + "/" + segments[1], true
}

func itemChangeFromAccum(acc *itemAccum, kind Kind) ItemChange {
	paths := append([]string(nil), acc.paths...)
	sort.Strings(paths)
	return ItemChange{
		Type:  acc.key.Type,
		Name:  acc.key.Name,
		Kind:  kind,
		Paths: paths,
	}
}

func allStatus(statuses []byte, want byte) bool {
	if len(statuses) == 0 {
		return false
	}
	for _, status := range statuses {
		if status != want {
			return false
		}
	}
	return true
}

func gitDirExists(repoDir, ref, dir string) (bool, error) {
	out, err := runGit(repoDir, "ls-tree", "-d", ref, "--", dir)
	if err != nil {
		return false, fmt.Errorf("git ls-tree %s -- %s: %w", ref, dir, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func runGit(repoDir string, args ...string) ([]byte, error) {
	allArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", allArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func cleanRelDir(dir string) string {
	dir = strings.TrimSpace(dir)
	dir = strings.TrimPrefix(dir, "./")
	return strings.Trim(dir, "/")
}
