package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateKeyCmd)
}

var generateKeyCmd = &cobra.Command{
	Use:   "generate-encryption-key",
	Short: "Generate a secure encryption key for Marmot",
	Long: `Generate a secure 32-byte encryption key for encrypting sensitive pipeline credentials.

This key is used to encrypt sensitive fields (passwords, API keys, tokens) in pipeline
configurations before storing them in the database.

IMPORTANT SECURITY NOTES:
- Store this key securely (password manager, secrets vault, etc.)
- Back up this key in a secure location
- Loss of this key means permanent loss of encrypted credentials
- Do not commit this key to version control
- Use environment variables or Kubernetes secrets to configure it`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateEncryptionKey()
	},
}

func generateEncryptionKey() error {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}

	encodedKey := base64.StdEncoding.EncodeToString(key)

	fmt.Println()
	fmt.Printf("  Your encryption key:\n\n")
	fmt.Printf("    %s\n\n", encodedKey)
	fmt.Printf("  Set it as an environment variable:\n\n")
	fmt.Printf("    export MARMOT_SERVER_ENCRYPTION_KEY=\"%s\"\n\n", encodedKey)
	fmt.Printf("  Store this key somewhere safe — losing it means losing access\n")
	fmt.Printf("  to encrypted credentials. See https://marmotdata.io/docs/Deploy\n")
	fmt.Printf("  for full configuration options.\n")
	fmt.Println()

	return nil
}
