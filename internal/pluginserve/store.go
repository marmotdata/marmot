package pluginserve

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marmotdata/marmot/internal/core/auth"
)

var errNotFound = errors.New("not found")

// Store persists the serving identity, enrolments and which replica holds each live connection.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Enrolment is one certificate issued to a serving process and the sources it may serve.
type Enrolment struct {
	ID              string
	PrincipalType   string
	PrincipalID     string
	AuditSubject    string
	Sources         []string
	PublicKeySHA256 []byte
	Credential      auth.Credential
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

// active reports whether the enrolment may still carry queries at now.
func (e *Enrolment) active(now time.Time) bool {
	return now.Before(e.ExpiresAt)
}

// instanceID returns this Marmot's serving instance id, creating it on first use.
func (s *Store) instanceID(ctx context.Context) (string, error) {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO plugin_serve_instance (instance_id) VALUES ($1) ON CONFLICT (singleton) DO NOTHING`,
		uuid.NewString()); err != nil {
		return "", fmt.Errorf("creating serving instance: %w", err)
	}
	var id string
	if err := s.db.QueryRow(ctx, `SELECT instance_id FROM plugin_serve_instance WHERE singleton`).Scan(&id); err != nil {
		return "", fmt.Errorf("reading serving instance: %w", err)
	}
	return id, nil
}

// generationRecord is a key generation as stored: each key either encrypted (CAKey, SigningKey) or named by KMS URI.
type generationRecord struct {
	ID               string
	CACert           string
	CAKey            *string
	CAKeyURI         *string
	SigningPublicKey string
	SigningKey       *string
	SigningKeyURI    *string
	ActivatesAt      time.Time
	RetiresAt        *time.Time
}

func (s *Store) trustedGenerations(ctx context.Context) ([]*generationRecord, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, ca_cert, ca_key, ca_key_uri, signing_public_key, signing_key, signing_key_uri, activates_at, retires_at
		 FROM plugin_serve_keys
		 WHERE retires_at IS NULL OR retires_at > NOW()
		 ORDER BY activates_at`)
	if err != nil {
		return nil, fmt.Errorf("reading key generations: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*generationRecord, error) {
		var r generationRecord
		err := row.Scan(&r.ID, &r.CACert, &r.CAKey, &r.CAKeyURI, &r.SigningPublicKey, &r.SigningKey, &r.SigningKeyURI, &r.ActivatesAt, &r.RetiresAt)
		return &r, err
	})
}

// keyTx is the set of key changes made under the key lock.
type keyTx struct {
	tx pgx.Tx
}

// withKeyLock runs fn in a transaction holding an advisory lock, so replicas never create or rotate keys at the same time.
func (s *Store) withKeyLock(ctx context.Context, fn func(context.Context, keyTx) error) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('plugin_serve_keys'))`); err != nil {
			return fmt.Errorf("locking serving keys: %w", err)
		}
		return fn(ctx, keyTx{tx: tx})
	})
}

func (t keyTx) anyGeneration(ctx context.Context) (bool, error) {
	var exists bool
	err := t.tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM plugin_serve_keys WHERE retires_at IS NULL OR retires_at > NOW())`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking key generations: %w", err)
	}
	return exists, nil
}

func (t keyTx) insertGeneration(ctx context.Context, r *generationRecord) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO plugin_serve_keys (ca_cert, ca_key, ca_key_uri, signing_public_key, signing_key, signing_key_uri, activates_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		r.CACert, r.CAKey, r.CAKeyURI, r.SigningPublicKey, r.SigningKey, r.SigningKeyURI, r.ActivatesAt)
	if err != nil {
		return fmt.Errorf("storing key generation: %w", err)
	}
	return nil
}

// retireGenerations schedules every generation still trusted to stop being trusted at retires, never extending one already due to retire sooner.
func (t keyTx) retireGenerations(ctx context.Context, retires time.Time) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE plugin_serve_keys SET retires_at = $1
		 WHERE (retires_at IS NULL OR retires_at > $1)`, retires)
	if err != nil {
		return fmt.Errorf("retiring key generations: %w", err)
	}
	return nil
}

func (s *Store) createEnrolment(ctx context.Context, e *Enrolment) error {
	err := s.db.QueryRow(ctx,
		`INSERT INTO plugin_serve_enrolments (principal_type, principal_id, audit_subject, sources, public_key_sha256, credential_type, credential_lookup, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		e.PrincipalType, e.PrincipalID, e.AuditSubject, e.Sources, e.PublicKeySHA256, e.Credential.Type, nilIfEmpty(e.Credential.Lookup), e.ExpiresAt,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating enrolment: %w", err)
	}
	return nil
}

