package main

import (
	"encoding/json"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Manage usage analytics settings",
	Long:  "View and control anonymous usage data collection. Run 'syllago telemetry status' for details.",
	Example: `  syllago telemetry status
  syllago telemetry off
  syllago telemetry on
  syllago telemetry reset`,
	RunE: runTelemetryStatus,
}

var telemetryStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show telemetry state and anonymous ID",
	Example: `  syllago telemetry status`,
	RunE:    runTelemetryStatus,
}

func runTelemetryStatus(cmd *cobra.Command, args []string) error {
	cfg := telemetry.Status()

	if output.JSON {
		type statusOut struct {
			Enabled         bool   `json:"enabled"`
			ConsentRecorded bool   `json:"consentRecorded"`
			AnonymousID     string `json:"anonymousId"`
			Endpoint        string `json:"endpoint,omitempty"`
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = "https://us.i.posthog.com/capture/"
		}
		data, _ := json.MarshalIndent(statusOut{
			Enabled:         cfg.Enabled,
			ConsentRecorded: cfg.ConsentRecorded,
			AnonymousID:     cfg.AnonymousID,
			Endpoint:        endpoint,
		}, "", "  ")
		fmt.Fprintln(output.Writer, string(data))
		return nil
	}

	state := "disabled (opt-in)"
	switch {
	case cfg.Enabled && cfg.ConsentRecorded:
		state = "enabled (you opted in)"
	case !cfg.Enabled && cfg.ConsentRecorded:
		state = "disabled (you opted out)"
	case !cfg.ConsentRecorded:
		state = "disabled (awaiting your decision)"
	}
	fmt.Fprintf(output.Writer, "Telemetry: %s\n", state)
	id := cfg.AnonymousID
	if id == "" {
		id = "(not yet generated)"
	}
	fmt.Fprintf(output.Writer, "Anonymous ID: %s\n\n", id)
	fmt.Fprintf(output.Writer, "Telemetry is opt-in. Nothing is sent unless you have explicitly\n")
	fmt.Fprintf(output.Writer, "agreed via the consent prompt or `syllago telemetry on`.\n\n")
	fmt.Fprintf(output.Writer, "What we collect (only with your consent):\n")
	for _, item := range telemetry.CollectedItems() {
		fmt.Fprintf(output.Writer, "  - %s\n", item)
	}
	fmt.Fprintln(output.Writer)
	fmt.Fprintf(output.Writer, "What we never collect:\n")
	for _, item := range telemetry.NeverItems() {
		fmt.Fprintf(output.Writer, "  - %s\n", item)
	}
	fmt.Fprintln(output.Writer)
	fmt.Fprintf(output.Writer, "Enable:   syllago telemetry on\n")
	fmt.Fprintf(output.Writer, "Disable:  syllago telemetry off\n")
	fmt.Fprintf(output.Writer, "Reset ID: syllago telemetry reset\n")
	fmt.Fprintf(output.Writer, "Docs:     %s\n", telemetry.DocsURL)
	fmt.Fprintf(output.Writer, "Code:     %s\n", telemetry.CodeURL)
	return nil
}

var telemetryOnCmd = &cobra.Command{
	Use:     "on",
	Short:   "Enable telemetry",
	Example: `  syllago telemetry on`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetEnabled(true); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not save telemetry config: %v\n", err)
			fmt.Fprintf(output.ErrWriter, "Telemetry state may not persist across sessions.\n")
			return nil
		}
		if output.JSON {
			fmt.Fprintln(output.Writer, `{"enabled":true}`)
			return nil
		}
		fmt.Fprintln(output.Writer, "Telemetry enabled.")
		return nil
	},
}

var telemetryOffCmd = &cobra.Command{
	Use:     "off",
	Short:   "Disable telemetry",
	Example: `  syllago telemetry off`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetEnabled(false); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not save telemetry config: %v\n", err)
			fmt.Fprintf(output.ErrWriter, "Telemetry state may not persist across sessions.\n")
			return nil
		}
		if output.JSON {
			fmt.Fprintln(output.Writer, `{"enabled":false}`)
			return nil
		}
		fmt.Fprintln(output.Writer, "Telemetry disabled.")
		return nil
	},
}

var telemetryResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Generate a new anonymous ID",
	Long: `Generates a new anonymous ID. Previously collected data under your old ID
is not deleted from PostHog. To request deletion, email privacy@syllago.dev
with your old ID.`,
	Example: `  syllago telemetry reset`,
	RunE: func(cmd *cobra.Command, args []string) error {
		newID, err := telemetry.Reset()
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not reset telemetry ID: %v\n", err)
			return nil
		}
		if output.JSON {
			data, _ := json.MarshalIndent(map[string]string{"anonymousId": newID}, "", "  ")
			fmt.Fprintln(output.Writer, string(data))
			return nil
		}
		fmt.Fprintf(output.Writer, "Anonymous ID rotated: %s\n\n", newID)
		fmt.Fprintf(output.Writer, "Note: Previously collected data under your old ID is not deleted.\n")
		fmt.Fprintf(output.Writer, "To request deletion, email privacy@syllago.dev with your old ID.\n")
		return nil
	},
}

func init() {
	telemetryCmd.AddCommand(telemetryStatusCmd, telemetryOnCmd, telemetryOffCmd, telemetryResetCmd)
	rootCmd.AddCommand(telemetryCmd)
}
