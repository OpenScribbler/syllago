package capmon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ghRunner is the function used to invoke the gh CLI. Overridable in tests.
var ghRunner = func(args ...string) ([]byte, error) {
	return exec.Command("gh", args...).Output()
}

// GHRunner invokes the gh CLI with the given arguments, returning combined output.
// Exported for use by Stage 4 PR/issue creation (Task 9.2).
func GHRunner(args ...string) ([]byte, error) {
	return ghRunner(args...)
}

// SetGHCommandForTest replaces the gh runner with a test stub.
// Pass nil to restore the default.
// Must only be called from test code.
func SetGHCommandForTest(fn func(args ...string) ([]byte, error)) {
	if fn == nil {
		ghRunner = func(args ...string) ([]byte, error) {
			return exec.Command("gh", args...).Output()
		}
		return
	}
	ghRunner = fn
}

// SanitizeSlug validates a provider slug is safe for use in branch names and PR bodies.
// Applied to both branch name construction and PR body construction in Stage 4.
func SanitizeSlug(slug string) (string, error) {
	if !slugRegex.MatchString(slug) {
		return "", fmt.Errorf("invalid slug: %q", slug)
	}
	return slug, nil
}

// DeduplicatePR checks if an open PR exists for capmon/drift-<provider>.
// Returns (existingPRURL, true) if found; ("", false) if not found.
func DeduplicatePR(_ context.Context, provider string) (string, bool, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return "", false, fmt.Errorf("invalid provider slug: %w", err)
	}
	branch := "capmon/drift-" + slug
	out, err := ghRunner("pr", "list", "--label", "capmon", "--head", branch, "--json", "url")
	if err != nil {
		return "", false, fmt.Errorf("gh pr list: %w", err)
	}
	var prs []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", false, fmt.Errorf("parse gh output: %w", err)
	}
	if len(prs) == 0 {
		return "", false, nil
	}
	return prs[0].URL, true, nil
}

// failureCountFile returns the path to the consecutive-failure counter for a provider.
func failureCountFile(cacheRoot, provider string) string {
	return filepath.Join(cacheRoot, provider, "consecutive-failures.json")
}

// RecordConsecutiveFailure increments the failure counter for a provider.
// Opens a GitHub issue after the 3rd consecutive failure.
func RecordConsecutiveFailure(_ context.Context, cacheRoot, provider string) error {
	path := failureCountFile(cacheRoot, provider)
	var count int
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &count) //nolint:errcheck // count stays 0 on unmarshal error; still valid
	}
	count++
	os.MkdirAll(filepath.Dir(path), 0755) //nolint:errcheck // failure handled by subsequent WriteFile error
	data, _ := json.Marshal(count)
	os.WriteFile(path, data, 0644) //nolint:errcheck // failure counter is best-effort; issue creation still proceeds

	if count >= 3 {
		slug, _ := SanitizeSlug(provider)
		title := fmt.Sprintf("capmon: %d consecutive extraction failures for %s", count, slug)
		_, err := ghRunner("issue", "create",
			"--title", title,
			"--label", "capmon",
			"--body", fmt.Sprintf("Provider %s has failed extraction %d consecutive times. Manual intervention required.", slug, count),
		)
		return err
	}
	return nil
}

