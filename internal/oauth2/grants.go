package oauth2

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Grant records that a person allowed an OAuth client to act as them. Marmot issues stateless access tokens and no refresh tokens, so a grant is live only while a token from its latest sign-in can still be valid.
type Grant struct {
	// ID changes whenever the grant is created afresh, and tokens carry it, so a token outlives neither a revoke nor a revoke followed by a new allow.
	ID               string    `json:"-"`
	ClientID         string    `json:"client_id"`
	ClientName       string    `json:"client_name,omitempty"`
	RedirectURI      string    `json:"redirect_uri,omitempty"`
	Scopes           []string  `json:"scopes"`
	CreatedAt        time.Time `json:"created_at"`
	LastAuthorisedAt time.Time `json:"last_authorised_at"`
	// ExpiresAt is when the newest token issued under the grant stops working, after which the client has to ask again.
	ExpiresAt time.Time `json:"expires_at"`
} // @name OAuthGrant

// GrantRepository stores grants. Revoking deletes the row, and GrantValid is what makes that take effect on tokens already issued.
type GrantRepository interface {
	// RecordGrant is called before a token is issued, creating the grant or refreshing its last sign-in, and returns the grant id the token must carry.
	RecordGrant(ctx context.Context, userID string, g Grant) (string, error)
	ListGrants(ctx context.Context, userID string) ([]Grant, error)
	// RevokeGrant reports whether there was a grant to revoke.
	RevokeGrant(ctx context.Context, userID, clientID string) (bool, error)
	// GrantValid reports whether the grant a token was issued under still exists for that user.
	GrantValid(ctx context.Context, userID, grantID string) (bool, error)
}

type pgGrantRepository struct {
	db       *pgxpool.Pool
	tokenTTL time.Duration
}

// NewPostgresGrantRepository returns the grant store. tokenTTL is how long an issued access token lives, which bounds how long a grant without a fresh sign-in stays worth showing.
func NewPostgresGrantRepository(db *pgxpool.Pool, tokenTTL time.Duration) GrantRepository {
	return &pgGrantRepository{db: db, tokenTTL: tokenTTL}
}

func (p *pgGrantRepository) RecordGrant(ctx context.Context, userID string, g Grant) (string, error) {
	// Grants with no live token are pruned for this user on each sign-in, which keeps the table bounded without a sweeper. The CLI registers a fresh client on every login, so without this each one would linger.
	if _, err := p.db.Exec(ctx,
		`DELETE FROM oauth_grants WHERE user_id = $1 AND last_authorised_at < NOW() - make_interval(secs => $2)`,
		userID, p.tokenTTL.Seconds()); err != nil {
		return "", fmt.Errorf("pruning OAuth grants: %w", err)
	}

	var id string
	err := p.db.QueryRow(ctx, `
		INSERT INTO oauth_grants (user_id, client_id, client_name, redirect_uri, scopes)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
		ON CONFLICT (user_id, client_id) DO UPDATE SET
			client_name        = COALESCE(EXCLUDED.client_name, oauth_grants.client_name),
			redirect_uri       = COALESCE(EXCLUDED.redirect_uri, oauth_grants.redirect_uri),
			scopes             = EXCLUDED.scopes,
			last_authorised_at = NOW()
		RETURNING id`,
		userID, g.ClientID, g.ClientName, g.RedirectURI, nonNil(g.Scopes)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("recording OAuth grant: %w", err)
	}
	return id, nil
}

func (p *pgGrantRepository) ListGrants(ctx context.Context, userID string) ([]Grant, error) {
	rows, err := p.db.Query(ctx, `
		SELECT client_id, COALESCE(client_name, ''), COALESCE(redirect_uri, ''), scopes, created_at, last_authorised_at
		  FROM oauth_grants
		 WHERE user_id = $1 AND last_authorised_at >= NOW() - make_interval(secs => $2)
		 ORDER BY last_authorised_at DESC`,
		userID, p.tokenTTL.Seconds())
	if err != nil {
		return nil, fmt.Errorf("listing OAuth grants: %w", err)
	}
	defer rows.Close()

	grants := []Grant{}
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ClientID, &g.ClientName, &g.RedirectURI, &g.Scopes, &g.CreatedAt, &g.LastAuthorisedAt); err != nil {
			return nil, fmt.Errorf("scanning OAuth grant: %w", err)
		}
		g.ExpiresAt = g.LastAuthorisedAt.Add(p.tokenTTL)
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (p *pgGrantRepository) RevokeGrant(ctx context.Context, userID, clientID string) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM oauth_grants WHERE user_id = $1 AND client_id = $2`, userID, clientID)
	if err != nil {
		return false, fmt.Errorf("revoking OAuth grant: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (p *pgGrantRepository) GrantValid(ctx context.Context, userID, grantID string) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM oauth_grants WHERE id::text = $2 AND user_id = $1)`,
		userID, grantID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("checking OAuth grant: %w", err)
	}
	return ok, nil
}

// MemoryGrantRepository keeps grants in process, for tests and single-replica use.
type MemoryGrantRepository struct {
	mu     sync.Mutex
	grants map[string]Grant
	seq    int
}

func NewMemoryGrantRepository() *MemoryGrantRepository {
	return &MemoryGrantRepository{grants: make(map[string]Grant)}
}

func grantKey(userID, clientID string) string { return userID + "\x00" + clientID }

func (m *MemoryGrantRepository) RecordGrant(_ context.Context, userID string, g Grant) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	key := grantKey(userID, g.ClientID)
	if existing, ok := m.grants[key]; ok {
		g.ID = existing.ID
		g.CreatedAt = existing.CreatedAt
		if g.ClientName == "" {
			g.ClientName = existing.ClientName
		}
		if g.RedirectURI == "" {
			g.RedirectURI = existing.RedirectURI
		}
	} else {
		m.seq++
		g.ID = fmt.Sprintf("grant-%d", m.seq)
		g.CreatedAt = now
	}
	g.LastAuthorisedAt = now
	g.Scopes = append([]string(nil), g.Scopes...)
	m.grants[key] = g
	return g.ID, nil
}

func (m *MemoryGrantRepository) ListGrants(_ context.Context, userID string) ([]Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	grants := []Grant{}
	for key, g := range m.grants {
		if strings.HasPrefix(key, userID+"\x00") {
			grants = append(grants, g)
		}
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].LastAuthorisedAt.After(grants[j].LastAuthorisedAt) })
	return grants, nil
}

func (m *MemoryGrantRepository) RevokeGrant(_ context.Context, userID, clientID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := grantKey(userID, clientID)
	_, ok := m.grants[key]
	delete(m.grants, key)
	return ok, nil
}

func (m *MemoryGrantRepository) GrantValid(_ context.Context, userID, grantID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, g := range m.grants {
		if g.ID == grantID && strings.HasPrefix(key, userID+"\x00") {
			return true, nil
		}
	}
	return false, nil
}
