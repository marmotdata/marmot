package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// TLSConfig holds TLS configuration for connecting to services with custom certificates.
type TLSConfig struct {
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
	CACertPath         string `mapstructure:"ca_cert_path"`
	CertPath           string `mapstructure:"cert_path"`
	KeyPath            string `mapstructure:"key_path"`
}

// ToTLSConfig builds a *crypto/tls.Config from the struct fields.
func (t *TLSConfig) ToTLSConfig() (*tls.Config, error) {
	if t == nil {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: t.InsecureSkipVerify,
	}

	if t.CACertPath != "" {
		caCert, err := os.ReadFile(t.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", t.CACertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert from %s", t.CACertPath)
		}
		tlsCfg.RootCAs = pool
	}

	if t.CertPath != "" && t.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(t.CertPath, t.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("loading client cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// HTTPClient builds an *http.Client configured with the TLS settings.
// Returns nil, nil when the receiver is nil (callers should use the default client).
func (t *TLSConfig) HTTPClient() (*http.Client, error) {
	if t == nil {
		return nil, nil
	}

	tlsCfg, err := t.ToTLSConfig()
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}, nil
}
