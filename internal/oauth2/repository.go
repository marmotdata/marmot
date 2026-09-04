package oauth2

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ory/fosite"
	"github.com/rs/zerolog/log"
)

// ErrNoRecord is returned when a lookup finds nothing, including when the row
// is present but has expired.
var ErrNoRecord = errors.New("oauth: record not found")

// Session kinds. One table holds all of them, keyed by kind and id.
const (
	KindAuthorizeCode = "authorize_code"
	KindPKCE          = "pkce"
	KindPendingAuth   = "pending_authorize"
	KindLoginHandoff  = "login_handoff"
)

// ClientRecord is a dynamically registered OAuth client. Every one of these is
// public with a loopback redirect URI, so there is no secret to keep.
type ClientRecord struct {
	ID            string
	RedirectURIs  []string
	GrantTypes    []string
	ResponseTypes []string
	Scopes        []string
}

func (c *ClientRecord) toFosite() fosite.Client {
	return &fosite.DefaultClient{
		ID:            c.ID,
		Public:        true,
		RedirectURIs:  c.RedirectURIs,
		GrantTypes:    c.GrantTypes,
		ResponseTypes: c.ResponseTypes,
		Scopes:        c.Scopes,
	}
}

func (c *ClientRecord) clone() *ClientRecord {
	return &ClientRecord{
		ID:            c.ID,
		RedirectURIs:  append([]string(nil), c.RedirectURIs...),
		GrantTypes:    append([]string(nil), c.GrantTypes...),
		ResponseTypes: append([]string(nil), c.ResponseTypes...),
		Scopes:        append([]string(nil), c.Scopes...),
	}
}

// Repository holds the state a login needs between requests. It has to be
// shared rather than per-process: a sign-in spreads over four requests and a
// Marmot instance can serve each of them from a different replica.
type Repository interface {
	PutClient(ctx context.Context, client *ClientRecord, ttl time.Duration) error
	GetClient(ctx context.Context, clientID string) (*ClientRecord, error)
	// TouchClient extends a client's expiry. Reading must not do this: the
	// read happens on an unauthenticated endpoint, so a caller who could
	// refresh a row by reading it could keep any number of them alive for
	// ever. Only a completed sign-in extends.
	TouchClient(ctx context.Context, clientID string, ttl time.Duration) error

	PutSession(ctx context.Context, kind, id string, data []byte, ttl time.Duration) error
	GetSession(ctx context.Context, kind, id string) ([]byte, error)
	// TakeSession reads and removes in one step, so a single-use code or
	// ticket stays single-use even when two replicas redeem it at once.
	TakeSession(ctx context.Context, kind, id string) ([]byte, error)
	DeleteSession(ctx context.Context, kind, id string) error

	DeleteExpired(ctx context.Context) error
}

// StartCleanup deletes expired rows on an interval. Every replica runs it; the
// deletes are idempotent.
func StartCleanup(ctx context.Context, repo Repository, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := repo.DeleteExpired(ctx); err != nil {
					// Rows only pile up until the next tick succeeds, but a
					// sweep that fails forever is worth seeing.
					log.Error().Err(err).Msg("Failed to delete expired OAuth state")
				}
			}
		}
	}()
}

// MemoryRepository keeps everything in process. It is correct for a single
// replica and is what the tests use; deployments use the Postgres one.
type MemoryRepository struct {
	mu       sync.Mutex
	clients  map[string]memoryEntry[*ClientRecord]
	sessions map[string]memoryEntry[[]byte]
}

type memoryEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		clients:  make(map[string]memoryEntry[*ClientRecord]),
		sessions: make(map[string]memoryEntry[[]byte]),
	}
}

func sessionKey(kind, id string) string { return kind + "\x00" + id }

func (m *MemoryRepository) PutClient(_ context.Context, client *ClientRecord, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client.ID] = memoryEntry[*ClientRecord]{value: client.clone(), expiresAt: time.Now().Add(ttl)}
	return nil
}

func (m *MemoryRepository) GetClient(_ context.Context, clientID string) (*ClientRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.clients[clientID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, ErrNoRecord
	}
	// Copy, so this behaves like the Postgres one and a caller cannot mutate
	// what the next reader sees.
	return e.value.clone(), nil
}

func (m *MemoryRepository) TouchClient(_ context.Context, clientID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.clients[clientID]
	if !ok || time.Now().After(e.expiresAt) {
		return ErrNoRecord
	}
	e.expiresAt = time.Now().Add(ttl)
	m.clients[clientID] = e
	return nil
}

func (m *MemoryRepository) PutSession(_ context.Context, kind, id string, data []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionKey(kind, id)] = memoryEntry[[]byte]{
		value:     append([]byte(nil), data...),
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (m *MemoryRepository) GetSession(_ context.Context, kind, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.sessions[sessionKey(kind, id)]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, ErrNoRecord
	}
	return append([]byte(nil), e.value...), nil
}

func (m *MemoryRepository) TakeSession(_ context.Context, kind, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey(kind, id)
	e, ok := m.sessions[key]
	if !ok {
		return nil, ErrNoRecord
	}
	delete(m.sessions, key)
	if time.Now().After(e.expiresAt) {
		return nil, ErrNoRecord
	}
	return append([]byte(nil), e.value...), nil
}

func (m *MemoryRepository) DeleteSession(_ context.Context, kind, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionKey(kind, id))
	return nil
}

func (m *MemoryRepository) DeleteExpired(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.clients {
		if now.After(e.expiresAt) {
			delete(m.clients, k)
		}
	}
	for k, e := range m.sessions {
		if now.After(e.expiresAt) {
			delete(m.sessions, k)
		}
	}
	return nil
}
