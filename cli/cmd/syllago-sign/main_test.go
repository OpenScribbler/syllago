package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunKeygen_ProducesValidKeyPair(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := run([]string{"keygen"}, &buf); err != nil {
		t.Fatalf("keygen failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 output lines; got %d: %q", len(lines), buf.String())
	}
	privHex := strings.TrimPrefix(lines[0], "private=")
	pubHex := strings.TrimPrefix(lines[1], "public=")
	if privHex == lines[0] {
		t.Fatalf("first line missing 'private=' prefix: %q", lines[0])
	}
	if pubHex == lines[1] {
		t.Fatalf("second line missing 'public=' prefix: %q", lines[1])
	}

	seed, err := hex.DecodeString(privHex)
	if err != nil {
		t.Fatalf("private hex invalid: %v", err)
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		t.Fatalf("public hex invalid: %v", err)
	}
	if len(seed) != ed25519.SeedSize {
		t.Errorf("seed len = %d; want %d", len(seed), ed25519.SeedSize)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("pub len = %d; want %d", len(pub), ed25519.PublicKeySize)
	}

	derived := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if !bytes.Equal(derived, pub) {
		t.Error("printed public key does not match the public key derived from the printed seed")
	}
}

func TestRunSign_RoundTripsThroughUpdaterVerifier(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seedHex := hex.EncodeToString(priv.Seed())

	dir := t.TempDir()
	payload := []byte("abc123  syllago-linux-amd64\ndef456  syllago-darwin-arm64\n")
	payloadPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(payloadPath, payload, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SYLLAGO_SIGNING_PRIVATE_KEY", seedHex)

	var sigBuf bytes.Buffer
	if err := run([]string{"sign", payloadPath}, &sigBuf); err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	sig := sigBuf.Bytes()
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("signature size = %d; want %d", len(sig), ed25519.SignatureSize)
	}
	if !ed25519.Verify(pub, payload, sig) {
		t.Fatal("ed25519.Verify rejected the produced signature — sign/verify symmetry broken")
	}
}

func TestRunSign_RejectsBadKey(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		wantSub string
	}{
		{"missing", "", "is not set"},
		{"bad hex", "zzzz", "decoding private key"},
		{"too short", strings.Repeat("aa", 16), "private key must be"},
		{"too long", strings.Repeat("aa", 64), "private key must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SYLLAGO_SIGNING_PRIVATE_KEY", tc.env)
			err := run([]string{"sign", "irrelevant.txt"}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q should contain %q", err, tc.wantSub)
			}
		})
	}
}

func TestRunSign_MissingInputFile(t *testing.T) {
	t.Setenv("SYLLAGO_SIGNING_PRIVATE_KEY", strings.Repeat("aa", 32))
	err := run([]string{"sign", filepath.Join(t.TempDir(), "does-not-exist")}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("want error for missing input file, got nil")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("error %q should mention reading the input", err)
	}
}

func TestRun_UsageErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"unknown subcommand", []string{"verify"}},
		{"keygen with extra args", []string{"keygen", "extra"}},
		{"sign without file", []string{"sign"}},
		{"sign with too many args", []string{"sign", "a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, &bytes.Buffer{})
			if err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}
