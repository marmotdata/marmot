package pluginserve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/plugin"
	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
	"github.com/marmotdata/marmot/internal/store/postgres"
)

// harness is a migrated database with a serving identity, shared by the hubs and serving processes in one test. Tests are skipped without MARMOT_TEST_POSTGRES_DSN, so `go test ./...` stays offline.
type harness struct {
	t         *testing.T
	ctx       context.Context
	store     *Store
	pool      *pgxpool.Pool
	keyring   *Keyring
	schedules *runs.ScheduleService
	checker   *fakeChecker
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_POSTGRES_DSN to run the serving integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, postgres.NewSetup(pool).Initialize(ctx))

	// Each run makes a fresh encryption key, so keys left by an earlier run could never be decrypted.
	_, err = pool.Exec(ctx, `DELETE FROM plugin_serve_keys`)
	require.NoError(t, err)

	store := NewStore(pool)
	keyring, err := LoadKeyring(ctx, store, testEncryptor(t), KeyConfig{})
	require.NoError(t, err)

	registerServeTestPlugin(t)
	return &harness{
		t:         t,
		ctx:       ctx,
		store:     store,
		pool:      pool,
		keyring:   keyring,
		schedules: runs.NewScheduleService(runs.NewSchedulePostgresRepository(pool)),
		checker:   &fakeChecker{denied: map[string]bool{}},
	}
}

func registerServeTestPlugin(t *testing.T) {
	t.Helper()
	if _, err := plugin.GetRegistry().Get("servetest"); err == nil {
		return
	}
	require.NoError(t, plugin.GetRegistry().Register(pluginsdk.Meta{
		ID:            "servetest",
		SupportsQuery: true,
		ConfigSpec: []pluginsdk.ConfigField{
			{Name: "host"},
			{Name: "password", Sensitive: true},
		},
	}, nil))
}

type fakeChecker struct {
	mu     sync.Mutex
	denied map[string]bool
}

func (c *fakeChecker) MayServe(_ context.Context, e *Enrolment) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.denied[e.PrincipalID], nil
}

func (c *fakeChecker) deny(principalID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.denied[principalID] = true
}

func (h *harness) owners(ctx context.Context, source string) (Owner, error) {
	s, err := h.schedules.GetScheduleByName(ctx, source)
	if err != nil {
		return Owner{}, err
	}
	if !s.Served() {
		return Owner{}, errors.New("not served")
	}
	return Owner{Type: *s.ServedByType, ID: *s.ServedByID}, nil
}

func (h *harness) startHub() *Hub {
	h.t.Helper()
	hub := NewHub(h.keyring, h.store, h.owners, h.checker, HubConfig{ListenAddress: "127.0.0.1:0"})
	require.NoError(h.t, hub.Start())
	hub.cfg.ReplicaAddress = hub.Addr().String()
	h.t.Cleanup(hub.Stop)
	return hub
}

// sessionCredential stands in for a login session in tests that are not about API key revocation.
var sessionCredential = auth.Credential{Type: auth.CredentialSession}

func servePrincipal() auth.Principal {
	return auth.NewServiceAccountPrincipal(uuid.NewString(), "ci", nil, []string{"ingestion:serve"})
}

func newSourceName() string {
	return "served-" + uuid.NewString()[:8]
}

func (h *harness) enroller(hub *Hub) *Enroller {
	return NewEnroller(h.keyring, h.store, h.schedules, hub.Addr().String(), "")
}

// serve runs a serving process connected to hub until the test ends and waits for its session.
func (h *harness) serve(hub *Hub, principal auth.Principal, source string, querier Querier) {
	h.t.Helper()
	h.serveWith(h.enroller(hub), hub, principal, sessionCredential, source, querier)
}