func (s *Store) enrolment(ctx context.Context, id string) (*Enrolment, error) {
	var e Enrolment
	err := s.db.QueryRow(ctx,
		`SELECT id, principal_type, principal_id, audit_subject, sources, public_key_sha256, credential_type, COALESCE(credential_lookup, ''), created_at, expires_at
		 FROM plugin_serve_enrolments WHERE id = $1`, id,
	).Scan(&e.ID, &e.PrincipalType, &e.PrincipalID, &e.AuditSubject, &e.Sources, &e.PublicKeySHA256, &e.Credential.Type, &e.Credential.Lookup, &e.CreatedAt, &e.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading enrolment: %w", err)
	}
	return &e, nil
}

// pruneEnrolments deletes enrolments that expired more than a day ago; their certificates can no longer connect and their sessions are gone.
func (s *Store) pruneEnrolments(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM plugin_serve_enrolments WHERE expires_at < NOW() - INTERVAL '1 day'`)
	if err != nil {
		return fmt.Errorf("pruning enrolments: %w", err)
	}
	return nil
}

func (s *Store) upsertSession(ctx context.Context, enrolmentID, replicaID, replicaAddress string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO plugin_serve_sessions (enrolment_id, replica_id, replica_address) VALUES ($1, $2, $3)
		 ON CONFLICT (enrolment_id, replica_id) DO UPDATE SET replica_address = EXCLUDED.replica_address, heartbeat_at = NOW()`,
		enrolmentID, replicaID, replicaAddress)
	if err != nil {
		return fmt.Errorf("recording session: %w", err)
	}
	return nil
}

func (s *Store) heartbeat(ctx context.Context, replicaID string, enrolmentIDs []string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE plugin_serve_sessions SET heartbeat_at = NOW() WHERE replica_id = $1 AND enrolment_id = ANY($2::uuid[])`,
		replicaID, enrolmentIDs)
	if err != nil {
		return fmt.Errorf("heartbeating sessions: %w", err)
	}
	return nil
}

func (s *Store) deleteSession(ctx context.Context, enrolmentID, replicaID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM plugin_serve_sessions WHERE enrolment_id = $1 AND replica_id = $2`, enrolmentID, replicaID)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// remoteReplicas returns the addresses of other replicas with a fresh connection from a live enrolment that serves source for its owner.
func (s *Store) remoteReplicas(ctx context.Context, source, ownerType, ownerID, selfReplicaID string, freshness time.Duration) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT s.replica_address
		 FROM plugin_serve_sessions s
		 JOIN plugin_serve_enrolments e ON e.id = s.enrolment_id
		 WHERE $1 = ANY(e.sources)
		   AND e.principal_type = $2 AND e.principal_id = $3
		   AND e.expires_at > NOW()
		   AND s.replica_id <> $4
		   AND s.heartbeat_at > NOW() - $5::interval`,
		source, ownerType, ownerID, selfReplicaID, freshness.String())
	if err != nil {
		return nil, fmt.Errorf("finding replicas for %s: %w", source, err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// revokedCredentials returns which of the enrolments' API keys no longer authenticate: deleted, expired, or belonging to a deleted or inactive service account. One query covers every live connection on a replica.
func (s *Store) revokedCredentials(ctx context.Context, enrolmentIDs []string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx,
		`SELECT e.id,
		        CASE e.credential_type
		          WHEN $2 THEN NOT EXISTS (
		            SELECT 1 FROM api_keys k
		             WHERE k.key_lookup = e.credential_lookup
		               AND (k.expires_at IS NULL OR k.expires_at > NOW()))
		          WHEN $3 THEN NOT EXISTS (
		            SELECT 1 FROM service_account_api_keys k
		              JOIN service_accounts sa ON sa.id = k.service_account_id
		             WHERE k.key_lookup = e.credential_lookup
		               AND (k.expires_at IS NULL OR k.expires_at > NOW())
		               AND sa.deleted_at IS NULL AND sa.active)
		          ELSE false
		        END
		   FROM plugin_serve_enrolments e
		  WHERE e.id = ANY($1::uuid[])`,
		enrolmentIDs, auth.CredentialUserAPIKey, auth.CredentialServiceAccountAPIKey)
	if err != nil {
		return nil, fmt.Errorf("checking enrolment credentials: %w", err)
	}
	defer rows.Close()

	revoked := make(map[string]bool, len(enrolmentIDs))
	for rows.Next() {
		var id string
		var gone bool
		if err := rows.Scan(&id, &gone); err != nil {
			return nil, err
		}
		revoked[id] = gone
	}
	return revoked, rows.Err()
}

// apiKeyExists reports whether an API key with this credential's lookup currently authenticates. Keys issued before lookups existed have none, so they are never found and cannot be used to serve.
func (s *Store) apiKeyExists(ctx context.Context, c auth.Credential) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM api_keys WHERE key_lookup = $1 AND (expires_at IS NULL OR expires_at > NOW()))`
	if c.Type == auth.CredentialServiceAccountAPIKey {
		query = `SELECT EXISTS (SELECT 1 FROM service_account_api_keys WHERE key_lookup = $1 AND (expires_at IS NULL OR expires_at > NOW()))`
	}
	var exists bool
	if err := s.db.QueryRow(ctx, query, c.Lookup).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking API key: %w", err)
	}
	return exists, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
