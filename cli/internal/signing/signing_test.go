package signing

import (
	"testing"
	"time"
)

// These tests verify that the signing package's interfaces and types compile
// correctly. The package is currently unimplemented (interfaces only, no
// concrete implementations). These tests serve as documentation of the
// expected contract and will catch accidental breakage of the type definitions.

func TestMethodConstants(t *testing.T) {
	t.Parallel()

	if MethodSigstore != "sigstore" {
		t.Errorf("MethodSigstore = %q, want %q", MethodSigstore, "sigstore")
	}
	if MethodGPG != "gpg" {
		t.Errorf("MethodGPG = %q, want %q", MethodGPG, "gpg")
	}
}

func TestSignerInterfaceCompiles(t *testing.T) {
	t.Parallel()

	// Verify the Signer interface is well-formed by declaring a variable of its type.
	var _ Signer = nil
	_ = (*Signer)(nil)
}

func TestVerifierInterfaceCompiles(t *testing.T) {
	t.Parallel()

	// Verify the Verifier interface is well-formed by declaring a variable of its type.
	var _ Verifier = nil
	_ = (*Verifier)(nil)
}

func TestSignatureBundleFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	bundle := SignatureBundle{
		Method:    MethodSigstore,
		Signature: []byte("test-sig"),
		Identity:  "user@example.com",
		Issuer:    "https://accounts.google.com",
		Timestamp: now,
		LogIndex:  42,
		PublicKey: nil,
	}

	if bundle.Method != MethodSigstore {
		t.Errorf("Method = %q, want %q", bundle.Method, MethodSigstore)
	}
	if bundle.Identity != "user@example.com" {
		t.Errorf("Identity = %q, want %q", bundle.Identity, "user@example.com")
	}
	if bundle.LogIndex != 42 {
		t.Errorf("LogIndex = %d, want 42", bundle.LogIndex)
	}
}

func TestTrustPolicyDefaults(t *testing.T) {
	t.Parallel()

	// A zero-value TrustPolicy should be permissive (no requirements).
	policy := TrustPolicy{}

	if policy.RequireSignature {
		t.Error("zero-value RequireSignature should be false")
	}
	if len(policy.AllowedMethods) != 0 {
		t.Error("zero-value AllowedMethods should be empty")
	}
	if len(policy.AllowedIdentities) != 0 {
		t.Error("zero-value AllowedIdentities should be empty")
	}
	if len(policy.AllowedIssuers) != 0 {
		t.Error("zero-value AllowedIssuers should be empty")
	}
}

func TestRevocationListStructure(t *testing.T) {
	t.Parallel()

	list := RevocationList{
		Version: 1,
		Entries: []RevocationEntry{
			{
				Type:      "hook",
				HookName:  "dangerous-hook",
				Reason:    "malicious content",
				RevokedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				Type:      "identity",
				Identity:  "bad-actor@example.com",
				Reason:    "compromised key",
				RevokedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	if list.Version != 1 {
		t.Errorf("Version = %d, want 1", list.Version)
	}
	if len(list.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(list.Entries))
	}
	if list.Entries[0].Type != "hook" {
		t.Errorf("Entries[0].Type = %q, want %q", list.Entries[0].Type, "hook")
	}
	if list.Entries[1].Type != "identity" {
		t.Errorf("Entries[1].Type = %q, want %q", list.Entries[1].Type, "identity")
	}
}
