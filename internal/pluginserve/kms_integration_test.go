package pluginserve

import (
	"bytes"
	"context"
	"crypto/x509"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/store/postgres"
)

// vaultTransitKey creates an ECDSA P-256 key in the Vault transit engine named by VAULT_ADDR and VAULT_TOKEN and returns its sigstore URI. Without MARMOT_TEST_VAULT the test is skipped.
func vaultTransitKey(t *testing.T) string {
	t.Helper()
	if os.Getenv("MARMOT_TEST_VAULT") == "" {
		t.Skip("set MARMOT_TEST_VAULT, VAULT_ADDR and VAULT_TOKEN to run the KMS tests against Vault transit")
	}
	name := "marmot-test-" + uuid.NewString()[:8]
	req, err := http.NewRequest(http.MethodPost, os.Getenv("VAULT_ADDR")+"/v1/transit/keys/"+name, bytes.NewBufferString(`{"type":"ecdsa-p256"}`)) //nolint:gosec // G704: the test's own Vault, from its environment
	require.NoError(t, err)
	req.Header.Set("X-Vault-Token", os.Getenv("VAULT_TOKEN"))
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: the test's own Vault, from its environment
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Less(t, resp.StatusCode, 300, "creating transit key")
	return "hashivault://" + name
}

// With KMS keys configured, Marmot stores only their URIs, never key material, and the whole serving path, certificates and signed queries alike, runs on keys that never leave the KMS.
func TestServeWithVaultTransitKeys(t *testing.T) {
	cfg := KeyConfig{CAKeyURI: vaultTransitKey(t), SigningKeyURI: vaultTransitKey(t)}
	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_POSTGRES_DSN as well")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, postgres.NewSetup(pool).Initialize(ctx))
	_, err = pool.Exec(ctx, `DELETE FROM plugin_serve_keys`)
	require.NoError(t, err)

	store := NewStore(pool)
	keyring, err := LoadKeyring(ctx, store, testEncryptor(t), cfg)
	require.NoError(t, err)
	assert.True(t, keyring.UsesKMS())

	var caKey, signingKey *string
	var caURI, signingURI string
	require.NoError(t, pool.QueryRow(ctx, `SELECT ca_key, signing_key, ca_key_uri, signing_key_uri FROM plugin_serve_keys`).Scan(&caKey, &signingKey, &caURI, &signingURI))
	assert.Nil(t, caKey, "no CA key material is stored")
	assert.Nil(t, signingKey, "no signing key material is stored")
	assert.Equal(t, cfg.CAKeyURI, caURI)
	assert.Equal(t, cfg.SigningKeyURI, signingURI)

	registerServeTestPlugin(t)
	h := &harness{t: t, ctx: ctx, store: store, pool: pool, keyring: keyring,
		schedules: runs.NewScheduleService(runs.NewSchedulePostgresRepository(pool)),
		checker:   &fakeChecker{denied: map[string]bool{}}}
	hub := h.startHub()
	source := newSourceName()
	h.serve(hub, servePrincipal(), source, &fakeQuerier{rows: 4})

	stream, err := hub.ExecuteQuery(ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	assert.Equal(t, 4, collect(t, stream), "a query signed in Vault verifies at the serving process")

	assert.ErrorIs(t, keyring.Rotate(ctx, KeyConfig{}, false), ErrKMSKeysRequired, "a rotation never moves keys out of the KMS")

	// Rotating to new KMS keys works the same way as with stored keys.
	next := KeyConfig{CAKeyURI: vaultTransitKey(t), SigningKeyURI: vaultTransitKey(t)}
	require.NoError(t, keyring.Rotate(ctx, next, true))
	assert.True(t, keyring.UsesKMS())
	hub.sweep()
	cert := hub.cert.Load().tls.Leaf
	_, err = cert.Verify(x509.VerifyOptions{Roots: keyring.Pool(), DNSName: keyring.ServerName(), CurrentTime: time.Now()})
	assert.NoError(t, err, "the replica certificate is reissued by the new KMS-held CA")
}
