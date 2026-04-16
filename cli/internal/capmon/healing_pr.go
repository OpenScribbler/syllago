package capmon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// healPRLabel is applied to every auto-opened heal PR. Used by
// FindOpenCapmonHealPR for dedup and by reviewers to filter triage.
const healPRLabel = "capmon-heal"

// HealPRInputs bundles everything needed to open a heal PR. Kept as a
// single struct so the callsite (pipeline.go Stage 1) doesn't need to
// thread six parameters.
type HealPRInputs struct {
	ManifestPath string // absolute path to docs/provider-sources/<slug>.yaml
	Provider     string // sanitized slug
	ContentType  string // e.g. "skills"
	SourceIndex  int    // index into content_types.<ct>.sources
	RunID        string // unique per pipeline run (short SHA or timestamp)
	OldURL       string
	Heal         HealResult
}

// UpdateManifestURL opens the given manifest YAML, replaces the URL at
// content_types.<contentType>.sources[sourceIndex] with newURL, and
// writes the file back preserving comments and formatting via the
// yaml.v3 Node API.
//
// Returns an error if the target node is not found or the old URL at
// that location does not match expectedOldURL (defensive check against
// the manifest having been edited between fetch and heal).
func UpdateManifestURL(manifestPath, contentType string, sourceIndex int, expectedOldURL, newURL string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("manifest root is not a document")
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("manifest top-level is not a mapping")
	}

	ctTypes := findMappingValue(doc, "content_types")
	if ctTypes == nil {
		return fmt.Errorf("content_types key missing")
	}
	if ctTypes.Kind != yaml.MappingNode {
		return fmt.Errorf("content_types is not a mapping")
	}
	ctNode := findMappingValue(ctTypes, contentType)
	if ctNode == nil {
		return fmt.Errorf("content_types.%s missing", contentType)
	}
	sources := findMappingValue(ctNode, "sources")
	if sources == nil || sources.Kind != yaml.SequenceNode {
		return fmt.Errorf("content_types.%s.sources missing or not a sequence", contentType)
	}
	if sourceIndex < 0 || sourceIndex >= len(sources.Content) {
		return fmt.Errorf("source index %d out of range (have %d)", sourceIndex, len(sources.Content))
	}
	src := sources.Content[sourceIndex]
	if src.Kind != yaml.MappingNode {
		return fmt.Errorf("source[%d] is not a mapping", sourceIndex)
	}
	urlNode := findMappingValue(src, "url")
	if urlNode == nil {
		return fmt.Errorf("source[%d].url missing", sourceIndex)
	}
	if urlNode.Value != expectedOldURL {
		return fmt.Errorf("source[%d].url mismatch: manifest has %q, expected %q (manifest changed since heal?)", sourceIndex, urlNode.Value, expectedOldURL)
	}
	urlNode.Value = newURL

	out, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, out, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// findMappingValue returns the value Node for the given key in a mapping
// Node, or nil if the key is absent. Mapping Nodes store [k1, v1, k2, v2, ...]
// pairs in Content.
func findMappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// HealPRAnchor returns the hidden HTML comment used to deduplicate heal
// PRs and issues. The anchor is embedded in the PR body and searched by
// FindOpenCapmonHealPR.
func HealPRAnchor(provider, contentType string, sourceIndex int) string {
	return fmt.Sprintf("<!-- capmon-heal: %s/%s/%d -->", provider, contentType, sourceIndex)
}

// BuildHealPRBody renders the PR body for a heal. The body is deliberately
// terse: reviewers need to see the old URL, the new URL, and the proof.
// Everything else (logs, audit trail) goes in the run artifact.
func BuildHealPRBody(in HealPRInputs) string {
	anchor := HealPRAnchor(in.Provider, in.ContentType, in.SourceIndex)
	var b strings.Builder
	b.WriteString(anchor)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "# capmon heal: %s/%s source[%d]\n\n", in.Provider, in.ContentType, in.SourceIndex)
	fmt.Fprintf(&b, "Run ID: %s\n\n", in.RunID)
	fmt.Fprintf(&b, "The pipeline fetched the old URL, got a failure, and found a replacement using the **%s** strategy.\n\n", in.Heal.Strategy)
	b.WriteString("| | |\n")
	b.WriteString("|---|---|\n")
	fmt.Fprintf(&b, "| Old URL | <%s> |\n", in.OldURL)
	fmt.Fprintf(&b, "| New URL | <%s> |\n", in.Heal.NewURL)
	fmt.Fprintf(&b, "| Strategy | `%s` |\n", in.Heal.Strategy)
	fmt.Fprintf(&b, "| Proof | %s |\n\n", in.Heal.Proof)
	if len(in.Heal.TriedURLs) > 1 {
		b.WriteString("<details><summary>All candidates probed</summary>\n\n")
		for _, u := range in.Heal.TriedURLs {
			fmt.Fprintf(&b, "- <%s>\n", u)
		}
		b.WriteString("\n</details>\n\n")
	}
	b.WriteString("---\n")
	b.WriteString("Auto-opened by capmon. **The healed URL must be reviewed before merge** — ")
	b.WriteString("content passed syllago's readability gate (min body size, text content-type, same-host) ")
	b.WriteString("but a human should confirm this is the correct replacement for the originating source.\n")
	return b.String()
}

