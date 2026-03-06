package metadata

import (
	"time"
)

// Backfill creates an .syllago.yaml for an item that doesn't have one.
// Returns nil if the file already exists (idempotent).
func Backfill(itemDir, name, contentType, author string) error {
	existing, err := Load(itemDir)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // already has metadata
	}

	now := time.Now()
	meta := &Meta{
		ID:      NewID(),
		Name:    name,
		Type:    contentType,
		Author:  author,
		Source:  "created",
		AddedAt: &now,
	}
	return Save(itemDir, meta)
}
