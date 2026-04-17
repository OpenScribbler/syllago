package capmon

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// RealisticUA is a current Chrome user-agent string. Headless Chrome's default
// UA contains "HeadlessChrome" which triggers bot detection on many sites.
const RealisticUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// chromedpRenderWait is how long to wait after DOMContentLoaded for JS frameworks
// (React, Next.js) to finish rendering. Configurable for testing.
var chromedpRenderWait = 3 * time.Second

// ChromedpRemoteURL returns the CHROMEDP_URL env var value.
// Empty string means use local Chrome via DefaultExecAllocatorOptions.
func ChromedpRemoteURL() string {
	return os.Getenv("CHROMEDP_URL")
}

// stealthAllocatorOpts returns exec allocator options that avoid common
// bot-detection signals: realistic user-agent, no navigator.webdriver flag,
// and standard window size.
func stealthAllocatorOpts() []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.UserAgent(RealisticUA),
		chromedp.Flag("enable-automation", false),
		chromedp.WindowSize(1920, 1080),
	)
	return opts
}

// FetchChromedp fetches a URL using a headless Chromium browser.
// When CHROMEDP_URL is set, connects to a remote headless-shell sidecar.
// When unset, uses a local Chrome installation with stealth options to avoid
// bot detection (realistic UA, no automation flag, standard viewport).
//
// After navigation, waits for JS frameworks to finish rendering before
// capturing the outer HTML of <body>.
func FetchChromedp(ctx context.Context, cacheRoot, provider, sourceID, rawURL string) (*CacheEntry, error) {
	var allocCtx context.Context
	var cancel context.CancelFunc

	if remoteURL := ChromedpRemoteURL(); remoteURL != "" {
		allocCtx, cancel = chromedp.NewRemoteAllocator(ctx, remoteURL)
	} else {
		allocCtx, cancel = chromedp.NewExecAllocator(ctx, stealthAllocatorOpts()...)
	}
	defer cancel()

	chromedpCtx, chromedpCancel := chromedp.NewContext(allocCtx)
	defer chromedpCancel()

	var bodyHTML string
	if err := chromedp.Run(chromedpCtx,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(chromedpRenderWait),
		chromedp.OuterHTML("body", &bodyHTML),
	); err != nil {
		return nil, fmt.Errorf("chromedp fetch %s: %w", rawURL, err)
	}

	raw := []byte(bodyHTML)
	newHash := SHA256Hex(raw)

	// Check if content changed
	if IsCached(cacheRoot, provider, sourceID) {
		existing, readErr := ReadCacheEntry(cacheRoot, provider, sourceID)
		if readErr == nil && existing.Meta.ContentHash == newHash {
			existing.Meta.Cached = true
			return existing, nil
		}
	}

	meta := CacheMeta{
		FetchedAt:   time.Now().UTC(),
		ContentHash: newHash,
		FetchStatus: "ok",
		FetchMethod: "chromedp",
	}
	entry := CacheEntry{
		Provider: provider,
		SourceID: sourceID,
		Raw:      raw,
		Meta:     meta,
	}
	if err := WriteCacheEntry(cacheRoot, entry); err != nil {
		return nil, fmt.Errorf("write cache entry: %w", err)
	}
	return &entry, nil
}
