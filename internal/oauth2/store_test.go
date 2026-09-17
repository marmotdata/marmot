package oauth2

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/ory/fosite"
)

func newTestStore() *Store {
	return NewStore(NewMemoryRepository())
}

func testClientRecord(id string) *ClientRecord {
	return &ClientRecord{
		ID:            id,
		RedirectURIs:  []string{"http://localhost:9999/cb"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}
}

func TestStore_GetClient_Builtin(t *testing.T) {
	s := newTestStore()
	c, err := s.GetClient(context.Background(), "marmot-cli")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.GetID() != "marmot-cli" {
		t.Fatalf("expected marmot-cli, got %q", c.GetID())
	}
	if !c.IsPublic() {
		t.Fatal("expected public client")
	}
}

func TestStore_GetClient_NotFound(t *testing.T) {
	s := newTestStore()
	_, err := s.GetClient(context.Background(), "nonexistent")
	if err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_RegisterClient(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	if err := s.RegisterClient(ctx, testClientRecord("test-client")); err != nil {
		t.Fatalf("register: %v", err)
	}

	c, err := s.GetClient(ctx, "test-client")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.GetID() != "test-client" {
		t.Fatalf("expected test-client, got %q", c.GetID())
	}
	if !c.IsPublic() {
		t.Fatal("expected public client")
	}
	if got := c.GetRedirectURIs(); len(got) != 1 || got[0] != "http://localhost:9999/cb" {
		t.Fatalf("unexpected redirect URIs: %v", got)
	}
}

// Reading a client happens on the unauthenticated authorize endpoint. If it
// refreshed the row, anyone could keep unlimited registrations alive for ever
// with one request per row per lease.
func TestStore_GetClientDoesNotExtendTheLease(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()

	// Registered a moment before the short lease runs out.
	if err := repo.PutClient(ctx, testClientRecord("short"), 50*time.Millisecond); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := s.GetClient(ctx, "short"); err != nil {
		t.Fatalf("get: %v", err)
	}

	time.Sleep(80 * time.Millisecond)
	if _, err := s.GetClient(ctx, "short"); err != fosite.ErrNotFound {
		t.Fatalf("reading the client extended its lease, got %v", err)
	}
}

// A completed sign-in is what earns a client its full life.
func TestStore_KeepClientAliveExtendsTheLease(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()

	if err := repo.PutClient(ctx, testClientRecord("kept"), 50*time.Millisecond); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := s.KeepClientAlive(ctx, "kept"); err != nil {
		t.Fatalf("keep alive: %v", err)
	}

	time.Sleep(80 * time.Millisecond)
	if _, err := s.GetClient(ctx, "kept"); err != nil {
		t.Fatalf("expected the client to survive a completed sign-in, got %v", err)
	}
}

// An expired id must not be resurrected by touching it.
func TestStore_KeepClientAliveDoesNotRevive(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()

	if err := repo.PutClient(ctx, testClientRecord("gone"), -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = s.KeepClientAlive(ctx, "gone")
	if _, err := s.GetClient(ctx, "gone"); err != fosite.ErrNotFound {
		t.Fatalf("an expired client was revived, got %v", err)
	}
}

// A registration nobody signs in with must not outlive its short lease.
func TestStore_RegisterUsesTheShortLease(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	if err := s.RegisterClient(ctx, testClientRecord("fresh")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if registrationTTL >= clientTTL {
		t.Fatalf("registration lease %s should be far shorter than %s", registrationTTL, clientTTL)
	}
}

// The handoff ticket is the credential that redeems a session, so the table
// must hold a digest of it rather than the ticket itself.
func TestStore_LoginHandoffTicketIsHashedAtRest(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()

	if err := s.PutLoginHandoff(ctx, "the-ticket", "user-9"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := repo.GetSession(ctx, KindLoginHandoff, "the-ticket"); err != ErrNoRecord {
		t.Fatal("the raw ticket is usable as a key, so it was stored in the clear")
	}
	if _, err := repo.GetSession(ctx, KindLoginHandoff, hashTicket("the-ticket")); err != nil {
		t.Fatalf("expected the row under the hashed key, got %v", err)
	}

	userID, err := s.TakeLoginHandoff(ctx, "the-ticket")
	if err != nil || userID != "user-9" {
		t.Fatalf("redeeming with the raw ticket failed: %q %v", userID, err)
	}
}

func TestStore_AuthorizeCode_CRUD(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{
		ID:      "req-1",
		Client:  builtinCLIClient,
		Session: session,
	}

	if err := s.CreateAuthorizeCodeSession(ctx, "code-sig-1", req); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetAuthorizeCodeSession(ctx, "code-sig-1", NewMarmotSession("", ""))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetID() != "req-1" {
		t.Fatalf("expected req-1, got %q", got.GetID())
	}

	if err := s.InvalidateAuthorizeCodeSession(ctx, "code-sig-1"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	if _, err := s.GetAuthorizeCodeSession(ctx, "code-sig-1", session); err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound after invalidate, got %v", err)
	}
}

// Fosite checks the code against the session it hands the store, then swaps in
// the stored one. Without hydrating the first, the expiry check is skipped.
func TestStore_AuthorizeCode_HydratesCallerSession(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	expiry := time.Now().Add(time.Minute).UTC().Round(time.Second)
	session := NewMarmotSession("user-42", "alice")
	session.SetExpiresAt(fosite.AuthorizeCode, expiry)

	req := &fosite.Request{ID: "req-hydrate", Client: builtinCLIClient, Session: session}
	if err := s.CreateAuthorizeCodeSession(ctx, "sig", req); err != nil {
		t.Fatalf("create: %v", err)
	}

	into := NewMarmotSession("", "")
	if _, err := s.GetAuthorizeCodeSession(ctx, "sig", into); err != nil {
		t.Fatalf("get: %v", err)
	}
	if into.UserID != "user-42" || into.Username != "alice" {
		t.Fatalf("caller session was not hydrated: %+v", into)
	}
	if !into.GetExpiresAt(fosite.AuthorizeCode).Equal(expiry) {
		t.Fatalf("expiry was not hydrated: %v", into.GetExpiresAt(fosite.AuthorizeCode))
	}
}

func TestStore_AuthorizeCode_RoundTripsRequest(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	session := NewMarmotSession("user-42", "alice")

	req := &fosite.Request{
		ID:             "req-session",
		Client:         builtinCLIClient,
		Session:        session,
		RequestedScope: fosite.Arguments{"openid"},
		GrantedScope:   fosite.Arguments{"openid"},
		Form:           url.Values{"code_challenge": {"abc"}, "redirect_uri": {"http://localhost:1/cb"}},
	}
	if err := s.CreateAuthorizeCodeSession(ctx, "sig", req); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetAuthorizeCodeSession(ctx, "sig", NewMarmotSession("", ""))
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	loaded, ok := got.GetSession().(*MarmotSession)
	if !ok {
		t.Fatalf("expected *MarmotSession, got %T", got.GetSession())
	}
	if loaded.UserID != "user-42" || loaded.Username != "alice" {
		t.Fatalf("unexpected session identity: %+v", loaded)
	}
	if got.GetRequestForm().Get("code_challenge") != "abc" {
		t.Fatalf("form did not survive the round trip: %v", got.GetRequestForm())
	}
	if got.GetRequestForm().Get("redirect_uri") != "http://localhost:1/cb" {
		t.Fatalf("redirect_uri did not survive the round trip: %v", got.GetRequestForm())
	}
	if got.GetClient().GetID() != builtinCLIClient.ID {
		t.Fatalf("client was not resolved, got %q", got.GetClient().GetID())
	}
	if !got.GetGrantedScopes().Has("openid") {
		t.Fatalf("granted scopes did not survive: %v", got.GetGrantedScopes())
	}
}

func TestStore_AuthorizeCode_Expired(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")

	data, err := encodeRequest(&fosite.Request{ID: "req-2", Client: builtinCLIClient, Session: session})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := repo.PutSession(ctx, KindAuthorizeCode, "expired-code", data, -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, err := s.GetAuthorizeCodeSession(ctx, "expired-code", session); err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound for expired code, got %v", err)
	}
}

// The PKCE row is what makes a code single use, so reading it has to consume
// it. Two replicas reading at once must not both get it back.
func TestStore_PKCE_ReadConsumesTheRow(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{ID: "pkce-1", Client: builtinCLIClient, Session: session}

	if err := s.CreatePKCERequestSession(ctx, "pkce-sig-1", req); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetPKCERequestSession(ctx, "pkce-sig-1", session)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetID() != "pkce-1" {
		t.Fatalf("expected pkce-1, got %q", got.GetID())
	}

	if _, err := s.GetPKCERequestSession(ctx, "pkce-sig-1", session); err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound on the second read, got %v", err)
	}

	// Fosite deletes after reading; the row is already gone.
	if err := s.DeletePKCERequestSession(ctx, "pkce-sig-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestStore_PKCE_ConcurrentReadsYieldOne(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{ID: "pkce-race", Client: builtinCLIClient, Session: session}
	if err := s.CreatePKCERequestSession(ctx, "sig", req); err != nil {
		t.Fatalf("create: %v", err)
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
			if _, err := s.GetPKCERequestSession(ctx, "sig", NewMarmotSession("", "")); err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if won != 1 {
		t.Fatalf("expected exactly one reader to consume the PKCE row, got %d", won)
	}
}

func TestStore_PendingAuthorize_RoundTrip(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	redirect, err := url.Parse("http://localhost:9999/callback")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ar := &fosite.AuthorizeRequest{
		ResponseTypes: fosite.Arguments{"code"},
		RedirectURI:   redirect,
		State:         "state-1",
		Request: fosite.Request{
			ID:             "pending-1",
			Client:         builtinCLIClient,
			RequestedScope: fosite.Arguments{"openid"},
			GrantedScope:   fosite.Arguments{"openid"},
			Form:           url.Values{"code_challenge_method": {"S256"}},
		},
	}

	if err := s.PutPendingAuthorize(ctx, "sess-1", ar); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := s.GetPendingAuthorize(ctx, "sess-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetState() != "state-1" {
		t.Fatalf("expected state-1, got %q", got.GetState())
	}
	if got.GetRedirectURI().String() != "http://localhost:9999/callback" {
		t.Fatalf("unexpected redirect URI: %v", got.GetRedirectURI())
	}
	if !got.GetResponseTypes().Has("code") {
		t.Fatalf("unexpected response types: %v", got.GetResponseTypes())
	}
	if got.GetRequestForm().Get("code_challenge_method") != "S256" {
		t.Fatalf("form did not survive the round trip: %v", got.GetRequestForm())
	}

	if err := s.DeletePendingAuthorize(ctx, "sess-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetPendingAuthorize(ctx, "sess-1"); err != ErrNoRecord {
		t.Fatalf("expected ErrNoRecord after delete, got %v", err)
	}
}

// The pending row is the only one holding an unauthenticated request's query
// string, so everything the flow does not need is dropped.
func TestStore_PendingAuthorize_DropsUnknownFormKeys(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	ar := &fosite.AuthorizeRequest{
		ResponseTypes: fosite.Arguments{"code"},
		State:         "state-1",
		Request: fosite.Request{
			ID:     "pending-junk",
			Client: builtinCLIClient,
			Form: url.Values{
				"code_challenge": {"abc"},
				"redirect_uri":   {"http://localhost:9999/callback"},
				"junk":           {"a very long value that has no business being kept"},
			},
		},
	}
	if err := s.PutPendingAuthorize(ctx, "sess-junk", ar); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := s.GetPendingAuthorize(ctx, "sess-junk")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetRequestForm().Has("junk") {
		t.Fatalf("unknown form key was stored: %v", got.GetRequestForm())
	}
	if got.GetRequestForm().Get("code_challenge") != "abc" {
		t.Fatalf("code_challenge was dropped: %v", got.GetRequestForm())
	}
	if got.GetRequestForm().Get("redirect_uri") != "http://localhost:9999/callback" {
		t.Fatalf("redirect_uri was dropped: %v", got.GetRequestForm())
	}
}

func TestStore_LoginHandoff_SingleUse(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	if err := s.PutLoginHandoff(ctx, "ticket-1", "user-9"); err != nil {
		t.Fatalf("put: %v", err)
	}

	userID, err := s.TakeLoginHandoff(ctx, "ticket-1")
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	if userID != "user-9" {
		t.Fatalf("expected user-9, got %q", userID)
	}

	if _, err := s.TakeLoginHandoff(ctx, "ticket-1"); err != ErrNoRecord {
		t.Fatalf("expected ErrNoRecord on the second take, got %v", err)
	}
}

// A session fosite hands us that is not a MarmotSession would lose the user
// identity on the way to disk, so storing it has to fail loudly.
func TestStore_RejectsForeignSession(t *testing.T) {
	s := newTestStore()
	req := &fosite.Request{
		ID:      "req-foreign",
		Client:  builtinCLIClient,
		Session: &fosite.DefaultSession{},
	}
	if err := s.CreateAuthorizeCodeSession(context.Background(), "sig", req); err == nil {
		t.Fatal("expected storing a non-Marmot session to fail")
	}
}

func TestStore_Cleanup(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewStore(repo)
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")

	expired, err := encodeRequest(&fosite.Request{ID: "exp", Client: builtinCLIClient, Session: session})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	valid, err := encodeRequest(&fosite.Request{ID: "val", Client: builtinCLIClient, Session: session})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := repo.PutSession(ctx, KindAuthorizeCode, "expired", expired, -time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := repo.PutSession(ctx, KindAuthorizeCode, "valid", valid, time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := repo.GetSession(ctx, KindAuthorizeCode, "expired"); err != ErrNoRecord {
		t.Fatalf("expected expired code to be cleaned up, got %v", err)
	}
	if _, err := s.GetAuthorizeCodeSession(ctx, "valid", session); err != nil {
		t.Fatalf("expected valid code to remain, got %v", err)
	}
}

// A caller must not be able to change what the next reader sees.
func TestMemoryRepository_ReturnsCopies(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	if err := repo.PutClient(ctx, testClientRecord("c-1"), time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}
	first, err := repo.GetClient(ctx, "c-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	first.RedirectURIs[0] = "http://evil.example.com/cb"

	second, err := repo.GetClient(ctx, "c-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if second.RedirectURIs[0] != "http://localhost:9999/cb" {
		t.Fatalf("a caller mutated the stored record: %v", second.RedirectURIs)
	}
}

func TestStore_Concurrent(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "c-" + string(rune('A'+i%26))
			if err := s.RegisterClient(ctx, testClientRecord(id)); err != nil {
				t.Errorf("register: %v", err)
				return
			}
			if _, err := s.GetClient(ctx, id); err != nil {
				t.Errorf("get client: %v", err)
				return
			}

			session := NewMarmotSession("u", "x")
			sig := "code-" + string(rune('A'+i%26))
			req := &fosite.Request{ID: "r", Client: builtinCLIClient, Session: session}
			if err := s.CreateAuthorizeCodeSession(ctx, sig, req); err != nil {
				t.Errorf("create code: %v", err)
				return
			}
			if _, err := s.GetAuthorizeCodeSession(ctx, sig, NewMarmotSession("", "")); err != nil {
				t.Errorf("get code: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
