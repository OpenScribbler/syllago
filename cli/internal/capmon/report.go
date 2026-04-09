package capmon

import (
	"fmt"
	"io"
	"regexp"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ghCommand is the gh CLI command, overridable in tests.
var ghCommand = "gh"

// GHRunner returns the current gh CLI command name.
// Exported for use by Stage 4 PR/issue creation (Task 9.2) and tests (Task 6.2).
func GHRunner() string {
	return ghCommand
}

// SetGHCommandForTest overrides the gh command for testing.
// Must only be called from test code.
func SetGHCommandForTest(cmd string) {
	ghCommand = cmd
}

// SanitizeSlug validates a provider slug is safe for use in branch names and PR bodies.
// Applied to both branch name construction and PR body construction in Stage 4.
func SanitizeSlug(slug string) (string, error) {
	if !slugRegex.MatchString(slug) {
		return "", fmt.Errorf("invalid slug: %q", slug)
	}
	return slug, nil
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
