package main

import (
	"testing"
)

func TestImportRequiresFrom(t *testing.T) {
	// Reset flags
	importCmd.Flags().Set("from", "")
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import without --from should fail")
	}
}

func TestImportUnknownProvider(t *testing.T) {
	importCmd.Flags().Set("from", "nonexistent")
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import with unknown provider should fail")
	}
}
