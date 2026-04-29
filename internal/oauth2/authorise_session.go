package oauth2

import (
	"context"
	"sync"
	"time"

	"github.com/ory/fosite"
)

type PendingAuthorize struct {
	Request   fosite.AuthorizeRequester
	CreatedAt time.Time
}

type AuthorizeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*PendingAuthorize
}

func NewAuthorizeSessionStore() *AuthorizeSessionStore {
	return &AuthorizeSessionStore{
		sessions: make(map[string]*PendingAuthorize),
	}
}

func (s *AuthorizeSessionStore) Put(sessionID string, req fosite.AuthorizeRequester) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = &PendingAuthorize{
		Request:   req,
		CreatedAt: time.Now(),
	}
}

func (s *AuthorizeSessionStore) Get(sessionID string) (*PendingAuthorize, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(p.CreatedAt) > 10*time.Minute {
		delete(s.sessions, sessionID)
		return nil, false
	}
	return p, true
}

func (s *AuthorizeSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *AuthorizeSessionStore) StartCleanup(ctx context.Context, interval time.Duration) {
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

func (s *AuthorizeSessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, p := range s.sessions {
		if now.Sub(p.CreatedAt) > 10*time.Minute {
			delete(s.sessions, k)
		}
	}
}