func (h *harness) serveWith(enroller *Enroller, hub *Hub, principal auth.Principal, credential auth.Credential, source string, querier Querier) {
	h.t.Helper()
	ctx, cancel := context.WithCancel(h.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Serve(ctx, ServeOptions{
			Sources: []LocalSource{{
				Name:          source,
				PluginID:      "servetest",
				Config:        map[string]any{"host": "db.internal", "password": "s3cret"},
				DisplayConfig: map[string]any{"host": "db.internal", "password": "s3cret"},
			}},
			Enrol: func(ctx context.Context, in EnrolInput) (*EnrolResult, error) {
				return enroller.Enrol(ctx, principal, credential, in)
			},
			Querier: querier,
		})
	}()
	h.t.Cleanup(func() { cancel(); <-done })

	owner := Owner{Type: string(principal.Type()), ID: principal.ID()}
	require.Eventually(h.t, func() bool { return hub.localSession(source, owner) != nil }, 5*time.Second, 20*time.Millisecond)
}

// fakeQuerier stands in for the local plugin processes and records what it was asked.
type fakeQuerier struct {
	mu      sync.Mutex
	rows    int
	configs []map[string]any
	reqs    []pluginsdk.QueryRequest
}

func (f *fakeQuerier) record(config map[string]any, req pluginsdk.QueryRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, config)
	f.reqs = append(f.reqs, req)
}

func (f *fakeQuerier) PlanQuery(_ context.Context, _ string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	f.record(config, req)
	return &pluginsdk.QueryPlan{ReferencedMRNs: []string{"mrn://table/postgresql/orders"}}, nil
}

func (f *fakeQuerier) ExecuteQuery(_ context.Context, _ string, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	f.record(config, req)
	return &fakeStream{left: f.rows}, nil
}

type fakeStream struct {
	left  int
	first bool
}

func (s *fakeStream) Recv() (*pluginsdk.QueryResultChunk, error) {
	if s.left == 0 {
		return nil, io.EOF
	}
	chunk := &pluginsdk.QueryResultChunk{}
	if !s.first {
		chunk.Columns = []pluginsdk.QueryColumn{{Name: "id", Type: "int"}}
		s.first = true
	}
	for i := 0; i < 10 && s.left > 0; i++ {
		chunk.Rows = append(chunk.Rows, []any{float64(s.left)})
		s.left--
	}
	return chunk, nil
}

func (s *fakeStream) Close() error { return nil }

func collect(t *testing.T, stream pluginsdk.QueryStream) int {
	t.Helper()
	defer func() { _ = stream.Close() }()
	rows := 0
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return rows
		}
		require.NoError(t, err)
		rows += len(chunk.Rows)
	}
}

func TestServeEndToEnd(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	querier := &fakeQuerier{rows: 25}
	h.serve(hub, principal, source, querier)

	plan, err := hub.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT * FROM orders", MaxRows: 100, Identity: "user:alice"})
	require.NoError(t, err)
	assert.Equal(t, []string{"mrn://table/postgresql/orders"}, plan.ReferencedMRNs)

	stream, err := hub.ExecuteQuery(h.ctx, source, uuid.NewString(), pluginsdk.QueryRequest{Statement: "SELECT * FROM orders", MaxRows: 100, Identity: "user:alice"})
	require.NoError(t, err)
	assert.Equal(t, 25, collect(t, stream))

	querier.mu.Lock()
	assert.Equal(t, "s3cret", querier.configs[0]["password"], "the plugin runs with the serving process's own config")
	assert.Equal(t, "user:alice", querier.reqs[0].Identity)
	querier.mu.Unlock()

	stored, err := h.schedules.GetScheduleByName(h.ctx, source)
	require.NoError(t, err)
	assert.Equal(t, plugin.SensitiveMask, stored.Config["password"], "Marmot never stores a served source's credentials")
	assert.True(t, stored.Served())
}

func TestServeEnforcesSignedRowLimit(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	h.serve(hub, servePrincipal(), source, &fakeQuerier{rows: 1000})

	stream, err := hub.ExecuteQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 15})
	require.NoError(t, err)
	assert.Equal(t, 15, collect(t, stream), "the serving process stops at the row limit Marmot signed, whatever the plugin returns")
}

func TestServeForwardsBetweenReplicas(t *testing.T) {
	h := newHarness(t)
	connected, other := h.startHub(), h.startHub()
	source := newSourceName()
	h.serve(connected, servePrincipal(), source, &fakeQuerier{rows: 3})

	plan, err := other.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, plan.ReferencedMRNs)

	stream, err := other.ExecuteQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	assert.Equal(t, 3, collect(t, stream))
}

