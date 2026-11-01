package serviceaccount

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/marmotdata/marmot/internal/core/role"
)

var (
	ErrNotFound      = errors.New("service account not found")
	ErrAlreadyExists = errors.New("service account already exists")
	ErrKeyNotFound   = errors.New("api key not found")
)

type ServiceAccount struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Active      bool         `json:"active"`
	Roles       []*role.Role `json:"roles"`
	CreatedBy   *string      `json:"created_by,omitempty"`
	// CreatedByName is the creator's display name, empty once that user is deleted.
	CreatedByName string    `json:"created_by_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// LastUsedAt is the latest use of any of the account's keys.
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	APIKeyCount int        `json:"api_key_count"`
	// CallsLast7Days counts authenticated requests made with the account's keys over the last seven days, including today.
	CallsLast7Days int64 `json:"calls_last_7_days"`
	// UsedBy lists the agents that have reported runs with one of this account's keys.
	UsedBy []AgentRef `json:"used_by"`
} // @name ServiceAccount

// AgentRef names an agent asset that authenticates as a service account.
type AgentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MRN  string `json:"mrn"`
} // @name ServiceAccountAgentRef

type APIKey struct {
	ID               string `json:"id"`
	ServiceAccountID string `json:"service_account_id"`
	Name             string `json:"name"`
	Key              string `json:"key,omitempty"`
	// Prefix is the start of the key kept in clear so it can be recognised later; empty for keys issued before it was stored.
	Prefix     string     `json:"prefix,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
} // @name ServiceAccountAPIKey

type CreateInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	RoleIDs     []string `json:"role_ids,omitempty"`
}

