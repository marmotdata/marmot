package cmd

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"encoding/pem"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIHTTPClientTrustsCAFile(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()

	plain, err := apiHTTPClient(0)
	require.NoError(t, err)
	_, err = plain.Get(server.URL)
	assert.Error(t, err, "a private CA is not trusted without --ca-file")

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600))
	viper.Set("ca_file", caFile)
	t.Cleanup(func() { viper.Set("ca_file", "") })

	trusting, err := apiHTTPClient(0)
	require.NoError(t, err)
	resp, err := trusting.Get(server.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.False(t, trusting.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
	assert.GreaterOrEqual(t, trusting.Transport.(*http.Transport).TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
}

func TestAPIHTTPClientRejectsBadCAFile(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, []byte("not a certificate"), 0o600))
	viper.Set("ca_file", caFile)
	t.Cleanup(func() { viper.Set("ca_file", "") })

	_, err := apiHTTPClient(0)
	assert.Error(t, err)
}
