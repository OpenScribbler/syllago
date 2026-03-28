// provider-monitor checks provider source manifests for broken URLs and version drift.
//
// Usage:
//
//	go run ./cmd/provider-monitor [flags]
//	go run ./cmd/provider-monitor -provider gemini-cli
//	go run ./cmd/provider-monitor -json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/provmon"
)

func main() {
	var (
		manifestDir string
		provider    string
		jsonOutput  bool
		concurrency int
		timeout     time.Duration
	)

	// Default manifest dir: relative to the repo root.
	defaultDir := findManifestDir()

	flag.StringVar(&manifestDir, "dir", defaultDir, "path to provider-sources directory")
	flag.StringVar(&provider, "provider", "", "check only this provider slug (default: all)")
	flag.BoolVar(&jsonOutput, "json", false, "output results as JSON")
	flag.IntVar(&concurrency, "concurrency", 10, "max concurrent HTTP requests")
	flag.DurationVar(&timeout, "timeout", 60*time.Second, "overall timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	manifests, err := provmon.LoadAllManifests(manifestDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if provider != "" {
		filtered := make([]*provmon.Manifest, 0, 1)
		for _, m := range manifests {
			if m.Slug == provider {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "error: no manifest found for provider %q\n", provider)
			os.Exit(1)
		}
		manifests = filtered
	}

	var reports []*provmon.CheckReport
	for _, m := range manifests {
		report := provmon.RunCheck(ctx, m, concurrency)
		reports = append(reports, report)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(reports)
	} else {
		printReports(reports)
	}

	// Exit non-zero if any URLs failed or versions drifted.
	for _, r := range reports {
		if r.FailedURLs > 0 {
			os.Exit(1)
		}
		if r.VersionDrift != nil && r.VersionDrift.Drifted {
			os.Exit(1)
		}
	}
}

func printReports(reports []*provmon.CheckReport) {
	for _, r := range reports {
		status := "OK"
		if r.FailedURLs > 0 {
			status = fmt.Sprintf("FAIL (%d/%d URLs broken)", r.FailedURLs, r.TotalURLs)
		}

		fmt.Printf("%-15s %-10s %s\n", r.Slug, r.FetchTier, status)

		// Show failed URLs.
		for _, ur := range r.URLResults {
			if !ur.OK() {
				if ur.Error != nil {
					fmt.Printf("  BROKEN  %s  (%v)\n", ur.URL, ur.Error)
				} else {
					fmt.Printf("  HTTP %d  %s\n", ur.StatusCode, ur.URL)
				}
			}
		}

		// Show version drift.
		if r.VersionDrift != nil && r.VersionDrift.Drifted {
			fmt.Printf("  DRIFT   manifest=%s  latest=%s\n",
				r.VersionDrift.ManifestVersion, r.VersionDrift.LatestVersion)
		}

		// Show stale verification.
		if daysSince := daysSinceVerified(r.LastVerified); daysSince > 7 {
			fmt.Printf("  STALE   last verified %s (%d days ago)\n", r.LastVerified, daysSince)
		}
	}

	// Summary line.
	var totalURLs, failedURLs, drifted int
	for _, r := range reports {
		totalURLs += r.TotalURLs
		failedURLs += r.FailedURLs
		if r.VersionDrift != nil && r.VersionDrift.Drifted {
			drifted++
		}
	}
	fmt.Printf("\n%d providers, %d URLs checked, %d broken, %d version drifts\n",
		len(reports), totalURLs, failedURLs, drifted)
}

func daysSinceVerified(dateStr string) int {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}
	return int(time.Since(t).Hours() / 24)
}

// findManifestDir walks up from the binary to find docs/provider-sources/.
func findManifestDir() string {
	// Try relative to the source file (works with go run).
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// cli/cmd/provider-monitor/main.go -> repo root is 3 levels up
		root := filepath.Join(filepath.Dir(filename), "..", "..", "..")
		dir := filepath.Join(root, "docs", "provider-sources")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	// Fallback: try current working directory.
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "docs", "provider-sources"),
		filepath.Join(cwd, "..", "docs", "provider-sources"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}

	return filepath.Join("docs", "provider-sources")
}
