package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDisallowedIP(t *testing.T) {
	disallowed := []string{
		"127.0.0.1", "127.8.9.10", "::1",
		"0.0.0.0", "::", "0.1.2.3",
		"10.0.0.1", "172.16.0.1", "192.168.1.1", "fd00::1",
		"169.254.169.254", "fe80::1",
		"100.64.0.1", "100.100.100.200",
		"224.0.0.1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", "::ffff:0.0.0.0",
		"64:ff9b::7f00:1",
	}
	for _, s := range disallowed {
		assert.True(t, isDisallowedIP(netip.MustParseAddr(s)), s)
	}

	allowed := []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "::ffff:1.1.1.1"}
	for _, s := range allowed {
		assert.False(t, isDisallowedIP(netip.MustParseAddr(s)), s)
	}
}

func TestValidateWebhookURL(t *testing.T) {
	rejected := []string{
		"http://localhost:8080/hook",
		"http://LOCALHOST/hook",
		"http://127.0.0.1:18080/health",
		"http://0.0.0.0:18099/hook",
		"http://[::]:18099/hook",
		"http://[::1]/hook",
		"http://[::ffff:127.0.0.1]/hook",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/hook",
		"http://does-not-exist.invalid/hook",
		"ftp://example.com/hook",
	}
	for _, u := range rejected {
		assert.NotNil(t, validateWebhookURL(u), u)
	}

	assert.Nil(t, validateWebhookURL("https://1.1.1.1/hook"))
}

func TestDeliveryClientBlocksInternalDestinations(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	client := newDeliveryClient(5 * time.Second)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, strings.NewReader("{}"))
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errDisallowedAddress), err.Error())
	assert.False(t, hit)
}

func TestDeliveryClientDoesNotFollowRedirects(t *testing.T) {
	client := newDeliveryClient(time.Second)
	require.NotNil(t, client.CheckRedirect)
	assert.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestDialControl(t *testing.T) {
	assert.ErrorIs(t, dialControl("tcp", "[::]:18099", nil), errDisallowedAddress)
	assert.ErrorIs(t, dialControl("tcp", "127.0.0.1:80", nil), errDisallowedAddress)
	assert.ErrorIs(t, dialControl("tcp", "not-an-address", nil), errDisallowedAddress)
	assert.NoError(t, dialControl("tcp", "1.1.1.1:443", nil))
}