// CreateDriftPR creates a GitHub PR for field-level drift. Returns the PR URL.
// SanitizeSlug must be called on provider before reaching this function.
func CreateDriftPR(_ context.Context, provider, runID string, diff CapabilityDiff) (string, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return "", fmt.Errorf("invalid provider slug: %w", err)
	}
	branch := "capmon/drift-" + slug
	// Full git branch creation and push logic implemented in pipeline.go Stage 4
	out, err := ghRunner("pr", "create",
		"--title", fmt.Sprintf("capmon: drift detected for %s (run %s)", slug, runID),
		"--head", branch,
		"--label", "capmon",
	)
	if err != nil {
		return "", fmt.Errorf("create PR: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateStructuralIssue creates a GitHub issue for structural drift (new sections).
func CreateStructuralIssue(_ context.Context, provider, runID string, drift []string) error {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return fmt.Errorf("invalid provider slug: %w", err)
	}
	body := fmt.Sprintf("New sections detected in %s docs (run %s):\n", slug, runID)
	for _, d := range drift {
		body += "- " + d + "\n"
	}
	_, err = ghRunner("issue", "create",
		"--title", fmt.Sprintf("capmon: structural drift in %s", slug),
		"--label", "capmon",
		"--body", body,
	)
	return err
}

// FindOpenCapmonIssue searches for an open GitHub issue with the capmon-change label
// and the provider:slug label, then filters by the hidden anchor comment
// <!-- capmon-check: <provider>/<contentType> -->. Returns (issueNumber, true, nil)
// when found, or (0, false, nil) when no matching issue exists.
func FindOpenCapmonIssue(provider, contentType string) (int, bool, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return 0, false, err
	}
	anchor := fmt.Sprintf("<!-- capmon-check: %s/%s -->", slug, contentType)

	out, err := ghRunner("issue", "list",
		"--label", "capmon-change",
		"--label", "provider:"+slug,
		"--state", "open",
		"--json", "number,body",
	)
	if err != nil {
		return 0, false, fmt.Errorf("gh issue list: %w", err)
	}

	var issues []struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return 0, false, fmt.Errorf("parse issue list: %w", err)
	}

	for _, iss := range issues {
		if strings.Contains(iss.Body, anchor) {
			return iss.Number, true, nil
		}
	}
	return 0, false, nil
}

// CreateCapmonChangeIssue creates a GitHub issue for a content-change event.
// The issue body is prefixed with a hidden anchor comment
// <!-- capmon-check: <provider>/<contentType> --> for deduplication by
// FindOpenCapmonIssue. Returns the new issue number.
func CreateCapmonChangeIssue(_ context.Context, provider, contentType, title, body string) (int, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return 0, err
	}
	anchor := fmt.Sprintf("<!-- capmon-check: %s/%s -->", slug, contentType)
	fullBody := anchor + "\n" + body

	out, err := ghRunner("issue", "create",
		"--title", title,
		"--label", "capmon-change",
		"--label", "provider:"+slug,
		"--body", fullBody,
	)
	if err != nil {
		return 0, fmt.Errorf("gh issue create: %w", err)
	}

	// gh issue create prints the URL: https://github.com/owner/repo/issues/123
	issueURL := strings.TrimSpace(string(out))
	parts := strings.Split(issueURL, "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected gh issue create output: %q", issueURL)
	}
	num, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("parse issue number from %q: %w", issueURL, err)
	}
	return num, nil
}

// AppendCapmonChangeEvent appends a comment to an existing capmon issue.
// Used to record subsequent change detections on the same issue thread.
func AppendCapmonChangeEvent(_ context.Context, issueNumber int, body string) error {
	_, err := ghRunner("issue", "comment",
		fmt.Sprintf("%d", issueNumber),
		"--body", body,
	)
	if err != nil {
		return fmt.Errorf("gh issue comment: %w", err)
	}
	return nil
}

// BuildPRBody writes a PR body to w for the given CapabilityDiff.
// Extracted values are NEVER passed through a template engine — they are written
// directly to the io.Writer inside triple-backtick fences.
func BuildPRBody(w io.Writer, diff CapabilityDiff) error {
	// Fixed header — prose only (slug already sanitized before reaching here)
	fmt.Fprintf(w, "# capmon drift: %s\n\n", diff.Provider)
	fmt.Fprintf(w, "Run ID: %s\n", diff.RunID)
	fmt.Fprintf(w, "Changed fields: %d\n\n", len(diff.Changes))

	// Per-field — extracted values always in fenced blocks, never interpolated
	for _, change := range diff.Changes {
		fmt.Fprintf(w, "## %s\n\n", change.FieldPath)
		fmt.Fprintln(w, "Old value:")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, change.OldValue)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, "New value:")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, change.NewValue)
		fmt.Fprintln(w, "```")
	}

	// Fixed footer — non-ground-truth disclaimer
	fmt.Fprintln(w, "\n---")
	fmt.Fprintln(w, "**Pipeline output is not ground truth.** Verify each changed value against the linked source URL independently before approving.")
	return nil
}
