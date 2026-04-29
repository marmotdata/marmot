package oauth2

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ory/fosite"
)

func TestStore_GetClient_Builtin(t *testing.T) {
	s := NewStore()
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
	s := NewStore()
	_, err := s.GetClient(context.Background(), "nonexistent")
	if err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_RegisterClient(t *testing.T) {
	s := NewStore()
	client := &fosite.DefaultClient{
		ID:           "test-client",
		Public:       true,
		RedirectURIs: []string{"http://localhost:9999/cb"},
	}
	s.RegisterClient(client)

	c, err := s.GetClient(context.Background(), "test-client")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.GetID() != "test-client" {
		t.Fatalf("expected test-client, got %q", c.GetID())
	}
}

func TestStore_AuthorizeCode_CRUD(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{
		ID:      "req-1",
		Session: session,
	}

	if err := s.CreateAuthorizeCodeSession(ctx, "code-sig-1", req); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetAuthorizeCodeSession(ctx, "code-sig-1", session)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetID() != "req-1" {
		t.Fatalf("expected req-1, got %q", got.GetID())
	}

	if err := s.InvalidateAuthorizeCodeSession(ctx, "code-sig-1"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	_, err = s.GetAuthorizeCodeSession(ctx, "code-sig-1", session)
	if err != fosite.ErrInvalidatedAuthorizeCode {
		t.Fatalf("expected ErrInvalidatedAuthorizeCode, got %v", err)
	}
}

func TestStore_AuthorizeCode_Expired(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{
		ID:      "req-2",
		Session: session,
	}

	s.mu.Lock()
	s.authorizeCodes["expired-code"] = storedRequest{
		request:   req,
		expiresAt: time.Now().Add(-time.Minute),
	}
	s.mu.Unlock()

	_, err := s.GetAuthorizeCodeSession(ctx, "expired-code", session)
	if err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound for expired code, got %v", err)
	}
}

func TestStore_PKCE_CRUD(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	session := NewMarmotSession("user1", "alice")
	req := &fosite.Request{
		ID:      "pkce-1",
		Session: session,
	}

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

	if err := s.DeletePKCERequestSession(ctx, "pkce-sig-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = s.GetPKCERequestSession(ctx, "pkce-sig-1", session)
	if err != fosite.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStore_Cleanup(t *testing.T) {
	s := NewStore()
	session := NewMarmotSession("user1", "alice")

	s.mu.Lock()
	s.authorizeCodes["expired"] = storedRequest{
		request:   &fosite.Request{ID: "exp", Session: session},
		expiresAt: time.Now().Add(-time.Minute),
	}
	s.authorizeCodes["valid"] = storedRequest{
		request:   &fosite.Request{ID: "val", Session: session},
		expiresAt: time.Now().Add(time.Hour),
	}
	s.mu.Unlock()

	s.cleanup()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.authorizeCodes["expired"]; ok {
		t.Fatal("expected expired code to be cleaned up")
	}
	if _, ok := s.authorizeCodes["valid"]; !ok {
		t.Fatal("expected valid code to remain")
	}
}

func TestStore_Concurrent(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			client := &fosite.DefaultClient{
				ID:     "c-" + string(rune('A'+i%26)),
				Public: true,
			}
			s.RegisterClient(client)
			_, _ = s.GetClient(ctx, client.ID)

			session := NewMarmotSession("u", "x")
			req := &fosite.Request{ID: "r", Session: session}
			_ = s.CreateAuthorizeCodeSession(ctx, "code-"+string(rune('A'+i%26)), req)
			_, _ = s.GetAuthorizeCodeSession(ctx, "code-"+string(rune('A'+i%26)), session)
		}(i)
	}
	wg.Wait()
}
