package oauth2

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository returns the Repository every replica shares.
func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &pgRepository{db: db}
}

// Expiry is computed by Postgres rather than in Go, so that writing a row and
// reading it back use the same clock even when replicas disagree.
func (p *pgRepository) PutClient(ctx context.Context, client *ClientRecord, ttl time.Duration) error {
	const q = `
		INSERT INTO oauth_clients (client_id, redirect_uris, grant_types, response_types, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, NOW() + make_interval(secs => $6))
		ON CONFLICT (client_id) DO UPDATE SET
			redirect_uris  = EXCLUDED.redirect_uris,
			grant_types    = EXCLUDED.grant_types,
			response_types = EXCLUDED.response_types,
			scopes         = EXCLUDED.scopes,
			expires_at     = EXCLUDED.expires_at
	`
	_, err := p.db.Exec(ctx, q,
		client.ID,
		nonNil(client.RedirectURIs),
		nonNil(client.GrantTypes),
		nonNil(client.ResponseTypes),
		nonNil(client.Scopes),
		ttl.Seconds(),
	)
	return err
}

// nonNil keeps a nil slice out of a NOT NULL text[] column, which pgx would
// otherwise send as NULL.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (p *pgRepository) GetClient(ctx context.Context, clientID string) (*ClientRecord, error) {
	const q = `
		SELECT redirect_uris, grant_types, response_types, scopes
		FROM oauth_clients
		WHERE client_id = $1 AND expires_at > NOW()
	`
	rec := &ClientRecord{ID: clientID}
	err := p.db.QueryRow(ctx, q, clientID).Scan(
		&rec.RedirectURIs,
		&rec.GrantTypes,
		&rec.ResponseTypes,
		&rec.Scopes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoRecord
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// TouchClient never revives an expired row, so a client that lapsed has to be
// registered again rather than resurrected by a stale id.
func (p *pgRepository) TouchClient(ctx context.Context, clientID string, ttl time.Duration) error {
	const q = `
		UPDATE oauth_clients
		SET expires_at = NOW() + make_interval(secs => $2)
		WHERE client_id = $1 AND expires_at > NOW()
	`
	_, err := p.db.Exec(ctx, q, clientID, ttl.Seconds())
	return err
}

func (p *pgRepository) PutSession(ctx context.Context, kind, id string, data []byte, ttl time.Duration) error {
	const q = `
		INSERT INTO oauth_sessions (kind, session_id, data, expires_at)
		VALUES ($1, $2, $3, NOW() + make_interval(secs => $4))
		ON CONFLICT (kind, session_id) DO UPDATE SET
			data       = EXCLUDED.data,
			expires_at = EXCLUDED.expires_at
	`
	_, err := p.db.Exec(ctx, q, kind, id, data, ttl.Seconds())
	return err
}

func (p *pgRepository) GetSession(ctx context.Context, kind, id string) ([]byte, error) {
	const q = `
		SELECT data
		FROM oauth_sessions
		WHERE kind = $1 AND session_id = $2 AND expires_at > NOW()
	`
	return scanSession(p.db.QueryRow(ctx, q, kind, id))
}

func (p *pgRepository) TakeSession(ctx context.Context, kind, id string) ([]byte, error) {
	// One statement, so two replicas redeeming the same id serialise on the
	// row and only one of them gets it back.
	const q = `
		DELETE FROM oauth_sessions
		WHERE kind = $1 AND session_id = $2 AND expires_at > NOW()
		RETURNING data
	`
	return scanSession(p.db.QueryRow(ctx, q, kind, id))
}

func scanSession(row pgx.Row) ([]byte, error) {
	var data []byte
	err := row.Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoRecord
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *pgRepository) DeleteSession(ctx context.Context, kind, id string) error {
	const q = `DELETE FROM oauth_sessions WHERE kind = $1 AND session_id = $2`
	_, err := p.db.Exec(ctx, q, kind, id)
	return err
}

// DeleteExpired sweeps in batches so that a backlog does not turn into one
// long-running DELETE holding locks on every replica at once.
func (p *pgRepository) DeleteExpired(ctx context.Context) error {
	const sessions = `
		DELETE FROM oauth_sessions
		WHERE ctid IN (SELECT ctid FROM oauth_sessions WHERE expires_at < NOW() LIMIT 10000)
	`
	const clients = `
		DELETE FROM oauth_clients
		WHERE ctid IN (SELECT ctid FROM oauth_clients WHERE expires_at < NOW() LIMIT 10000)
	`
	for _, q := range []string{sessions, clients} {
		for {
			tag, err := p.db.Exec(ctx, q)
			if err != nil {
				return err
			}
			if tag.RowsAffected() < 10000 {
				break
			}
		}
	}
	return nil
}
