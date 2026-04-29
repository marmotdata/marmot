package oauth2

import (
	"context"
	"sync"
	"time"

	"github.com/ory/fosite"
)

type storedRequest struct {
	request   fosite.Requester
	expiresAt time.Time
}

type Store struct {
	mu sync.RWMutex

	clients        map[string]fosite.Client         // client_id → client
	authorizeCodes map[string]storedRequest          // code sig → request
	pkceRequests   map[string]storedRequest          // code sig → PKCE request
	accessTokens   map[string]storedRequest          // token sig → request (no-op for stateless)
	refreshTokens  map[string]storedRequest          // token sig → request (unused)
	invalidatedCodes map[string]struct{}             // invalidated auth codes
}

func NewStore() *Store {
	s := &Store{
		clients:          make(map[string]fosite.Client),
		authorizeCodes:   make(map[string]storedRequest),
		pkceRequests:     make(map[string]storedRequest),
		accessTokens:     make(map[string]storedRequest),
		refreshTokens:    make(map[string]storedRequest),
		invalidatedCodes: make(map[string]struct{}),
	}

	s.clients["marmot-cli"] = &fosite.DefaultClient{
		ID:            "marmot-cli",
		Public:        true,
		RedirectURIs:  []string{"http://localhost"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}

	return s
}

func (s *Store) GetClient(_ context.Context, id string) (fosite.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[id]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	return c, nil
}

func (s *Store) ClientAssertionJWTValid(_ context.Context, _ string) error {
	return nil
}

func (s *Store) SetClientAssertionJWT(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (s *Store) RegisterClient(client fosite.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.GetID()] = client
}

func (s *Store) CreateAuthorizeCodeSession(_ context.Context, code string, req fosite.Requester) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorizeCodes[code] = storedRequest{
		request:   req,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	return nil
}

func (s *Store) GetAuthorizeCodeSession(_ context.Context, code string, _ fosite.Session) (fosite.Requester, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.invalidatedCodes[code]; ok {
		// Fosite needs the request to extract info, but the code is spent.
		if sr, exists := s.authorizeCodes[code]; exists {
			return sr.request, fosite.ErrInvalidatedAuthorizeCode
		}
		return nil, fosite.ErrInvalidatedAuthorizeCode
	}

	sr, ok := s.authorizeCodes[code]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	if time.Now().After(sr.expiresAt) {
		return nil, fosite.ErrNotFound
	}
	return sr.request, nil
}

func (s *Store) InvalidateAuthorizeCodeSession(_ context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidatedCodes[code] = struct{}{}
	return nil
}

// Marmot JWTs are stateless; these methods exist only to satisfy the fosite interface.
func (s *Store) CreateAccessTokenSession(_ context.Context, signature string, req fosite.Requester) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessTokens[signature] = storedRequest{
		request:   req,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	return nil
}

func (s *Store) GetAccessTokenSession(_ context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sr, ok := s.accessTokens[signature]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	return sr.request, nil
}

func (s *Store) DeleteAccessTokenSession(_ context.Context, signature string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accessTokens, signature)
	return nil
}

// Marmot does not issue refresh tokens; these are unused.
func (s *Store) CreateRefreshTokenSession(_ context.Context, signature string, _ string, req fosite.Requester) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshTokens[signature] = storedRequest{request: req}
	return nil
}

func (s *Store) GetRefreshTokenSession(_ context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sr, ok := s.refreshTokens[signature]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	return sr.request, nil
}

func (s *Store) DeleteRefreshTokenSession(_ context.Context, signature string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, signature)
	return nil
}

func (s *Store) RotateRefreshToken(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *Store) RevokeRefreshToken(_ context.Context, _ string) error {
	return nil
}

func (s *Store) RevokeAccessToken(_ context.Context, _ string) error {
	return nil
}

func (s *Store) RevokeRefreshTokenMaybeGracePeriod(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *Store) GetPKCERequestSession(_ context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sr, ok := s.pkceRequests[signature]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	return sr.request, nil
}

func (s *Store) CreatePKCERequestSession(_ context.Context, signature string, req fosite.Requester) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pkceRequests[signature] = storedRequest{
		request:   req,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	return nil
}

func (s *Store) DeletePKCERequestSession(_ context.Context, signature string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pkceRequests, signature)
	return nil
}

func (s *Store) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
}

func (s *Store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()

	for k, sr := range s.authorizeCodes {
		if !sr.expiresAt.IsZero() && now.After(sr.expiresAt) {
			delete(s.authorizeCodes, k)
			delete(s.invalidatedCodes, k)
		}
	}
	for k, sr := range s.pkceRequests {
		if !sr.expiresAt.IsZero() && now.After(sr.expiresAt) {
			delete(s.pkceRequests, k)
		}
	}
	for k, sr := range s.accessTokens {
		if !sr.expiresAt.IsZero() && now.After(sr.expiresAt) {
			delete(s.accessTokens, k)
		}
	}
}
