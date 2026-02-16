package main

import (
	"bytes"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestParityNeedsMultipleProviders(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.JSON = false
		output.Writer = origWriter
	}()

	err := parityCmd.RunE(parityCmd, []string{})
	if err != nil {
		t.Fatalf("parity should not error with few providers: %v", err)
	}
}