// FindOpenCapmonHealPR searches for an open PR with label=capmon-heal whose
// body contains the anchor for (provider, contentType, sourceIndex).
// Returns the PR URL and true when a match exists.
func FindOpenCapmonHealPR(provider, contentType string, sourceIndex int) (string, bool, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return "", false, err
	}
	anchor := HealPRAnchor(slug, contentType, sourceIndex)
	out, err := ghRunner("pr", "list",
		"--label", healPRLabel,
		"--state", "open",
		"--json", "url,body",
	)
	if err != nil {
		return "", false, fmt.Errorf("gh pr list: %w", err)
	}
	var prs []struct {
		URL  string `json:"url"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", false, fmt.Errorf("parse pr list: %w", err)
	}
	for _, p := range prs {
		if strings.Contains(p.Body, anchor) {
			return p.URL, true, nil
		}
	}
	return "", false, nil
}

// gitRunner executes git commands and returns combined stdout/stderr.
// Overridable for tests so the PR flow can be exercised without a real
// repository. The default implementation uses exec.Command.
var gitRunner = func(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

// SetGitRunnerForTest swaps the git runner. Pass nil to restore default.
func SetGitRunnerForTest(fn func(dir string, args ...string) ([]byte, error)) {
	if fn == nil {
		gitRunner = func(dir string, args ...string) ([]byte, error) {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			err := cmd.Run()
			return buf.Bytes(), err
		}
		return
	}
	gitRunner = fn
}

// ProposeManifestHealPR updates the manifest, opens a branch, pushes it,
// and creates a PR tagged capmon-heal. Idempotent: if a matching PR is
// already open for this (provider, contentType, sourceIndex), the
// existing PR URL is returned and no new work is done.
//
// The repoDir must be the root of the syllago repo (the directory
// containing docs/provider-sources/). The manifestPath in HealPRInputs
// is expected to be absolute.
func ProposeManifestHealPR(ctx context.Context, repoDir string, in HealPRInputs) (string, error) {
	slug, err := SanitizeSlug(in.Provider)
	if err != nil {
		return "", err
	}

	// Dedup before doing any work — no sense branching if a heal PR already
	// exists waiting for review.
	if url, found, err := FindOpenCapmonHealPR(slug, in.ContentType, in.SourceIndex); err != nil {
		return "", fmt.Errorf("dedup check: %w", err)
	} else if found {
		return url, nil
	}

	branch := fmt.Sprintf("capmon/heal-%s/%s/%s", slug, in.ContentType, in.RunID)
	if _, err := gitRunner(repoDir, "checkout", "-b", branch); err != nil {
		return "", fmt.Errorf("git checkout -b %s: %w", branch, err)
	}

	if err := UpdateManifestURL(in.ManifestPath, in.ContentType, in.SourceIndex, in.OldURL, in.Heal.NewURL); err != nil {
		return "", fmt.Errorf("update manifest: %w", err)
	}

	if _, err := gitRunner(repoDir, "add", in.ManifestPath); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	commitMsg := fmt.Sprintf("capmon: heal %s/%s source[%d] via %s", slug, in.ContentType, in.SourceIndex, in.Heal.Strategy)
	if _, err := gitRunner(repoDir, "commit", "-m", commitMsg); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	if _, err := gitRunner(repoDir, "push", "-u", "origin", branch); err != nil {
		return "", fmt.Errorf("git push: %w", err)
	}

	title := fmt.Sprintf("capmon heal: %s/%s via %s", slug, in.ContentType, in.Heal.Strategy)
	body := BuildHealPRBody(in)
	out, err := ghRunner("pr", "create",
		"--title", title,
		"--head", branch,
		"--label", healPRLabel,
		"--label", "provider:"+slug,
		"--body", body,
	)
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
