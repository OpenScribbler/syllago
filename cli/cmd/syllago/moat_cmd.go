package main

// `moat` parent command + `moat trust status` subcommand.
//
// The MOAT subsystem exposes operator-facing commands for managing the
// trusted root (acquisition source + calendar freshness) and, in later
// slices, running full registry manifest verification. Slice 1 ships only
// `moat trust status` because the trusted-root lifecycle is the hardest
// piece to get right — it defines the exit-code contract that CI pipelines
// and health checks key on, and it must be stable before any other moat
// surface can depend on it.
//
// Exit codes (per ADR 0007):
//   0  fresh
//   1  warn / escalated  (verification still proceeds but warn operator)
//   2  expired / missing / corrupt  (verification must refuse to proceed)

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/spf13/cobra"
)

// moatClockOverride lets tests inject a fixed time without touching the
// command surface. The overhead of a package-level seam is trivial compared
// to the win of a single time.Now() call we can pin in tests.
var moatClockOverride func() time.Time

func moatNow() time.Time {
	if moatClockOverride != nil {
		return moatClockOverride()
	}
	return time.Now()
}

var moatCmd = &cobra.Command{
	Use:   "moat",
	Short: "Manage MOAT registry trust and verification",
	Long: `Inspect and manage the MOAT (MOAT Of Attested Trust) subsystem.

Slice 1 surface: trusted-root lifecycle via ` + "`moat trust status`" + `.
Later slices will add manifest verification, tier policy, and revocation.`,
}

var moatTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Trusted-root lifecycle (status, refresh)",
	Long:  "Operate on the Sigstore trusted-root bundle used to verify signed registry manifests.",
}

var moatTrustStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report the bundled Sigstore trusted root's freshness",
	Long: `Print the source and calendar age of the bundled Sigstore trusted root,
then exit with a stable code CI pipelines can gate on:

  0  fresh       (age < 90 days)
  1  warn        (90–179 days)  — verification still runs
  1  escalated   (180–364 days) — verification still runs
  2  expired     (365+ days)    — verification refuses
  2  missing     (embed is empty — build-time bug)
  2  corrupt     (issued-at constant malformed — build-time bug)

Use --json to emit a single JSON line for scripts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		info, exit := runMoatTrustStatus(cmd.OutOrStdout(), cmd.ErrOrStderr(), moatNow(), asJSON)
		// Exit codes 0/1/2 are the contract; cobra only maps RunE errors to
		// exit 1, so we bypass its error path and os.Exit directly after
		// making sure PersistentPostRun has a chance to fire telemetry.
		_ = info // reserved for future callers (e.g., structured audit log)
		os.Exit(exit)
		return nil
	},
}

// runMoatTrustStatus prints the trusted-root status snapshot to stdout
// (human or JSON) and any warning/error to stderr, then returns the
// computed info + exit code. Factored out of RunE so tests can call it
// directly and inspect the payload without trapping os.Exit.
func runMoatTrustStatus(stdout, stderr io.Writer, now time.Time, asJSON bool) (moat.TrustedRootInfo, int) {
	info := moat.BundledTrustedRoot(now)

	if asJSON {
		writeTrustStatusJSON(stdout, info)
	} else {
		writeTrustStatusHuman(stdout, info)
	}

	// The staleness message is stderr noise — keep it off stdout so scripts
	// grepping the JSON/human status line never see it interleaved.
	if msg := moat.StalenessMessage(info); msg != "" {
		fmt.Fprintln(stderr, msg)
	}

	return info, moat.ExitCodeForStatus(info.Status)
}

// writeTrustStatusHuman renders the status as key=value lines, one per
// field. This format is deliberately machine-parseable (awk/grep) without
// being JSON — a middle ground that humans can read at a glance and scripts
// can extract from without jq. The `moat.trusted_root=bundled` line per
// ADR 0007 is the primary audit breadcrumb.
func writeTrustStatusHuman(w io.Writer, info moat.TrustedRootInfo) {
	fmt.Fprintf(w, "moat.trusted_root=%s\n", info.Source)
	if !info.IssuedAt.IsZero() {
		fmt.Fprintf(w, "moat.trusted_root.issued_at=%s\n", info.IssuedAt.Format("2006-01-02"))
	}
	fmt.Fprintf(w, "moat.trusted_root.age_days=%d\n", info.AgeDays)
	if !info.CliffDate.IsZero() {
		fmt.Fprintf(w, "moat.trusted_root.cliff_date=%s\n", info.CliffDate.Format("2006-01-02"))
	}
	fmt.Fprintf(w, "moat.trusted_root.status=%s\n", info.Status)
}

// trustStatusJSON is the shape emitted under --json. Fields are lowercase
// snake_case for interoperability with the rest of syllago's CLI JSON
// envelopes.
type trustStatusJSON struct {
	Source    string `json:"source"`
	IssuedAt  string `json:"issued_at,omitempty"`
	AgeDays   int    `json:"age_days"`
	CliffDate string `json:"cliff_date,omitempty"`
	Status    string `json:"status"`
}

func writeTrustStatusJSON(w io.Writer, info moat.TrustedRootInfo) {
	payload := trustStatusJSON{
		Source:  string(info.Source),
		AgeDays: info.AgeDays,
		Status:  info.Status.String(),
	}
	if !info.IssuedAt.IsZero() {
		payload.IssuedAt = info.IssuedAt.Format("2006-01-02")
	}
	if !info.CliffDate.IsZero() {
		payload.CliffDate = info.CliffDate.Format("2006-01-02")
	}
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func init() {
	moatTrustStatusCmd.Flags().Bool("json", false, "Emit a single JSON line instead of key=value lines")
	moatTrustCmd.AddCommand(moatTrustStatusCmd)
	moatCmd.AddCommand(moatTrustCmd)
}
