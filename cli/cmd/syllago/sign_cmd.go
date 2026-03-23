package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var signCmd = &cobra.Command{
	Use:   "sign <name>",
	Short: "Sign hook content for provenance verification",
	Long: `Cryptographically signs a hook's manifest and scripts using
Sigstore (keyless OIDC, default) or GPG (traditional keys).

Signatures enable trust verification during installation.
Recipients can verify that content was signed by a known identity
and hasn't been tampered with since signing.`,
	Example: `  # Sign with Sigstore (keyless — uses GitHub/Google identity)
  syllago sign my-hook

  # Sign with GPG
  syllago sign my-hook --method gpg --key-id 0xABCDEF12

  # Verify a signed hook
  syllago verify my-hook`,
	Args: cobra.ExactArgs(1),
	RunE: runSign,
}

var verifyCmd = &cobra.Command{
	Use:   "verify <name>",
	Short: "Verify hook content signatures",
	Long: `Checks cryptographic signatures on hook content against the
configured trust policy. Reports the signer identity and whether
the content is trusted.`,
	Example: `  # Verify a hook's signature
  syllago verify my-hook

  # Verify with verbose output
  syllago verify my-hook --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: runVerify,
}

func init() {
	signCmd.Flags().String("method", "sigstore", "Signing method: sigstore (default) or gpg")
	signCmd.Flags().String("key-id", "", "GPG key ID (required when --method=gpg)")
	rootCmd.AddCommand(signCmd)

	verifyCmd.Flags().Bool("verbose", false, "Show detailed verification info")
	rootCmd.AddCommand(verifyCmd)
}

func runSign(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("signing is not yet implemented")
}

func runVerify(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("verification is not yet implemented")
}
