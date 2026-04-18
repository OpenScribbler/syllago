package moat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// rekorResponse mirrors the shape of GET /api/v1/log/entries?logIndex=<n>
// on rekor.sigstore.dev — a single-entry map keyed by the entry's UUID.
type rekorResponse map[string]rekorEntry

type rekorEntry struct {
	Body           string            `json:"body"`
	IntegratedTime int64             `json:"integratedTime"`
	LogID          string            `json:"logID"`
	LogIndex       int64             `json:"logIndex"`
	Verification   rekorVerification `json:"verification"`
}

type rekorVerification struct {
	InclusionProof       rekorInclusionProof `json:"inclusionProof"`
	SignedEntryTimestamp string              `json:"signedEntryTimestamp"`
}

type rekorInclusionProof struct {
	Checkpoint string   `json:"checkpoint"`
	Hashes     []string `json:"hashes"`
	LogIndex   int64    `json:"logIndex"`
	RootHash   string   `json:"rootHash"`
	TreeSize   int64    `json:"treeSize"`
}

// hashedrekordBody is the decoded (base64 + JSON) rekorEntry.Body for
// apiVersion=0.0.1, kind=hashedrekord — the structure Rekor records for
// the signature materials covered by a hashedrekord entry.
type hashedrekordBody struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Spec       hashedrekordSpec `json:"spec"`
}

type hashedrekordSpec struct {
	Data      hashedrekordData      `json:"data"`
	Signature hashedrekordSignature `json:"signature"`
}

type hashedrekordData struct {
	Hash hashedrekordHash `json:"hash"`
}

type hashedrekordHash struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type hashedrekordSignature struct {
	Content   string          `json:"content"`
	PublicKey hashedrekordKey `json:"publicKey"`
}

type hashedrekordKey struct {
	Content string `json:"content"`
}

// parseRekorEntry parses a Rekor API response into its single entry. The
// Rekor lookup API always returns a one-entry map keyed by UUID; this
// unwraps that convention so callers can work with a typed value.
func parseRekorEntry(raw []byte) (*rekorEntry, error) {
	var resp rekorResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing Rekor response: %w", err)
	}
	if len(resp) != 1 {
		return nil, fmt.Errorf("expected 1 Rekor entry, got %d", len(resp))
	}
	for _, entry := range resp {
		e := entry
		return &e, nil
	}
	return nil, fmt.Errorf("unreachable")
}

// decodeHashedRekordBody base64-decodes and JSON-parses a Rekor entry Body,
// validating kind=hashedrekord and apiVersion=0.0.1. Other apiVersions have
// different spec shapes and would require a separate decoder.
func decodeHashedRekordBody(body string) (*hashedrekordBody, error) {
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding body: %w", err)
	}
	var h hashedrekordBody
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("parsing hashedrekord body: %w", err)
	}
	if h.Kind != "hashedrekord" {
		return nil, fmt.Errorf("expected kind=hashedrekord, got %q", h.Kind)
	}
	if h.APIVersion != "0.0.1" {
		return nil, fmt.Errorf("expected apiVersion=0.0.1, got %q", h.APIVersion)
	}
	return &h, nil
}
