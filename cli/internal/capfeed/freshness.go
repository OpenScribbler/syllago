package capfeed

import (
	"fmt"
	"time"
)

// CheckFreshness enforces the feed's heartbeat contract (capmon ADR 0012):
// a feed whose generated_at is older than maxStalenessHours is stale and
// must not be acted on — the caller keeps last-known-good and exits red.
// The limit comes from the verified index itself (max_staleness_hours);
// exactly-at-limit is still fresh. A future generated_at (publisher clock
// ahead of ours) is fresh — failing on clock skew would be a false alarm.
//
// Deliberately shallow: this is the one policy point the design orders
// after provenance verification and before any file fetch; folding it into
// VerifyFeedProvenance would blur that ordering.
func CheckFreshness(generatedAt, now time.Time, maxStalenessHours int) error {
	limit := time.Duration(maxStalenessHours) * time.Hour
	age := now.Sub(generatedAt)
	if age > limit {
		return fmt.Errorf("capfeed: feed is stale: generated_at %s is %s old (limit %dh); keeping last-known-good",
			generatedAt.UTC().Format(time.RFC3339), age.Round(time.Minute), maxStalenessHours)
	}
	return nil
}