func TestServeQueryWithNoConnection(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()

	// Claimed, but nothing is connected for it.
	_, err := h.enroller(hub).Enrol(h.ctx, principal, sessionCredential, enrolInput(t, nil, source))
	require.NoError(t, err)

	_, err = hub.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
	assert.ErrorIs(t, err, ErrNotConnected)
}

// A connection serving one source is never used for another, even one its own principal owns.
func TestServeRoutesOnlyEnrolledSources(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	served, other := newSourceName(), newSourceName()
	h.serve(hub, principal, served, &fakeQuerier{rows: 1})
	_, err := h.enroller(hub).Enrol(h.ctx, principal, sessionCredential, enrolInput(t, nil, other))
	require.NoError(t, err)

	_, err = hub.PlanQuery(h.ctx, other, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestServeStopsWhenSourceDeleted(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	h.serve(hub, servePrincipal(), source, &fakeQuerier{rows: 1})

	stored, err := h.schedules.GetScheduleByName(h.ctx, source)
	require.NoError(t, err)
	require.NoError(t, h.schedules.DeleteSchedule(h.ctx, stored.ID))

	_, err = hub.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
	assert.Error(t, err)
}

func TestSweepClosesSessionsOfPrincipalsThatMayNoLongerServe(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	h.serve(hub, principal, source, &fakeQuerier{rows: 1})

	h.checker.deny(principal.ID())
	hub.sweep()

	owner := Owner{Type: string(principal.Type()), ID: principal.ID()}
	assert.Nil(t, hub.localSession(source, owner))
	_, err := hub.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
	assert.Error(t, err)
}

func TestEnrolOwnership(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	enroller := h.enroller(hub)
	owner, intruder := servePrincipal(), servePrincipal()
	source := newSourceName()

	_, err := enroller.Enrol(h.ctx, owner, sessionCredential, enrolInput(t, nil, source))
	require.NoError(t, err)

	_, err = enroller.Enrol(h.ctx, intruder, sessionCredential, enrolInput(t, nil, source))
	assert.ErrorIs(t, err, ErrForbidden, "another principal cannot take over a served source")

	noPermission := auth.NewServiceAccountPrincipal(uuid.NewString(), "reader", nil, []string{"assets:query"})
	_, err = enroller.Enrol(h.ctx, noPermission, sessionCredential, enrolInput(t, nil, newSourceName()))
	assert.ErrorIs(t, err, ErrForbidden)

	pipeline, err := h.schedules.CreateSchedule(h.ctx, newSourceName(), "servetest", map[string]any{}, "", true, true, false, nil)
	require.NoError(t, err)
	_, err = enroller.Enrol(h.ctx, owner, sessionCredential, enrolInput(t, nil, pipeline.Name))
	assert.ErrorIs(t, err, ErrForbidden, "a pipeline Marmot runs cannot be claimed")
}

func TestEnrolValidatesInput(t *testing.T) {
	h := newHarness(t)
	enroller := h.enroller(h.startHub())
	principal := servePrincipal()

	for name, in := range map[string]EnrolInput{
		"no csr":         {Sources: []EnrolSource{{Name: "ok", PluginID: "servetest"}}},
		"no sources":     enrolInput(t, nil),
		"bad name":       enrolInput(t, nil, "../etc"),
		"duplicate name": enrolInput(t, nil, "dup", "dup"),
		"no plugin":      {CSR: enrolInput(t, nil, "x").CSR, Sources: []EnrolSource{{Name: "x"}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := enroller.Enrol(h.ctx, principal, sessionCredential, in)
			assert.ErrorIs(t, err, ErrInvalidEnrolment)
		})
	}
}

// enrolInput builds an enrolment for sources, with a CSR for key or a fresh key.
func enrolInput(t *testing.T, key *ecdsa.PrivateKey, sources ...string) EnrolInput {
	t.Helper()
	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	require.NoError(t, err)
	in := EnrolInput{CSR: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))}
	for _, s := range sources {
		in.Sources = append(in.Sources, EnrolSource{Name: s, PluginID: "servetest"})
	}
	return in
}

