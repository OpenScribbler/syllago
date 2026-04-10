package capmon

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// ChromedpRemoteURL returns the CHROMEDP_URL env var value.
// Empty string means use local Chrome via DefaultExecAllocatorOptions.
func ChromedpRemoteURL() string {
	return os.Getenv("CHROMEDP_URL")
}

// FetchChromedp fetches a URL using a headless Chromium browser.
// When CHROMEDP_URL is set, connects to a remote headless-shell sidecar.
// When unset, uses a local Chrome installation via exec allocator.
func FetchChromedp(ctx context.Context, cacheRoot, provider, sourceID, url string) (*CacheEntry, error) {
	var allocCtx context.Context
	var cancel context.CancelFunc

	if remoteURL := ChromedpRemoteURL(); remoteURL != "" {
		allocCtx, cancel = chromedp.NewRemoteAllocator(ctx, remoteURL)
	} else {
		allocCtx, cancel = chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	}
	defer cancel()

	chromedpCtx, chromedpCancel := chromedp.NewContext(allocCtx)
	defer chromedpCancel()

	var bodyHTML string
	if err := chromedp.Run(chromedpCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("body", &bodyHTML),
	); err != nil {
		return nil, fmt.Errorf("chromedp fetch %s: %w", url, err)
	}

	raw := []byte(bodyHTML)
	meta := CacheMeta{
		FetchedAt:   time.Now().UTC(),
		ContentHash: SHA256Hex(raw),
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
