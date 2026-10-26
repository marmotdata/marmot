package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/marmotdata/marmot/internal/pluginserve"
)

const (
	apiServeKeys       = "/api/v1/query/serve/keys"
	apiServeKeysRotate = "/api/v1/query/serve/keys/rotate"
)

var (
	rotateImmediate     bool
	rotateCAKeyURI      string
	rotateSigningKeyURI string
)

var adminServeKeysCmd = &cobra.Command{
	Use:   "serve-keys",
	Short: "List the key generations that secure serving plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		var gens []pluginserve.GenerationStatus
		if err := serveKeysRequest(cmd.Context(), http.MethodGet, apiServeKeys, nil, &gens); err != nil {
			return err
		}
		return printGenerations(gens)
	},
}

var adminRotateServeKeysCmd = &cobra.Command{
	Use:   "rotate-serve-keys",
	Short: "Rotate the CA and query signing keys that secure serving plugins",
	Long: `Starts a new generation of the keys that secure plugins serving queries from outside Marmot.

By default the rotation is graceful: the new keys are published now, used after 20 minutes and the old ones are retired 20 minutes later, so running serving plugins move over without interruption.

Use --immediate when a key may be compromised. Marmot switches at once and stops trusting the old keys, and every serving plugin must be restarted to trust the new ones.

When the keys are held in a KMS, name the new KMS keys with --ca-key-uri and --signing-key-uri.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{
			"immediate":       rotateImmediate,
			"ca_key_uri":      rotateCAKeyURI,
			"signing_key_uri": rotateSigningKeyURI,
		}
		var gens []pluginserve.GenerationStatus
		if err := serveKeysRequest(cmd.Context(), http.MethodPost, apiServeKeysRotate, body, &gens); err != nil {
			return err
		}
		if rotateImmediate {
			fmt.Println("Rotated immediately. Restart every serving plugin so it trusts the new keys.")
		} else {
			fmt.Println("Rotation started. Serving plugins move to the new keys on their own.")
		}
		return printGenerations(gens)
	},
}

func serveKeysRequest(ctx context.Context, method, path string, body, out any) error {
	token, isBearer := getAuthToken()
	client, err := newAPIClient(getHost(), token, isBearer)
	if err != nil {
		return err
	}
	req, err := client.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	return client.do(req, out)
}

func printGenerations(gens []pluginserve.GenerationStatus) error {
	p := getPrinter()
	if p.IsRaw() {
		return p.PrintJSON(gens)
	}
	for _, g := range gens {
		state := "trusted"
		if g.Active {
			state = "active"
		} else if g.ActivatesAt.After(time.Now()) {
			state = "next, active from " + g.ActivatesAt.Format(time.RFC3339)
		}
		if g.RetiresAt != nil {
			state += ", retires " + g.RetiresAt.Format(time.RFC3339)
		}
		fmt.Printf("%s  %-8s  %s\n", g.ID, g.Storage, state)
	}
	return nil
}

func init() {
	adminRotateServeKeysCmd.Flags().BoolVar(&rotateImmediate, "immediate", false, "Switch at once and stop trusting the old keys; serving plugins must be restarted")
	adminRotateServeKeysCmd.Flags().StringVar(&rotateCAKeyURI, "ca-key-uri", "", "KMS URI of the new CA key (ECDSA P-256)")
	adminRotateServeKeysCmd.Flags().StringVar(&rotateSigningKeyURI, "signing-key-uri", "", "KMS URI of the new query signing key (ECDSA P-256)")
	adminCmd.AddCommand(adminServeKeysCmd)
	adminCmd.AddCommand(adminRotateServeKeysCmd)
}
