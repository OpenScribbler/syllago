package capmon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// healFailureThreshold is the number of consecutive heal attempts that
// must fail before capmon escalates with a GitHub issue. Set to 2 (vs.
// the drift threshold of 3) because healing is already a "something is
// broken" signal — dragging the escalation out makes the pipeline look
// silent when it's actively struggling.
const healFailureThreshold = 2

// healFailureLabel is the GitHub label applied to heal-failure issues.
const healFailureLabel = "capmon-heal-fail"

// healFailureCountFile returns the path to the consecutive-heal-failure
// counter for a single source within a provider. Counters are scoped to
// (provider, contentType, sourceIndex) so fixing one source's URL
// doesn't reset the counter for another.
func healFailureCountFile(cacheRoot, provider, contentType string, sourceIndex int) string {
	return filepath.Join(cacheRoot, provider, fmt.Sprintf("heal-failures-%s-%d.json", contentType, sourceIndex))
}

// HealFailureAnchor returns the hidden HTML comment for heal-failure
// issue dedup. Distinct from HealPRAnchor so a PR and an issue for the
// same source don't step on each other when gh searches.
func HealFailureAnchor(provider, contentType string, sourceIndex int) string {
	return fmt.Sprintf("<!-- capmon-heal-fail: %s/%s/%d -->", provider, contentType, sourceIndex)
}

// RecordConsecutiveHealFailure increments the heal-failure counter for
// a specific source. When the counter reaches healFailureThreshold, a
// GitHub issue is opened (or a comment is appended to the existing one)
// so a human can intervene — manual URL update, disable healing, or
// mark the source supported:false.
//
// Returns (issueNumber, error). issueNumber is 0 when no issue was
// created this run (under threshold or append-to-existing path).
//
// reason is a human-readable description of why the last heal attempt
// failed; it ends up in the issue body.
func RecordConsecutiveHealFailure(cacheRoot, provider, contentType string, sourceIndex int, reason string) (int, error) {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return 0, err
	}

	path := healFailureCountFile(cacheRoot, slug, contentType, sourceIndex)
	var count int
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &count)
	}
	count++
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, fmt.Errorf("mkdir for failure counter: %w", err)
	}
	data, _ := json.Marshal(count)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("write failure counter: %w", err)
	}

	if count < healFailureThreshold {
		return 0, nil
	}

	// Threshold hit — look for an open issue; comment if found, create otherwise.
	if issueNum, found, err := findOpenHealFailureIssue(slug, contentType, sourceIndex); err != nil {
		return 0, err
	} else if found {
		comment := fmt.Sprintf("Heal attempt #%d also failed: %s", count, reason)
		if _, err := ghRunner("issue", "comment",
			fmt.Sprintf("%d", issueNum),
			"--body", comment,
		); err != nil {
			return issueNum, fmt.Errorf("append comment: %w", err)
		}
		return issueNum, nil
	}

	return createHealFailureIssue(slug, contentType, sourceIndex, count, reason)
}

// findOpenHealFailureIssue looks for an open capmon-heal-fail issue
// whose body contains the anchor for (provider, contentType, sourceIndex).
func findOpenHealFailureIssue(provider, contentType string, sourceIndex int) (int, bool, error) {
	anchor := HealFailureAnchor(provider, contentType, sourceIndex)
	out, err := ghRunner("issue", "list",
		"--label", healFailureLabel,
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

// createHealFailureIssue opens a new capmon-heal-fail issue. Caller
// ensures provider was already sanitized.
func createHealFailureIssue(provider, contentType string, sourceIndex, count int, reason string) (int, error) {
	anchor := HealFailureAnchor(provider, contentType, sourceIndex)
	title := fmt.Sprintf("capmon: heal failed %dx for %s/%s source[%d]", count, provider, contentType, sourceIndex)
	body := anchor + "\n\n" +
		fmt.Sprintf("Heal attempts have failed %d times in a row for this source.\n\n", count) +
		fmt.Sprintf("**Provider:** `%s`\n", provider) +
		fmt.Sprintf("**Content type:** `%s`\n", contentType) +
		fmt.Sprintf("**Source index:** `%d`\n\n", sourceIndex) +
		"**Last failure reason:**\n```\n" + reason + "\n```\n\n" +
		"Possible actions:\n" +
		"- Manually update the URL in `docs/provider-sources/" + provider + ".yaml`\n" +
		"- Disable healing for this source (`healing.enabled: false`) if the URL is actually correct\n" +
		"- Mark the content type `supported: false` if the provider no longer documents it\n"

	out, err := ghRunner("issue", "create",
		"--title", title,
		"--label", healFailureLabel,
		"--label", "provider:"+provider,
		"--body", body,
	)
	if err != nil {
		return 0, fmt.Errorf("gh issue create: %w", err)
	}
	issueURL := strings.TrimSpace(string(out))
	parts := strings.Split(issueURL, "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected gh output: %q", issueURL)
	}
	num, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("parse issue number from %q: %w", issueURL, err)
	}
	return num, nil
}

// ResolveHealFailure clears the heal-failure counter and closes any
// open capmon-heal-fail issue for (provider, contentType, sourceIndex).
// Called when a heal attempt finally succeeds, so the next failure
// restarts the threshold count cleanly and reviewers don't see stale
// "broken" issues.
func ResolveHealFailure(cacheRoot, provider, contentType string, sourceIndex int) error {
	slug, err := SanitizeSlug(provider)
	if err != nil {
		return err
	}
	// Remove counter file — best effort, a missing file is fine.
	_ = os.Remove(healFailureCountFile(cacheRoot, slug, contentType, sourceIndex))

	// Close any open issue for this source.
	issueNum, found, err := findOpenHealFailureIssue(slug, contentType, sourceIndex)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if _, err := ghRunner("issue", "close",
		fmt.Sprintf("%d", issueNum),
		"--comment", "Resolved: capmon healed this source successfully on the latest run.",
	); err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	return nil
}
