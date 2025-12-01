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

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Generated Encryption Key")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  %s\n", encodedKey)
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Configuration Instructions")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("1. Environment Variable (recommended for bare metal):")
	fmt.Println()
	fmt.Printf("   export MARMOT_SERVER_ENCRYPTION_KEY=\"%s\"\n", encodedKey)
	fmt.Println()
	fmt.Println("2. Configuration File:")
	fmt.Println()
	fmt.Println("   server:")
	fmt.Printf("     encryption_key: \"%s\"\n", encodedKey)
	fmt.Println()
	fmt.Println("3. Kubernetes Secret (recommended for Kubernetes):")
	fmt.Println()
	fmt.Printf("   kubectl create secret generic marmot-encryption-key \\\n")
	fmt.Printf("     --from-literal=encryption-key=\"%s\"\n", encodedKey)
	fmt.Println()
	fmt.Println("   Then in your values.yaml:")
	fmt.Println()
	fmt.Println("   config:")
	fmt.Println("     server:")
	fmt.Println("       encryptionKeySecretRef:")
	fmt.Println("         name: marmot-encryption-key")
	fmt.Println("         key: encryption-key")
	fmt.Println()
	fmt.Println("4. Docker Environment Variable:")
	fmt.Println()
	fmt.Printf("   docker run -e MARMOT_SERVER_ENCRYPTION_KEY=\"%s\" \\\n", encodedKey)
	fmt.Println("     ghcr.io/marmotdata/marmot:latest")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("⚠️  IMPORTANT SECURITY REMINDERS")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  • DO NOT share this key.")
	fmt.Println("  • LOSS OF THIS KEY means permanent loss of encrypted credentials")
	fmt.Println("  • Rotate this key periodically for enhanced security")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")

	return nil
}
