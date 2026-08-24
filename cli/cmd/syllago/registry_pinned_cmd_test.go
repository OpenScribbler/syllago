package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestRegistryPinnedCommandFlow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	// 1. Setup bare repository with custom tagging
	bare, work, branch := createBareGitRegistryForSyncTest(t)

	// Create tag "v2.0.0" at initial commit
	gitRunForSyncTest(t, work, "tag", "v2.0.0")
	// Push tag to bare
	gitRunForSyncTest(t, work, "push", bare, "v2.0.0")

	// Get the SHA of v2.0.0
	tagSHA := gitOutputForSyncTest(t, work, "rev-parse", "v2.0.0")
	shortTagSHA := tagSHA
	if len(shortTagSHA) > 12 {
		shortTagSHA = shortTagSHA[:12]
	}

	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{
				Name: "git-pinned",
				URL:  bare,
				Ref:  "v2.0.0",
			},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) {
		return registry.VisibilityPublic, nil
	})

	// 2. Run sync to perform the initial checkout at v2.0.0
	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-pinned"}); err != nil {
		t.Fatalf("initial registry sync failed: %v", err)
	}

	gotOut := stdout.String()
	// Pinned initial sync should print Sync (pinned) output
	expectedSyncedLine := "Synced (pinned): git-pinned — checkout held at " + shortTagSHA + "\n"
	if !strings.Contains(gotOut, expectedSyncedLine) {
		t.Errorf("stdout = %q; want to contain %q", gotOut, expectedSyncedLine)
	}

	// 3. Status on pinned registry before upstream moves (should be up to date)
	stdout.Reset()
	registryStatusCmd.SilenceUsage = true
	registryStatusCmd.SilenceErrors = true
	if err := registryStatusCmd.RunE(registryStatusCmd, []string{"git-pinned"}); err != nil {
		t.Fatalf("registry status failed: %v", err)
	}
	gotStatusOut := stdout.String()
	expectedStatusLine := "git-pinned (git, pinned): up to date\n"
	if gotStatusOut != expectedStatusLine {
		t.Errorf("status stdout = %q; want %q", gotStatusOut, expectedStatusLine)
	}

	// 4. Update upstream bare repository to force a diff
	updateBareGitRegistryForSyncDiffTest(t, work, bare, branch)
	upstreamSHA := gitOutputForSyncTest(t, work, "rev-parse", "HEAD")
	shortUpstreamSHA := upstreamSHA
	if len(shortUpstreamSHA) > 12 {
		shortUpstreamSHA = shortUpstreamSHA[:12]
	}

	// 5. Run status again to see changes without applying them
	stdout.Reset()
	if err := registryStatusCmd.RunE(registryStatusCmd, []string{"git-pinned"}); err != nil {
		t.Fatalf("registry status after upstream update failed: %v", err)
	}
	gotStatusDiffOut := stdout.String()
	expectedStatusDiffLine := "git-pinned (git, pinned): 3 upstream change(s)\n"
	if !strings.Contains(gotStatusDiffOut, expectedStatusDiffLine) {
		t.Errorf("status diff stdout = %q; want to contain %q", gotStatusDiffOut, expectedStatusDiffLine)
	}

	// 6. Run sync to fetch but hold the checkout
	stdout.Reset()
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-pinned"}); err != nil {
		t.Fatalf("registry sync after upstream update failed: %v", err)
	}
	gotSyncDiffOut := stdout.String()
	expectedHeldLine := "Synced (pinned): git-pinned — checkout held at " + shortTagSHA + "\n"
	expectedMovedLine := "Upstream has moved to " + shortUpstreamSHA + " — changes below are NOT applied while pinned:\n"
	if !strings.Contains(gotSyncDiffOut, expectedHeldLine) {
		t.Errorf("sync diff stdout = %q; want to contain %q", gotSyncDiffOut, expectedHeldLine)
	}
	if !strings.Contains(gotSyncDiffOut, expectedMovedLine) {
		t.Errorf("sync diff stdout = %q; want to contain %q", gotSyncDiffOut, expectedMovedLine)
	}

	// 7. Test JSON output of status command to verify pinned attribute
	stdout.Reset()
	output.JSON = true
	t.Cleanup(func() { output.JSON = false })

	if err := registryStatusCmd.RunE(registryStatusCmd, []string{"git-pinned"}); err != nil {
		t.Fatalf("registry status JSON failed: %v", err)
	}

	var rows []struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		State    string `json:"state"`
		Pinned   bool   `json:"pinned"`
		Added    int    `json:"added"`
		Modified int    `json:"modified"`
		Removed  int    `json:"removed"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal registry status JSON failed: %v\nOutput: %s", err, stdout.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 status row, got %d", len(rows))
	}

	if rows[0].Name != "git-pinned" {
		t.Errorf("row name = %q, want 'git-pinned'", rows[0].Name)
	}
	if !rows[0].Pinned {
		t.Error("expected row to have Pinned = true")
	}
}