// dialHub opens a TLS connection to hub with cfg and reports whether the hub kept it: TLS 1.3 completes the client's side of the handshake before the server has checked the client certificate, so refusal shows up as the connection being closed.
func dialHub(t *testing.T, hub *Hub, cfg *tls.Config) bool {
	t.Helper()
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", hub.Addr().String(), cfg)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// enrolledCert enrols a fresh key and returns a client TLS config presenting its certificate.
func (h *harness) enrolledCert(hub *Hub, principal auth.Principal) (*tls.Config, *EnrolResult, *ecdsa.PrivateKey) {
	h.t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(h.t, err)
	res, err := h.enroller(hub).Enrol(h.ctx, principal, sessionCredential, enrolInput(h.t, key, newSourceName()))
	require.NoError(h.t, err)
	block, _ := pem.Decode([]byte(res.Certificate))
	pool := x509.NewCertPool()
	require.True(h.t, pool.AppendCertsFromPEM([]byte(res.CABundle)))
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      pool,
		ServerName:   ServerNameFor(res.InstanceID),
		Certificates: []tls.Certificate{{Certificate: [][]byte{block.Bytes}, PrivateKey: key}},
	}, res, key
}

func TestHubRefusesUnauthenticatedConnections(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()

	valid, _, _ := h.enrolledCert(hub, principal)
	require.True(t, dialHub(t, hub, valid), "control: an enrolled certificate is accepted")

	t.Run("no client certificate", func(t *testing.T) {
		cfg := valid.Clone()
		cfg.Certificates = nil
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("certificate from another CA naming this instance", func(t *testing.T) {
		rogue := newTestKeyring(t)
		rogue.InstanceID = h.keyring.InstanceID
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		csr, err := ParseCSR(enrolInput(t, key, "x").CSR)
		require.NoError(t, err)
		der, err := rogue.IssueServeCert(csr, uuid.NewString(), time.Now().Add(time.Hour))
		require.NoError(t, err)
		cfg := valid.Clone()
		cfg.Certificates = []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("TLS 1.2", func(t *testing.T) {
		cfg := valid.Clone()
		cfg.MinVersion, cfg.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("principal that may no longer serve", func(t *testing.T) {
		denied := servePrincipal()
		cfg, _, _ := h.enrolledCert(hub, denied)
		h.checker.deny(denied.ID())
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("certificate whose key differs from its enrolment", func(t *testing.T) {
		_, res, _ := h.enrolledCert(hub, principal)
		otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		csr, err := ParseCSR(enrolInput(t, otherKey, "x").CSR)
		require.NoError(t, err)
		der, err := h.keyring.IssueServeCert(csr, res.EnrolmentID, time.Now().Add(time.Hour))
		require.NoError(t, err)
		cfg := valid.Clone()
		cfg.Certificates = []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: otherKey}}
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("expired certificate", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		_, res, _ := h.enrolledCert(hub, principal)
		csr, err := ParseCSR(enrolInput(t, key, "x").CSR)
		require.NoError(t, err)
		der, err := h.keyring.IssueServeCert(csr, res.EnrolmentID, time.Now().Add(-time.Minute))
		require.NoError(t, err)
		cfg := valid.Clone()
		cfg.Certificates = []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}
		assert.False(t, dialHub(t, hub, cfg))
	})

	t.Run("serving process certificate used to impersonate a replica", func(t *testing.T) {
		replica := replicaClientWith(t, hub, valid)
		_, err := replica.PlanQuery(h.ctx, forwarded(1, "anything", "", pluginsdk.QueryRequest{}))
		assert.Error(t, err)
	})
}

func TestServingProcessRejectsAnImpostorMarmot(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	impostor := NewHub(newTestKeyring(t), h.store, h.owners, h.checker, HubConfig{ListenAddress: "127.0.0.1:0"})
	require.NoError(t, impostor.Start())
	t.Cleanup(impostor.Stop)

	cfg, _, _ := h.enrolledCert(hub, principal)
	_, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", impostor.Addr().String(), cfg)
	assert.Error(t, err, "a server whose certificate this instance's CA did not issue is refused")
}

// replicaClientWith dials hub's replica service presenting cfg's certificate, as a replica would.
func replicaClientWith(t *testing.T, hub *Hub, cfg *tls.Config) pb.ReplicaClient {
	t.Helper()
	conn, err := grpc.NewClient(hub.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(cfg.Clone())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewReplicaClient(conn)
}

// A serving process replaces its certificate at half-life with a fresh key and enrolment, and keeps answering across the switch.
func TestServeRenewsCertificateBeforeExpiry(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	enroller := h.enroller(hub)
	enroller.lifetime = 12 * time.Second
	h.serveWith(enroller, hub, principal, sessionCredential, source, &fakeQuerier{rows: 1})

	owner := Owner{Type: string(principal.Type()), ID: principal.ID()}
	first := hub.localSession(source, owner).enrolment.ID

	require.Eventually(t, func() bool {
		s := hub.localSession(source, owner)
		return s != nil && s.enrolment.ID != first
	}, 10*time.Second, 100*time.Millisecond, "a new enrolment takes over at half-life")

	stream, err := hub.ExecuteQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, collect(t, stream))
}

func (h *harness) sessionFor(hub *Hub, principal auth.Principal, source string) *session {
	return hub.localSession(source, Owner{Type: string(principal.Type()), ID: principal.ID()})
}

func (h *harness) planWorks(hub *Hub, source string) error {
	_, err := hub.PlanQuery(h.ctx, source, pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
	return err
}

// generationIDs returns the stored generations, oldest first.
func (h *harness) generationIDs() []string {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx, `SELECT id::text FROM plugin_serve_keys ORDER BY created_at`)
	require.NoError(h.t, err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(h.t, err)
	return ids
}

// A graceful rotation moves a running serving process to the new keys without a restart: it learns them while the old ones are still in use, keeps answering once Marmot signs with the new key, and reconnects under the new CA when the old one is retired.
func TestGracefulRotationKeepsServing(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	enroller := h.enroller(hub)
	enroller.lifetime = 12 * time.Second
	h.serveWith(enroller, hub, principal, sessionCredential, source, &fakeQuerier{rows: 1})
	before := h.sessionFor(hub, principal, source).enrolment.ID

	require.NoError(t, h.keyring.Rotate(h.ctx, KeyConfig{}, false))
	gens := h.generationIDs()
	require.Len(t, gens, 2)
	oldGen, newGen := gens[0], gens[1]
	assert.Equal(t, oldGen, h.keyring.activeID(), "the new keys are published but not yet used")

	// Wait for the process to renew, which pins both generations.
	require.Eventually(t, func() bool {
		s := h.sessionFor(hub, principal, source)
		return s != nil && s.enrolment.ID != before
	}, 10*time.Second, 100*time.Millisecond)

	// Fast-forward to activation: Marmot now signs with the new key.
	_, err := h.pool.Exec(h.ctx, `UPDATE plugin_serve_keys SET activates_at = NOW() WHERE id = $1`, newGen)
	require.NoError(t, err)
	hub.sweep()
	require.Equal(t, newGen, h.keyring.activeID())
	assert.Equal(t, newGen, hub.cert.Load().generation, "the replica renews its certificate under the new CA")
	require.NoError(t, h.planWorks(hub, source), "a query signed with the new key verifies")

	// Fast-forward to retirement: the old CA is no longer trusted.
	_, err = h.pool.Exec(h.ctx, `UPDATE plugin_serve_keys SET retires_at = NOW() WHERE id = $1`, oldGen)
	require.NoError(t, err)
	hub.sweep()
	assert.Equal(t, []GenerationStatus{{ID: newGen, Storage: "database", Active: true, ActivatesAt: h.keyring.Generations()[0].ActivatesAt}}, h.keyring.Generations())

	require.Eventually(t, func() bool { return h.planWorks(hub, source) == nil }, 10*time.Second, 100*time.Millisecond,
		"the process reconnects under the new CA without being restarted")
	s := h.sessionFor(hub, principal, source)
	_, err = s.leaf.Verify(x509.VerifyOptions{Roots: h.keyring.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	assert.NoError(t, err)
}

// An immediate rotation is for a compromise: Marmot drops every connection made under the old keys at once, and a running serving process refuses the new identity it has no overlap with until it is restarted.
func TestImmediateRotationCutsOffServingProcesses(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	h.serve(hub, principal, source, &fakeQuerier{rows: 1})

	require.NoError(t, h.keyring.Rotate(h.ctx, KeyConfig{}, true))
	hub.sweep()

	assert.Nil(t, h.sessionFor(hub, principal, source))
	assert.Never(t, func() bool { return h.sessionFor(hub, principal, source) != nil }, 3*time.Second, 100*time.Millisecond,
		"the running process does not accept keys it was never told about")
	assert.Error(t, h.planWorks(hub, source))

	// A restarted process trusts the new identity from its first enrolment.
	h.serve(hub, principal, source, &fakeQuerier{rows: 1})
	assert.NoError(t, h.planWorks(hub, source))
}

// serviceAccountKey creates a service account and an API key for it, returning the credential an enrolment with that key records and the key's id.
func (h *harness) serviceAccountKey(withLookup bool) (auth.Principal, auth.Credential, string) {
	h.t.Helper()
	var saID string
	require.NoError(h.t, h.pool.QueryRow(h.ctx, `INSERT INTO service_accounts (name) VALUES ($1) RETURNING id`, "serve-"+uuid.NewString()).Scan(&saID))

	secret := uuid.NewString()
	lookup := serviceaccount.APIKeyLookup(secret)
	var stored *string
	if withLookup {
		stored = &lookup
	}
	var keyID string
	require.NoError(h.t, h.pool.QueryRow(h.ctx,
		`INSERT INTO service_account_api_keys (service_account_id, name, key_hash, key_lookup) VALUES ($1, 'serve', 'unused', $2) RETURNING id`,
		saID, stored).Scan(&keyID))

	principal := auth.NewServiceAccountPrincipal(saID, "ci", nil, []string{"ingestion:serve"})
	return principal, auth.Credential{Type: auth.CredentialServiceAccountAPIKey, Lookup: lookup}, keyID
}

// Deleting the API key a serving process enrolled with cuts it off within one sweep, not when its certificate expires.
func TestDeletingTheAPIKeyClosesTheConnection(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal, credential, keyID := h.serviceAccountKey(true)
	source := newSourceName()
	h.serveWith(h.enroller(hub), hub, principal, credential, source, &fakeQuerier{rows: 1})
	require.NoError(t, h.planWorks(hub, source))

	_, err := h.pool.Exec(h.ctx, `DELETE FROM service_account_api_keys WHERE id = $1`, keyID)
	require.NoError(t, err)
	hub.sweep()

	assert.Nil(t, h.sessionFor(hub, principal, source))
	assert.Error(t, h.planWorks(hub, source))
	_, err = h.enroller(hub).Enrol(h.ctx, principal, credential, enrolInput(t, nil, source))
	assert.ErrorIs(t, err, ErrForbidden, "the deleted key cannot enrol again")
}

func TestExpiredAPIKeyClosesTheConnection(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal, credential, keyID := h.serviceAccountKey(true)
	source := newSourceName()
	h.serveWith(h.enroller(hub), hub, principal, credential, source, &fakeQuerier{rows: 1})

	_, err := h.pool.Exec(h.ctx, `UPDATE service_account_api_keys SET expires_at = NOW() - INTERVAL '1 second' WHERE id = $1`, keyID)
	require.NoError(t, err)
	hub.sweep()
	assert.Nil(t, h.sessionFor(hub, principal, source))
}

// A key issued before keys had lookups could not be watched for revocation, so it cannot serve.
func TestUntrackableAPIKeyCannotEnrol(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal, credential, _ := h.serviceAccountKey(false)

	_, err := h.enroller(hub).Enrol(h.ctx, principal, credential, enrolInput(t, nil, newSourceName()))
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestEnrolRefusesUnknownCredential(t *testing.T) {
	h := newHarness(t)
	_, err := h.enroller(h.startHub()).Enrol(h.ctx, servePrincipal(), auth.Credential{}, enrolInput(t, nil, newSourceName()))
	assert.ErrorIs(t, err, ErrForbidden)
}
