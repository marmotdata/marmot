package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"context"

	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/marmotdata/marmot/internal/plugin/install"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	globalHost   string
	globalAPIKey string
	globalOutput string
)

var rootCmd = &cobra.Command{
	Use:   "marmot",
	Short: "Marmot is a simple to use Data Catalog.",
	// A failed request is not a usage error, so do not print the usage text
	// after it. Flag and argument mistakes still show it.
	SilenceUsage: true,
	// Subcommands must not define their own PersistentPreRunE, which would replace this one and skip read-only mode.
	PersistentPreRunE: enforceReadOnly,
}

var globalReadOnly bool

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&globalHost, "host", "", "Marmot API host (default: http://localhost:8080)")
	rootCmd.PersistentFlags().StringVar(&globalAPIKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().String("ca-file", "", "PEM file of a CA to trust for the Marmot API, in addition to the system's (also MARMOT_CA_FILE)")
	rootCmd.PersistentFlags().StringVarP(&globalOutput, "output", "o", "", "Output format: table, json, yaml (default: table)")

	rootCmd.PersistentFlags().BoolVar(&globalReadOnly, "read-only", false, "Refuse every command that is not a read (also MARMOT_READ_ONLY)")

	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	_ = viper.BindPFlag("read_only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	_ = viper.BindPFlag("ca_file", rootCmd.PersistentFlags().Lookup("ca-file"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	viper.SetDefault("host", "http://localhost:8080")
	viper.SetDefault("output", "table")
}

func initConfig() {
	viper.SetEnvPrefix("MARMOT")
	viper.AutomaticEnv()

	// Map env var names: MARMOT_HOST, MARMOT_API_KEY, MARMOT_OUTPUT
	_ = viper.BindEnv("host", "MARMOT_HOST")
	_ = viper.BindEnv("api_key", "MARMOT_API_KEY")
	_ = viper.BindEnv("output", "MARMOT_OUTPUT")
	_ = viper.BindEnv("read_only", "MARMOT_READ_ONLY")
	_ = viper.BindEnv("ca_file", "MARMOT_CA_FILE")

	// Config file: ~/.config/marmot/config.yaml
	configDir, err := os.UserConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, "marmot")
		viper.AddConfigPath(configPath)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		_ = viper.ReadInConfig() // config file is optional
	}
}

// getPrinter creates a Printer based on resolved config.
func getPrinter() *output.Printer {
	return output.NewPrinter(viper.GetString("output"), os.Stdout)
}

// getHost returns the resolved host: --host flag > active context > legacy config.
func getHost() string {
	return resolveHost()
}

// getAPIKey returns the resolved API key for commands that need it directly.
func getAPIKey() string {
	return viper.GetString("api_key")
}

// k8sTokenPaths are the in-cluster ServiceAccount tokens the CLI can authenticate with, in order. Operator-launched pods mount only the first, projected for the Marmot audience so it is useless against the Kubernetes API; the default token is kept as a fallback for pods that authenticate by OIDC token exchange with the cluster's issuer.
var k8sTokenPaths = []string{
	"/var/run/secrets/marmot/token",
	"/var/run/secrets/kubernetes.io/serviceaccount/token", //nolint:gosec
}

// getAuthToken returns the auth token and whether it should use Bearer auth.
// Priority: --api-key flag > active context OAuth token > config/env API key > K8s SA token.
func getAuthToken() (token string, isBearerToken bool) {
	if globalAPIKey != "" {
		return globalAPIKey, false
	}
	if ctx := currentContextName(); ctx != "" {
		if cached, ok := getCachedToken(ctx); ok {
			return cached, true
		}
	}
	if key := getAPIKey(); key != "" {
		return key, false
	}
	for _, path := range k8sTokenPaths {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)), true
		}
	}
	return "", false
}

// configDir returns the marmot config directory path.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(base, "marmot"), nil
}

// loadPlugins downloads core plugins that this build's manifest pins
// but the cache does not hold yet, then registers plugins: locally
// installed ones first, so they shadow the pinned cached core plugins.
// Commands that run plugins, like ingest, call this before doing their
// work; the server has its own startup equivalent driven by its config
// file.
func loadPlugins() {
	opts := install.Options{Registry: os.Getenv("MARMOT_PLUGINS_REGISTRY")}
	if os.Getenv("MARMOT_PLUGINS_AUTOINSTALL") != "false" {
		if err := install.EnsureCore(context.Background(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to install core plugins: %v\n", err)
		}
	}
	if err := install.LoadPlugins(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load external plugins: %v\n", err)
	}
}

// Execute runs the CLI; pass its error to ExitCode for the process exit code.
func Execute() error {
	markUsageErrors(rootCmd)
	err := rootCmd.Execute()
	if err != nil && strings.HasPrefix(err.Error(), "unknown command") {
		return withExitCode(ExitUsage, err)
	}
	return err
}

// apiHTTPClient returns the HTTP client for the Marmot API, trusting the CA in ca_file as well as the system's so a Marmot behind a private CA needs no verification turned off.
func apiHTTPClient(timeout time.Duration) (*http.Client, error) {
	client := &http.Client{Timeout: timeout}
	caFile := viper.GetString("ca_file")
	if caFile == "" {
		return client, nil
	}

	data, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA file: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("CA file %s holds no PEM certificates", caFile)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	client.Transport = transport
	return client, nil
}
