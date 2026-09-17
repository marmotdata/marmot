package oauth2

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/fosite"
)

// newPostgresRepository connects to the database named by
// MARMOT_TEST_POSTGRES_DSN and creates the two tables the oauth flow state
// migration adds. It finds that migration by name rather than by number, so
// renumbering it cannot silently skip the schema and fail every test below.
// Without that variable the test is skipped, so `go test ./...` stays offline.
func newPostgresRepository(t *testing.T) (Repository, context.Context) {
	t.Helper()

	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_POSTGRES_DSN to run the Postgres persistence tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	matches, err := filepath.Glob("../store/postgres/migrations/*_oauth_flow_state.sql")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one oauth flow state migration, got %v (%v)", matches, err)
	}
	schema, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	create, _, ok := strings.Cut(string(schema), "---- create above / drop below ----")
	if !ok {
		t.Fatal("migration is missing the create/drop separator")
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS oauth_sessions, oauth_clients"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := pool.Exec(ctx, create); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	return NewPostgresRepository(pool), ctx
}

func TestPostgres_ClientRoundTrip(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	want := &ClientRecord{
		ID:            "client-1",
		RedirectURIs:  []string{"http://localhost:9999/callback"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}
	if err := p.PutClient(ctx, want, time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := p.GetClient(ctx, "client-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.RedirectURIs) != 1 || got.RedirectURIs[0] != want.RedirectURIs[0] {
		t.Fatalf("redirect URIs did not round trip: %v", got.RedirectURIs)
	}
	if len(got.GrantTypes) != 1 || got.GrantTypes[0] != "authorization_code" {
		t.Fatalf("grant types did not round trip: %v", got.GrantTypes)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "openid" {
		t.Fatalf("scopes did not round trip: %v", got.Scopes)
	}
}

func TestPostgres_ClientMissingAndExpired(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if _, err := p.GetClient(ctx, "never-registered"); err != ErrNoRecord {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}

	if err := p.PutClient(ctx, &ClientRecord{ID: "stale", RedirectURIs: []string{}, GrantTypes: []string{}, ResponseTypes: []string{}, Scopes: []string{}}, -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := p.GetClient(ctx, "stale"); err != ErrNoRecord {
		t.Fatalf("expected an expired client to read as missing, got %v", err)
	}
}

func TestPostgres_SessionRoundTrip(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindAuthorizeCode, "sig-1", []byte(`{"id":"req-1"}`), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := p.GetSession(ctx, KindAuthorizeCode, "sig-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Postgres normalises jsonb, so compare parsed rather than byte for byte.
	var decoded map[string]string
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["id"] != "req-1" {
		t.Fatalf("data did not round trip: %s", got)
	}
}

func TestPostgres_PutSessionOverwrites(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindPKCE, "sig-2", []byte(`{"v":"first"}`), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := p.PutSession(ctx, KindPKCE, "sig-2", []byte(`{"v":"second"}`), time.Hour); err != nil {
		t.Fatalf("put again: %v", err)
	}

	got, err := p.GetSession(ctx, KindPKCE, "sig-2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["v"] != "second" {
		t.Fatalf("expected the second write to win, got %s", got)
	}
}

func TestPostgres_TakeSessionIsSingleUse(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindLoginHandoff, "ticket-1", []byte(`{"token":"jwt"}`), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, err := p.TakeSession(ctx, KindLoginHandoff, "ticket-1"); err != nil {
		t.Fatalf("first take: %v", err)
	}
	if _, err := p.TakeSession(ctx, KindLoginHandoff, "ticket-1"); err != ErrNoRecord {
		t.Fatalf("expected the second take to find nothing, got %v", err)
	}
}

func TestPostgres_ExpiredSessionReadsAsMissing(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindPKCE, "sig-old", []byte(`{}`), -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := p.GetSession(ctx, KindPKCE, "sig-old"); err != ErrNoRecord {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}
	if _, err := p.TakeSession(ctx, KindPKCE, "sig-old"); err != ErrNoRecord {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}
}

func TestPostgres_DeleteExpired(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindPKCE, "keep", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := p.PutSession(ctx, KindPKCE, "sweep", []byte(`{}`), -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := p.PutClient(ctx, &ClientRecord{ID: "sweep-client", RedirectURIs: []string{}, GrantTypes: []string{}, ResponseTypes: []string{}, Scopes: []string{}}, -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := p.DeleteExpired(ctx); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	if _, err := p.GetSession(ctx, KindPKCE, "keep"); err != nil {
		t.Fatalf("expected the unexpired session to survive, got %v", err)
	}
	if _, err := p.GetSession(ctx, KindPKCE, "sweep"); err != ErrNoRecord {
		t.Fatalf("expected the expired session to be gone, got %v", err)
	}
	if _, err := p.GetClient(ctx, "sweep-client"); err != ErrNoRecord {
		t.Fatalf("expected the expired client to be gone, got %v", err)
	}
}

// TestPostgres_StoreFlow exercises the fosite store, and so the request codec,
// against the real tables.
func TestPostgres_StoreFlow(t *testing.T) {
	p, ctx := newPostgresRepository(t)
	s := NewStore(p)

	if err := s.RegisterClient(ctx, &ClientRecord{
		ID:            "flow-client",
		RedirectURIs:  []string{"http://localhost:9999/callback"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	client, err := s.GetClient(ctx, "flow-client")
	if err != nil {
		t.Fatalf("get client: %v", err)
	}

	req := &fosite.Request{
		ID:             "flow-req",
		Client:         client,
		Session:        NewMarmotSession("user-1", "alice"),
		RequestedScope: fosite.Arguments{"openid"},
		Form:           url.Values{"code_challenge": {"abc"}},
	}
	if err := s.CreateAuthorizeCodeSession(ctx, "flow-sig", req); err != nil {
		t.Fatalf("create code: %v", err)
	}

	got, err := s.GetAuthorizeCodeSession(ctx, "flow-sig", NewMarmotSession("", ""))
	if err != nil {
		t.Fatalf("get code: %v", err)
	}
	if got.GetClient().GetID() != "flow-client" {
		t.Fatalf("client was not resolved, got %q", got.GetClient().GetID())
	}
	if got.GetSession().GetSubject() != "user-1" {
		t.Fatalf("session did not round trip: %v", got.GetSession())
	}
}

// TakeSession is what stops two replicas from redeeming one code. Prove it
// against a real database rather than the in-process map.
func TestPostgres_TakeSessionUnderConcurrency(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutSession(ctx, KindPKCE, "contended", []byte(`{"v":"x"}`), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}

	const readers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			data, err := p.TakeSession(ctx, KindPKCE, "contended")
			if err == nil && len(data) > 0 {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if won != 1 {
		t.Fatalf("expected exactly one taker, got %d", won)
	}
}

// A completed sign-in extends the lease; reading the client must not.
func TestPostgres_TouchClientSlidesExpiry(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutClient(ctx, testClientRecord("sliding"), 2*time.Second); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, err := p.GetClient(ctx, "sliding"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if remaining := clientExpiresIn(t, p, "sliding"); remaining > 5*time.Second {
		t.Fatalf("reading the client extended its lease to %s", remaining)
	}

	if err := p.TouchClient(ctx, "sliding", time.Hour); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if remaining := clientExpiresIn(t, p, "sliding"); remaining < 50*time.Minute {
		t.Fatalf("expected the touch to push expiry out by an hour, got %s", remaining)
	}
}

func clientExpiresIn(t *testing.T, p Repository, clientID string) time.Duration {
	t.Helper()
	var d time.Duration
	err := p.(*pgRepository).db.QueryRow(context.Background(),
		"SELECT expires_at - NOW() FROM oauth_clients WHERE client_id = $1", clientID).Scan(&d)
	if err != nil {
		t.Fatalf("read expiry: %v", err)
	}
	return d
}

func TestPostgres_TouchClientDoesNotReviveAnExpiredRow(t *testing.T) {
	p, ctx := newPostgresRepository(t)

	if err := p.PutClient(ctx, testClientRecord("dead"), -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := p.TouchClient(ctx, "dead", time.Hour); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if _, err := p.GetClient(ctx, "dead"); err != ErrNoRecord {
		t.Fatalf("expected an expired client to stay expired, got %v", err)
	}
}
