package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfig_ToTLSConfig_Nil(t *testing.T) {
	var tc *TLSConfig
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestTLSConfig_ToTLSConfig_Defaults(t *testing.T) {
	tc := &TLSConfig{}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.RootCAs)
	assert.Empty(t, cfg.Certificates)
}

func TestTLSConfig_ToTLSConfig_InsecureSkipVerify(t *testing.T) {
	tc := &TLSConfig{InsecureSkipVerify: true}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	assert.True(t, cfg.InsecureSkipVerify)
}

func TestTLSConfig_ToTLSConfig_CACert(t *testing.T) {
	caPath := writeTestCACert(t)

	tc := &TLSConfig{CACertPath: caPath}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.RootCAs)
}

func TestTLSConfig_ToTLSConfig_CACertNotFound(t *testing.T) {
	tc := &TLSConfig{CACertPath: "/nonexistent/ca.pem"}
	_, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA cert")
}

func TestTLSConfig_ToTLSConfig_CACertInvalid(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "bad-ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("not a cert"), 0o600))

	tc := &TLSConfig{CACertPath: caPath}
	_, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CA cert")
}

func TestTLSConfig_ToTLSConfig_ClientCert(t *testing.T) {
	certPath, keyPath := writeTestClientCert(t)

	tc := &TLSConfig{CertPath: certPath, KeyPath: keyPath}
	cfg, err := tc.ToTLSConfig()
	require.NoError(t, err)
	assert.Len(t, cfg.Certificates, 1)
}

func TestTLSConfig_ToTLSConfig_ClientCertInvalid(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, []byte("not a cert"), 0o600))
	require.NoError(t, os.WriteFile(keyPath, []byte("not a key"), 0o600))

	tc := &TLSConfig{CertPath: certPath, KeyPath: keyPath}
	_, err := tc.ToTLSConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading client cert/key")
}

func TestTLSConfig_HTTPClient_Nil(t *testing.T) {
	var tc *TLSConfig
	client, err := tc.HTTPClient()
	require.NoError(t, err)
	assert.Nil(t, client)
}

func TestTLSConfig_HTTPClient_ReturnsClient(t *testing.T) {
	tc := &TLSConfig{InsecureSkipVerify: true}
	client, err := tc.HTTPClient()
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.Transport)
}

func TestTLSConfig_HTTPClient_PropagatesError(t *testing.T) {
	tc := &TLSConfig{CACertPath: "/nonexistent/ca.pem"}
	_, err := tc.HTTPClient()
	require.Error(t, err)
}

func writeTestCACert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, certPEM, 0o600))
	return caPath
}

func writeTestClientCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	return certPath, keyPath
}
