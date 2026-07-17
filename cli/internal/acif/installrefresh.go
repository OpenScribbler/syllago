package acif

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const installEntryPointsURL = "https://raw.githubusercontent.com/OpenScribbler/agent-content-interchange-format/main/conformance/install-entry-points.yaml"

// RefreshInstallEntryPoints fetches the current ACIF install entry-point
// matrix and swaps it in for the vendored fallback after successful parsing.
// ACIF_INSTALL_ENTRY_POINTS remains higher precedence because loadInstallMatrix
// reads that path before consulting this swappable fallback.
func RefreshInstallEntryPoints(client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	resp, err := client.Get(installEntryPointsURL)
	if err != nil {
		return fmt.Errorf("fetch install entry points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch install entry points: status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read install entry points: %w", err)
	}

	matrix, err := parseInstallMatrix(body)
	if err != nil {
		return fmt.Errorf("parse refreshed install entry points: %w", err)
	}
	swapInstallMatrix(matrix)
	return nil
}