type UpdateInput struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	RoleIDs     []string `json:"role_ids,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, input CreateInput, createdBy *string) (*ServiceAccount, error)
	Get(ctx context.Context, id string) (*ServiceAccount, error)
	List(ctx context.Context) ([]*ServiceAccount, error)
	Update(ctx context.Context, id string, input UpdateInput) (*ServiceAccount, error)
	SoftDelete(ctx context.Context, id string) error
	AssignRoles(ctx context.Context, saID string, roleIDs []string) error

	CreateAPIKey(ctx context.Context, saID string, apiKey *APIKey, keyHash string) error
	GetAPIKey(ctx context.Context, id string) (*APIKey, error)
	GetAPIKeyByHash(ctx context.Context, keyToValidate string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, saID string) ([]*APIKey, error)
	CountAPIKeys(ctx context.Context, saID string) (int, error)
	DeleteAPIKey(ctx context.Context, id string) error
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error
	RotateAPIKey(ctx context.Context, id, keyHash, keyLookup, keyPrefix string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateInput, createdBy *string) (*ServiceAccount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO service_accounts (name, description, created_by) VALUES ($1, $2, $3) RETURNING id`,
		input.Name, input.Description, createdBy,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("inserting service account: %w", err)
	}

	if len(input.RoleIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO service_account_roles (service_account_id, role_id) SELECT $1, unnest($2::uuid[])`,
			id, input.RoleIDs,
		); err != nil {
			return nil, fmt.Errorf("assigning roles: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing: %w", err)
	}

	return r.Get(ctx, id)
}

// usageColumns adds the creator's name, key usage and the agents using the account. They are correlated subqueries rather than joins so they do not multiply the rows the roles aggregate runs over, and every one is backed by an index on the account id.
const usageColumns = `
		       (SELECT u.name FROM users u WHERE u.id = sa.created_by) AS created_by_name,
		       (SELECT MAX(k.last_used_at) FROM service_account_api_keys k WHERE k.service_account_id = sa.id) AS last_used_at,
		       (SELECT COUNT(*) FROM service_account_api_keys k WHERE k.service_account_id = sa.id) AS api_key_count,
		       (SELECT COALESCE(SUM(d.calls), 0) FROM service_account_usage_daily d
		         WHERE d.service_account_id = sa.id AND d.day > CURRENT_DATE - 7) AS calls_last_7_days,
		       (SELECT COALESCE(json_agg(json_build_object('id', a.id, 'name', a.name, 'mrn', a.mrn) ORDER BY a.name), '[]'::json)
		          FROM assets a
		         WHERE a.id IN (SELECT DISTINCT ar.agent_id FROM agent_runs ar WHERE ar.service_account_id = sa.id)) AS used_by`

func (r *PostgresRepository) Get(ctx context.Context, id string) (*ServiceAccount, error) {
	query := `
		SELECT sa.id, sa.name, sa.description, sa.active, sa.created_by, sa.created_at, sa.updated_at,
		       ` + usageColumns + `,
		       COALESCE(json_agg(json_build_object(
		           'id', ro.id, 'name', ro.name, 'description', ro.description,
		           'is_system', ro.is_system, 'created_at', ro.created_at, 'updated_at', ro.updated_at,
		           'permissions', COALESCE(rp.permissions, '[]'::json)
		       )) FILTER (WHERE ro.id IS NOT NULL), '[]'::json) AS roles
		FROM service_accounts sa
		LEFT JOIN service_account_roles sar ON sar.service_account_id = sa.id
		LEFT JOIN roles ro ON ro.id = sar.role_id AND ro.deleted_at IS NULL
		LEFT JOIN LATERAL (
		    SELECT json_agg(json_build_object(
		               'id', p.id, 'name', p.name, 'description', p.description,
		               'resource_type', p.resource_type, 'action', p.action
		           )) AS permissions
		      FROM role_permissions rpm
		      JOIN permissions p ON p.id = rpm.permission_id
		     WHERE rpm.role_id = ro.id
		) rp ON TRUE
		WHERE sa.id = $1 AND sa.deleted_at IS NULL
		GROUP BY sa.id`

	return scanServiceAccount(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresRepository) List(ctx context.Context) ([]*ServiceAccount, error) {
	query := `
		SELECT sa.id, sa.name, sa.description, sa.active, sa.created_by, sa.created_at, sa.updated_at,
		       ` + usageColumns + `,
		       COALESCE(json_agg(json_build_object(
		           'id', ro.id, 'name', ro.name, 'description', ro.description,
		           'is_system', ro.is_system, 'created_at', ro.created_at, 'updated_at', ro.updated_at,
		           'permissions', COALESCE(rp.permissions, '[]'::json)
		       )) FILTER (WHERE ro.id IS NOT NULL), '[]'::json) AS roles
		FROM service_accounts sa
		LEFT JOIN service_account_roles sar ON sar.service_account_id = sa.id
		LEFT JOIN roles ro ON ro.id = sar.role_id AND ro.deleted_at IS NULL
		LEFT JOIN LATERAL (
		    SELECT json_agg(json_build_object(
		               'id', p.id, 'name', p.name, 'description', p.description,
		               'resource_type', p.resource_type, 'action', p.action
		           )) AS permissions
		      FROM role_permissions rpm
		      JOIN permissions p ON p.id = rpm.permission_id
		     WHERE rpm.role_id = ro.id
		) rp ON TRUE
		WHERE sa.deleted_at IS NULL
		GROUP BY sa.id
		ORDER BY sa.created_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing service accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*ServiceAccount
	for rows.Next() {
		sa, err := scanServiceAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, id string, input UpdateInput) (*ServiceAccount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	sets := []string{"updated_at = NOW()"}
	args := []any{}
	n := 1

	if input.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", n))
		args = append(args, *input.Name)
		n++
	}
	if input.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", n))
		args = append(args, *input.Description)
		n++
	}
	if input.Active != nil {
		sets = append(sets, fmt.Sprintf("active = $%d", n))
		args = append(args, *input.Active)
		n++
	}
	args = append(args, id)

	tag, err := tx.Exec(ctx,
		fmt.Sprintf("UPDATE service_accounts SET %s WHERE id = $%d AND deleted_at IS NULL", strings.Join(sets, ", "), n),
		args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("updating service account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	if input.RoleIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM service_account_roles WHERE service_account_id = $1`, id); err != nil {
			return nil, fmt.Errorf("clearing roles: %w", err)
		}
		if len(input.RoleIDs) > 0 {
			if _, err := tx.Exec(ctx,
				`INSERT INTO service_account_roles (service_account_id, role_id) SELECT $1, unnest($2::uuid[])`,
				id, input.RoleIDs,
			); err != nil {
				return nil, fmt.Errorf("assigning roles: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing: %w", err)
	}

	return r.Get(ctx, id)
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE service_accounts SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft-deleting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) AssignRoles(ctx context.Context, saID string, roleIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM service_account_roles WHERE service_account_id = $1`, saID); err != nil {
		return fmt.Errorf("clearing roles: %w", err)
	}

	if len(roleIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO service_account_roles (service_account_id, role_id) SELECT $1, unnest($2::uuid[])`,
			saID, roleIDs,
		); err != nil {
			return fmt.Errorf("assigning roles: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) CreateAPIKey(ctx context.Context, saID string, apiKey *APIKey, keyHash string) error {
	query := `
		INSERT INTO service_account_api_keys
			(service_account_id, name, key_hash, key_lookup, key_prefix, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7)
		RETURNING id`

	err := r.db.QueryRow(ctx, query,
		saID, apiKey.Name, keyHash, APIKeyLookup(apiKey.Key), apiKey.Prefix, apiKey.ExpiresAt, apiKey.CreatedAt,
	).Scan(&apiKey.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating api key: %w", err)
	}
	apiKey.ServiceAccountID = saID
	return nil
}

func (r *PostgresRepository) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var k APIKey
	err := r.db.QueryRow(ctx,
		`SELECT id, service_account_id, name, COALESCE(key_prefix, ''), expires_at, last_used_at, created_at
		 FROM service_account_api_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.ServiceAccountID, &k.Name, &k.Prefix, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("getting api key: %w", err)
	}
	return &k, nil
}

// APIKeyLookup is the indexed handle for a key: a SHA-256 of the key itself.
//
// It exists so authentication can find the one row to verify instead of
// bcrypt-comparing every row in the table. A fast hash is right here and wrong
// for a password, because the input is 32 bytes from crypto/rand rather than
// something a person chose, so there is no dictionary to attack. bcrypt still
// does the verifying.
func APIKeyLookup(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (r *PostgresRepository) GetAPIKeyByHash(ctx context.Context, keyToValidate string) (*APIKey, error) {
	// Fast path: one indexed lookup, one bcrypt comparison.
	var k APIKey
	var keyHash string
	err := r.db.QueryRow(ctx,
		`SELECT id, service_account_id, name, key_hash, expires_at, last_used_at, created_at
		   FROM service_account_api_keys
		  WHERE key_lookup = $1 AND (expires_at IS NULL OR expires_at > NOW())`,
		APIKeyLookup(keyToValidate)).
		Scan(&k.ID, &k.ServiceAccountID, &k.Name, &keyHash, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt)
	switch {
	case err == nil:
		if bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(keyToValidate)) == nil {
			return &k, nil
		}
		// A lookup hit whose bcrypt does not verify means the row was tampered
		// with. Refuse rather than falling through to the scan.
		return nil, ErrKeyNotFound
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("looking up api key: %w", err)
	}

	// Slow path, for keys issued before key_lookup existed. Keys of a deleted or
	// inactive account are excluded: they could never authenticate anyway, so
	// comparing them was pure cost.
	rows, err := r.db.Query(ctx,
		`SELECT k.id, k.service_account_id, k.name, k.key_hash,
		        k.expires_at, k.last_used_at, k.created_at
		   FROM service_account_api_keys k
		   JOIN service_accounts sa ON sa.id = k.service_account_id
		  WHERE k.key_lookup IS NULL
		    AND (k.expires_at IS NULL OR k.expires_at > NOW())
		    AND sa.deleted_at IS NULL AND sa.active`)
	if err != nil {
		return nil, fmt.Errorf("querying api keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k APIKey
		var keyHash string
		if err := rows.Scan(&k.ID, &k.ServiceAccountID, &k.Name, &keyHash, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(keyToValidate)); err == nil {
			return &k, nil
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating api keys: %w", err)
	}
	return nil, ErrKeyNotFound
}

func (r *PostgresRepository) ListAPIKeys(ctx context.Context, saID string) ([]*APIKey, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, service_account_id, name, COALESCE(key_prefix, ''), expires_at, last_used_at, created_at
		 FROM service_account_api_keys WHERE service_account_id = $1 ORDER BY created_at DESC`,
		saID)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.ServiceAccountID, &k.Name, &k.Prefix, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (r *PostgresRepository) CountAPIKeys(ctx context.Context, saID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM service_account_api_keys WHERE service_account_id = $1`, saID,
	).Scan(&count)
	return count, err
}

func (r *PostgresRepository) DeleteAPIKey(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM service_account_api_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// UpdateAPIKeyLastUsed also counts the call against the account's day, in the same round trip, which is what the detail page's recent call count reads.
func (r *PostgresRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		WITH used AS (
		    UPDATE service_account_api_keys SET last_used_at = NOW() WHERE id = $1
		    RETURNING service_account_id
		)
		INSERT INTO service_account_usage_daily (service_account_id, day, calls)
		SELECT service_account_id, CURRENT_DATE, 1 FROM used
		ON CONFLICT (service_account_id, day) DO UPDATE SET calls = service_account_usage_daily.calls + 1`, id)
	return err
}

// RotateAPIKey swaps the secret behind a key, clearing last_used_at because it described the old secret.
func (r *PostgresRepository) RotateAPIKey(ctx context.Context, id, keyHash, keyLookup, keyPrefix string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE service_account_api_keys
		   SET key_hash = $2, key_lookup = $3, key_prefix = NULLIF($4, ''), last_used_at = NULL
		 WHERE id = $1`,
		id, keyHash, keyLookup, keyPrefix)
	if err != nil {
		return fmt.Errorf("rotating api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func scanServiceAccount(row pgx.Row) (*ServiceAccount, error) {
	var sa ServiceAccount
	var rolesJSON []byte
	var description *string

	var createdByName *string
	var usedByJSON []byte
	err := row.Scan(&sa.ID, &sa.Name, &description, &sa.Active, &sa.CreatedBy, &sa.CreatedAt, &sa.UpdatedAt,
		&createdByName, &sa.LastUsedAt, &sa.APIKeyCount, &sa.CallsLast7Days, &usedByJSON, &rolesJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning service account: %w", err)
	}

	if description != nil {
		sa.Description = *description
	}
	if createdByName != nil {
		sa.CreatedByName = *createdByName
	}
	sa.UsedBy = []AgentRef{}
	if len(usedByJSON) > 0 {
		if err := json.Unmarshal(usedByJSON, &sa.UsedBy); err != nil {
			return nil, fmt.Errorf("parsing used by: %w", err)
		}
	}

	if err := json.Unmarshal(rolesJSON, &sa.Roles); err != nil {
		return nil, fmt.Errorf("parsing roles: %w", err)
	}

	return &sa, nil
}

func scanServiceAccountRow(rows pgx.Rows) (*ServiceAccount, error) {
	var sa ServiceAccount
	var rolesJSON []byte
	var description *string

	var createdByName *string
	var usedByJSON []byte
	err := rows.Scan(&sa.ID, &sa.Name, &description, &sa.Active, &sa.CreatedBy, &sa.CreatedAt, &sa.UpdatedAt,
		&createdByName, &sa.LastUsedAt, &sa.APIKeyCount, &sa.CallsLast7Days, &usedByJSON, &rolesJSON)
	if err != nil {
		return nil, fmt.Errorf("scanning service account: %w", err)
	}

	if description != nil {
		sa.Description = *description
	}
	if createdByName != nil {
		sa.CreatedByName = *createdByName
	}
	sa.UsedBy = []AgentRef{}
	if len(usedByJSON) > 0 {
		if err := json.Unmarshal(usedByJSON, &sa.UsedBy); err != nil {
			return nil, fmt.Errorf("parsing used by: %w", err)
		}
	}

	if err := json.Unmarshal(rolesJSON, &sa.Roles); err != nil {
		return nil, fmt.Errorf("parsing roles: %w", err)
	}

	return &sa, nil
}
