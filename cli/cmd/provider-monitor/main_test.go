package main

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provmon"
)

// TestExitCode_DefaultFailOn_DriftedOnly verifies that the default --fail-on=drifted
// policy exits 0 when only fetch_failed / content_invalid / skipped statuses appear,
// and exits non-zero only when drifted appears.
func TestExitCode_DefaultFailOn_DriftedOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reports  []*provmon.CheckReport
		wantExit int
	}{
		{
			name: "all stable",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusStable},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "fetch_failed but default fail_on",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusFetchFailed, ErrorMessage: "500"},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "drifted triggers exit",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method:  "source-hash",
					Drifted: true,
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusDrifted},
					},
				},
			}},
			wantExit: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := computeExitCode(tc.reports, []string{"drifted"})
			if got != tc.wantExit {
				t.Errorf("computeExitCode = %d, want %d", got, tc.wantExit)
			}
		})
	}
}

// TestExitCode_WideFailOn_FetchFailed verifies that widening --fail-on to
// include fetch_failed flips exit to non-zero when any source fetch-fails,
// while the default --fail-on=drifted policy continues to exit 0.
func TestExitCode_WideFailOn_FetchFailed(t *testing.T) {
	t.Parallel()

	reports := []*provmon.CheckReport{{
		VersionDrift: &provmon.VersionDrift{
			Method: "source-hash",
			Sources: []provmon.SourceDrift{
				{Status: provmon.StatusFetchFailed, ErrorMessage: "500"},
				{Status: provmon.StatusStable},
			},
		},
	}}

	if got := computeExitCode(reports, []string{"drifted"}); got != 0 {
		t.Errorf("default fail_on: got exit %d, want 0", got)
	}

	if got := computeExitCode(reports, []string{"drifted", "fetch_failed"}); got == 0 {
		t.Error("widened fail_on should exit non-zero on fetch_failed")
	}
}
