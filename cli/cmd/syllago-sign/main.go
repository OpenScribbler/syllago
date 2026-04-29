// Build-time Ed25519 signing utility for syllago release artifacts.
// Not shipped to end users — invoked from .github/workflows/release.yml via
// `go run ./cmd/syllago-sign`. The companion verifier lives in
// cli/internal/updater and runs inside the user-facing binary at self-update
// time (see verifyChecksumSignature in cli/internal/updater/updater.go).
//
// Subcommands:
//
//	syllago-sign keygen        Print a fresh "private=<hex>\npublic=<hex>" pair.
//	syllago-sign sign <file>   Sign <file> with the seed in $SYLLAGO_SIGNING_PRIVATE_KEY
//	                           and write the 64-byte raw signature to stdout.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "syllago-sign:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: syllago-sign <keygen|sign> [args]")
	}
	switch args[0] {
	case "keygen":
		if len(args) != 1 {
			return errors.New("usage: syllago-sign keygen")
		}
		return doKeygen(out)
	case "sign":
		if len(args) != 2 {
			return errors.New("usage: syllago-sign sign <file>")
		}
		return doSign(args[1], out)
	default:
		return fmt.Errorf("unknown command %q (want keygen or sign)", args[0])
	}
}

func doKeygen(out io.Writer) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}
	_, err = fmt.Fprintf(out, "private=%s\npublic=%s\n",
		hex.EncodeToString(priv.Seed()),
		hex.EncodeToString(pub))
	return err
}

func doSign(path string, out io.Writer) error {
	keyHex := os.Getenv("SYLLAGO_SIGNING_PRIVATE_KEY")
	if keyHex == "" {
		return errors.New("SYLLAGO_SIGNING_PRIVATE_KEY is not set")
	}
	seed, err := hex.DecodeString(keyHex)
	if err != nil {
		return fmt.Errorf("decoding private key: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("private key must be %d hex characters (got %d)", ed25519.SeedSize*2, len(keyHex))
	}
	priv := ed25519.NewKeyFromSeed(seed)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	sig := ed25519.Sign(priv, data)
	if _, err := out.Write(sig); err != nil {
		return fmt.Errorf("writing signature: %w", err)
	}
	return nil
}
